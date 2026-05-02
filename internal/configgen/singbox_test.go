package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

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

	// Find fakeip server.
	var foundFakeIP bool
	for _, s := range servers {
		m := s.(map[string]any)
		if m["address"] == "fakeip" {
			foundFakeIP = true
			break
		}
	}
	require.True(t, foundFakeIP, "dns.servers must include a fakeip server")

	// Find remote server with detour=proxy (so DNS leaks via tunnel, not LAN).
	var foundRemote bool
	for _, s := range servers {
		m := s.(map[string]any)
		if m["detour"] == "proxy" {
			foundRemote = true
			break
		}
	}
	require.True(t, foundRemote, "dns.servers must include a server with detour=proxy")

	// FakeIP block.
	fakeip := dns["fakeip"].(map[string]any)
	require.Equal(t, true, fakeip["enabled"])
	require.Equal(t, "198.18.0.0/15", fakeip["inet4_range"])

	// independent_cache must be true (sing-box requires it for FakeIP).
	require.Equal(t, true, dns["independent_cache"])
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
	require.GreaterOrEqual(t, len(rs), 2)
	// First rule must be the fakeip rule (sing-box is first-match).
	first := rs[0].(map[string]any)
	require.Equal(t, "fakeip", first["server"], "fakeip rule must come before remote rule")
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
	require.Nil(t, dns["fakeip"], "sysproxy mode must not emit fakeip block")
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
