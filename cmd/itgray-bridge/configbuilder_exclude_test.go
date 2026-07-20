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
	got, _ := json.Marshal(v)
	if string(got) != `["203.0.113.7/32"]` {
		t.Fatalf("route_exclude_address = %s, want [\"203.0.113.7/32\"]", got)
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
