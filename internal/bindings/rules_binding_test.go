package bindings

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func bindingRule(id, serverID string) rules.Rule {
	return rules.Rule{ID: id, Name: id, Enabled: true, Action: rules.ActionProxy, ServerID: serverID, ServerSourceID: "sub", ServerName: "srv " + serverID, Conditions: rules.Conditions{Processes: []string{id}}}
}

func findView(v hub.RulesView, id string) hub.RuleView {
	for _, g := range v.Groups {
		for _, r := range g.Rules {
			if r.ID == id {
				return r
			}
		}
	}
	return hub.RuleView{}
}

func TestRulesList_BindingStates(t *testing.T) {
	dir := t.TempDir()
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(srvPath, []server.Server{
		{ID: "ok", SourceID: "sub", Name: "srv ok", Origin: server.OriginSubscription},
		{ID: "dns", SourceID: "sub", Name: "srv dns", Origin: server.OriginSubscription},
	}))
	store := rules.NewStore(dir)
	unbound := rules.Rule{ID: "plain", Name: "plain", Enabled: true, Action: rules.ActionProxy, Conditions: rules.Conditions{Processes: []string{"p"}}}
	require.NoError(t, store.Save(rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{{ID: "user", Name: "u", Enabled: true, Rules: []rules.Rule{
		bindingRule("r-ok", "ok"), bindingRule("r-gone", "gone"), bindingRule("r-dns", "dns"), unbound,
	}}}}))
	issues := &routebind.Issues{}
	issues.Set(map[string]routebind.Binding{
		"r-dns": {RuleID: "r-dns", ServerID: "dns", State: routebind.StateUnusable, Reason: "dns"},
		"r-ok":  {RuleID: "r-ok", ServerID: "stale", State: routebind.StateUnusable, Reason: "dns"},
	})
	h := hub.New()
	t.Cleanup(h.Close)
	svc := NewRulesService(RulesDeps{Store: store, Hub: h, Issues: issues, ServersPath: srvPath})

	v, err := svc.List()
	require.NoError(t, err)
	require.Equal(t, "ok", findView(v, "r-ok").Binding.State)
	gone := findView(v, "r-gone")
	require.Equal(t, "missing", gone.Binding.State)
	require.Equal(t, "srv gone", gone.ServerName)
	dns := findView(v, "r-dns")
	require.Equal(t, "unusable", dns.Binding.State)
	require.Equal(t, "dns", dns.Binding.Reason)
	require.Nil(t, findView(v, "plain").Binding)
}

func TestRulesService_RuleAddKeepsBinding(t *testing.T) {
	svc, _, _ := newRulesService(t)
	r := bindingRule("", "x")
	id, err := svc.RuleAdd("user", r)
	require.NoError(t, err)
	v, err := svc.List()
	require.NoError(t, err)
	got := findView(v, id)
	require.Equal(t, "x", got.ServerID)
	require.Equal(t, "sub", got.ServerSourceID)
	require.Equal(t, "srv x", got.ServerName)
}
