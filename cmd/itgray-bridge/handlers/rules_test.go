package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/bindings"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func newRulesHandlers(t *testing.T) RulesHandlers {
	t.Helper()
	dir := t.TempDir()
	store := rules.NewStore(dir)
	h := hub.New()
	t.Cleanup(h.Close)
	svc := bindings.NewRulesService(bindings.RulesDeps{Store: store, Hub: h})
	return RulesHandlers{Svc: svc}
}

func TestRulesHandlers_List(t *testing.T) {
	h := newRulesHandlers(t)
	res, err := h.List(context.Background(), nil)
	require.NoError(t, err)
	v, ok := res.(hub.RulesView)
	require.True(t, ok, "handler returns hub.RulesView for the dispatcher to marshal")
	require.Len(t, v.Groups, 2)
}

func TestRulesHandlers_GroupAdd_RoundTripsID(t *testing.T) {
	h := newRulesHandlers(t)
	params, _ := json.Marshal(map[string]string{"name": "Streaming"})
	res, err := h.GroupAdd(context.Background(), params)
	require.NoError(t, err)
	out, ok := res.(map[string]string)
	require.True(t, ok, "handler returns map[string]string{id} for the dispatcher to marshal")
	require.NotEmpty(t, out["id"])
}
