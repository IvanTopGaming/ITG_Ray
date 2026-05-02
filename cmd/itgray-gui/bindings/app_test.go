package bindings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// fileServerStore is the test-side adapter over the package-level
// server.Load / server.Save free functions. main.go uses an equivalent shim.
type fileServerStore struct{ path string }

func (s fileServerStore) Load() ([]server.Server, error) { return server.Load(s.path) }
func (s fileServerStore) Save(list []server.Server) error {
	return server.Save(s.path, list)
}

// fakeConfigViewer is a test-only ConfigViewer that returns a fixed view.
type fakeConfigViewer struct{ view hub.SettingsView }

func (f fakeConfigViewer) View() (hub.SettingsView, error) { //nolint:gocritic // hugeParam: hub.SettingsView is large but copying is fine in test fakes
	return f.view, nil
}

// errConfigViewer returns an error; used in TestGetSnapshot_ConfigViewerError.
type errConfigViewer struct{ err error }

func (e errConfigViewer) View() (hub.SettingsView, error) { //nolint:gocritic // hugeParam: hub.SettingsView is large but copying is fine in test fakes
	return hub.SettingsView{}, e.err
}

// fakeChain is a test-only ChainStatuser.
type fakeChain struct {
	status hub.ChainStatus
	srv    *server.Server
	mode   chainctl.Mode
}

func (f fakeChain) Status() (hub.ChainStatus, *server.Server, chainctl.Mode) {
	return f.status, f.srv, f.mode
}

func TestAppService_GetSnapshot_Empty(t *testing.T) {
	dir := t.TempDir()
	srv := fileServerStore{path: filepath.Join(dir, "servers.json")}
	sub := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  srv,
		SubStore:     sub,
		HelperProber: func() string { return "missing" },
		ConfigViewer: fakeConfigViewer{view: hub.SettingsView{}},
		// (Chain and NetworkLoader can be nil for this test.)
	})

	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.Equal(t, hub.StatusIdle, snap.Status)
	require.Equal(t, "missing", snap.HelperState)
	require.False(t, snap.Onboarded)
	require.Empty(t, snap.Servers)
	require.Empty(t, snap.Subs)
	require.Equal(t, "test", snap.Version)
}

func TestAppService_GetSnapshot_WithSeededData(t *testing.T) {
	dir := t.TempDir()
	serversPath := filepath.Join(dir, "servers.json")
	subsPath := filepath.Join(dir, "subscriptions.json")

	subStore := subscription.FileStore{Path: subsPath}
	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID:             "s1",
		Name:           "okins",
		URL:            "https://e.com/sub",
		UpdateInterval: subscription.Duration(time.Hour),
		LastSyncAt:     time.Now().Add(-30 * time.Second),
		LastStatus:     "OK",
	}}))

	latency := 15
	require.NoError(t, server.Save(serversPath, []server.Server{{
		ID:       "abc123",
		Origin:   server.OriginSubscription,
		SourceID: "s1",
		Name:     "DE_master",
		Vless: vless.Config{
			Address:   "gw.example.com",
			Port:      443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
			Security:  vless.SecurityReality,
		},
		LatencyMS: &latency,
	}}))

	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: serversPath},
		SubStore:     subStore,
		HelperProber: func() string { return "running" },
		ConfigViewer: fakeConfigViewer{view: hub.SettingsView{}},
	})

	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.Equal(t, "running", snap.HelperState)
	require.Len(t, snap.Servers, 1)
	require.Equal(t, "DE_master", snap.Servers[0].Name)
	require.Equal(t, "okins", snap.Servers[0].Origin)
	require.Equal(t, 15, snap.Servers[0].LatencyMs)
	require.Equal(t, "gw.example.com:443", snap.Servers[0].Address)
	require.Equal(t, "tcp", snap.Servers[0].Transport)
	require.Equal(t, "reality", snap.Servers[0].Security)
	require.Len(t, snap.Subs, 1)
	require.Equal(t, "OK", snap.Subs[0].LastSyncStatus)
	require.Equal(t, 1, snap.Subs[0].ServerCount)
	require.Equal(t, int(time.Hour/time.Second), snap.Subs[0].UpdateInterval)
}

func TestAppService_GetSnapshot_OnboardedMarker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeFile(filepath.Join(dir, ".onboarded"), nil))
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: filepath.Join(dir, "servers.json")},
		SubStore:     subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")},
		HelperProber: func() string { return "missing" },
		ConfigViewer: fakeConfigViewer{view: hub.SettingsView{}},
	})
	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.True(t, snap.Onboarded)
}

func TestToSubViews_SurfacesQuotaAndMessage(t *testing.T) {
	expire := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []subscription.Stored{{
		ID:          "s1",
		Name:        "A",
		URL:         "https://a.test",
		LastStatus:  "ok",
		LastMessage: "imported=3",
		Upload:      111,
		Download:    222,
		Total:       1024,
		Expire:      &expire,
	}}

	out := toSubViews(in, map[string]int{"s1": 3})
	require.Len(t, out, 1)
	require.Equal(t, "ok", out[0].LastSyncStatus)
	require.Equal(t, "imported=3", out[0].LastSyncMessage)
	require.EqualValues(t, 111, out[0].Upload)
	require.EqualValues(t, 222, out[0].Download)
	require.EqualValues(t, 1024, out[0].Total)
	require.NotNil(t, out[0].Expire)
	require.True(t, out[0].Expire.Equal(expire))
}

func writeFile(path string, b []byte) error {
	f, err := os.Create(path) //nolint:gosec // test-only marker file in t.TempDir
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	_ = f.Close()
	return err
}

func TestAppService_GetSnapshot_ReadsChainStatus(t *testing.T) {
	dir := t.TempDir()
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(srvPath, []server.Server{{
		ID: "abc", Name: "DE_master", Origin: server.OriginManual,
		Vless: vless.Config{Address: "1.2.3.4", Port: 443},
	}}))

	chainSrv := &server.Server{
		ID: "abc", Name: "DE_master", Origin: server.OriginManual,
		Vless: vless.Config{Address: "1.2.3.4", Port: 443},
	}
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: srvPath},
		SubStore:     subscription.FileStore{Path: filepath.Join(dir, "subs.json")},
		HelperProber: func() string { return "running" },
		ConfigViewer: fakeConfigViewer{},
		Chain:        fakeChain{status: hub.StatusConnecting, srv: chainSrv, mode: chainctl.ModeTUN},
	})

	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.Equal(t, hub.StatusConnecting, snap.Status)
	require.Equal(t, "tun", snap.Mode)
	require.NotNil(t, snap.CurrentServer)
	require.Equal(t, "abc", snap.CurrentServer.ID)
	require.Equal(t, "DE_master", snap.CurrentServer.Name)
}

func TestAppService_GetSnapshot_NilCurrentServer(t *testing.T) {
	dir := t.TempDir()
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: filepath.Join(dir, "servers.json")},
		SubStore:     subscription.FileStore{Path: filepath.Join(dir, "subs.json")},
		HelperProber: func() string { return "missing" },
		ConfigViewer: fakeConfigViewer{},
		Chain:        fakeChain{status: hub.StatusIdle, srv: nil, mode: chainctl.ModeTUN},
	})

	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.Nil(t, snap.CurrentServer)
}

func TestAppService_GetSnapshot_ReadsConfigViewer(t *testing.T) {
	dir := t.TempDir()
	customView := hub.SettingsView{
		Subscriptions: hub.SubscriptionSettings{
			DefaultUpdateInterval: 1800,
			UserAgent:             "Custom/0.2",
			HWIDEnabled:           false,
		},
	}
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: filepath.Join(dir, "servers.json")},
		SubStore:     subscription.FileStore{Path: filepath.Join(dir, "subs.json")},
		HelperProber: func() string { return "missing" },
		ConfigViewer: fakeConfigViewer{view: customView},
	})

	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.Equal(t, customView, snap.Settings)
}

func TestAppService_GetSnapshot_ConfigViewerError(t *testing.T) {
	dir := t.TempDir()
	app := NewAppService(&AppDeps{
		DataDir:      dir,
		Hub:          hub.New(),
		Version:      "test",
		ServerStore:  fileServerStore{path: filepath.Join(dir, "servers.json")},
		SubStore:     subscription.FileStore{Path: filepath.Join(dir, "subs.json")},
		HelperProber: func() string { return "missing" },
		ConfigViewer: errConfigViewer{err: errors.New("disk read failed")},
	})

	_, err := app.GetSnapshot()
	require.Error(t, err)
	require.Contains(t, err.Error(), "settings.View")
	require.Contains(t, err.Error(), "disk read failed")
}
