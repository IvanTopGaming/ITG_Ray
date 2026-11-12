package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func dnsRulesOf(t *testing.T, in *SingboxInput) []map[string]any {
	t.Helper()
	raw, err := BuildSingbox(in)
	require.NoError(t, err)
	var doc struct {
		DNS struct {
			Rules []map[string]any `json:"rules"`
		} `json:"dns"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc.DNS.Rules
}

func ipRuleModel(action rules.Action, serverID string) rules.Model {
	m := rules.DefaultModel()
	m.Groups = append(m.Groups, rules.Group{ID: "u", Name: "u", Enabled: true, Rules: []rules.Rule{
		{ID: "ads", Enabled: true, Action: rules.ActionBlock, Conditions: rules.Conditions{Geo: []string{"geosite:category-ads-all"}}},
		{ID: "ru", Enabled: true, Action: action, ServerID: serverID, Conditions: rules.Conditions{
			Geo:     []string{"geosite:category-ru", "geoip:ru"},
			IPCIDRs: []string{"185.224.11.73"},
		}},
	}})
	return m
}

func tunFakeIP(m rules.Model) *SingboxInput {
	return &SingboxInput{
		Mode: ModeTun, FakeIP: true, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081, Rules: m,
		GeoRuleSets: map[string]string{
			"geosite-category-ads-all": "/x/ads.srs", "geosite-category-ru": "/x/ru.srs", "geoip-ru": "/x/geoip-ru.srs",
		},
	}
}

func TestDNS_FakeIPResolvesForNonProxyIPRules(t *testing.T) {
	rs := dnsRulesOf(t, tunFakeIP(ipRuleModel(rules.ActionDirect, "")))
	require.Len(t, rs, 4, "%v", rs)
	require.Equal(t, "evaluate", rs[0]["action"])
	require.Equal(t, "remote", rs[0]["server"])
	require.Equal(t, []any{"A", "AAAA"}, rs[0]["query_type"])

	require.Equal(t, true, rs[1]["match_response"])
	require.Equal(t, []any{"geoip-ru"}, rs[1]["rule_set"])
	require.Equal(t, "local", rs[1]["server"])

	require.Equal(t, true, rs[2]["match_response"])
	require.Equal(t, []any{"185.224.11.73/32"}, rs[2]["ip_cidr"])
	require.Equal(t, "local", rs[2]["server"])

	require.Equal(t, "fakeip", rs[3]["server"])
}

func TestDNS_FakeIPUnchangedWithoutQualifyingIPRules(t *testing.T) {
	for name, m := range map[string]rules.Model{
		"default model (Safety only)": rules.DefaultModel(),
		"ip rule to main proxy":       ipRuleModel(rules.ActionProxy, ""),
	} {
		rs := dnsRulesOf(t, tunFakeIP(m))
		require.Len(t, rs, 1, name)
		require.Equal(t, "fakeip", rs[0]["server"], name)
	}
}

func TestDNS_FakeIPResolvesForIPRuleBoundToOtherServer(t *testing.T) {
	in := tunFakeIP(ipRuleModel(rules.ActionProxy, "x"))
	in.RuleOutbound = func(r *rules.Rule) string {
		if r.ServerID == "x" {
			return "proxy-x"
		}
		return ""
	}
	rs := dnsRulesOf(t, in)
	require.Equal(t, "evaluate", rs[0]["action"])
}

func TestDNS_DisabledIPRuleIgnored(t *testing.T) {
	m := ipRuleModel(rules.ActionDirect, "")
	m.Groups[len(m.Groups)-1].Rules[1].Enabled = false
	rs := dnsRulesOf(t, tunFakeIP(m))
	require.Len(t, rs, 1)
}

func TestDNS_NonDirectIPRulesNeverQueryLocal(t *testing.T) {
	for name, action := range map[string]rules.Action{"block": rules.ActionBlock, "other server": rules.ActionProxy} {
		in := tunFakeIP(ipRuleModel(action, "x"))
		in.RuleOutbound = func(r *rules.Rule) string {
			if r.ServerID == "x" {
				return "proxy-x"
			}
			return ""
		}
		rs := dnsRulesOf(t, in)
		for _, r := range rs {
			require.NotEqual(t, "local", r["server"], "%s: %v", name, rs)
		}
		require.Equal(t, "evaluate", rs[0]["action"], name)
		var responds int
		for _, r := range rs {
			if r["action"] == "respond" {
				require.Equal(t, true, r["match_response"], name)
				responds++
			}
		}
		require.Equal(t, 2, responds, "%s: %v", name, rs)
	}
}

func TestDNS_DirectBeforeRespondWhenBothPresent(t *testing.T) {
	m := ipRuleModel(rules.ActionDirect, "")
	m.Groups = append(m.Groups, rules.Group{ID: "b", Name: "b", Enabled: true, Rules: []rules.Rule{
		{ID: "blk", Enabled: true, Action: rules.ActionBlock, Conditions: rules.Conditions{IPCIDRs: []string{"203.0.113.0/24"}}},
	}})
	rs := dnsRulesOf(t, tunFakeIP(m))
	localAt, respondAt := -1, -1
	for i, r := range rs {
		if r["server"] == "local" && localAt < 0 {
			localAt = i
		}
		if r["action"] == "respond" && respondAt < 0 {
			respondAt = i
			require.Equal(t, []any{"203.0.113.0/24"}, r["ip_cidr"])
		}
	}
	require.True(t, localAt > 0 && respondAt > localAt, "%v", rs)
}
