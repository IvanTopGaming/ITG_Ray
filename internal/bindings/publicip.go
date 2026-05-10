package bindings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/itg-team/itg-ray/internal/hub"
)

// ErrNotConnected is returned by GetPublicIP when no chain is up; the
// frontend treats it as "show em-dash" rather than an error banner.
var ErrNotConnected = errors.New("not connected")

const publicIPRequestTO = 5 * time.Second

// publicIPEndpoint and publicIPCacheTTL are var-not-const so tests can
// override them.
var (
	publicIPEndpoint = "https://api.ipify.org"
	publicIPCacheTTL = 30 * time.Second
)

// publicIPCache stores the last successfully-fetched IP and its expiry.
// Zero value is valid (empty value, zero expiresAt — always treated as
// expired).
type publicIPCache struct {
	mu        sync.Mutex
	value     string
	expiresAt time.Time
}

// GetPublicIP returns the egress public IP (the IP an outside observer
// sees the connection coming from). When status==connected and
// mode==sysproxy, the request routes via the chainctl SOCKS5 listener;
// in mode==tun the request goes direct (TUN intercepts it). Any other
// status returns ErrNotConnected. Result cached for publicIPCacheTTL.
func (a *AppService) GetPublicIP() (string, error) {
	a.ipCache.mu.Lock()
	if !a.ipCache.expiresAt.IsZero() && time.Now().Before(a.ipCache.expiresAt) {
		v := a.ipCache.value
		a.ipCache.mu.Unlock()
		return v, nil
	}
	a.ipCache.mu.Unlock()

	if a.d.Chain == nil {
		return "", ErrNotConnected
	}
	st, _, _ := a.d.Chain.Status()
	if st != hub.StatusConnected {
		return "", ErrNotConnected
	}

	client, err := publicIPClient(a.d)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), publicIPRequestTO)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicIPEndpoint, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid response: %q", ip)
	}

	a.ipCache.mu.Lock()
	a.ipCache.value = ip
	a.ipCache.expiresAt = time.Now().Add(publicIPCacheTTL)
	a.ipCache.mu.Unlock()
	return ip, nil
}

// publicIPClient is a package-level test seam: production builds a SOCKS5
// http.Client routing via xray's local inbound; tests swap it (see
// withPublicIPClient) to use a bare client when the endpoint is an
// httptest server. Same code path in TUN and SysProxy: xray resolves the
// hostname remotely and egresses through the proxy chain. We deliberately
// do NOT use a bare http.Client (Go runtime DNS) — on Windows the pure-Go
// resolver opens DNS servers in order, and the TUN adapter's hijack-dns
// endpoint (198.18.0.2) is unresponsive, hanging the whole publicIPRequestTO.
var publicIPClient = func(d *AppDeps) (*http.Client, error) {
	if d.XraySOCKSPort <= 0 {
		return nil, errors.New("xray socks port not configured")
	}
	socksURL := &url.URL{
		Scheme: "socks5",
		Host:   fmt.Sprintf("127.0.0.1:%d", d.XraySOCKSPort),
	}
	dialer, err := proxy.FromURL(socksURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("build socks dialer: %w", err)
	}
	// proxy.FromURL returns a socks5 dialer that satisfies proxy.ContextDialer;
	// fall back to wrapping Dial when the implementation pre-dates that
	// interface so we always set the modern DialContext field on Transport.
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, network, addr)
		}
		return dialer.Dial(network, addr)
	}
	return &http.Client{
		Timeout:   publicIPRequestTO,
		Transport: &http.Transport{DialContext: dialContext},
	}, nil
}
