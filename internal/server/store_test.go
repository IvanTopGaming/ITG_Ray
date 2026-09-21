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

func TestMerge_ConnectionVariantsKeepIndependentIDsAndLocalFields(t *testing.T) {
	a := New(vless.Config{Address: "node.example", Port: 443, UUID: "u", SNI: "a.example"}, OriginSubscription, "sub1")
	b := a
	b.Vless.SNI = "b.example"
	original := a
	original.Favorite = true
	original.Tags = []string{"keep"}
	first := Merge([]Server{original}, []Server{b, a}, "sub1")
	require.Len(t, first, 2)
	require.NotEqual(t, first[0].ID, first[1].ID)
	require.Equal(t, original.ID, first[1].ID)
	require.True(t, first[1].Favorite)
	require.Equal(t, []string{"keep"}, first[1].Tags)
	require.False(t, first[0].Favorite)
	first[0].Disabled = true
	first[0].Tags = []string{"variant-b"}
	second := Merge(first, []Server{a, b}, "sub1")
	require.Equal(t, first[1].ID, second[0].ID)
	require.Equal(t, first[0].ID, second[1].ID)
	require.True(t, second[1].Disabled)
	require.Equal(t, []string{"variant-b"}, second[1].Tags)
	remaining := Merge(second, []Server{b}, "sub1")
	require.Len(t, remaining, 1)
	require.Equal(t, first[0].ID, remaining[0].ID)
	require.True(t, remaining[0].Disabled)
}

func TestMerge_NewVariantIDsDoNotDependOnOrderOrName(t *testing.T) {
	a := New(vless.Config{Address: "node.example", Port: 443, UUID: "u", SNI: "a.example"}, OriginSubscription, "sub1")
	b := a
	b.Vless.SNI = "b.example"
	first := Merge(nil, []Server{a, b}, "sub1")
	a.Vless.Remark = "renamed"
	second := Merge(nil, []Server{b, a}, "sub1")
	require.NotEqual(t, first[0].ID, first[1].ID)
	require.Equal(t, first[0].ID, second[1].ID)
	require.Equal(t, first[1].ID, second[0].ID)
}

func TestMerge_SameEndpointInAnotherSourceIsPreserved(t *testing.T) {
	a := New(vless.Config{Address: "node.example", Port: 443, UUID: "u"}, OriginSubscription, "sub1")
	b := a
	b.SourceID = "sub2"
	a.Favorite = true
	merged := Merge([]Server{a}, []Server{b}, "sub2")
	require.Len(t, merged, 2)
	require.NotEqual(t, merged[0].ID, merged[1].ID)
	require.False(t, merged[0].Favorite)
	require.Equal(t, a, merged[1])
}

func TestMerge_PreviouslyReusedVariantIDDoesNotCollide(t *testing.T) {
	a := New(vless.Config{Address: "node.example", Port: 443, UUID: "u", SNI: "a.example"}, OriginSubscription, "sub1")
	b := a
	b.Vless.SNI = "b.example"
	initial := Merge(nil, []Server{a, b}, "sub1")
	onlyA := Merge(initial, []Server{a}, "sub1")
	onlyB := Merge(onlyA, []Server{b}, "sub1")
	require.Equal(t, onlyA[0].ID, onlyB[0].ID)
	onlyB[0].Favorite = true
	both := Merge(onlyB, []Server{a, b}, "sub1")
	require.Len(t, both, 2)
	require.NotEqual(t, both[0].ID, both[1].ID)
	require.Equal(t, onlyB[0].ID, both[1].ID)
	require.False(t, both[0].Favorite)
	require.True(t, both[1].Favorite)
	reordered := Merge(both, []Server{b, a}, "sub1")
	require.Equal(t, both[0].ID, reordered[1].ID)
	require.Equal(t, both[1].ID, reordered[0].ID)
}
