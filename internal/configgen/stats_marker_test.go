package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func markedModel() rules.Model {
	m := rules.DefaultModel()
	m.Groups = append(m.Groups, rules.Group{ID: "u", Name: "u", Enabled: true, Rules: []rules.Rule{
		{ID: "ads", Enabled: true, Action: rules.ActionBlock, Conditions: rules.Conditions{Geo: []string{"geosite:category-ads-all"}}},
		{ID: "ru", Enabled: true, Action: rules.ActionDirect, Conditions: rules.Conditions{Geo: []string{"geoip:ru"}}},
		{ID: "off", Enabled: false, Action: rules.ActionBlock, Conditions: rules.Conditions{Processes: []string{"x"}}},
	}})
	return m
}

func markedInput() *SingboxInput {
	in := tunFakeIP(markedModel())
	in.GeoRuleSets = map[string]string{"geosite-category-ads-all": "/x/ads.srs", "geoip-ru": "/x/ru.srs"}
	in.MarkRules = true
	return in
}

func TestBuildSingbox_MarkRulesDeclaresBlockOutboundsAndFindProcess(t *testing.T) {
	raw, err := BuildSingbox(markedInput())
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	tags := map[string]string{}
	for _, o := range doc["outbounds"].([]any) {
		m := o.(map[string]any)
		tags[m["tag"].(string)] = m["type"].(string)
	}
	require.Equal(t, "block", tags["block-ads"])
	_, disabled := tags["block-off"]
	require.False(t, disabled)
	require.Equal(t, true, doc["route"].(map[string]any)["find_process"])
}

func TestBuildSingbox_MarkRulesKeepsRuleSetsAndFakeIPDNS(t *testing.T) {
	raw, err := BuildSingbox(markedInput())
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	var sets []string
	for _, s := range doc["route"].(map[string]any)["rule_set"].([]any) {
		sets = append(sets, s.(map[string]any)["tag"].(string))
	}
	require.ElementsMatch(t, []string{"geosite-category-ads-all", "geoip-ru"}, sets)
	rs := dnsRulesOf(t, markedInput())
	require.Equal(t, "evaluate", rs[0]["action"])
}

func TestBuildSingbox_UnmarkedHasNoFindProcess(t *testing.T) {
	in := markedInput()
	in.MarkRules = false
	raw, err := BuildSingbox(in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	_, ok := doc["route"].(map[string]any)["find_process"]
	require.False(t, ok)
}

func TestBuildSingbox_MarkedSafetyLanRuleIsNotDuplicated(t *testing.T) {
	in := &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules: rules.DefaultModel(), MarkRules: true,
	}
	raw, err := BuildSingbox(in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	var safetyID string
	for _, g := range rules.DefaultModel().Groups {
		if g.Locked {
			safetyID = g.Rules[0].ID
		}
	}
	firstLan := -1
	var firstLanRule map[string]any
	for i, r := range doc["route"].(map[string]any)["rules"].([]any) {
		m := r.(map[string]any)
		cidrs, _ := m["ip_cidr"].([]any)
		if nested, ok := m["rules"].([]any); ok {
			cidrs, _ = nested[0].(map[string]any)["ip_cidr"].([]any)
		}
		for _, c := range cidrs {
			if c == "192.168.0.0/16" {
				firstLan = i
				firstLanRule = m
				break
			}
		}
		if firstLan >= 0 {
			break
		}
	}
	require.True(t, firstLan >= 0)
	require.Equal(t, "logical", firstLanRule["type"], "the user's Safety rule must be the one that matches LAN, got %v", firstLanRule)
	nested := firstLanRule["rules"].([]any)
	require.Equal(t, []any{rules.MarkerInbound(safetyID)}, nested[1].(map[string]any)["inbound"])
}
