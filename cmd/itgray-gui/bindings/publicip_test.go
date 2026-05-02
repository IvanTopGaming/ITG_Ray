package bindings

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/config"

	"github.com/stretchr/testify/require"
)

func newAppWithChain(t *testing.T, chain ChainStatuser, netLoad NetworkLoader) *AppService {
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
		NetworkLoader: netLoad,
	})
}

func TestGetPublicIP_NotConnected(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusIdle, mode: chainctl.ModeTUN},
		nil,
	)
	_, err := app.GetPublicIP()
	require.ErrorIs(t, err, ErrNotConnected)
}

func TestGetPublicIP_NilChain(t *testing.T) {
	app := newAppWithChain(t, nil, nil)
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

func TestGetPublicIP_TUN_Direct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.45"))
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		nil,
	)
	ip, err := app.GetPublicIP()
	require.NoError(t, err)
	require.Equal(t, "203.0.113.45", ip)
}

func TestGetPublicIP_SysProxy_BuildsSocksTransport(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeSysProxy},
		func() (config.Network, error) {
			return config.Network{SysProxy: config.SysProxy{SOCKSPort: 1080}}, nil
		},
	)
	client, err := app.publicIPClient(chainctl.ModeSysProxy)
	require.NoError(t, err)
	require.NotNil(t, client.Transport)
	tr := client.Transport.(*http.Transport)
	require.NotNil(t, tr.DialContext)
}

func TestGetPublicIP_SysProxy_NetworkLoaderError(t *testing.T) {
	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeSysProxy},
		func() (config.Network, error) {
			return config.Network{}, errors.New("disk read failed")
		},
	)
	_, err := app.GetPublicIP()
	require.Error(t, err)
	require.Contains(t, err.Error(), "load network config")
}

func TestGetPublicIP_InvalidBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an ip</html>"))
	}))
	defer srv.Close()
	withPublicIPEndpoint(t, srv.URL)

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		nil,
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

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		nil,
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

	app := newAppWithChain(t,
		fakeChain{status: hub.StatusConnected, mode: chainctl.ModeTUN},
		nil,
	)
	ip1, err := app.GetPublicIP()
	require.NoError(t, err)
	ip2, err := app.GetPublicIP()
	require.NoError(t, err)
	require.NotEqual(t, ip1, ip2)
	require.Equal(t, 2, hits)
}
