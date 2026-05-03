package bindings

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// seedServers writes a servers.json fixture via the package-level
// server.Save free function. The bindings package shares fileServerStore
// (defined in app_test.go) as the Load+Save adapter the services consume.
func seedServers(t *testing.T, path string, list []server.Server) {
	t.Helper()
	require.NoError(t, server.Save(path, list))
}

func TestServersService_List(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{
		{
			ID:     "a",
			Origin: server.OriginManual,
			Name:   "DE",
			Vless: vless.Config{
				Address:   "h",
				Port:      443,
				UUID:      "00000000-0000-0000-0000-000000000000",
				Transport: vless.TransportTCP,
				Security:  vless.SecurityNone,
			},
		},
		{
			ID:     "b",
			Origin: server.OriginManual,
			Name:   "NL",
			Vless: vless.Config{
				Address:   "h2",
				Port:      443,
				UUID:      "00000000-0000-0000-0000-000000000000",
				Transport: vless.TransportWS,
				Security:  vless.SecurityNone,
			},
		},
	})

	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})
	got, err := svc.List()
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestServersService_TestLatency_OneServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:     "a",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless: vless.Config{
			Address:   "127.0.0.1",
			Port:      1, // closed port → fast TCP RST or timeout
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
		},
	}})

	h := hub.New()
	rcv := h.Subscribe(4)
	defer h.Close()

	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         h,
	})
	require.NoError(t, svc.TestLatency("a"))

	select {
	case e := <-rcv:
		require.Equal(t, hub.EventProbeResult, e.Name)
		results, ok := e.Payload["results"].([]map[string]any)
		require.True(t, ok, "results payload should be []map[string]any")
		require.Len(t, results, 1)
		require.Equal(t, "a", results[0]["id"])
	case <-timeAfter500ms():
		t.Fatal("no probe:result event")
	}
}

func TestServersService_ToggleFavorite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:     "a",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless: vless.Config{
			Address: "h",
			Port:    443,
			UUID:    "00000000-0000-0000-0000-000000000000",
		},
	}})

	store := fileServerStore{path: path}
	svc := NewServersService(ServersDeps{ServerStore: store, Hub: hub.New()})

	require.NoError(t, svc.ToggleFavorite("a"))
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.True(t, loaded[0].Favorite)

	require.NoError(t, svc.ToggleFavorite("a")) // toggle twice → off
	loaded, err = store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.False(t, loaded[0].Favorite)
}

func timeAfter500ms() <-chan time.Time { return time.After(500 * time.Millisecond) }

func TestServersService_Add_GeneratesIDAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}

	h := hub.New()
	rcv := h.Subscribe(4)
	defer h.Close()

	svc := NewServersService(ServersDeps{ServerStore: store, Hub: h})

	uri := "vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none#remark"
	view, err := svc.Add(uri, "Frankfurt")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(view.ID, "m"), "ID = %q, want m-prefix", view.ID)
	require.Equal(t, "Frankfurt", view.Name)
	require.Equal(t, "manual", view.Origin)

	loaded, err := store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, view.ID, loaded[0].ID)

	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event")
	}
}

func TestServersService_Add_RejectsEmptyURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	_, err := svc.Add("", "Name")
	require.Error(t, err)
	require.Contains(t, err.Error(), "uri required")
}

func TestServersService_Add_RejectsInvalidURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	_, err := svc.Add("not-a-vless-uri", "Name")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid VLESS URI")
}

func TestServersService_Add_AllowsDuplicateURIs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}
	svc := NewServersService(ServersDeps{ServerStore: store, Hub: hub.New()})

	uri := "vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none"
	_, err := svc.Add(uri, "First")
	require.NoError(t, err)
	_, err = svc.Add(uri, "Second")
	require.NoError(t, err, "duplicate URI must succeed (no dedup)")

	loaded, _ := store.Load()
	require.Len(t, loaded, 2)
	require.NotEqual(t, loaded[0].ID, loaded[1].ID, "duplicate adds must yield distinct IDs")
}

func TestServersService_Add_NameFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	view, err := svc.Add("vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none#FromRemark", "")
	require.NoError(t, err)
	require.Equal(t, "FromRemark", view.Name, "empty name should fall back to remark")
}
