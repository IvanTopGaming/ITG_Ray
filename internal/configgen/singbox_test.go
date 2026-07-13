package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func TestBuildSingbox_LogLevel(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantLvl string
	}{
		{"explicit debug", "debug", "debug"},
		{"explicit trace", "trace", "trace"},
		{"empty falls back to info", "", "info"},
		{"unknown falls back to info", "verbose", "info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := SingboxInput{
				SocksInboundPort: 1080,
				XraySOCKSHost:    "127.0.0.1",
				XraySOCKSPort:    1081,
				LogLevel:         tc.input,
				Rules:            rules.Model{DefaultAction: rules.ActionProxy},
			}
			b, err := BuildSingbox(&in)
			require.NoError(t, err)
			var doc map[string]any
			require.NoError(t, json.Unmarshal(b, &doc))
			logBlock := doc["log"].(map[string]any)
			require.Equal(t, tc.wantLvl, logBlock["level"])
		})
	}
}

func TestBuildSingbox_Minimal(t *testing.T) {
	in := SingboxInput{
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules: rules.Model{
			DefaultAction: rules.ActionProxy,
			Groups: []rules.Group{
				{
					ID: "g", Enabled: true, Rules: []rules.Rule{
						{
							ID: "r", Enabled: true, Action: rules.ActionDirect,
							Conditions: rules.Conditions{IPCIDRs: []string{"10.0.0.0/8"}},
						},
					},
				},
			},
		},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	require.Equal(t, "mixed", inbounds[0].(map[string]any)["type"])

	outbounds := doc["outbounds"].([]any)
	tags := map[string]bool{}
	for _, o := range outbounds {
		tags[o.(map[string]any)["tag"].(string)] = true
	}
	require.True(t, tags["proxy"])
	require.True(t, tags["direct"])
	require.True(t, tags["block"])

	rt := doc["route"].(map[string]any)
	require.Equal(t, "proxy", rt["final"])
}

func TestBuildSingbox_FakeIP(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	dns := doc["dns"].(map[string]any)
	servers := dns["servers"].([]any)
	require.GreaterOrEqual(t, len(servers), 2, "expect remote + fakeip servers")

	// 1.12+ schema: fakeip is a server type with inline inet4_range, NOT a
	// top-level dns.fakeip block (the legacy block is rejected with a
	// deprecation WARN and degraded hijack-dns behavior).
	var fakeipServer map[string]any
	var remoteServer map[string]any
	for _, s := range servers {
		m := s.(map[string]any)
		if m["type"] == "fakeip" {
			fakeipServer = m
		}
		if m["detour"] == "proxy" {
			remoteServer = m
		}
	}
	require.NotNil(t, fakeipServer, "dns.servers must include a server with type=fakeip")
	require.Equal(t, "198.18.0.0/15", fakeipServer["inet4_range"],
		"fakeip server must declare inet4_range inline (not in legacy dns.fakeip)")
	require.Nil(t, dns["fakeip"], "legacy top-level dns.fakeip block must not be emitted")

	require.NotNil(t, remoteServer, "dns.servers must include a server with detour=proxy")
	require.Equal(t, "tls", remoteServer["type"],
		"remote server must use DoT (type=tls) so DNS queries are encrypted to the upstream resolver — plain UDP/53 leaks domains in cleartext to the exit server's network")

	require.Equal(t, true, dns["independent_cache"], "sing-box requires independent_cache for FakeIP")

	// 1.12+: route.default_domain_resolver is mandatory-warned (hard-required in 1.14).
	route := doc["route"].(map[string]any)
	require.Equal(t, "remote", route["default_domain_resolver"],
		"route.default_domain_resolver must point to the proxy-detoured server in TUN+FakeIP")
}

func TestBuildSingbox_FakeIP_RuleOrder(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	dns := doc["dns"].(map[string]any)
	rs := dns["rules"].([]any)
	// 1.12+ schema: only the A/AAAA→fakeip rule is needed. The legacy
	// {outbound:any, server:remote} fallback is replaced by
	// route.default_domain_resolver.
	require.Len(t, rs, 1, "1.12+ DNS rules contain only A/AAAA→fakeip; fallback moved to route.default_domain_resolver")
	first := rs[0].(map[string]any)
	require.Equal(t, "fakeip", first["server"], "fakeip rule must dispatch A/AAAA to fakeip server")
	qts, ok := first["query_type"].([]any)
	require.True(t, ok)
	require.Contains(t, qts, "A")
	require.Contains(t, qts, "AAAA")
}

func TestBuildSingbox_NoFakeIPInSysProxy(t *testing.T) {
	// FakeIP only makes sense in TUN mode. Sys-proxy mode must NOT emit it.
	in := SingboxInput{
		Mode:             ModeSysProxy,
		FakeIP:           true, // even if requested
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	dns := doc["dns"].(map[string]any)
	require.Nil(t, dns["fakeip"], "legacy top-level dns.fakeip block must never be emitted in sysproxy")
	servers := dns["servers"].([]any)
	for _, s := range servers {
		m := s.(map[string]any)
		require.NotEqual(t, "fakeip", m["type"],
			"sysproxy mode must not emit a fakeip-typed server")
	}
}

func TestBuildSingbox_TunMode_KillswitchBlock(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	inbounds := doc["inbounds"].([]any)
	inbound := inbounds[0].(map[string]any)
	require.Equal(t, true, inbound["auto_route"],
		"TUN inbound must enable auto_route so sing-box installs catch-all routes + DNS hijack natively")

	route := doc["route"].(map[string]any)
	// sing-box's route schema names the default outbound "final"
	// (not "default_outbound" — the latter would fail library validation).
	require.Equal(t, "block", route["final"],
		"TUN mode default outbound must be block (killswitch)")
}

func TestBuildSingbox_TunMode_LANException(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	route := doc["route"].(map[string]any)
	rules := route["rules"].([]any)

	// Find a rule with ip_cidr including 192.168.0.0/16 routing to direct.
	var found bool
	for _, r := range rules {
		m := r.(map[string]any)
		if m["outbound"] != "direct" {
			continue
		}
		cidrs, ok := m["ip_cidr"].([]any)
		if !ok {
			continue
		}
		for _, c := range cidrs {
			if c == "192.168.0.0/16" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found, "TUN mode must have ip_cidr rule sending RFC1918 LAN to direct")
}

func TestBuildSingbox_SysProxy_DefaultOutboundIsProxy(t *testing.T) {
	// SysProxy mode keeps the legacy default-outbound=proxy behavior.
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	route := doc["route"].(map[string]any)
	// sing-box's route schema uses "final" (not "default_outbound") for
	// the default outbound; SysProxy mode is untouched by the killswitch
	// branch, so the value compiled from rules.Model.DefaultAction stands.
	require.Equal(t, "proxy", route["final"])
}

func TestBuildSingbox_TunMode_CatchAllProxyBeforeFinal(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	require.NotEmpty(t, rs)

	// Last rule should be a catch-all to proxy: just {outbound: "proxy"}
	last := rs[len(rs)-1].(map[string]any)
	require.Equal(t, "proxy", last["outbound"])
	// And it should NOT have any match conditions (catch-all).
	require.Nil(t, last["domain"])
	require.Nil(t, last["ip_cidr"])
	require.Nil(t, last["geosite"])
}

func TestBuildSingbox_TUNEmitsMTUWhenSet(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		MTU:           1400,
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	tun := inbounds[0].(map[string]any)
	require.EqualValues(t, 1400, tun["mtu"])
}

func TestBuildSingbox_TUNOmitsMTUWhenZero(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		MTU:           0,
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	inbounds := doc["inbounds"].([]any)
	tun := inbounds[0].(map[string]any)
	require.Equal(t, "tun", tun["type"])
	_, present := tun["mtu"]
	require.False(t, present, "mtu key must be absent when MTU=0")
}

func TestBuildSingbox_DualSysProxyInbounds_SocksAndHTTP(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		HTTPInboundPort:  8888,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 2, "expected socks + http inbounds")
	socks := inbounds[0].(map[string]any)
	httpIn := inbounds[1].(map[string]any)
	require.Equal(t, "socks", socks["type"])
	require.EqualValues(t, 1080, socks["listen_port"])
	require.Equal(t, "http", httpIn["type"])
	require.EqualValues(t, 8888, httpIn["listen_port"])
}

func TestBuildSingbox_FallbackMixed_WhenHTTPPortZero(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		HTTPInboundPort:  0, // CLI compat
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	mixed := inbounds[0].(map[string]any)
	require.Equal(t, "mixed", mixed["type"])
}

func TestBuildSingbox_FallbackMixed_WhenPortsCollide(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		HTTPInboundPort:  1080, // collision
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	mixed := inbounds[0].(map[string]any)
	require.Equal(t, "mixed", mixed["type"])
}

func TestBuildSingbox_AllowLAN_PrependsLanBypass(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
		AllowLAN:         true,
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	// Rule[0] is sniff action (existing), Rule[1] should be the LAN bypass
	require.GreaterOrEqual(t, len(rs), 2)
	rule1 := rs[1].(map[string]any)
	require.Equal(t, "direct", rule1["outbound"])
	cidrs, ok := rule1["ip_cidr"].([]any)
	require.True(t, ok, "ip_cidr should be present")
	require.Contains(t, cidrs, "10.0.0.0/8")
	require.Contains(t, cidrs, "192.168.0.0/16")
}

func TestBuildSingbox_AllowLAN_False_NoBypassPrepended(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
		AllowLAN:         false,
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	// Walk every rule; AllowLAN=false must not produce any direct+ip_cidr rule
	// (the LAN bypass shape). Robust against rule-order changes.
	for i, r := range rs {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if m["outbound"] == "direct" {
			_, hasIPCidr := m["ip_cidr"]
			require.False(t, hasIPCidr,
				"AllowLAN=false must not emit ip_cidr direct rule (found at rule index %d)", i)
		}
	}
}

func TestBuildSingbox_AllowLAN_TUN_NoDuplicateWithKillswitch(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
		AllowLAN:      true,
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	// Count rules with ip_cidr that includes "10.0.0.0/8"
	count := 0
	for _, r := range rs {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		cidrs, ok := m["ip_cidr"].([]any)
		if !ok {
			continue
		}
		for _, c := range cidrs {
			if c == "10.0.0.0/8" {
				count++
				break
			}
		}
	}
	require.Equal(t, 1, count, "expected exactly one LAN-bypass rule (TUN+killswitch+AllowLAN)")
}

// Production loads the default safety group when rules.json is absent (see
// cmd/itgray-bridge/configbuilder.go loadRulesFromDataDir). That model
// compiles to an {ip_cidr:[RFC1918+...], outbound:"direct"} rule. With TUN
// mode + AllowLAN=false the killswitch path previously emitted its own
// identical lanBypassRule(), producing two equivalent rules. Sing-box
// evaluates the duplicate as a no-op but the config is noisy. The
// killswitch must scan existing rules and skip the emit when an equivalent
// rule is already present.
func TestBuildSingbox_TUN_KillswitchDoesNotDuplicateSafetyGroupLanBypass(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		AllowLAN:      false,
		Rules: rules.Model{
			DefaultAction: rules.ActionProxy,
			Groups: []rules.Group{
				{ID: "safety", Name: "Safety", Locked: true, Enabled: true, Rules: []rules.Rule{
					{ID: "private", Name: "Private IPs", Enabled: true, Action: rules.ActionDirect,
						Conditions: rules.Conditions{IPCIDRs: []string{
							"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8",
							"fc00::/7", "fe80::/10", "224.0.0.0/4",
						}}},
				}},
				{ID: "user", Name: "My Rules", Enabled: true},
			},
		},
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	count := 0
	for _, r := range rs {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if m["outbound"] != "direct" {
			continue
		}
		cidrs, ok := m["ip_cidr"].([]any)
		if !ok {
			continue
		}
		for _, c := range cidrs {
			if c == "10.0.0.0/8" {
				count++
				break
			}
		}
	}
	require.Equal(t, 1, count,
		"safety-group already emits {ip_cidr:[RFC1918+...], outbound:direct} — killswitch must not emit a duplicate")
}

func TestBuildSingbox_IPv6Strategy_PreferV6(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
		IPv6Strategy:     "prefer_ipv6",
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	dns := doc["dns"].(map[string]any)
	require.Equal(t, "prefer_ipv6", dns["strategy"])
}

func TestBuildSingbox_IPv6Strategy_Disabled(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
		IPv6Strategy:     "ipv4_only",
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	dns := doc["dns"].(map[string]any)
	require.Equal(t, "ipv4_only", dns["strategy"])
}

func TestBuildSingbox_IPv6Strategy_EmptyFallsBackToPreferV4(t *testing.T) {
	in := SingboxInput{
		Mode:             ModeSysProxy,
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules:            rules.Model{DefaultAction: rules.ActionProxy},
		IPv6Strategy:     "",
	}
	out, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	dns := doc["dns"].(map[string]any)
	require.Equal(t, "prefer_ipv4", dns["strategy"])
}

func TestBuildSingbox_TunMode_HijackDNS(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	route := doc["route"].(map[string]any)
	rs := route["rules"].([]any)
	require.GreaterOrEqual(t, len(rs), 3, "expect sniff + hijack-dns + LAN minimum")

	// Index 0 must be sniff (B10.1/B5 carryover invariant)
	first := rs[0].(map[string]any)
	require.Equal(t, "sniff", first["action"])

	// Index 1 must be hijack-dns
	second := rs[1].(map[string]any)
	require.Equal(t, "dns", second["protocol"])
	require.Equal(t, "hijack-dns", second["action"])

	// Index 2 must be LAN-direct
	third := rs[2].(map[string]any)
	require.Equal(t, "direct", third["outbound"])
	cidrs := third["ip_cidr"].([]any)
	require.Contains(t, cidrs, "192.168.0.0/16")
}

func TestBuildSingbox_Fakeip_Inet6Range(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		TunIPv6:       "fdfe:dcba:9876::1/126",
		FakeIPv6Range: "fc00::/18",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	servers := doc["dns"].(map[string]any)["servers"].([]any)
	var fakeip map[string]any
	for _, s := range servers {
		m := s.(map[string]any)
		if m["type"] == "fakeip" {
			fakeip = m
		}
	}
	require.NotNil(t, fakeip)
	require.Equal(t, "198.18.0.0/15", fakeip["inet4_range"])
	require.Equal(t, "fc00::/18", fakeip["inet6_range"])
}

func TestBuildSingbox_Fakeip_NoInet6RangeWhenUnset(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	servers := doc["dns"].(map[string]any)["servers"].([]any)
	for _, s := range servers {
		m := s.(map[string]any)
		if m["type"] == "fakeip" {
			_, has := m["inet6_range"]
			require.False(t, has, "inet6_range must be absent when FakeIPv6Range is empty")
		}
	}
}

func TestBuildSingbox_DisabledMode_BlocksIPv6(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		TunIPv6:       "fdfe:dcba:9876::1/126",
		FakeIPv6Range: "", // disabled: capture but do not tunnel
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	routeRules := doc["route"].(map[string]any)["rules"].([]any)
	blockIdx, proxyIdx := -1, -1
	for i, r := range routeRules {
		m := r.(map[string]any)
		if m["outbound"] == "block" {
			cidr, _ := json.Marshal(m["ip_cidr"])
			require.Equal(t, `["::/0"]`, string(cidr))
			blockIdx = i
		}
		if m["outbound"] == "proxy" {
			if _, hasCidr := m["ip_cidr"]; !hasCidr {
				proxyIdx = i // the catch-all
			}
		}
	}
	require.NotEqual(t, -1, blockIdx, "expected a ::/0 → block rule in disabled mode")
	require.NotEqual(t, -1, proxyIdx, "expected the proxy catch-all rule")
	require.Less(t, blockIdx, proxyIdx, "block rule must precede the proxy catch-all")
}

func TestBuildSingbox_TunnelMode_NoIPv6Block(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		FakeIP:        true,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		TunIPv6:       "fdfe:dcba:9876::1/126",
		FakeIPv6Range: "fc00::/18", // tunnelling mode
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	routeRules := doc["route"].(map[string]any)["rules"].([]any)
	for _, r := range routeRules {
		require.NotEqual(t, "block", r.(map[string]any)["outbound"],
			"tunnelling modes must not emit a ::/0 → block rule")
	}
}
