package routebind

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func srv(id, source, name string) server.Server {
	origin := server.OriginSubscription
	if source == "" {
		origin = server.OriginManual
	}
	return server.Server{ID: id, SourceID: source, Name: name, Origin: origin}
}

func model(r rules.Rule) rules.Model {
	r.Enabled = true
	if r.ID == "" {
		r.ID = "r1"
	}
	if r.Conditions.IsEmpty() {
		r.Conditions = rules.Conditions{Processes: []string{"app"}}
	}
	return rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{{ID: "g", Name: "g", Enabled: true, Rules: []rules.Rule{r}}}}
}

func bound(id, source, name string) rules.Rule {
	return rules.Rule{Action: rules.ActionProxy, ServerID: id, ServerSourceID: source, ServerName: name}
}

func TestResolve(t *testing.T) {
	disabled := srv("a", "sub", "NL")
	disabled.Disabled = true
	cases := []struct {
		name    string
		rule    rules.Rule
		servers []server.Server
		state   State
		reason  string
		wantID  string
		absent  bool
	}{
		{name: "id present", rule: bound("a", "sub", "NL"), servers: []server.Server{srv("a", "sub", "NL")}, state: StateOK, wantID: "a"},
		{name: "rebound by name in same sub", rule: bound("a", "sub", "NL"), servers: []server.Server{srv("b", "sub", "NL"), srv("c", "sub", "FI")}, state: StateRebound, wantID: "b"},
		{name: "ambiguous name", rule: bound("a", "sub", "NL"), servers: []server.Server{srv("b", "sub", "NL"), srv("c", "sub", "NL")}, state: StateMissing, wantID: "a"},
		{name: "name only in another sub", rule: bound("a", "sub", "NL"), servers: []server.Server{srv("b", "other", "NL")}, state: StateMissing, wantID: "a"},
		{name: "manual server gone", rule: bound("m", "", "Home"), servers: []server.Server{srv("x", "", "Home")}, state: StateMissing, wantID: "m"},
		{name: "disabled", rule: bound("a", "sub", "NL"), servers: []server.Server{disabled}, state: StateUnusable, reason: "disabled", wantID: "a"},
		{name: "direct rule ignored", rule: rules.Rule{Action: rules.ActionDirect, ServerID: "a"}, servers: []server.Server{srv("a", "sub", "NL")}, absent: true, wantID: "a"},
		{name: "unbound proxy ignored", rule: rules.Rule{Action: rules.ActionProxy}, servers: []server.Server{srv("a", "sub", "NL")}, absent: true},
		{name: "name-only binds within its sub", rule: bound("", "sub", "NL"), servers: []server.Server{srv("x", "other", "NL"), srv("b", "sub", "NL")}, state: StateRebound, wantID: "b"},
		{name: "name-only never crosses subs", rule: bound("", "sub", "NL"), servers: []server.Server{srv("x", "other", "NL"), srv("m", "", "NL")}, state: StateMissing},
		{name: "name-only in its sub but disabled", rule: bound("", "sub", "NL"), servers: []server.Server{disabled}, state: StateUnusable, reason: "disabled", wantID: "a"},
		{name: "imported name binds when unique", rule: bound("", "", "NL"), servers: []server.Server{srv("b", "sub", "NL"), srv("c", "sub", "FI")}, state: StateRebound, wantID: "b"},
		{name: "imported name ambiguous", rule: bound("", "", "NL"), servers: []server.Server{srv("b", "sub", "NL"), srv("c", "other", "NL")}, state: StateMissing},
		{name: "imported name unknown", rule: bound("", "", "NL"), servers: []server.Server{srv("c", "sub", "FI")}, state: StateMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := model(tc.rule)
			out, bindings := Resolve(in, tc.servers)
			require.Equal(t, tc.wantID, out.Groups[0].Rules[0].ServerID)
			b, ok := bindings["r1"]
			if tc.absent {
				require.False(t, ok)
				return
			}
			require.True(t, ok)
			require.Equal(t, tc.state, b.State)
			require.Equal(t, tc.reason, b.Reason)
			require.Equal(t, tc.wantID, b.ServerID)
		})
	}
}

func TestResolve_RebindRefreshesNameAndKeepsInputIntact(t *testing.T) {
	in := model(bound("a", "sub", "NL"))
	out, _ := Resolve(in, []server.Server{srv("b", "sub", "NL")})
	require.Equal(t, "b", out.Groups[0].Rules[0].ServerID)
	require.Equal(t, "a", in.Groups[0].Rules[0].ServerID)
}
