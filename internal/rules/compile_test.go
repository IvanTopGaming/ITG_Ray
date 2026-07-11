package rules

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompile_ProduceExpectedSingboxRules(t *testing.T) {
	m := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{
			{
				ID: "g1", Name: "My", Enabled: true,
				Rules: []Rule{
					{
						ID: "r1", Name: "Steam", Enabled: true, Action: ActionDirect,
						Conditions: Conditions{
							Processes: []string{"steam.exe"},
							Ports:     []PortSpec{{From: 27015, To: 27030}, {Single: 443}},
						},
					},
					{
						ID: "r2", Name: "Ads", Enabled: true, Action: ActionBlock,
						Conditions: Conditions{Geo: []string{"geosite:category-ads-all"}},
					},
					{
						ID: "r3", Name: "Disabled", Enabled: false, Action: ActionProxy,
						Conditions: Conditions{Domains: []DomainMatcher{{Kind: "suffix", Value: "example.com"}}},
					},
				},
			},
		},
	}
	out, err := Compile(m)
	require.NoError(t, err)

	var doc struct {
		Rules []map[string]any `json:"rules"`
		Final string           `json:"final"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Equal(t, "proxy", doc.Final)
	require.Len(t, doc.Rules, 2) // disabled rule skipped

	r0 := doc.Rules[0]
	require.Equal(t, "direct", r0["outbound"])
	require.Equal(t, []any{"steam.exe"}, r0["process_name"])
	require.Equal(t, []any{float64(443)}, r0["port"])
	require.Equal(t, []any{"27015:27030"}, r0["port_range"])

	r1 := doc.Rules[1]
	require.Equal(t, "block", r1["outbound"])
	require.Equal(t, []any{"geosite-category-ads-all"}, r1["rule_set"])
}

func TestCompile_RejectsInvalidModel(t *testing.T) {
	m := Model{DefaultAction: Action("nope")}
	_, err := Compile(m)
	require.Error(t, err)
}

func TestCompile_Domains(t *testing.T) {
	m := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{{ID: "g", Enabled: true, Rules: []Rule{
			{
				ID: "r", Enabled: true, Action: ActionProxy,
				Conditions: Conditions{Domains: []DomainMatcher{
					{Kind: "exact", Value: "a.com"},
					{Kind: "suffix", Value: "b.com"},
					{Kind: "keyword", Value: "cnn"},
					{Kind: "regex", Value: "^ads-.*"},
				}},
			},
		}}},
	}
	out, err := Compile(m)
	require.NoError(t, err)

	var doc struct {
		Rules []map[string]any `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	r := doc.Rules[0]
	require.Equal(t, []any{"a.com"}, r["domain"])
	require.Equal(t, []any{"b.com"}, r["domain_suffix"])
	require.Equal(t, []any{"cnn"}, r["domain_keyword"])
	require.Equal(t, []any{"^ads-.*"}, r["domain_regex"])
}

func TestCompile_GeoIPEmitsPrefixedRuleSet(t *testing.T) {
	m := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{{ID: "g", Enabled: true, Rules: []Rule{
			{ID: "r", Enabled: true, Action: ActionDirect,
				Conditions: Conditions{Geo: []string{"geoip:ru", "geoip:private"}}},
		}}},
	}
	out, err := Compile(m)
	require.NoError(t, err)
	var doc struct{ Rules []map[string]any }
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Equal(t, []any{"geoip-ru", "geoip-private"}, doc.Rules[0]["rule_set"])
	_, ok := doc.Rules[0]["geoip"]
	require.False(t, ok)
}

func TestCompile_GeoEmitsPrefixedRuleSetTags(t *testing.T) {
	m := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{{
			ID: "g", Enabled: true,
			Rules: []Rule{{
				ID: "r", Enabled: true, Action: ActionDirect,
				Conditions: Conditions{Geo: []string{"geosite:ru", "geoip:ru"}},
			}},
		}},
	}
	raw, err := Compile(m)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	var doc struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(doc.Rules))
	}
	r := doc.Rules[0]
	if _, ok := r["geoip"]; ok {
		t.Fatal("legacy geoip field must be gone")
	}
	got, _ := json.Marshal(r["rule_set"])
	if string(got) != `["geosite-ru","geoip-ru"]` {
		t.Fatalf(`rule_set = %s, want ["geosite-ru","geoip-ru"]`, got)
	}
}

func TestCompile_GroupWithOnlyDisabledRulesEmitsNothing(t *testing.T) {
	m := Model{
		DefaultAction: ActionProxy,
		Groups: []Group{
			{ID: "g1", Enabled: true, Rules: []Rule{
				{ID: "r1", Enabled: false, Action: ActionDirect,
					Conditions: Conditions{IPCIDRs: []string{"10.0.0.0/8"}}},
			}},
		},
	}
	out, err := Compile(m)
	require.NoError(t, err)
	var doc struct {
		Rules []map[string]any `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Empty(t, doc.Rules)
}
