package bindings

import (
	"testing"
	"time"

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

func waitForRulesChanged(t *testing.T, rcv <-chan hub.Event, timeout time.Duration) {
	t.Helper()
	select {
	case e := <-rcv:
		require.Equal(t, hub.EventRulesChanged, e.Name)
	case <-time.After(timeout):
		t.Fatal("timeout waiting for rules:changed")
	}
}

func TestRulesService_GroupAdd_AssignsIDAndPublishesEvent(t *testing.T) {
	svc, _, h := newRulesService(t)
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	id, err := svc.GroupAdd("Streaming")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Contains(t, id, "g")

	v, err := svc.List()
	require.NoError(t, err)
	require.Len(t, v.Groups, 3)
	require.Equal(t, "Streaming", v.Groups[2].Name)
	require.True(t, v.Groups[2].Enabled)
	waitForRulesChanged(t, rcv, time.Second)
}

func TestRulesService_GroupEdit_RenamesAndToggles(t *testing.T) {
	svc, _, h := newRulesService(t)
	id, err := svc.GroupAdd("First")
	require.NoError(t, err)
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	require.NoError(t, svc.GroupEdit(id, "Renamed", false))
	waitForRulesChanged(t, rcv, time.Second)
	v, _ := svc.List()
	last := v.Groups[len(v.Groups)-1]
	require.Equal(t, "Renamed", last.Name)
	require.False(t, last.Enabled)
}

func TestRulesService_GroupRemove_DeletesNonLocked(t *testing.T) {
	svc, _, h := newRulesService(t)
	id, err := svc.GroupAdd("Tmp")
	require.NoError(t, err)
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	require.NoError(t, svc.GroupRemove(id))
	waitForRulesChanged(t, rcv, time.Second)
	v, _ := svc.List()
	for _, g := range v.Groups {
		require.NotEqual(t, id, g.ID)
	}
}

func TestRulesService_GroupEdit_RejectsLocked(t *testing.T) {
	svc, _, _ := newRulesService(t)
	err := svc.GroupEdit("safety", "Renamed", true)
	require.ErrorContains(t, err, "safety group is locked")
}

func TestRulesService_GroupRemove_RejectsLocked(t *testing.T) {
	svc, _, _ := newRulesService(t)
	err := svc.GroupRemove("safety")
	require.ErrorContains(t, err, "safety group is locked")
}

func TestRulesService_GroupAdd_RejectsEmptyName(t *testing.T) {
	svc, _, _ := newRulesService(t)
	_, err := svc.GroupAdd("")
	require.ErrorContains(t, err, "group name cannot be empty")
	_, err = svc.GroupAdd("   ")
	require.ErrorContains(t, err, "group name cannot be empty")
}

func TestRulesService_GroupEdit_UnknownIDReturnsError(t *testing.T) {
	svc, _, _ := newRulesService(t)
	err := svc.GroupEdit("nope", "X", true)
	require.ErrorContains(t, err, "group not found")
}
