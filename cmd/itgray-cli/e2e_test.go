//go:build e2e

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/core"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/vless"
	sbinclude "github.com/sagernet/sing-box/include"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/proxy"
)

// TestSmoke_EndToEnd spins up an xray server instance, points an ITG Ray
// core chain at it, then proxies an HTTP GET through the local SOCKS5
// and expects the target server to receive the request.
func TestSmoke_EndToEnd(t *testing.T) {
	if os.Getenv("ITGRAY_E2E") == "" {
		t.Skip("set ITGRAY_E2E=1 to run")
	}
	// Local HTTP target
	var got string
	target := startTargetServer(t, &got)

	// VLESS+none+tcp server on a random port
	serverPort := freePort(t)
	clientXraySocks := freePort(t)
	localSocks := freePort(t)

	stopServer := startXrayVlessServer(t, serverPort)
	defer stopServer()
	time.Sleep(300 * time.Millisecond)

	require.True(t, serverPort > 0 && serverPort <= 65535)
	srv := vless.Config{
		Address: "127.0.0.1", Port: uint16(serverPort), //nolint:gosec // bounded above
		UUID:       "00000000-0000-0000-0000-000000000001",
		Encryption: "none",
		Security:   vless.SecurityNone,
		Transport:  vless.TransportTCP,
	}
	sb, err := configgen.BuildSingbox(&configgen.SingboxInput{
		SocksInboundPort: localSocks, XraySOCKSHost: "127.0.0.1", XraySOCKSPort: clientXraySocks,
		Rules: rules.Model{DefaultAction: rules.ActionProxy},
	})
	require.NoError(t, err)
	xr, err := configgen.BuildXray(&configgen.XrayInput{Server: srv, SocksPort: clientXraySocks})
	require.NoError(t, err)

	mgr := core.NewManager()
	// sing-box options unmarshal needs the type registries from include.Context.
	ctx := sbinclude.Context(context.Background())
	require.NoError(t, mgr.Start(ctx, sb, xr))
	defer func() { _ = mgr.Stop() }()
	time.Sleep(500 * time.Millisecond)

	// Issue an HTTP request via our local SOCKS5
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", localSocks), nil, proxy.Direct)
	require.NoError(t, err)
	tr := &http.Transport{Dial: dialer.Dial}
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, "/hello", got)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func startTargetServer(t *testing.T, sink *string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		*sink = r.URL.Path
		w.WriteHeader(200)
	})
	port := freePort(t)
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Close() })
	return fmt.Sprintf("http://127.0.0.1:%d/hello", port)
}

// startXrayVlessServer runs a VLESS+none+tcp server with a freeport listener
// and a direct outbound. Returns a stop function.
func startXrayVlessServer(t *testing.T, port int) func() {
	t.Helper()
	cfg := fmt.Sprintf(`{
      "log": {"loglevel":"warning"},
      "inbounds": [{
        "port": %d, "listen":"127.0.0.1", "protocol":"vless",
        "settings":{"clients":[{"id":"00000000-0000-0000-0000-000000000001"}],"decryption":"none"},
        "streamSettings":{"network":"tcp","security":"none"}
      }],
      "outbounds": [{"protocol":"freedom","tag":"out"}]
    }`, port)

	a := core.NewXrayAdapter()
	require.NoError(t, a.Start(context.Background(), []byte(cfg)))
	return func() { _ = a.Close() }
}
