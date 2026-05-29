package chainctl

import (
	"log/slog"

	"github.com/itg-team/itg-ray/internal/config"
)

// ClampMTU returns mtu when 576 <= mtu <= 9000, otherwise 0 ("use OS
// default"). Tier 2b's defense in depth — the frontend and the
// configstore.applyNetwork patch already validate, but a hand-edited
// config.json can still land out-of-range values that wintun would
// reject mid-Connect.
func ClampMTU(mtu int) int {
	if mtu < 576 || mtu > 9000 {
		if mtu != 0 {
			slog.Info("chainctl: MTU out of [576,9000] range, falling back to OS default",
				slog.String("scope", "chainctl.network"), slog.Int("mtu", mtu))
		}
		return 0
	}
	return mtu
}

// ResolveDNS picks the DNS server list used by the runtime. Mode=="custom"
// uses the configured Servers (when non-empty); any other case falls back
// to ["1.1.1.1","8.8.8.8"] (Cloudflare + Google — mirrors the GUI's pre-
// Tier-2b defaults for backward compatibility). Tier 2b's runtime defense
// in depth: the frontend already gates Mode=="custom" on a non-empty
// Servers entry, but a hand-edited config.json could still arrive with
// Mode=="custom" and an empty Servers list — that case still gets the
// fallback rather than wedging the runtime on a missing DNS entry.
func ResolveDNS(d config.DNS) []string {
	if d.Mode == "custom" && len(d.Servers) > 0 {
		out := make([]string, len(d.Servers))
		copy(out, d.Servers)
		return out
	}
	if d.Mode == "custom" {
		slog.Info("chainctl: DNS Mode=custom with empty Servers, falling back to defaults",
			slog.String("scope", "chainctl.network"))
	}
	return []string{"1.1.1.1", "8.8.8.8"}
}

// MapIPv6Strategy translates the user-facing config.Network.IPv6Mode value
// (one of "prefer-v4", "prefer-v6", "disabled") into the sing-box
// dns.strategy enum ("prefer_ipv4", "prefer_ipv6", "ipv4_only"). Empty or
// unknown input maps to "prefer_ipv4" so the runtime never wedges on a
// hand-edited config.json.
func MapIPv6Strategy(mode string) string {
	switch mode {
	case "prefer-v6":
		return "prefer_ipv6"
	case "disabled":
		return "ipv4_only"
	default:
		return "prefer_ipv4"
	}
}

// DefaultNetworkLoader returns a Network accessor that always returns
// config.Defaults().Network with no error. Used by chainctl.New when
// Deps.Network is nil so existing tests / code paths that don't supply
// a real loader keep their pre-Tier-2b behaviour.
func DefaultNetworkLoader() func() (config.Network, error) {
	return func() (config.Network, error) {
		return config.Defaults().Network, nil
	}
}

// DefaultKillSwitchLoader returns the built-in kill-switch defaults
// (Enabled:true), used when Deps.KillSwitch is nil so the failure handler
// defaults to the protective (blocking) behavior.
func DefaultKillSwitchLoader() func() (config.KillSwitch, error) {
	return func() (config.KillSwitch, error) { return config.Defaults().KillSwitch, nil }
}
