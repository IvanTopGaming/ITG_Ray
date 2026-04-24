package server

import (
	"path/filepath"
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func TestStore_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	s := New(vless.Config{Address: "h", Port: 443, UUID: "u", Remark: "n"}, OriginManual, "")

	require.NoError(t, Save(path, []Server{s}))
	got, err := Load(path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, s, got[0])
}

func TestStore_LoadMissingReturnsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestStore_Merge_PreservesLocalFields(t *testing.T) {
	existing := New(vless.Config{Address: "h", Port: 443, UUID: "u"}, OriginSubscription, "sub1")
	latency := 42
	existing.LatencyMS = &latency
	existing.Favorite = true
	existing.Tags = []string{"main"}

	incoming := New(vless.Config{Address: "h", Port: 443, UUID: "u", Remark: "updated"}, OriginSubscription, "sub1")
	incoming.Vless.SNI = "changed.example"

	merged := Merge([]Server{existing}, []Server{incoming}, "sub1")
	require.Len(t, merged, 1)
	require.Equal(t, "changed.example", merged[0].Vless.SNI)
	require.Equal(t, &latency, merged[0].LatencyMS)
	require.True(t, merged[0].Favorite)
	require.Equal(t, []string{"main"}, merged[0].Tags)
	require.Equal(t, "updated", merged[0].Remark)
}

func TestStore_Merge_RemovesOnlyOwnOrigin(t *testing.T) {
	subServer := New(vless.Config{Address: "a", Port: 1, UUID: "u"}, OriginSubscription, "sub1")
	manual := New(vless.Config{Address: "b", Port: 2, UUID: "u"}, OriginManual, "")
	other := New(vless.Config{Address: "c", Port: 3, UUID: "u"}, OriginSubscription, "sub2")

	// sub1 sync returns empty — sub1's server must go, manual and sub2 must stay.
	merged := Merge([]Server{subServer, manual, other}, nil, "sub1")
	require.Len(t, merged, 2)
	ids := []string{merged[0].ID, merged[1].ID}
	require.Contains(t, ids, manual.ID)
	require.Contains(t, ids, other.ID)
}
