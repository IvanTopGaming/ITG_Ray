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
