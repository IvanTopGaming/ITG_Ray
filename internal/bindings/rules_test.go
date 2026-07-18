package bindings

import (
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/ruleshare"
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

func makeRule(action rules.Action, ipcidr string) rules.Rule {
	return rules.Rule{
		Name: "Test", Enabled: true, Action: action,
		Conditions: rules.Conditions{IPCIDRs: []string{ipcidr}},
	}
}

func TestRulesService_RuleAdd_AssignsIDAndAppendsToGroup(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, _ := svc.GroupAdd("Custom")
	rid, err := svc.RuleAdd(gid, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	require.NoError(t, err)
	require.Contains(t, rid, "r")
	v, _ := svc.List()
	last := v.Groups[len(v.Groups)-1]
	require.Len(t, last.Rules, 1)
	require.Equal(t, rid, last.Rules[0].ID)
}

func TestRulesService_RuleAdd_ValidatesShape(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, _ := svc.GroupAdd("Custom")
	bad := rules.Rule{Name: "Bad", Enabled: true, Action: rules.ActionProxy} // no conditions
	_, err := svc.RuleAdd(gid, bad)
	require.ErrorContains(t, err, "conditions cannot be all-empty")
}

func TestRulesService_RuleAdd_RejectsLockedGroup(t *testing.T) {
	svc, _, _ := newRulesService(t)
	_, err := svc.RuleAdd("safety", makeRule(rules.ActionDirect, "1.2.3.4/32"))
	require.ErrorContains(t, err, "safety group is locked")
}

func TestRulesService_RuleEdit_ReplacesByID(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, _ := svc.GroupAdd("Custom")
	rid, _ := svc.RuleAdd(gid, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	updated := rules.Rule{
		ID: rid, Name: "Renamed", Enabled: false, Action: rules.ActionProxy,
		Conditions: rules.Conditions{IPCIDRs: []string{"5.6.7.8/32"}},
	}
	require.NoError(t, svc.RuleEdit(updated))
	v, _ := svc.List()
	last := v.Groups[len(v.Groups)-1].Rules[0]
	require.Equal(t, "Renamed", last.Name)
	require.False(t, last.Enabled)
	require.Equal(t, []string{"5.6.7.8/32"}, last.Conditions.IPCIDRs)
}

func TestRulesService_RuleToggle_FlipsEnabled(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, _ := svc.GroupAdd("Custom")
	rid, _ := svc.RuleAdd(gid, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	require.NoError(t, svc.RuleToggle(rid))
	v, _ := svc.List()
	require.False(t, v.Groups[len(v.Groups)-1].Rules[0].Enabled)
	require.NoError(t, svc.RuleToggle(rid))
	v, _ = svc.List()
	require.True(t, v.Groups[len(v.Groups)-1].Rules[0].Enabled)
}

func TestRulesService_RuleRemove_DeletesAcrossGroups(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, _ := svc.GroupAdd("Custom")
	rid, _ := svc.RuleAdd(gid, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	require.NoError(t, svc.RuleRemove(rid))
	v, _ := svc.List()
	require.Len(t, v.Groups[len(v.Groups)-1].Rules, 0)
}

func TestRulesService_RuleMove_BetweenGroups(t *testing.T) {
	svc, _, _ := newRulesService(t)
	src, _ := svc.GroupAdd("A")
	dst, _ := svc.GroupAdd("B")
	rid, _ := svc.RuleAdd(src, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	require.NoError(t, svc.RuleMove(rid, dst))
	v, _ := svc.List()
	var srcLen int
	var moved hub.RuleView
	var movedFound bool
	for _, g := range v.Groups {
		if g.ID == src {
			srcLen = len(g.Rules)
		}
		if g.ID == dst {
			require.Len(t, g.Rules, 1)
			moved = g.Rules[0]
			movedFound = true
		}
	}
	require.Equal(t, 0, srcLen)
	require.True(t, movedFound)
	require.Equal(t, rid, moved.ID, "moved rule keeps its id")
	require.Equal(t, "Test", moved.Name)
	require.True(t, moved.Enabled)
	require.Equal(t, "block", moved.Action)
	require.Equal(t, []string{"1.2.3.4/32"}, moved.Conditions.IPCIDRs)
}

func TestRulesService_RuleMove_RejectsLockedTarget(t *testing.T) {
	svc, _, _ := newRulesService(t)
	src, _ := svc.GroupAdd("A")
	rid, _ := svc.RuleAdd(src, makeRule(rules.ActionBlock, "1.2.3.4/32"))
	require.ErrorContains(t, svc.RuleMove(rid, "safety"), "safety group is locked")
}

func TestRulesService_ReplaceAll_PersistsModelAtomically(t *testing.T) {
	svc, _, h := newRulesService(t)
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	m := rules.Model{
		DefaultAction: rules.ActionProxy,
		Groups: []rules.Group{
			{ID: "safety", Name: "Safety", Locked: true, Enabled: true, Rules: []rules.Rule{
				// Rule ID must match the canonical "private" id from
				// rules.DefaultModel so the strict guardSafetyUnchanged
				// check passes — the renderer always sends back exactly
				// what List returned for Safety.
				{ID: "private", Name: "Private IPs", Enabled: true, Action: rules.ActionDirect,
					Conditions: rules.Conditions{IPCIDRs: []string{
						"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
						"127.0.0.0/8", "fc00::/7", "fe80::/10", "224.0.0.0/4",
					}}},
			}},
			{ID: "user", Name: "My Rules", Enabled: true, Rules: []rules.Rule{
				{ID: "r1", Name: "B", Enabled: true, Action: rules.ActionBlock,
					Conditions: rules.Conditions{IPCIDRs: []string{"1.2.3.4/32"}}},
			}},
		},
	}
	require.NoError(t, svc.ReplaceAll(m))
	v, _ := svc.List()
	require.Len(t, v.Groups, 2)
	require.Equal(t, "r1", v.Groups[1].Rules[0].ID)
	waitForRulesChanged(t, rcv, time.Second)
}

func TestRulesService_ReplaceAll_RejectsInvalidRule(t *testing.T) {
	svc, _, _ := newRulesService(t)
	bad := rules.Model{
		DefaultAction: rules.ActionProxy,
		Groups: []rules.Group{{ID: "user", Name: "My Rules", Enabled: true, Rules: []rules.Rule{
			{ID: "x", Name: "X", Enabled: true, Action: rules.ActionProxy}, // no conditions
		}}},
	}
	err := svc.ReplaceAll(bad)
	require.ErrorContains(t, err, "conditions cannot be all-empty")
}

func TestRulesService_ReplaceAll_RejectsSafetyMutation(t *testing.T) {
	svc, _, _ := newRulesService(t)
	tampered := rules.Model{
		DefaultAction: rules.ActionProxy,
		Groups: []rules.Group{
			{ID: "safety", Name: "Tampered", Locked: true, Enabled: true},
			{ID: "user", Name: "My Rules", Enabled: true},
		},
	}
	err := svc.ReplaceAll(tampered)
	require.ErrorContains(t, err, "safety group is locked")
}

func TestImportApplyAppendsNewGroup(t *testing.T) {
	svc, _, _ := newRulesService(t)
	link, err := ruleshare.Encode("Imported", []rules.Group{{
		Name:    "Streaming",
		Enabled: true,
		Rules: []rules.Rule{{
			Name:       "Netflix",
			Enabled:    true,
			Action:     rules.ActionDirect,
			Conditions: rules.Conditions{Domains: []rules.DomainMatcher{{Kind: "suffix", Value: "netflix.com"}}},
		}},
	}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	before, _ := svc.List()
	nBefore := len(before.Groups)

	if err := svc.ImportApply(link); err != nil {
		t.Fatalf("ImportApply: %v", err)
	}

	after, _ := svc.List()
	if len(after.Groups) != nBefore+1 {
		t.Fatalf("group count %d, want %d", len(after.Groups), nBefore+1)
	}
	added := after.Groups[len(after.Groups)-1]
	if added.Name != "Streaming" {
		t.Errorf("added group name = %q", added.Name)
	}
	if added.ID == "" || added.Rules[0].ID == "" {
		t.Errorf("fresh IDs not minted: %q / %q", added.ID, added.Rules[0].ID)
	}
	if !added.Enabled {
		t.Errorf("imported group should be enabled")
	}

	if err := svc.ImportApply(link); err != nil {
		t.Fatalf("second ImportApply: %v", err)
	}
	after2, _ := svc.List()
	if len(after2.Groups) != nBefore+2 {
		t.Errorf("re-import group count %d, want %d", len(after2.Groups), nBefore+2)
	}
}

func TestImportApplyLeavesSafetyUntouched(t *testing.T) {
	svc, _, _ := newRulesService(t)
	before, _ := svc.List()
	var safetyBefore *hub.GroupView
	for i := range before.Groups {
		if before.Groups[i].ID == "safety" {
			safetyBefore = &before.Groups[i]
		}
	}
	if safetyBefore == nil {
		t.Skip("no safety group in default model")
	}
	link, _ := ruleshare.Encode("x", []rules.Group{{Name: "g", Rules: []rules.Rule{{
		Name: "r", Action: rules.ActionProxy, Conditions: rules.Conditions{IPCIDRs: []string{"8.8.8.8/32"}},
	}}}})
	_ = svc.ImportApply(link)
	after, _ := svc.List()
	for i := range after.Groups {
		if after.Groups[i].ID == "safety" {
			if len(after.Groups[i].Rules) != len(safetyBefore.Rules) {
				t.Errorf("safety rules changed")
			}
		}
	}
}

func TestImportPreviewCounts(t *testing.T) {
	svc, _, _ := newRulesService(t)
	link, _ := ruleshare.Encode("Set", []rules.Group{{Name: "g", Rules: []rules.Rule{
		{Name: "a", Action: rules.ActionProxy, Conditions: rules.Conditions{IPCIDRs: []string{"1.1.1.1/32"}}},
		{Name: "b", Action: rules.ActionDirect, Conditions: rules.Conditions{IPCIDRs: []string{"2.2.2.2/32"}}},
		{Name: "c", Action: rules.ActionBlock, Conditions: rules.Conditions{IPCIDRs: []string{"3.3.3.3/32"}}},
	}}})

	before, _ := svc.List()
	pv, err := svc.ImportPreview(link)
	if err != nil {
		t.Fatalf("ImportPreview: %v", err)
	}
	if pv.Name != "Set" || pv.ProxyCount != 1 || pv.DirectCount != 1 || pv.BlockCount != 1 {
		t.Errorf("preview = %+v", pv)
	}
	after, _ := svc.List()
	if len(after.Groups) != len(before.Groups) {
		t.Errorf("ImportPreview mutated the store")
	}
}

func TestExportGroupRoundTrips(t *testing.T) {
	svc, _, _ := newRulesService(t)
	gid, err := svc.GroupAdd("Sharable")
	if err != nil {
		t.Fatalf("GroupAdd: %v", err)
	}
	if _, err := svc.RuleAdd(gid, rules.Rule{
		Name: "r", Enabled: true, Action: rules.ActionProxy,
		Conditions: rules.Conditions{Domains: []rules.DomainMatcher{{Kind: "keyword", Value: "youtube"}}},
	}); err != nil {
		t.Fatalf("RuleAdd: %v", err)
	}
	link, err := svc.ExportGroup(gid)
	if err != nil {
		t.Fatalf("ExportGroup: %v", err)
	}
	p, err := ruleshare.Decode(link)
	if err != nil {
		t.Fatalf("Decode exported: %v", err)
	}
	if len(p.Groups) != 1 || p.Groups[0].Name != "Sharable" || len(p.Groups[0].Rules) != 1 {
		t.Errorf("round trip shape = %+v", p.Groups)
	}
}

func TestExportGroupRejectsSafety(t *testing.T) {
	svc, _, _ := newRulesService(t)
	if _, err := svc.ExportGroup("safety"); err == nil {
		t.Errorf("expected error exporting safety group")
	}
}
