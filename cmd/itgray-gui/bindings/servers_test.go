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

	// TestLatency persists new latencies to the store, so it must also
	// publish servers:changed for the frontend's serversStore to refetch.
	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event after probe")
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
	h := hub.New()
	rcv := h.Subscribe(4)
	defer h.Close()
	svc := NewServersService(ServersDeps{ServerStore: store, Hub: h})

	require.NoError(t, svc.ToggleFavorite("a"))
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.True(t, loaded[0].Favorite)

	// ToggleFavorite mutates persisted state and must publish
	// servers:changed so the frontend list refetches.
	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event after first toggle")
	}

	require.NoError(t, svc.ToggleFavorite("a")) // toggle twice → off
	loaded, err = store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.False(t, loaded[0].Favorite)

	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event after second toggle")
	}
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

// fakeActiveProbe is a minimal ActiveServerProbe used in Remove tests.
type fakeActiveProbe struct{ id string }

func (p fakeActiveProbe) ActiveServerID() string { return p.id }

func TestServersService_Remove_ManualWhileIdle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless:  vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"},
	}})

	h := hub.New()
	rcv := h.Subscribe(4)
	defer h.Close()

	svc := NewServersService(ServersDeps{
		ServerStore:  store,
		Hub:          h,
		ActiveServer: fakeActiveProbe{}, // idle
	})

	require.NoError(t, svc.Remove("m1"))

	loaded, _ := store.Load()
	require.Len(t, loaded, 0)

	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event")
	}
}

func TestServersService_Remove_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	svc := NewServersService(ServersDeps{
		ServerStore:  fileServerStore{path: path},
		Hub:          hub.New(),
		ActiveServer: fakeActiveProbe{},
	})

	err := svc.Remove("nope")
	require.ErrorIs(t, err, ErrServerNotFound)
}

func TestServersService_Remove_RejectsSubOrigin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:       "sub1",
		Origin:   server.OriginSubscription,
		SourceID: "src",
		Name:     "DE",
		Vless:    vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"},
	}})

	svc := NewServersService(ServersDeps{
		ServerStore:  fileServerStore{path: path},
		Hub:          hub.New(),
		ActiveServer: fakeActiveProbe{},
	})

	err := svc.Remove("sub1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only manual servers can be deleted")
}

func TestServersService_Remove_BlocksActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless:  vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"},
	}})

	svc := NewServersService(ServersDeps{
		ServerStore:  fileServerStore{path: path},
		Hub:          hub.New(),
		ActiveServer: fakeActiveProbe{id: "m1"},
	})

	err := svc.Remove("m1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "disconnect first to delete this server")
}

func TestServersService_Edit_NameOnly_PreservesVlessAndID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "Old",
		Vless:  vless.Config{Address: "h.example", Port: 443, UUID: "00000000-0000-0000-0000-000000000000", Transport: vless.TransportTCP, Security: vless.SecurityNone, Encryption: "none"},
	}})

	h := hub.New()
	rcv := h.Subscribe(4)
	defer h.Close()

	svc := NewServersService(ServersDeps{ServerStore: store, Hub: h})

	uri := "vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none"
	view, vlessChanged, err := svc.Edit("m1", uri, "New")
	require.NoError(t, err)
	require.False(t, vlessChanged, "name-only edit must report vlessChanged=false")
	require.Equal(t, "m1", view.ID, "ID must stay stable across edit")
	require.Equal(t, "New", view.Name)

	loaded, _ := store.Load()
	require.Len(t, loaded, 1)
	require.Equal(t, "New", loaded[0].Name)

	select {
	case e := <-rcv:
		require.Equal(t, hub.EventServersChanged, e.Name)
	case <-timeAfter500ms():
		t.Fatal("no servers:changed event")
	}
}

func TestServersService_Edit_URIChange_FlagsVlessChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless:  vless.Config{Address: "old.example", Port: 443, UUID: "00000000-0000-0000-0000-000000000000", Transport: vless.TransportTCP, Security: vless.SecurityNone},
	}})

	svc := NewServersService(ServersDeps{ServerStore: store, Hub: hub.New()})

	uri := "vless://00000000-0000-0000-0000-000000000000@new.example:443?type=tcp&security=none"
	_, vlessChanged, err := svc.Edit("m1", uri, "DE")
	require.NoError(t, err)
	require.True(t, vlessChanged, "URI change must report vlessChanged=true")

	loaded, _ := store.Load()
	require.Equal(t, "new.example", loaded[0].Vless.Address)
}

func TestServersService_Edit_PreservesUserFlags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	store := fileServerStore{path: path}
	latency := 42
	seedServers(t, path, []server.Server{{
		ID:        "m1",
		Origin:    server.OriginManual,
		Name:      "DE",
		Favorite:  true,
		Disabled:  false,
		Tags:      []string{"home"},
		LatencyMS: &latency,
		Vless:     vless.Config{Address: "h.example", Port: 443, UUID: "00000000-0000-0000-0000-000000000000", Transport: vless.TransportTCP, Security: vless.SecurityNone},
	}})

	svc := NewServersService(ServersDeps{ServerStore: store, Hub: hub.New()})

	uri := "vless://00000000-0000-0000-0000-000000000000@new.example:443?type=tcp&security=none"
	_, _, err := svc.Edit("m1", uri, "DE-renamed")
	require.NoError(t, err)

	loaded, _ := store.Load()
	require.True(t, loaded[0].Favorite)
	require.Equal(t, []string{"home"}, loaded[0].Tags)
	require.NotNil(t, loaded[0].LatencyMS)
	require.Equal(t, 42, *loaded[0].LatencyMS)
}

func TestServersService_Edit_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	uri := "vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none"
	_, _, err := svc.Edit("nope", uri, "X")
	require.ErrorIs(t, err, ErrServerNotFound)
}

func TestServersService_Edit_RejectsSubOrigin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:       "sub1",
		Origin:   server.OriginSubscription,
		SourceID: "src",
		Name:     "DE",
		Vless:    vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"},
	}})

	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	uri := "vless://00000000-0000-0000-0000-000000000000@h.example:443?type=tcp&security=none"
	_, _, err := svc.Edit("sub1", uri, "DE")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only manual servers can be edited")
}

func TestServersService_Edit_RejectsInvalidURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless:  vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"},
	}})

	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})

	_, _, err := svc.Edit("m1", "garbage", "DE")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid VLESS URI")
}

func TestServersService_List_PopulatesURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	seedServers(t, path, []server.Server{{
		ID:     "m1",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless: vless.Config{
			Address:    "h.example",
			Port:       443,
			UUID:       "00000000-0000-0000-0000-000000000000",
			Transport:  vless.TransportTCP,
			Security:   vless.SecurityNone,
			Encryption: "none",
		},
	}})

	svc := NewServersService(ServersDeps{
		ServerStore: fileServerStore{path: path},
		Hub:         hub.New(),
	})
	got, err := svc.List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotEmpty(t, got[0].URI, "URI must be populated for the projection")
	require.Contains(t, got[0].URI, "vless://", "URI must start with vless://")
	require.Contains(t, got[0].URI, "h.example", "URI must contain the host")

	// Round-trip: parsed URI must equal the original config.
	parsed, err := vless.ParseURL(got[0].URI)
	require.NoError(t, err)
	require.Equal(t, "h.example", parsed.Address)
	require.Equal(t, uint16(443), parsed.Port)
	require.Equal(t, "00000000-0000-0000-0000-000000000000", parsed.UUID)
}
