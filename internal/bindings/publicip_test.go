package bindings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"

	"github.com/stretchr/testify/require"
)

func newAppWithChain(t *testing.T, chain ChainStatuser, xraySocksPort int) *AppService {
	t.Helper()
	dir := t.TempDir()
	return NewAppService(&AppDeps{
		DataDir:       dir,
		Hub:           hub.New(),
		Version:       "test",
		ServerStore:   fileServerStore{path: dir + "/servers.json"},
		SubStore:      nil, // unused by GetPublicIP
		HelperProber:  func() string { return "running" },
		ConfigViewer:  fakeConfigViewer{},
		Chain:         chain,
		XraySOCKSPort: xraySocksPort,
	})
}

func TestGetPublicIP_NotConnected(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusIdle, mode: chainctl.ModeTUN},
		1081,
	)
	_, err := app.GetPublicIP()
	require.ErrorIs(t, err, ErrNotConnected)
}

func TestGetPublicIP_NilChain(t *testing.T) {
	app := newAppWithChain(t, nil, 1081)
	_, err := app.GetPublicIP()
	require.ErrorIs(t, err, ErrNotConnected)
}

// withPublicIPEndpoint swaps publicIPEndpoint for the duration of t.
func withPublicIPEndpoint(t *testing.T, url string) {
	t.Helper()
	prev := publicIPEndpoint
	publicIPEndpoint = url
	t.Cleanup(func() { publicIPEndpoint = prev })
}

// withBarePublicIPClient swaps publicIPClient for the duration of t to
// return a bare http.Client (no SOCKS5). Tests that point publicIPEndpoint
// at httptest.NewServer use this to bypass the SOCKS5 transport — the
// httptest server has no SOCKS5 stub in front of it.
func withBarePublicIPClient(t *testing.T) {
	t.Helper()
	prev := publicIPClient
	publicIPClient = func(_ *AppDeps) (*http.Client, error) {
		return &http.Client{Timeout: publicIPRequestTO}, nil
	}
	t.Cleanup(func() { publicIPClient = prev })
}

func TestGetPublicIP_FetchesAndParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.45"))
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)
	withBarePublicIPClient(t)

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		1081,
	)
	ip, err := app.GetPublicIP()
	require.NoError(t, err)
	require.Equal(t, "203.0.113.45", ip)
}

// TestPublicIPClient_BuildsSocksTransport asserts the production builder
// emits a SOCKS5-wired http.Transport pointing at xray's local inbound.
// The same code path serves both TUN and SysProxy — mode isn't an input.
func TestPublicIPClient_BuildsSocksTransport(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		1081,
	)
	client, err := publicIPClient(app.d)
	require.NoError(t, err)
	require.NotNil(t, client.Transport, "must use SOCKS5 transport via xray local inbound")
	tr := client.Transport.(*http.Transport)
	require.NotNil(t, tr.DialContext)
}

func TestPublicIPClient_NoXraySOCKSPort_ReturnsError(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		0,
	)
	_, err := publicIPClient(app.d)
	require.Error(t, err)
	require.Contains(t, err.Error(), "xray socks port not configured")
}

func TestGetPublicIP_InvalidBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an ip</html>"))
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)

	withBarePublicIPClient(t)
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		1081,
	)
	_, err := app.GetPublicIP()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response")
}

func TestGetPublicIP_CacheHit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = fmt.Fprintf(w, "198.51.100.%d", hits)
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)

	withBarePublicIPClient(t)
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		1081,
	)
	ip1, err := app.GetPublicIP()
	require.NoError(t, err)
	ip2, err := app.GetPublicIP()
	require.NoError(t, err)
	require.Equal(t, ip1, ip2)
	require.Equal(t, 1, hits)
}

func TestGetPublicIP_CacheExpiry(t *testing.T) {
	prev := publicIPCacheTTL
	publicIPCacheTTL = 0
	t.Cleanup(func() { publicIPCacheTTL = prev })

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = fmt.Fprintf(w, "203.0.113.%d", hits)
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)

	withBarePublicIPClient(t)
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		1081,
	)
	ip1, err := app.GetPublicIP()
	require.NoError(t, err)
	ip2, err := app.GetPublicIP()
	require.NoError(t, err)
	require.NotEqual(t, ip1, ip2)
	require.Equal(t, 2, hits)
}

func TestGetPublicIP_ServerSwitch_Refetches(t *testing.T) {
	hits := 0
	ips := []string{"203.0.113.1", "203.0.113.2"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, ips[hits])
		hits++
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)
	withBarePublicIPClient(t)

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, srv: &server.Server{ID: "srv-a"}, mode: chainctl.ModeTUN},
		1081,
	)
	ipA, err := app.GetPublicIP()
	require.NoError(t, err)
	require.Equal(t, "203.0.113.1", ipA)

	app.d.Chain = fakeChain{status: hub.StatusConnected, srv: &server.Server{ID: "srv-b"}, mode: chainctl.ModeTUN}
	ipB, err := app.GetPublicIP()
	require.NoError(t, err)
	require.Equal(t, "203.0.113.2", ipB)
	require.Equal(t, 2, hits)
}
