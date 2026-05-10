package chainctl

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/config"

	"github.com/stretchr/testify/require"
)

func TestClampMTU(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{in: 0, want: 0},       // OS default sentinel
		{in: 1500, want: 1500}, // typical Ethernet
		{in: 576, want: 576},   // lower bound
		{in: 9000, want: 9000}, // upper bound (jumbo)
		{in: 575, want: 0},     // below lower → OS default
		{in: 9001, want: 0},    // above upper → OS default
		{in: -1, want: 0},      // negative → OS default
	}
	for _, tc := range cases {
		got := ClampMTU(tc.in)
		require.Equal(t, tc.want, got, "ClampMTU(%d)", tc.in)
	}
}

func TestResolveDNS(t *testing.T) {
	cases := []struct {
		name string
		in   config.DNS
		want []string
	}{
		{name: "auto returns defaults", in: config.DNS{Mode: "auto"}, want: []string{"1.1.1.1", "8.8.8.8"}},
		{name: "custom with servers", in: config.DNS{Mode: "custom", Servers: []string{"9.9.9.9", "1.0.0.1"}}, want: []string{"9.9.9.9", "1.0.0.1"}},
		{name: "custom empty falls back", in: config.DNS{Mode: "custom", Servers: nil}, want: []string{"1.1.1.1", "8.8.8.8"}},
		{name: "unknown mode falls back", in: config.DNS{Mode: "weird"}, want: []string{"1.1.1.1", "8.8.8.8"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveDNS(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestResolveDNS_DefensiveCopy guards the no-aliasing property: mutating a
// returned slice must not corrupt subsequent calls. Tests both code paths
// (auto fallback + custom Servers) so a future regression in either branch
// fails immediately.
func TestResolveDNS_DefensiveCopy(t *testing.T) {
	// Auto/fallback path
	got1 := ResolveDNS(config.DNS{Mode: "auto"})
	got1[0] = "mutated"
	got2 := ResolveDNS(config.DNS{Mode: "auto"})
	require.Equal(t, "1.1.1.1", got2[0], "auto path must return a fresh slice each call")

	// Custom path: also assert mutation through the returned slice does not
	// reach back into the caller's input slice (full isolation in both
	// directions).
	servers := []string{"9.9.9.9", "1.0.0.1"}
	got3 := ResolveDNS(config.DNS{Mode: "custom", Servers: servers})
	got3[0] = "mutated"
	require.Equal(t, "9.9.9.9", servers[0], "caller's input slice must not be aliased by the return")
}

func TestMapIPv6Strategy(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "prefer-v4", want: "prefer_ipv4"},
		{in: "prefer-v6", want: "prefer_ipv6"},
		{in: "disabled", want: "ipv4_only"},
		{in: "", want: "prefer_ipv4"},      // empty → default
		{in: "weird", want: "prefer_ipv4"}, // unknown → default
	}
	for _, tc := range cases {
		got := MapIPv6Strategy(tc.in)
		require.Equal(t, tc.want, got, "MapIPv6Strategy(%q)", tc.in)
	}
}

func TestDefaultNetworkLoader(t *testing.T) {
	loader := DefaultNetworkLoader()
	net, err := loader()
	require.NoError(t, err)
	require.Equal(t, config.Defaults().Network, net)
}

// TestNetworkSettingsView_AllKeys pins all 9 camelCase keys emitted on the
// vpn:status connected payload. A typo in any key (e.g. tunMtu → tunMTU)
// would silently mismatch the frontend; this test catches it at unit time.
func TestNetworkSettingsView_AllKeys(t *testing.T) {
	expire := []string{"1.1.1.1", "8.8.8.8"}
	n := config.Network{
		Mode:     "tun",
		TUN:      config.TUN{IPv4CIDR: "10.99.0.1/24", MTU: 1400},
		SysProxy: config.SysProxy{HTTPPort: 8889, SOCKSPort: 1090},
		AllowLAN: true,
		IPv6Mode: "prefer-v6",
		DNS:      config.DNS{Mode: "custom", Servers: expire},
	}
	view := networkSettingsView(n)

	require.Equal(t, "tun", view["defaultMode"])
	require.Equal(t, "10.99.0.1/24", view["tunCidr"])
	require.Equal(t, 1400, view["tunMtu"])
	require.Equal(t, "ITGRay-TUN", view["tunName"])
	require.Equal(t, 1090, view["socksPort"])
	require.Equal(t, 8889, view["httpPort"])
	require.Equal(t, true, view["allowLan"])
	require.Equal(t, "prefer-v6", view["ipv6Mode"])

	dns := view["dns"].(map[string]any)
	require.Equal(t, "custom", dns["mode"])
	require.Equal(t, expire, dns["servers"])
}

// TestNetworkSettingsView_NormalizesLegacyAutoMode verifies the legacy
// pre-Tier-2a "auto" on-disk sentinel is collapsed to "tun" via the
// shared config.Network.EffectiveMode helper, keeping the vpn:status
// projection consistent with bindings.ConfigStore.toView.
func TestNetworkSettingsView_NormalizesLegacyAutoMode(t *testing.T) {
	n := config.Network{Mode: "auto"} // pre-Tier-2a legacy on-disk shape
	view := networkSettingsView(n)
	require.Equal(t, "tun", view["defaultMode"], "auto must normalize to tun")
}
