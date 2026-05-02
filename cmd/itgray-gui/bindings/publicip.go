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

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
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
	st, _, mode := a.d.Chain.Status()
	if st != hub.StatusConnected {
		return "", ErrNotConnected
	}

	client, err := a.publicIPClient(mode)
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

// publicIPClient builds an HTTP client wired to the right transport for
// the given mode. TUN → direct; sysproxy → SOCKS5 via 127.0.0.1:SOCKSPort.
func (a *AppService) publicIPClient(mode chainctl.Mode) (*http.Client, error) {
	if mode == chainctl.ModeTUN {
		return &http.Client{Timeout: publicIPRequestTO}, nil
	}
	if a.d.NetworkLoader == nil {
		return nil, errors.New("network loader not configured")
	}
	netCfg, err := a.d.NetworkLoader()
	if err != nil {
		return nil, fmt.Errorf("load network config: %w", err)
	}
	socksURL := &url.URL{
		Scheme: "socks5",
		Host:   fmt.Sprintf("127.0.0.1:%d", netCfg.SysProxy.SOCKSPort),
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
