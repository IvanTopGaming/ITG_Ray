// Package configgen generates sing-box and xray JSON from the single UI model.
package configgen

import (
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/rules"
)

// Mode selects the sing-box inbound shape.
type Mode string

const (
	// ModeSysProxy is the default — sing-box exposes a mixed HTTP+SOCKS5
	// inbound on a loopback port that user apps target via the Windows
	// "system proxy" registry knob.
	ModeSysProxy Mode = ""
	// ModeTun attaches sing-box to an externally-created WinTUN adapter
	// by interface name. The Helper has already created the adapter,
	// configured its IPv4 and routes, and (optionally) overridden DNS.
	ModeTun Mode = "tun"
)

// SingboxInput collects everything BuildSingbox needs to emit a sing-box config.
// TunName and TunIPv4 are only consumed when Mode == ModeTun.
type SingboxInput struct {
	Mode             Mode
	SocksInboundPort int
	TunName          string
	TunIPv4          string
	XraySOCKSHost    string
	XraySOCKSPort    int
	Rules            rules.Model
	DNSUpstreams     []string
	// FakeIP, when true and Mode==ModeTun, enables sing-box's FakeIP DNS
	// module. A/AAAA queries return synthetic IPs in 198.18.0.0/15 (which
	// the TunIPv4 prefix covers), so DNS round-trips don't traverse the
	// proxy. Real DNS happens once per (domain, TTL) via the "remote"
	// server which is detoured through the proxy outbound — no LAN leak.
	// In ModeSysProxy this field is ignored (no TUN to attach FakeIP to).
	FakeIP bool
}

// buildDNSBlock builds the sing-box dns block. In TUN mode with FakeIP
// enabled, it emits a remote+fakeip server pair plus the rules that send
// A/AAAA queries to fakeip and everything else (PTR, MX, etc.) to remote.
// In any other configuration, it emits a single "default" server with no
// rules.
func buildDNSBlock(in *SingboxInput, upstreams []string) map[string]any {
	if in.Mode == ModeTun && in.FakeIP {
		return map[string]any{
			"servers": []map[string]any{
				{"tag": "remote", "address": upstreams[0], "detour": "proxy"},
				{"tag": "fakeip", "address": "fakeip"},
			},
			"rules": []map[string]any{
				{"query_type": []string{"A", "AAAA"}, "server": "fakeip"},
				{"outbound": "any", "server": "remote"},
			},
			"fakeip": map[string]any{
				"enabled":     true,
				"inet4_range": "198.18.0.0/15",
			},
			"independent_cache": true,
			"strategy":          "prefer_ipv4",
		}
	}
	return map[string]any{
		"servers": []map[string]any{
			{"tag": "default", "address": upstreams[0]},
		},
		"strategy": "prefer_ipv4",
	}
}

// applyTunModeKillswitch hardens the route block for TUN mode: it sets
// the route's "final" outbound to "block" (killswitch — unmatched traffic
// is dropped rather than leaked to the proxy) and prepends an
// RFC1918+loopback ip_cidr→direct rule immediately after the leading
// sniff action so the user can still reach their own LAN (printers, ssh,
// NAS) without the tunnel. The type-switch mirrors the sniff-prepend
// logic in BuildSingbox to handle both []map[string]any and []any rule
// slices that can arise from JSON round-tripping. Note: sing-box's route
// schema names this key "final" (not "default_outbound"); we keep that
// nomenclature so library validation accepts the document.
func applyTunModeKillswitch(route map[string]any) {
	route["final"] = "block"
	// hijackDnsRule feeds DNS traffic (detected via sniff in the rule
	// preceding it) into sing-box's DNS engine so that FakeIP and other
	// DNS rules can take effect. Without this, TUN inbound treats UDP/53
	// as opaque traffic and bypasses the DNS module entirely.
	hijackDnsRule := map[string]any{"protocol": "dns", "action": "hijack-dns"}
	lanRule := map[string]any{
		"ip_cidr": []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
		},
		"outbound": "direct",
	}
	// Insert hijack-dns then lanRule right after the first rule (which
	// is the sniff action prepended in BuildSingbox). Order matters:
	// sniff must run first to populate protocol metadata; hijack-dns
	// must run before LAN-direct so DNS to the LAN gateway is also
	// hijacked into the DNS engine, not shunted around it.
	switch existing := route["rules"].(type) {
	case []map[string]any:
		if len(existing) > 0 {
			route["rules"] = append([]map[string]any{existing[0], hijackDnsRule, lanRule}, existing[1:]...)
		} else {
			route["rules"] = []map[string]any{hijackDnsRule, lanRule}
		}
	case []any:
		if len(existing) > 0 {
			route["rules"] = append([]any{existing[0], hijackDnsRule, lanRule}, existing[1:]...)
		} else {
			route["rules"] = []any{hijackDnsRule, lanRule}
		}
	default:
		route["rules"] = []map[string]any{hijackDnsRule, lanRule}
	}
	// Append a catch-all proxy rule at the END so default traffic that
	// didn't match LAN exception or user rules goes through the proxy
	// instead of falling through to final="block" (which would drop
	// every default packet, including DNS to the FakeIP layer).
	// final="block" stays as defense-in-depth for malformed configs;
	// killswitch behavior on proxy outbound failure is preserved by
	// sing-box's drop-on-outbound-failure semantics.
	catchAllProxy := map[string]any{"outbound": "proxy"}
	switch existing := route["rules"].(type) {
	case []map[string]any:
		route["rules"] = append(existing, catchAllProxy)
	case []any:
		route["rules"] = append(existing, catchAllProxy)
	}
}

// BuildSingbox generates a sing-box config: mixed (HTTP+SOCKS5) inbound for
// local apps, three outbounds (proxy/direct/block), and the compiled rule
// engine driving routing decisions. The "proxy" outbound is a SOCKS5 client
// pointed at the embedded xray-core's local listener.
func BuildSingbox(in *SingboxInput) ([]byte, error) {
	ruleBytes, err := rules.Compile(in.Rules)
	if err != nil {
		return nil, err
	}
	var route map[string]any
	if err := json.Unmarshal(ruleBytes, &route); err != nil {
		return nil, err
	}

	// Prepend sniff action rule (sing-box 1.13+: legacy inbound sniff fields replaced by route rule actions).
	sniffRule := map[string]any{"action": "sniff"}
	switch existing := route["rules"].(type) {
	case []map[string]any:
		route["rules"] = append([]map[string]any{sniffRule}, existing...)
	case []any:
		route["rules"] = append([]any{sniffRule}, existing...)
	default:
		route["rules"] = []map[string]any{sniffRule}
	}

	if in.Mode == ModeTun {
		applyTunModeKillswitch(route)
	}

	upstreams := in.DNSUpstreams
	if len(upstreams) == 0 {
		upstreams = []string{"1.1.1.1", "8.8.8.8"}
	}

	var inbound map[string]any
	switch in.Mode {
	case ModeTun:
		// Sniffing is configured via the route-rule action prepended above
		// (sing-box 1.13 removed legacy per-inbound sniff fields).
		inbound = map[string]any{
			"type":           "tun",
			"tag":            "in-tun",
			"interface_name": in.TunName,
			"address":        []string{in.TunIPv4},
			"auto_route":     true,
			"strict_route":   false,
		}
	default:
		// Note: legacy inbound `sniff` / `sniff_override_destination` fields
		// were removed in sing-box 1.13; sniffing is now a route-rule action
		// (see sniffRule prepended above), so we deliberately omit them here.
		inbound = map[string]any{
			"type":        "mixed",
			"tag":         "in-local",
			"listen":      "127.0.0.1",
			"listen_port": in.SocksInboundPort,
		}
	}

	doc := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": buildDNSBlock(in, upstreams),
		"outbounds": []map[string]any{
			{
				"type":        "socks",
				"tag":         "proxy",
				"server":      in.XraySOCKSHost,
				"server_port": in.XraySOCKSPort,
				"version":     "5",
			},
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
		"route": route,
	}
	doc["inbounds"] = []map[string]any{inbound}
	return json.MarshalIndent(doc, "", "  ")
}
