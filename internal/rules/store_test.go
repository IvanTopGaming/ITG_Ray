package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_LoadMissingFile_ReturnsDefault(t *testing.T) {
	s := NewStore(t.TempDir())
	m, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, ActionProxy, m.DefaultAction)
	require.Len(t, m.Groups, 2)
	require.Equal(t, "safety", m.Groups[0].ID)
	require.True(t, m.Groups[0].Locked)
	require.Equal(t, "user", m.Groups[1].ID)
	require.False(t, m.Groups[1].Locked)
}

func TestStore_LoadCorruptFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules.json"), []byte("not json"), 0o600))
	s := NewStore(dir)
	m, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, ActionProxy, m.DefaultAction, "corrupt file degrades to default rather than failing Load")
}

func TestStore_LoadEmptyDefaultAction_SelfHealsToProxy(t *testing.T) {
	dir := t.TempDir()
	// A model with an empty default_action (the camelCase-key write bug) —
	// valid JSON, parseable, but BuildSingbox would reject it. Load must
	// normalize it to proxy so chain bring-up can't be bricked.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules.json"),
		[]byte(`{"groups":[],"default_action":""}`), 0o600))
	s := NewStore(dir)
	m, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, ActionProxy, m.DefaultAction)
	require.NoError(t, m.Validate(), "self-healed model must pass validation")
}

func TestStore_SaveRoundTrips(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	want := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{{
			ID: "g1", Name: "Custom", Enabled: true, Rules: []Rule{{
				ID: "r1", Name: "Block ads", Enabled: true, Action: ActionBlock,
				Conditions: Conditions{IPCIDRs: []string{"1.2.3.4/32"}},
			}},
		}},
	}
	require.NoError(t, s.Save(want))
	got, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestStore_SaveIsAtomic_NoPartialFileVisible(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	for i := 0; i < 5; i++ {
		m := Model{DefaultAction: ActionProxy, Groups: []Group{{ID: "g", Enabled: true}}}
		require.NoError(t, s.Save(m))
		raw, err := os.ReadFile(filepath.Join(dir, "rules.json"))
		require.NoError(t, err)
		var probe Model
		require.NoError(t, json.Unmarshal(raw, &probe), "every visible state must be a complete JSON document")
	}
}
