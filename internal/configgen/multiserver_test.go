package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func tcpServer(addr string) vless.Config {
	return vless.Config{Address: addr, Port: 443, UUID: "u-" + addr, Transport: vless.TransportTCP, Security: vless.SecurityTLS, SNI: addr}
}

func TestExtraTag(t *testing.T) {
	require.Equal(t, "proxy-abc", ExtraTag("abc"))
}

func TestBuildXray_ExtraServers(t *testing.T) {
	raw, err := BuildXray(&XrayInput{
		Server: tcpServer("main.example"), ServerIP: "1.1.1.1", SocksPort: 1081,
		Extra: []XrayServer{
			{Tag: ExtraTag("a"), Server: tcpServer("a.example"), ServerIP: "2.2.2.2", SocksPort: 1082},
			{Tag: ExtraTag("b"), Server: tcpServer("b.example"), ServerIP: "3.3.3.3", SocksPort: 1083},
		},
	})
	require.NoError(t, err)
	var doc struct {
		Inbounds  []map[string]any `json:"inbounds"`
		Outbounds []struct {
			Tag      string `json:"tag"`
			Settings struct {
				Vnext []struct {
					Address string `json:"address"`
				} `json:"vnext"`
			} `json:"settings"`
		} `json:"outbounds"`
		Routing struct {
			Rules []struct {
				InboundTag  []string `json:"inboundTag"`
				OutboundTag string   `json:"outboundTag"`
			} `json:"rules"`
		} `json:"routing"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Len(t, doc.Outbounds, 3)
	require.Equal(t, "proxy", doc.Outbounds[0].Tag)
	require.Equal(t, "1.1.1.1", doc.Outbounds[0].Settings.Vnext[0].Address)
	require.Equal(t, "proxy-a", doc.Outbounds[1].Tag)
	require.Equal(t, "2.2.2.2", doc.Outbounds[1].Settings.Vnext[0].Address)
	require.Equal(t, "3.3.3.3", doc.Outbounds[2].Settings.Vnext[0].Address)

	ports := map[string]float64{}
	for _, ib := range doc.Inbounds {
		ports[ib["tag"].(string)] = ib["port"].(float64)
	}
	require.Equal(t, float64(1081), ports["socks-in"])
	require.Equal(t, float64(1082), ports["socks-in-proxy-a"])
	require.Equal(t, float64(1083), ports["socks-in-proxy-b"])

	routes := map[string]string{}
	for _, r := range doc.Routing.Rules {
		routes[r.InboundTag[0]] = r.OutboundTag
	}
	require.Equal(t, "api", routes["api"])
	require.Equal(t, "proxy-a", routes["socks-in-proxy-a"])
	require.Equal(t, "proxy-b", routes["socks-in-proxy-b"])
}

func boundModel() rules.Model {
	return rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{{ID: "g", Name: "g", Enabled: true, Rules: []rules.Rule{
		{ID: "r1", Enabled: true, Action: rules.ActionProxy, ServerID: "a", Conditions: rules.Conditions{Processes: []string{"firefox"}}},
	}}}}
}

func routeOf(t *testing.T, raw []byte) (map[string]any, []map[string]any) {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	route := doc["route"].(map[string]any)
	var rs []map[string]any
	for _, r := range route["rules"].([]any) {
		rs = append(rs, r.(map[string]any))
	}
	return doc, rs
}

func indexWhere(rs []map[string]any, pred func(map[string]any) bool) int {
	for i, r := range rs {
		if pred(r) {
			return i
		}
	}
	return -1
}

func TestBuildSingbox_ExtraProxiesAndClashAPI(t *testing.T) {
	raw, err := BuildSingbox(&SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules:        boundModel(),
		ExtraProxies: []SingboxProxy{{Tag: "proxy-a", Port: 1082}},
		RuleOutbound: func(r *rules.Rule) string { return ExtraTag(r.ServerID) },
		ClashAPI:     &ClashAPI{Port: ClashAPIPort, Secret: "s3cret"},
	})
	require.NoError(t, err)
	doc, rs := routeOf(t, raw)

	var extra map[string]any
	for _, o := range doc["outbounds"].([]any) {
		if m := o.(map[string]any); m["tag"] == "proxy-a" {
			extra = m
		}
	}
	require.NotNil(t, extra)
	require.Equal(t, "socks", extra["type"])
	require.Equal(t, float64(1082), extra["server_port"])

	exp := doc["experimental"].(map[string]any)
	_, hasCache := exp["cache_file"]
	require.False(t, hasCache)
	api := exp["clash_api"].(map[string]any)
	require.Equal(t, "127.0.0.1:13356", api["external_controller"])
	require.Equal(t, "s3cret", api["secret"])
	require.Equal(t, "Rule", api["default_mode"])

	hijack := indexWhere(rs, func(r map[string]any) bool { return r["action"] == "hijack-dns" })
	direct := indexWhere(rs, func(r map[string]any) bool { return r["clash_mode"] == "Direct" })
	global := indexWhere(rs, func(r map[string]any) bool { return r["clash_mode"] == "Global" && r["ip_cidr"] == nil })
	user := indexWhere(rs, func(r map[string]any) bool { return r["process_name"] != nil })
	require.True(t, hijack >= 0 && direct > hijack && global > hijack, "clash rules after hijack-dns: %v", rs)
	require.True(t, user > direct && user > global, "clash rules before user rules: %v", rs)
	require.Equal(t, "direct", rs[direct]["outbound"])
	require.Equal(t, "proxy", rs[global]["outbound"])
	require.Equal(t, "proxy-a", rs[user]["outbound"])
}

func TestBuildSingbox_NoClashAPIByDefault(t *testing.T) {
	raw, err := BuildSingbox(&SingboxInput{Mode: ModeSysProxy, SocksInboundPort: 1080, XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081, Rules: rules.Model{DefaultAction: rules.ActionProxy}})
	require.NoError(t, err)
	doc, rs := routeOf(t, raw)
	_, ok := doc["experimental"]
	require.False(t, ok)
	require.Equal(t, -1, indexWhere(rs, func(r map[string]any) bool { return r["clash_mode"] != nil }))
}

func TestBuildSingbox_SysproxyExtraProxies(t *testing.T) {
	raw, err := BuildSingbox(&SingboxInput{
		Mode: ModeSysProxy, SocksInboundPort: 1080, XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules:        boundModel(),
		ExtraProxies: []SingboxProxy{{Tag: "proxy-a", Port: 1082}},
		RuleOutbound: func(r *rules.Rule) string { return ExtraTag(r.ServerID) },
		ClashAPI:     &ClashAPI{Port: ClashAPIPort, Secret: "x"},
	})
	require.NoError(t, err)
	doc, rs := routeOf(t, raw)
	tags := []string{}
	for _, o := range doc["outbounds"].([]any) {
		tags = append(tags, o.(map[string]any)["tag"].(string))
	}
	require.Contains(t, tags, "proxy-a")
	user := indexWhere(rs, func(r map[string]any) bool { return r["process_name"] != nil })
	require.Equal(t, "proxy-a", rs[user]["outbound"])
	require.True(t, indexWhere(rs, func(r map[string]any) bool { return r["clash_mode"] == "Global" }) < user)
}

func clashGlobalIndexes(rs []map[string]any) (lan, v6block, proxy int) {
	lan, v6block, proxy = -1, -1, -1
	for i, r := range rs {
		if r["clash_mode"] != "Global" {
			continue
		}
		cidrs, _ := r["ip_cidr"].([]any)
		switch {
		case r["outbound"] == "proxy" && cidrs == nil:
			proxy = i
		case r["outbound"] == "block" && len(cidrs) == 1 && cidrs[0] == "::/0":
			v6block = i
		case r["outbound"] == "direct":
			for _, c := range cidrs {
				if c == "192.168.0.0/16" {
					lan = i
				}
			}
		}
	}
	return
}

func TestBuildSingbox_GlobalModeKeepsLanBypassAndV6Block_DefaultModel(t *testing.T) {
	raw, err := BuildSingbox(&SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15", TunIPv6: "fdfe:dcba:9876::1/126",
		XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules:    rules.DefaultModel(),
		ClashAPI: &ClashAPI{Port: ClashAPIPort, Secret: "x"},
	})
	require.NoError(t, err)
	_, rs := routeOf(t, raw)
	lan, v6, proxy := clashGlobalIndexes(rs)
	require.True(t, lan >= 0 && lan < proxy, "LAN bypass must precede Global→proxy: %v", rs)
	require.True(t, v6 >= 0 && v6 < proxy, "::/0 block must precede Global→proxy when IPv6 is disabled: %v", rs)
}

func TestBuildSingbox_GlobalModeKeepsLanBypass_Sysproxy(t *testing.T) {
	raw, err := BuildSingbox(&SingboxInput{
		Mode: ModeSysProxy, SocksInboundPort: 1080, XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules:    rules.DefaultModel(),
		ClashAPI: &ClashAPI{Port: ClashAPIPort, Secret: "x"},
	})
	require.NoError(t, err)
	_, rs := routeOf(t, raw)
	lan, v6, proxy := clashGlobalIndexes(rs)
	require.True(t, lan >= 0 && lan < proxy, "LAN bypass must precede Global→proxy: %v", rs)
	require.Equal(t, -1, v6)
}

func TestBuildSingbox_RuleModeKeepsUnconditionalLanBypass_SafetyDisabled(t *testing.T) {
	m := rules.DefaultModel()
	for i := range m.Groups {
		m.Groups[i].Enabled = false
	}
	raw, err := BuildSingbox(&SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules:    m,
		ClashAPI: &ClashAPI{Port: ClashAPIPort, Secret: "x"},
	})
	require.NoError(t, err)
	_, rs := routeOf(t, raw)
	found := indexWhere(rs, func(r map[string]any) bool {
		_, conditional := r["clash_mode"]
		cidrs, _ := r["ip_cidr"].([]any)
		return !conditional && r["outbound"] == "direct" && len(cidrs) > 1
	})
	require.True(t, found >= 0, "unconditional LAN bypass missing: %v", rs)
}
