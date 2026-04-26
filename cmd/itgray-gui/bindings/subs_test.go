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
