package bindings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

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

// TestSubsService_Add_GeneratesIDAndPersists checks the happy path: a valid
// http(s) URL produces a non-empty ID, the friendly name round-trips, and
// the entry lands in the on-disk file. The auto-kicked SyncOne goroutine is
// not awaited — it will fail to dial example.com:443 in CI, but since Add
// returns *before* spawning the goroutine and the test uses t.TempDir, the
// goroutine cannot race with assertions on the store contents.
func TestSubsService_Add_GeneratesIDAndPersists(t *testing.T) {
	svc, store := newSubsServiceForTest(t, t.TempDir())

	view, err := svc.Add("https://example.com/sub", "test")
	require.NoError(t, err)
	require.NotEmpty(t, view.ID)
	require.Equal(t, "test", view.Name)
	require.Equal(t, "https://example.com/sub", view.URL)

	all, err := store.Load()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, view.ID, all[0].ID)
	require.Equal(t, "test", all[0].Name)
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

func TestSubsService_Edit_RenameOnly_PreservesServersAndLastSync(t *testing.T) {
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

	view, err := svc.Edit("s1", "https://provider.example/sub", "new name")
	require.NoError(t, err)
	require.Equal(t, "new name", view.Name)
	require.Equal(t, "OK", view.LastSyncStatus)
	require.True(t, view.LastSyncAt.Equal(syncedAt), "LastSyncAt must be preserved on rename")
	require.Equal(t, 1, view.ServerCount, "servers must not be cascaded on rename")

	loaded, err := subStore.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "new name", loaded[0].Name)
	require.True(t, loaded[0].LastSyncAt.Equal(syncedAt))

	srvs, err := server.Load(srvPath)
	require.NoError(t, err)
	require.Len(t, srvs, 1, "server with this SourceID must survive rename")
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
		mkSrv("c", "s2"),    // belongs to a different sub — must survive
		mkSrv("d", ""),       // manual entry — must survive
	}))

	view, err := svc.Edit("s1", "https://new.example/sub", "renamed")
	require.NoError(t, err)
	require.Equal(t, "https://new.example/sub", view.URL)
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
