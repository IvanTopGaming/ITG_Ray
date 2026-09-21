package bindings

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

func TestSubsService_SyncOne_IgnoresResponseAfterURLChange(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	releaseResponse := sync.OnceFunc(func() { close(release) })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.Header().Set("Profile-Title", "Old Provider")
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(ts.Close)
	t.Cleanup(releaseResponse)
	svc, store := newSubsServiceForTest(t, t.TempDir())
	require.NoError(t, store.Save([]subscription.Stored{{ID: "s1", Name: "Old Provider", URL: ts.URL}}))
	done := make(chan error, 1)
	go func() { done <- svc.SyncOne("s1") }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not start")
	}
	_, err := svc.Edit("s1", "https://new.example/sub", "")
	require.NoError(t, err)
	releaseResponse()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not finish")
	}
	subs, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "new.example", subs[0].Name)
	require.True(t, subs[0].LastSyncAt.IsZero())
}

func TestSubsService_SyncOne_ProviderName(t *testing.T) {
	const title = "Подписка 🚀"
	const validBody = "vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"
	for _, tc := range []struct {
		name   string
		header string
		body   string
		want   string
		fail   bool
	}{
		{"plain title replaces old name", "New Provider", validBody, "New Provider", false},
		{"encoded title replaces old name", "base64:" + base64.StdEncoding.EncodeToString([]byte(title)), validBody, title, false},
		{"missing title preserves name", "", validBody, "Previous Provider", false},
		{"empty title preserves name", "  ", validBody, "Previous Provider", false},
		{"malformed title preserves name", "base64:!", validBody, "Previous Provider", false},
		{"failed sync preserves name", "Error Page", "<html>bad gateway</html>", "Previous Provider", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Profile-Title", tc.header)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(ts.Close)
			svc, store := newSubsServiceForTest(t, t.TempDir())
			require.NoError(t, store.Save([]subscription.Stored{{ID: "s1", Name: "Previous Provider", URL: ts.URL}}))
			err := svc.SyncOne("s1")
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			subs, err := store.Load()
			require.NoError(t, err)
			require.Equal(t, tc.want, subs[0].Name)
			views, err := svc.List()
			require.NoError(t, err)
			require.Equal(t, tc.want, views[0].Name)
		})
	}
}

func TestSubsService_Add_FetchesProviderName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Profile-Title", "base64:UHJvdmlkZXI=")
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(ts.Close)
	svc, store := newSubsServiceForTest(t, t.TempDir())
	view, err := svc.Add(ts.URL+"/private-token", "")
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", view.Name)
	require.Eventually(t, func() bool {
		subs, err := store.Load()
		return err == nil && len(subs) == 1 && subs[0].Name == "Provider" && subs[0].LastStatus == "ok"
	}, 3*time.Second, 10*time.Millisecond)
}

// newSubsServiceForTest builds a SubsService over fresh FileStores rooted in
// dir. Shared helper for the Add/Remove unit tests below; List has its own
// inline setup because it pre-seeds servers.json with a richer fixture.
func newSubsServiceForTest(t *testing.T, dir string) (*SubsService, subscription.FileStore) {
	t.Helper()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	srvPath := filepath.Join(dir, "servers.json")
	svc := NewSubsService(SubsDeps{
		SubStore:    subStore,
		ServerStore: fileServerStore{path: srvPath},
		Hub:         hub.New(),
	})
	return svc, subStore
}

// TestSubsService_List exercises the read-only Subs.List binding shipped in
// C.T6: one subscription + one server linked by SourceID must surface as a
// SubView with the right name, server count, and last-sync status. The
// fileServerStore shim is shared with app_test.go (same package).
func TestSubsService_List(t *testing.T) {
	dir := t.TempDir()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	srvPath := filepath.Join(dir, "servers.json")

	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID:             "s1",
		Name:           "okins",
		URL:            "https://e.com/sub",
		UpdateInterval: subscription.Duration(time.Hour),
		LastSyncAt:     time.Now().Add(-30 * time.Second),
		LastStatus:     "OK",
	}}))
	require.NoError(t, server.Save(srvPath, []server.Server{{
		ID:       "a",
		Origin:   server.OriginSubscription,
		SourceID: "s1",
		Name:     "DE",
		Vless: vless.Config{
			Address:   "h",
			Port:      443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
			Security:  vless.SecurityNone,
		},
	}}))

	svc := NewSubsService(SubsDeps{
		SubStore:    subStore,
		ServerStore: fileServerStore{path: srvPath},
		Hub:         hub.New(),
	})

	got, err := svc.List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "s1", got[0].ID)
	require.Equal(t, "okins", got[0].Name)
	require.Equal(t, 1, got[0].ServerCount)
	require.Equal(t, "OK", got[0].LastSyncStatus)
	require.Equal(t, int(time.Hour/time.Second), got[0].UpdateInterval)
}

func TestSubsService_Add_GeneratesIDAndPersists(t *testing.T) {
	svc, store := newSubsServiceForTest(t, t.TempDir())

	view, err := svc.Add("https://example.com/sub", "")
	require.NoError(t, err)
	require.NotEmpty(t, view.ID)
	require.Equal(t, "example.com", view.Name)
	require.Equal(t, "https://example.com/sub", view.URL)

	all, err := store.Load()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, view.ID, all[0].ID)
	require.Equal(t, "example.com", all[0].Name)
	require.Equal(t, "https://example.com/sub", all[0].URL)
}

// TestSubsService_Add_RejectsInvalidURL covers the validation branch: a
// bare string with no scheme is rejected before any disk I/O. Verified by
// confirming the file was never written.
func TestSubsService_Add_RejectsInvalidURL(t *testing.T) {
	svc, store := newSubsServiceForTest(t, t.TempDir())

	_, err := svc.Add("not-a-url", "")
	require.Error(t, err)

	all, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, all)
}

// TestSubsService_Remove deletes by ID and asserts the slice shrinks. Uses
// FileStore.Save directly to seed (no Add() method on FileStore in plan-c).
func TestSubsService_Remove(t *testing.T) {
	svc, store := newSubsServiceForTest(t, t.TempDir())
	require.NoError(t, store.Save([]subscription.Stored{
		{ID: "s1", URL: "https://e/sub"},
		{ID: "s2", URL: "https://e/sub2"},
	}))

	require.NoError(t, svc.Remove("s1"))
	all, err := store.Load()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "s2", all[0].ID)
}

// TestSubsService_Remove_CascadesServers verifies removing a subscription
// also drops the servers it imported, so they don't linger as undeletable
// orphans. Servers belonging to other subs (or manual) are untouched.
func TestSubsService_Remove_CascadesServers(t *testing.T) {
	dir := t.TempDir()
	svc, store := newSubsServiceForTest(t, dir)
	require.NoError(t, store.Save([]subscription.Stored{
		{ID: "s1", URL: "https://e/sub"},
		{ID: "s2", URL: "https://e/sub2"},
	}))
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(srvPath, []server.Server{
		{ID: "a", Origin: server.OriginSubscription, SourceID: "s1", Name: "A", Vless: vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"}},
		{ID: "b", Origin: server.OriginSubscription, SourceID: "s2", Name: "B", Vless: vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"}},
		{ID: "m", Origin: server.OriginManual, Name: "M", Vless: vless.Config{Address: "h", Port: 443, UUID: "00000000-0000-0000-0000-000000000000"}},
	}))

	require.NoError(t, svc.Remove("s1"))

	loaded, err := server.Load(srvPath)
	require.NoError(t, err)
	ids := []string{}
	for _, s := range loaded {
		ids = append(ids, s.ID)
	}
	require.ElementsMatch(t, []string{"b", "m"}, ids, "only s1's server should be removed")
}

// failingSaveServerStore returns from Load but always errors on Save — used
// to exercise the SyncOne disk-failure-after-successful-fetch branch.
type failingSaveServerStore struct{}

func (failingSaveServerStore) Load() ([]server.Server, error) { return nil, nil }
func (failingSaveServerStore) Save([]server.Server) error     { return errors.New("disk full") }

// TestSubsService_SyncOne_PreservesUserinfoOnSaveFailure guards against the
// regression where ServerStore.Save failure overwrites syncErr and silently
// drops the freshly parsed Subscription-Userinfo, leaving subs.json with
// stale quota figures next to a red ERROR badge.
func TestSubsService_SyncOne_PreservesUserinfoOnSaveFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=900; download=800; total=1024")
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(ts.Close)

	dir := t.TempDir()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID:     "s1",
		Name:   "test",
		URL:    ts.URL,
		Upload: 100, Download: 200, Total: 1024,
	}}))

	svc := NewSubsService(SubsDeps{
		SubStore:    subStore,
		ServerStore: failingSaveServerStore{},
		Hub:         hub.New(),
	})

	err := svc.SyncOne("s1")
	require.Error(t, err, "Save failure must surface to caller")

	got, err := subStore.Load()
	require.NoError(t, err)
	require.EqualValues(t, 900, got[0].Upload, "fresh Upload persists despite Save failure")
	require.EqualValues(t, 800, got[0].Download, "fresh Download persists despite Save failure")
	require.EqualValues(t, 1024, got[0].Total, "fresh Total persists despite Save failure")
	require.Equal(t, "error", got[0].LastStatus, "status reflects disk failure")
}

// Two subscriptions syncing at the same time must (a) have their HTTP fetches
// overlap rather than queue behind one another, and (b) both end up in
// servers.json. Loading `existing` before the fetch and saving after means
// each sync merges into a snapshot taken before the other one wrote, so the
// later Save silently drops the earlier subscription's servers. That's a
// file-level lost update — the race detector cannot see it, hence this test.
func TestSubsService_SyncOne_ConcurrentSyncsKeepBothSubscriptions(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	subHandler := func(uuid, label string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			entered <- struct{}{}
			<-release // hold the fetch open so both syncs are in flight
			_, _ = w.Write([]byte("vless://" + uuid + "@1.2.3.4:443?type=tcp&security=tls&sni=x#" + label + "\n"))
		}
	}
	ts1 := httptest.NewServer(subHandler("11111111-1111-1111-1111-111111111111", "A"))
	t.Cleanup(ts1.Close)
	ts2 := httptest.NewServer(subHandler("22222222-2222-2222-2222-222222222222", "B"))
	t.Cleanup(ts2.Close)

	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)
	require.NoError(t, subStore.Save([]subscription.Stored{
		{ID: "s1", Name: "first", URL: ts1.URL},
		{ID: "s2", Name: "second", URL: ts2.URL},
	}))

	done := make(chan error, 2)
	go func() { done <- svc.SyncOne("s1") }()
	go func() { done <- svc.SyncOne("s2") }()

	for i := range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of 2 subscription fetches were in flight: syncs are serialized", i)
		}
	}
	close(release)
	for range 2 {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("SyncOne did not return")
		}
	}

	list, err := server.Load(filepath.Join(dir, "servers.json"))
	require.NoError(t, err)
	bySource := map[string]int{}
	for i := range list {
		bySource[list[i].SourceID]++
	}
	require.Equal(t, 1, bySource["s1"], "servers from the first subscription survived the concurrent sync")
	require.Equal(t, 1, bySource["s2"], "servers from the second subscription survived the concurrent sync")
}

func TestSubsService_Edit_UserAgentPreservesNameServersAndLastSync(t *testing.T) {
	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)
	srvPath := filepath.Join(dir, "servers.json")

	syncedAt := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID:         "s1",
		Name:       "old name",
		URL:        "https://provider.example/sub",
		LastSyncAt: syncedAt,
		LastStatus: "OK",
	}}))
	require.NoError(t, server.Save(srvPath, []server.Server{{
		ID:       "srv1",
		Origin:   server.OriginSubscription,
		SourceID: "s1",
		Name:     "DE",
		Vless: vless.Config{
			Address: "h", Port: 443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
			Security:  vless.SecurityNone,
		},
	}}))

	view, err := svc.Edit("s1", "https://provider.example/sub", "Custom/2.0")
	require.NoError(t, err)
	require.Equal(t, "old name", view.Name)
	require.Equal(t, "Custom/2.0", view.UserAgent)
	require.Equal(t, "OK", view.LastSyncStatus)
	require.True(t, view.LastSyncAt.Equal(syncedAt), "LastSyncAt must be preserved when updating User-Agent")
	require.Equal(t, 1, view.ServerCount, "servers must not be cascaded when updating User-Agent")

	loaded, err := subStore.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "old name", loaded[0].Name)
	require.True(t, loaded[0].LastSyncAt.Equal(syncedAt))

	srvs, err := server.Load(srvPath)
	require.NoError(t, err)
	require.Len(t, srvs, 1, "server with this SourceID must survive a User-Agent change")
}

func TestSubsService_Edit_URLChange_CascadesServersAndResetsMeta(t *testing.T) {
	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)
	srvPath := filepath.Join(dir, "servers.json")

	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID:          "s1",
		Name:        "renamed",
		URL:         "https://old.example/sub",
		LastSyncAt:  time.Now().Add(-1 * time.Hour),
		LastStatus:  "OK",
		LastMessage: "fetched 5 servers",
		Upload:      100,
		Download:    200,
		Total:       1024,
	}}))
	mkSrv := func(id, src string) server.Server {
		return server.Server{
			ID: id, Origin: server.OriginSubscription, SourceID: src, Name: id,
			Vless: vless.Config{
				Address: "h", Port: 443,
				UUID:      "00000000-0000-0000-0000-000000000000",
				Transport: vless.TransportTCP,
				Security:  vless.SecurityNone,
			},
		}
	}
	require.NoError(t, server.Save(srvPath, []server.Server{
		mkSrv("a", "s1"),
		mkSrv("b", "s1"),
		mkSrv("c", "s2"), // belongs to a different sub — must survive
		mkSrv("d", ""),   // manual entry — must survive
	}))

	view, err := svc.Edit("s1", "https://new.example/sub", "")
	require.NoError(t, err)
	require.Equal(t, "https://new.example/sub", view.URL)
	require.Equal(t, "new.example", view.Name)
	require.True(t, view.LastSyncAt.IsZero(), "LastSyncAt must reset on URL change")
	require.Equal(t, "", view.LastSyncStatus, "LastSyncStatus must reset on URL change")
	require.Equal(t, 0, view.ServerCount, "old servers must be cascaded")
	require.Equal(t, int64(0), view.Upload)
	require.Equal(t, int64(0), view.Download)
	require.Equal(t, int64(0), view.Total)

	// On-disk verification: only s1 servers cascaded, s2 + manual survive.
	srvs, err := server.Load(srvPath)
	require.NoError(t, err)
	require.Len(t, srvs, 2)
	gotIDs := []string{srvs[0].ID, srvs[1].ID}
	require.Contains(t, gotIDs, "c")
	require.Contains(t, gotIDs, "d")
}

func TestSubsService_Edit_RejectsInvalidURL(t *testing.T) {
	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)

	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID: "s1", Name: "x", URL: "https://provider.example/sub",
	}}))

	_, err := svc.Edit("s1", "ftp://bad", "")
	require.ErrorIs(t, err, errInvalidURL)
}

func TestSubsService_Edit_ReturnsErrSubNotFound(t *testing.T) {
	dir := t.TempDir()
	svc, _ := newSubsServiceForTest(t, dir)

	_, err := svc.Edit("missing-id", "https://provider.example/sub", "")
	require.ErrorIs(t, err, errSubNotFound)
}

func TestSubsService_Add_PersistsUserAgent(t *testing.T) {
	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)

	_, err := svc.Add("https://provider.example/sub", "Custom/1.0")
	require.NoError(t, err)

	loaded, err := subStore.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "Custom/1.0", loaded[0].UserAgent)
}

func TestSubsService_Add_UsesConfiguredInterval(t *testing.T) {
	dir := t.TempDir()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, subStore.Save([]subscription.Stored{}))
	require.NoError(t, server.Save(srvPath, []server.Server{}))

	svc := NewSubsService(SubsDeps{
		SubStore:    subStore,
		ServerStore: fileServerStore{path: srvPath},
		Hub:         hub.New(),
		SettingsView: func() hub.SettingsView {
			return hub.SettingsView{
				Subscriptions: hub.SubscriptionSettings{DefaultUpdateInterval: 7200},
			}
		},
	})

	_, err := svc.Add("https://example.com/sub", "")
	require.NoError(t, err)

	all, err := subStore.Load()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, subscription.Duration(2*time.Hour), all[0].UpdateInterval)
}

func TestSubsService_Add_FallsBackWhenSettingsZero(t *testing.T) {
	dir := t.TempDir()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, subStore.Save([]subscription.Stored{}))
	require.NoError(t, server.Save(srvPath, []server.Server{}))

	svc := NewSubsService(SubsDeps{
		SubStore:    subStore,
		ServerStore: fileServerStore{path: srvPath},
		Hub:         hub.New(),
		SettingsView: func() hub.SettingsView {
			return hub.SettingsView{Subscriptions: hub.SubscriptionSettings{DefaultUpdateInterval: 0}}
		},
	})

	_, err := svc.Add("https://example.com/sub", "")
	require.NoError(t, err)
	all, err := subStore.Load()
	require.NoError(t, err)
	require.Equal(t, subscription.Duration(defaultUpdateInterval), all[0].UpdateInterval)
}

func TestSubsService_Add_FallsBackWhenSettingsViewNil(t *testing.T) {
	dir := t.TempDir()
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, subStore.Save([]subscription.Stored{}))
	require.NoError(t, server.Save(srvPath, []server.Server{}))

	// SettingsView nil exercises a distinct branch from value==0: the
	// closure is never invoked, so the constant fallback must still apply.
	svc := NewSubsService(SubsDeps{
		SubStore:     subStore,
		ServerStore:  fileServerStore{path: srvPath},
		Hub:          hub.New(),
		SettingsView: nil,
	})

	_, err := svc.Add("https://example.com/sub", "")
	require.NoError(t, err)
	all, err := subStore.Load()
	require.NoError(t, err)
	require.Equal(t, subscription.Duration(defaultUpdateInterval), all[0].UpdateInterval)
}

func TestSubsService_Edit_UpdatesUserAgent_IncludingClearToEmpty(t *testing.T) {
	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)

	require.NoError(t, subStore.Save([]subscription.Stored{{
		ID: "s1", Name: "x", URL: "https://provider.example/sub", UserAgent: "old/1.0",
	}}))

	_, err := svc.Edit("s1", "https://provider.example/sub", "")
	require.NoError(t, err)

	loaded, err := subStore.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Empty(t, loaded[0].UserAgent, "explicit empty must clear")
}

func TestSubsService_SyncOne_PreservesServersWhenNothingUsable(t *testing.T) {
	for _, body := range []string{
		"hysteria2://password@hy.example:443",
		`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":0,"id":"u"}}]}`,
		`{"error":"https://provider.example/private-token"}`,
	} {
		t.Run(body, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			t.Cleanup(ts.Close)
			dir := t.TempDir()
			svc, store := newSubsServiceForTest(t, dir)
			require.NoError(t, store.Save([]subscription.Stored{{ID: "s1", Name: "Provider", URL: ts.URL}}))
			original := []server.Server{{ID: "existing", Name: "Previous", Origin: server.OriginSubscription, SourceID: "s1"}}
			srvPath := filepath.Join(dir, "servers.json")
			require.NoError(t, server.Save(srvPath, original))
			require.Error(t, svc.SyncOne("s1"))
			actual, err := server.Load(srvPath)
			require.NoError(t, err)
			require.Equal(t, original, actual)
			subs, err := store.Load()
			require.NoError(t, err)
			require.Equal(t, "error", subs[0].LastStatus)
			require.NotContains(t, subs[0].LastMessage, "private-token")
			views, err := svc.List()
			require.NoError(t, err)
			require.Equal(t, 1, views[0].ServerCount)
		})
	}
}

func TestSubsService_SyncOne_ImportsXrayAndReportsSkipped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"remarks":"Alpha","outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"user"}}]},{"outbounds":[{"protocol":"hysteria"}]}]`))
	}))
	t.Cleanup(ts.Close)
	dir := t.TempDir()
	svc, store := newSubsServiceForTest(t, dir)
	require.NoError(t, store.Save([]subscription.Stored{{ID: "s1", Name: "Provider", URL: ts.URL}}))
	require.NoError(t, svc.SyncOne("s1"))
	actual, err := server.Load(filepath.Join(dir, "servers.json"))
	require.NoError(t, err)
	require.Len(t, actual, 1)
	require.Equal(t, "Alpha", actual[0].Name)
	subs, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "ok", subs[0].LastStatus)
	require.Equal(t, "imported=1 invalid=0 skipped=1", subs[0].LastMessage)
}
