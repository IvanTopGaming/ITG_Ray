package main

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// TestBuildConfigs_ThreadsNetworkValues constructs the closure with a
// temp dataDir, calls it with a non-default config.Network, parses the
// emitted singbox JSON and asserts the user-configured values land on
// the expected keys (no end-to-end runtime; just JSON shape).
func TestBuildConfigs_ThreadsNetworkValues_TUN(t *testing.T) {
	dir := t.TempDir()
	srv := &server.Server{
		ID: "s1",
		Vless: vless.Config{
			Address:    "1.2.3.4",
			Port:       443,
			UUID:       "00000000-0000-0000-0000-000000000000",
			Encryption: "none",
			Transport:  vless.TransportTCP,
			Security:   vless.SecurityNone,
		},
	}
	net := config.Defaults().Network
	net.TUN.IPv4CIDR = "10.99.0.1/24"
	net.TUN.MTU = 1400
	net.AllowLAN = true
	net.IPv6Mode = "prefer-v6"
	net.DNS = config.DNS{Mode: "custom", Servers: []string{"9.9.9.9"}}

	build := buildConfigs(dir)
	sb, _, err := build(srv, chainctl.ModeTUN, net)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(sb, &doc))
	inbounds := doc["inbounds"].([]any)
	tun := inbounds[0].(map[string]any)
	require.EqualValues(t, 1400, tun["mtu"])
	require.Equal(t, []any{"10.99.0.1/24"}, tun["address"])

	dns := doc["dns"].(map[string]any)
	require.Equal(t, "prefer_ipv6", dns["strategy"])
	servers := dns["servers"].([]any)
	remote := servers[0].(map[string]any)
	require.Equal(t, "9.9.9.9", remote["address"])

	route := doc["route"].(map[string]any)
	rules := route["rules"].([]any)
	// AllowLAN=true in TUN mode → ordering is
	//   [sniff, hijack-dns, lanBypass, ...userRules, catch-all proxy].
	// The LAN bypass rule (the AllowLAN signal we're verifying) sits at
	// index 2, after the sniff action and the TUN-mode DNS hijack rule.
	lanRule := rules[2].(map[string]any)
	cidrs := lanRule["ip_cidr"].([]any)
	require.Contains(t, cidrs, "10.0.0.0/8")
}

func TestBuildConfigs_ThreadsNetworkValues_SysProxy(t *testing.T) {
	dir := t.TempDir()
	srv := &server.Server{
		ID: "s1",
		Vless: vless.Config{
			Address:    "1.2.3.4",
			Port:       443,
			UUID:       "00000000-0000-0000-0000-000000000000",
			Encryption: "none",
			Transport:  vless.TransportTCP,
			Security:   vless.SecurityNone,
		},
	}
	net := config.Defaults().Network
	net.SysProxy.SOCKSPort = 1090
	net.SysProxy.HTTPPort = 8889

	build := buildConfigs(dir)
	sb, _, err := build(srv, chainctl.ModeSysProxy, net)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(sb, &doc))
	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 2, "expected socks + http inbounds")
	socks := inbounds[0].(map[string]any)
	require.EqualValues(t, 1090, socks["listen_port"])
	require.Equal(t, "socks", socks["type"])
	httpIn := inbounds[1].(map[string]any)
	require.EqualValues(t, 8889, httpIn["listen_port"])
	require.Equal(t, "http", httpIn["type"])
}
