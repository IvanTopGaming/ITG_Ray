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
