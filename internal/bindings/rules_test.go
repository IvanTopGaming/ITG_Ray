package bindings

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func newRulesService(t *testing.T) (*RulesService, *rules.Store, *hub.Hub) {
	t.Helper()
	dir := t.TempDir()
	store := rules.NewStore(dir)
	h := hub.New()
	t.Cleanup(h.Close)
	svc := NewRulesService(RulesDeps{Store: store, Hub: h})
	return svc, store, h
}

func TestRulesService_List_ReturnsDefaultWhenNoFile(t *testing.T) {
	svc, _, _ := newRulesService(t)
	v, err := svc.List()
	require.NoError(t, err)
	require.Equal(t, "proxy", v.DefaultAction)
	require.Len(t, v.Groups, 2)
	require.Equal(t, "safety", v.Groups[0].ID)
	require.True(t, v.Groups[0].Locked)
	require.Equal(t, "user", v.Groups[1].ID)
}
