package bindings

import (
	"encoding/json"
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

func TestRuleView_JSONFeedsBackIntoRule(t *testing.T) {
	svc, _, _ := newRulesService(t)
	id, err := svc.RuleAdd("user", bindingRule("", "x"))
	require.NoError(t, err)
	v, err := svc.List()
	require.NoError(t, err)
	raw, err := json.Marshal(findView(v, id))
	require.NoError(t, err)
	var back rules.Rule
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, "x", back.ServerID)
	require.Equal(t, "sub", back.ServerSourceID)
	require.Equal(t, "srv x", back.ServerName)
}

func TestRulesService_RuleSaveUsesRawServerIdentity(t *testing.T) {
	dir := t.TempDir()
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(srvPath, []server.Server{{ID: "x", SourceID: "real-sub", Name: "🇳🇱 Netherlands 2", Origin: server.OriginSubscription}}))
	h := hub.New()
	t.Cleanup(h.Close)
	svc := NewRulesService(RulesDeps{Store: rules.NewStore(dir), Hub: h, ServersPath: srvPath})
	r := bindingRule("", "x")
	r.ServerName = "Netherlands 2"
	r.ServerSourceID = ""
	id, err := svc.RuleAdd("user", r)
	require.NoError(t, err)
	v, err := svc.List()
	require.NoError(t, err)
	got := findView(v, id)
	require.Equal(t, "🇳🇱 Netherlands 2", got.ServerName)
	require.Equal(t, "real-sub", got.ServerSourceID)

	edited := r
	edited.ID = id
	edited.ServerName = "whatever"
	require.NoError(t, svc.RuleEdit(edited))
	v, _ = svc.List()
	require.Equal(t, "🇳🇱 Netherlands 2", findView(v, id).ServerName)
}
