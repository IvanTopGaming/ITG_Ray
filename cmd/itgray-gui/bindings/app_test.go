package bindings

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	})
	snap, err := app.GetSnapshot()
	require.NoError(t, err)
	require.True(t, snap.Onboarded)
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
