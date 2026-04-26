package bindings

import (
	"context"
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

	got, err := svc.List(context.Background())
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

	view, err := svc.Add(context.Background(), "https://example.com/sub", "test")
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

	_, err := svc.Add(context.Background(), "not-a-url", "")
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

	require.NoError(t, svc.Remove(context.Background(), "s1"))
	all, err := store.Load()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "s2", all[0].ID)
}
