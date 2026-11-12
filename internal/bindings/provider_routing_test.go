package bindings

import (
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/ruleshare"
	"github.com/stretchr/testify/require"
)

func providerLink(t *testing.T, name string, rs ...rules.Rule) string {
	t.Helper()
	link, err := ruleshare.Encode(name, []rules.Group{{Name: "a", Enabled: true, Rules: rs[:1]}, {Name: "b", Enabled: true, Rules: rs[1:]}})
	require.NoError(t, err)
	return link
}

func proxyRule(name, domain, server string) rules.Rule {
	return rules.Rule{Name: name, Enabled: true, Action: rules.ActionProxy, ServerName: server,
		Conditions: rules.Conditions{Domains: []rules.DomainMatcher{{Kind: "suffix", Value: domain}}}}
}

func directRule(name, domain string) rules.Rule {
	return rules.Rule{Name: name, Enabled: true, Action: rules.ActionDirect,
		Conditions: rules.Conditions{Domains: []rules.DomainMatcher{{Kind: "suffix", Value: domain}}}}
}

func sourceGroup(t *testing.T, store *rules.Store, subID string) *rules.Group {
	t.Helper()
	m, err := store.Load()
	require.NoError(t, err)
	for i := range m.Groups {
		if m.Groups[i].SourceID == subID {
			return &m.Groups[i]
		}
	}
	return nil
}

func TestApplyProviderRouting_CreatesLockedGroupBelowUserGroups(t *testing.T) {
	svc, store, h := newRulesService(t)
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	_, err := svc.GroupAdd("Mine")
	require.NoError(t, err)
	waitForRulesChanged(t, rcv, time.Second)

	link := providerLink(t, "Provider rules", proxyRule("YT", "youtube.com", "NL 2"), directRule("RU", "ya.ru"))
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "My VPN", link))
	waitForRulesChanged(t, rcv, time.Second)

	m, err := store.Load()
	require.NoError(t, err)
	last := m.Groups[len(m.Groups)-1]
	require.Equal(t, "sub-1", last.SourceID)
	require.Equal(t, "Provider rules", last.Name)
	require.True(t, last.Locked)
	require.True(t, last.Enabled)
	require.Len(t, last.Rules, 2)
	yt := last.Rules[0]
	require.Equal(t, "NL 2", yt.ServerName)
	require.Equal(t, "sub-1", yt.ServerSourceID)
	require.Empty(t, yt.ServerID)
	require.NotEmpty(t, yt.ID)
	require.NotEqual(t, yt.ID, last.Rules[1].ID)
	require.Empty(t, last.Rules[1].ServerSourceID)
}

func TestApplyProviderRouting_FallsBackToSubscriptionName(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "My VPN", providerLink(t, "", directRule("RU", "ya.ru"), directRule("B", "b.ru"))))
	require.Equal(t, "My VPN", sourceGroup(t, store, "sub-1").Name)
}

func TestApplyProviderRouting_ResyncReplacesRulesAndKeepsUserToggle(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", directRule("A", "a.ru"), directRule("B", "b.ru"))))
	g := sourceGroup(t, store, "sub-1")
	require.NoError(t, svc.GroupEdit(g.ID, "renamed", false))
	firstID := g.Rules[0].ID

	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", directRule("C", "c.ru"), directRule("D", "d.ru"))))
	g2 := sourceGroup(t, store, "sub-1")
	require.Equal(t, g.ID, g2.ID)
	require.False(t, g2.Enabled)
	require.Equal(t, "P", g2.Name)
	require.Equal(t, "C", g2.Rules[0].Name)
	require.Equal(t, firstID, g2.Rules[0].ID)
}

func TestApplyProviderRouting_EmptyLinkRemovesGroup(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", directRule("A", "a.ru"), directRule("B", "b.ru"))))
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", ""))
	require.Nil(t, sourceGroup(t, store, "sub-1"))
}

func TestApplyProviderRouting_BadLinkKeepsPreviousRules(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", directRule("A", "a.ru"), directRule("B", "b.ru"))))
	require.Error(t, svc.ApplyProviderRouting("sub-1", "VPN", "itgray://rules/import/!!!"))
	require.Len(t, sourceGroup(t, store, "sub-1").Rules, 2)
}

func TestApplyProviderRouting_UnchangedLinkDoesNotPublish(t *testing.T) {
	svc, _, h := newRulesService(t)
	link := providerLink(t, "P", directRule("A", "a.ru"), directRule("B", "b.ru"))
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", link))
	rcv := h.Subscribe(4)
	defer h.Unsubscribe(rcv)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", link))
	select {
	case e := <-rcv:
		t.Fatalf("unexpected event %s", e.Name)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestProviderGroup_RulesAreReadOnlyButGroupToggles(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", directRule("A", "a.ru"), directRule("B", "b.ru"))))
	g := sourceGroup(t, store, "sub-1")
	require.ErrorIs(t, svc.RuleToggle(g.Rules[0].ID), errLockedGroup)
	require.ErrorIs(t, svc.RuleRemove(g.Rules[0].ID), errLockedGroup)
	require.ErrorIs(t, svc.GroupRemove(g.ID), errLockedGroup)
	_, err := svc.RuleAdd(g.ID, directRule("X", "x.ru"))
	require.ErrorIs(t, err, errLockedGroup)
	_, err = svc.ExportGroup(g.ID)
	require.ErrorIs(t, err, errLockedGroup)

	require.NoError(t, svc.GroupEdit(g.ID, "ignored", false))
	g2 := sourceGroup(t, store, "sub-1")
	require.False(t, g2.Enabled)
	require.Equal(t, "P", g2.Name)
}

func TestReplaceAll_CannotTouchProviderGroups(t *testing.T) {
	svc, store, _ := newRulesService(t)
	require.NoError(t, svc.ApplyProviderRouting("sub-1", "VPN", providerLink(t, "P", proxyRule("A", "a.ru", "NL"), directRule("B", "b.ru"))))
	m, err := store.Load()
	require.NoError(t, err)
	var next rules.Model
	next.DefaultAction = m.DefaultAction
	for _, g := range m.Groups {
		if g.SourceID != "" {
			continue
		}
		next.Groups = append(next.Groups, g)
	}
	evil := rules.Group{ID: "user", Name: "Mine", Enabled: true, Rules: []rules.Rule{
		{ID: "r-evil", Name: "evil", Enabled: true, Action: rules.ActionProxy, ServerID: "other", Conditions: rules.Conditions{Processes: []string{"x"}}},
	}}
	next.Groups[1] = evil
	next.Groups = append(next.Groups, rules.Group{ID: "src-forged", Name: "forged", SourceID: "sub-1", Enabled: true,
		Rules: []rules.Rule{{ID: "f", Name: "f", Enabled: true, Action: rules.ActionBlock, Conditions: rules.Conditions{Processes: []string{"y"}}}}})
	require.NoError(t, svc.ReplaceAll(next))

	got, err := store.Load()
	require.NoError(t, err)
	var sources []string
	for _, g := range got.Groups {
		if g.SourceID != "" {
			sources = append(sources, g.ID)
		}
	}
	require.Len(t, sources, 1)
	g := sourceGroup(t, store, "sub-1")
	require.Equal(t, "P", g.Name)
	require.Len(t, g.Rules, 2)
}

func TestImportApply_KeepsServerNameForLaterBinding(t *testing.T) {
	svc, store, _ := newRulesService(t)
	link := providerLink(t, "Shared", proxyRule("YT", "youtube.com", "NL 2"), directRule("RU", "ya.ru"))
	require.NoError(t, svc.ImportApply(link))
	m, err := store.Load()
	require.NoError(t, err)
	imported := m.Groups[2].Rules[0]
	require.Equal(t, "NL 2", imported.ServerName)
	require.Empty(t, imported.ServerID)
	require.Empty(t, imported.ServerSourceID)
	require.Empty(t, m.Groups[2].SourceID)
}
