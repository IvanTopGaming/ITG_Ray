package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func watchFixture(t *testing.T) (*rules.Store, string, *routebind.Issues, *[]hub.Event) {
	t.Helper()
	dir := t.TempDir()
	store := rules.NewStore(dir)
	require.NoError(t, store.Save(rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{{ID: "user", Name: "u", Enabled: true, Rules: []rules.Rule{
		{ID: "r1", Name: "r1", Enabled: true, Action: rules.ActionProxy, ServerID: "old", ServerSourceID: "sub", ServerName: "NL", Conditions: rules.Conditions{Processes: []string{"a"}}},
	}}}}))
	path := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(path, []server.Server{{ID: "new", SourceID: "sub", Name: "NL", Origin: server.OriginSubscription}}))
	var published []hub.Event
	return store, path, &routebind.Issues{}, &published
}

func TestOnBindingEvent_SubSyncedRebindsAndPublishes(t *testing.T) {
	store, path, issues, published := watchFixture(t)
	publish := func(e hub.Event) { *published = append(*published, e) }
	onBindingEvent(hub.Event{Name: hub.EventSubSynced}, store, path, issues, publish)
	require.Len(t, *published, 1)
	require.Equal(t, hub.EventRulesChanged, (*published)[0].Name)
	m, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "new", m.Groups[0].Rules[0].ServerID)
	onBindingEvent(hub.Event{Name: hub.EventSubSynced}, store, path, issues, publish)
	require.Len(t, *published, 1)
}

func TestOnBindingEvent_ConnectedWithIssuesPublishes(t *testing.T) {
	store, path, issues, published := watchFixture(t)
	publish := func(e hub.Event) { *published = append(*published, e) }
	connected := hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "connected"}}
	onBindingEvent(connected, store, path, issues, publish)
	require.Empty(t, *published)
	issues.Set(map[string]routebind.Binding{"r1": {State: routebind.StateUnusable, Reason: "dns"}})
	onBindingEvent(connected, store, path, issues, publish)
	require.Len(t, *published, 1)
	onBindingEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "idle"}}, store, path, issues, publish)
	require.Len(t, *published, 1)
}
