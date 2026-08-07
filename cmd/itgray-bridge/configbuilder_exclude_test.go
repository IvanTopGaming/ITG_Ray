package main

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

// tunExcludeAddress builds the sing-box JSON for the given mode via
// buildConfigs and returns the tun inbound's route_exclude_address (or nil
// when absent). Uses an IP-literal server address so resolveServerIPv4 is
// deterministic (LookupIP returns the literal as-is, no DNS).
func tunExcludeAddress(t *testing.T, mode chainctl.Mode) (any, bool) {
	t.Helper()
	srv := server.New(vless.Config{
		Address: "203.0.113.7",
		Port:    443,
		UUID:    "00000000-0000-0000-0000-000000000000",
	}, server.OriginManual, "")
	net := config.Network{
		Mode:     string(mode),
		TUN:      config.TUN{IPv4CIDR: "198.18.0.1/15", MTU: 1500},
		SysProxy: config.SysProxy{HTTPPort: 8888, SOCKSPort: 1080},
		IPv6Mode: "prefer-v4",
	}
	build := buildConfigs(t.TempDir(), "", nil, nil)
	sbJSON, _, err := build(&srv, mode, net)
	if err != nil {
		t.Fatalf("buildConfigs(%s): %v", mode, err)
	}
	var doc struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(sbJSON, &doc); err != nil {
		t.Fatalf("unmarshal singbox JSON: %v", err)
	}
	for _, ib := range doc.Inbounds {
		if ib["type"] == "tun" {
			v, ok := ib["route_exclude_address"]
			return v, ok
		}
	}
	// No tun inbound (e.g. SysProxy mode) → no exclude.
	return nil, false
}

// excludeSet decodes a tun route_exclude_address value (a JSON array of CIDR
// strings) into a set for order-independent membership assertions.
func excludeSet(t *testing.T, v any) map[string]bool {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal route_exclude_address: %v", err)
	}
	var cidrs []string
	if err := json.Unmarshal(raw, &cidrs); err != nil {
		t.Fatalf("route_exclude_address is not a string array: %s", raw)
	}
	set := make(map[string]bool, len(cidrs))
	for _, c := range cidrs {
		set[c] = true
	}
	return set
}

func TestBuildConfigs_TUN_SetsServerExclude(t *testing.T) {
	v, ok := tunExcludeAddress(t, chainctl.ModeTUN)

	// On Windows serverExcludeForTUN deliberately returns nil: the server is
	// kept out of the tunnel via the helper's route table, not the sing-box
	// inbound. So route_exclude_address must be present only on non-Windows.
	if runtime.GOOS == "windows" {
		if ok {
			t.Fatalf("route_exclude_address should be absent on Windows, got %v", v)
		}
		return
	}

	if !ok {
		t.Fatal("expected route_exclude_address present for ModeTUN")
	}
	got := excludeSet(t, v)

	// The VPN server's own IP must stay out of the tunnel (otherwise the
	// entry-node handshake would loop back through the proxied path).
	if !got["203.0.113.7/32"] {
		t.Errorf("route_exclude_address missing server /32; got %v", got)
	}

	// RFC1918 + IPv4 link-local must be excluded so the OS routes LAN traffic
	// over the physical NIC instead of the TUN. Without this, reply packets
	// from host-local services (SSH, samba, web) to LAN clients get pulled
	// into the TUN and dropped by gvisor (src != tun addr) — incoming LAN
	// connections time out while the tunnel is up. See bug #8.
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
	} {
		if !got[cidr] {
			t.Errorf("route_exclude_address missing LAN range %s; got %v", cidr, got)
		}
	}
}

func TestBuildConfigs_SysProxy_NoExclude(t *testing.T) {
	if _, ok := tunExcludeAddress(t, chainctl.ModeSysProxy); ok {
		t.Fatal("route_exclude_address must be absent for ModeSysProxy (no tun inbound)")
	}
}

func TestBuildConfigs_TUN_DualStackAddress(t *testing.T) {
	srv := server.New(vless.Config{
		Address: "203.0.113.7",
		Port:    443,
		UUID:    "00000000-0000-0000-0000-000000000000",
	}, server.OriginManual, "")
	net := config.Network{
		Mode:     string(chainctl.ModeTUN),
		TUN:      config.TUN{IPv4CIDR: "198.18.0.1/15", IPv6CIDR: "fdfe:dcba:9876::1/126", MTU: 1500},
		SysProxy: config.SysProxy{HTTPPort: 8888, SOCKSPort: 1080},
		IPv6Mode: "prefer-v4",
	}
	build := buildConfigs(t.TempDir(), "", nil, nil)
	sbJSON, _, err := build(&srv, chainctl.ModeTUN, net)
	if err != nil {
		t.Fatalf("buildConfigs: %v", err)
	}
	var doc struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(sbJSON, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, ib := range doc.Inbounds {
		if ib["type"] == "tun" {
			addr, _ := json.Marshal(ib["address"])
			if string(addr) != `["198.18.0.1/15","fdfe:dcba:9876::1/126"]` {
				t.Fatalf("tun address = %s, want dual-stack", addr)
			}
			return
		}
	}
	t.Fatal("no tun inbound found")
}
