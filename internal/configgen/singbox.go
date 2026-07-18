// Package configgen generates sing-box and xray JSON from the single UI model.
package configgen

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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
	HTTPInboundPort  int // Tier 2b: separate http inbound; 0 → no http
	TunName          string
	TunIPv4          string
	MTU              int // Tier 2b: TUN interface MTU; 0 → OS default
	XraySOCKSHost    string
	XraySOCKSPort    int
	Rules            rules.Model
	DNSUpstreams     []string
	AllowLAN         bool   // Tier 2b: prepend LAN-bypass rule
	IPv6Strategy     string // Tier 2b: dns.strategy override
	// TunIPv6, when non-empty and Mode==ModeTun, is appended to the TUN
	// inbound "address" so auto_route captures ::/0. Empty ⇒ v4-only TUN
	// (byte-identical to pre-IPv6 output).
	TunIPv6 string
	// FakeIPv6Range, when non-empty and Mode==ModeTun with FakeIP, is the
	// fakeip server "inet6_range" so AAAA queries get synthetic v6. Empty ⇒
	// no v6 fake-IP (the "disabled" mode: v6 is captured but not tunnelled).
	FakeIPv6Range string
	// FakeIP, when true and Mode==ModeTun, enables sing-box's FakeIP DNS
	// module. A/AAAA queries return synthetic IPs in 198.18.0.0/15 (which
	// the TunIPv4 prefix covers), so DNS round-trips don't traverse the
	// proxy. Real DNS happens once per (domain, TTL) via the "remote"
	// server which is detoured through the proxy outbound — no LAN leak.
	// In ModeSysProxy this field is ignored (no TUN to attach FakeIP to).
	FakeIP bool
	// RouteExcludeAddress, when non-empty and Mode==ModeTun, is emitted as
	// the TUN inbound's "route_exclude_address". Used on Linux to exclude
	// the resolved VLESS server IP from the tunnel so xray's control
	// connection does not loop through TUN. Empty on Windows (the Windows
	// helper adds an explicit /32 peer-route instead), so output there is
	// unchanged.
	RouteExcludeAddress []string
	// LogLevel sets the sing-box log block's "level". Valid sing-box
	// values: trace|debug|info|warn|error|fatal|panic. Empty → "info".
	LogLevel string
	// GeoRuleSets maps each referenced geo rule_set tag (e.g. "geosite-ru")
	// to the absolute path of a pre-downloaded .srs file. The bridge fills
	// this in before BuildSingbox so sing-box loads rule-sets locally with
	// no network at startup. Empty → no geo rules referenced.
	GeoRuleSets map[string]string
}

// logLevelOrDefault returns a valid sing-box log level, defaulting to
// "info" for empty/unknown values so a stale setting can never produce an
// invalid config the library rejects.
func logLevelOrDefault(level string) string {
	switch level {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic":
		return level
	default:
		return "info"
	}
}

// buildDNSBlock builds the sing-box dns block in the 1.12+ schema. In TUN
// mode with FakeIP enabled, it emits a remote+fakeip server pair where the
// fakeip server type returns synthetic IPs in 198.18.0.0/15 for A/AAAA
// queries; everything else falls through to route.default_domain_resolver.
// In any other configuration, it emits a single "default" server. Both
// shapes also declare a "local" server that the direct outbound's
// domain_resolver points at, so direct-routed domains resolve off the
// physical NIC — correct GeoDNS answers for the user's real region, no proxy
// leak. Two deliberate choices make that geo-correct:
//   - detour:"direct" (not type:"local"): type:"local" reads the system
//     resolv.conf, which can loop back into the tunnel — e.g. a coexisting
//     Tailscale sets 100.100.100.100 as the system resolver, and the forward
//     gets hijacked into fakeip. Dialing an explicit resolver via the direct
//     outbound egresses the physical NIC instead.
//   - server localGeoResolver (Google 8.8.8.8), NOT the upstreams[0] used by
//     remote: the resolver must honor EDNS Client Subnet (ECS) so the
//     authoritative GeoDNS sees the user's real subnet. Cloudflare (1.1.1.1,
//     the usual upstreams[0]) strips ECS and, in regions where it has no PoP,
//     answers from a distant one — a RU user resolving a GeoDNS-guarded host
//     via 1.1.1.1 gets a Frankfurt node (~80ms), via 8.8.8.8 a Moscow node.
//     remote stays on upstreams[0]: it's tunnelled, so its geo is irrelevant.
//
// Upstream DNS uses DoT (RFC 7858) over the proxy outbound rather than
// plain UDP/53. Without TLS the queries are visible in cleartext to the
// exit server's network even though the user's ISP can't see them
// (because of the VLESS tunnel). DoT closes that exit-side leak and
// validates the resolver via TLS. Cloudflare (1.1.1.1), Google
// (8.8.8.8), and Quad9 (9.9.9.9) all publish valid DoT certificates.
// Custom user-configured servers that don't support DoT will fail —
// users with such servers must use a known-DoT-capable resolver.
//
// The legacy schema (top-level dns.fakeip block, address-based servers,
// {outbound:any, server:remote} catch-all rule) is rejected by sing-box
// 1.12+ with WARN-level deprecation messages and degraded DNS handling —
// hijack-dns silently fails to answer queries directed at the TUN gateway.
// See https://sing-box.sagernet.org/migration/ for the migration spec.
// localGeoResolver is the resolver the "local" DNS server queries for
// direct-routed domains. It must honor EDNS Client Subnet so GeoDNS-guarded
// hosts resolve to the user's real region; Google Public DNS does, Cloudflare
// does not. See buildDNSBlock's doc comment.
const localGeoResolver = "8.8.8.8"

func buildDNSBlock(in *SingboxInput, upstreams []string) map[string]any {
	strategy := in.IPv6Strategy
	if strategy == "" {
		strategy = "prefer_ipv4"
	}
	if in.Mode == ModeTun && in.FakeIP {
		fakeipServer := map[string]any{"tag": "fakeip", "type": "fakeip", "inet4_range": "198.18.0.0/15"}
		if in.FakeIPv6Range != "" {
			fakeipServer["inet6_range"] = in.FakeIPv6Range
		}
		return map[string]any{
			"servers": []map[string]any{
				{"tag": "remote", "type": "tls", "server": upstreams[0], "detour": "proxy"},
				fakeipServer,
				{"tag": "local", "type": "tls", "server": localGeoResolver, "detour": "direct"},
			},
			"rules": []map[string]any{
				{"query_type": []string{"A", "AAAA"}, "server": "fakeip"},
			},
			"independent_cache": true,
			"strategy":          strategy,
		}
	}
	return map[string]any{
		"servers": []map[string]any{
			{"tag": "default", "type": "tls", "server": upstreams[0], "detour": "proxy"},
			{"tag": "local", "type": "tls", "server": localGeoResolver, "detour": "direct"},
		},
		"strategy": strategy,
	}
}

// defaultDomainResolverFor picks the server tag that route.default_domain_resolver
// should reference. In 1.12+ this field is mandatory-warned; in 1.14 it
// becomes hard-required. We point it at the upstream resolver so any
// outbound that needs domain resolution (or any DNS query that doesn't
// match a rule) routes through the proxy-detoured server in TUN+FakeIP
// mode, or the plain default server otherwise.
func defaultDomainResolverFor(in *SingboxInput) string {
	if in.Mode == ModeTun && in.FakeIP {
		return "remote"
	}
	return "default"
}

// GeoTags compiles the model and returns the deduped geo rule_set tags
// (those prefixed geosite-/geoip-) referenced by its enabled rules, so the
// bridge knows which .srs files to pre-download. Returns nil on compile or
// unmarshal error.
func GeoTags(m rules.Model) []string {
	raw, err := rules.Compile(m)
	if err != nil {
		return nil
	}
	var route map[string]any
	if err := json.Unmarshal(raw, &route); err != nil {
		return nil
	}
	all := collectRuleSetTags(route)
	tags := make([]string, 0, len(all))
	for _, t := range all {
		if strings.HasPrefix(t, "geosite-") || strings.HasPrefix(t, "geoip-") {
			tags = append(tags, t)
		}
	}
	return tags
}

// collectRuleSetTags returns the deduped set of rule_set tags referenced by
// the route's rules, preserving first-seen order.
func collectRuleSetTags(route map[string]any) []string {
	var rules []any
	switch rs := route["rules"].(type) {
	case []any:
		rules = rs
	case []map[string]any:
		for _, m := range rs {
			rules = append(rules, m)
		}
	}
	seen := map[string]bool{}
	var tags []string
	appendTag := func(s string) {
		if !seen[s] {
			seen[s] = true
			tags = append(tags, s)
		}
	}
	for _, r := range rules {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		switch v := m["rule_set"].(type) {
		case []any:
			for _, t := range v {
				if s, ok := t.(string); ok {
					appendTag(s)
				}
			}
		case []string:
			for _, s := range v {
				appendTag(s)
			}
		}
	}
	return tags
}

// lanBypassCIDRs is the canonical RFC1918 + loopback + link-local + multicast
// IPv4/IPv6 list used for "send LAN traffic direct, bypass the proxy". Shared
// between AllowLAN-driven prepend (Tier 2b) and applyTunModeKillswitch's
// existing LAN-direct rule so both paths emit the identical sing-box rule
// shape.
var lanBypassCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"fc00::/7",
	"fe80::/10",
	"224.0.0.0/4",
}

// lanBypassRule returns the {ip_cidr, outbound:"direct"} rule that drops
// LAN-bound traffic out of the proxy path.
func lanBypassRule() map[string]any {
	cidrs := make([]any, len(lanBypassCIDRs))
	for i, s := range lanBypassCIDRs {
		cidrs[i] = s
	}
	return map[string]any{"ip_cidr": cidrs, "outbound": "direct"}
}

// prependLanBypass inserts lanBypassRule() immediately after the existing
// first rule (the sniff action prepended in BuildSingbox). If the rules
// slice is empty it just makes the bypass the first rule. Idempotency is
// caller-managed: BuildSingbox calls this once per build at most.
func prependLanBypass(rules []map[string]any) []map[string]any {
	rule := lanBypassRule()
	if len(rules) == 0 {
		return []map[string]any{rule}
	}
	out := make([]map[string]any, 0, len(rules)+1)
	out = append(out, rules[0], rule)
	out = append(out, rules[1:]...)
	return out
}

// hasEquivalentLanBypass reports whether the rules slice already contains a
// rule whose ip_cidr is a superset of lanBypassCIDRs with outbound:"direct".
// The safety-group default model loaded by cmd/itgray-bridge/configbuilder.go
// compiles to exactly such a rule; without this check applyTunModeKillswitch
// would emit a second identical one.
func hasEquivalentLanBypass(routeRules any) bool {
	check := func(m map[string]any) bool {
		if m["outbound"] != "direct" {
			return false
		}
		present := map[string]bool{}
		switch xs := m["ip_cidr"].(type) {
		case []any:
			for _, c := range xs {
				if s, ok := c.(string); ok {
					present[s] = true
				}
			}
		case []string:
			for _, s := range xs {
				present[s] = true
			}
		default:
			return false
		}
		for _, want := range lanBypassCIDRs {
			if !present[want] {
				return false
			}
		}
		return true
	}
	switch s := routeRules.(type) {
	case []map[string]any:
		for _, m := range s {
			if check(m) {
				return true
			}
		}
	case []any:
		for _, r := range s {
			if m, ok := r.(map[string]any); ok && check(m) {
				return true
			}
		}
	}
	return false
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
func applyTunModeKillswitch(route map[string]any, lanBypassPrepended bool, blockIPv6 bool) {
	route["final"] = "block"
	if !lanBypassPrepended && hasEquivalentLanBypass(route["rules"]) {
		lanBypassPrepended = true
	}
	// hijackDnsRule feeds DNS traffic (detected via sniff in the rule
	// preceding it) into sing-box's DNS engine so that FakeIP and other
	// DNS rules can take effect. Without this, TUN inbound treats UDP/53
	// as opaque traffic and bypasses the DNS module entirely.
	hijackDnsRule := map[string]any{"protocol": "dns", "action": "hijack-dns"}
	// Insert hijack-dns (and lanRule when not already prepended) right after
	// the first rule (the sniff action prepended in BuildSingbox). Order
	// matters: sniff must run first to populate protocol metadata; hijack-dns
	// must run before LAN-direct so DNS to the LAN gateway is also hijacked
	// into the DNS engine, not shunted around it.
	switch existing := route["rules"].(type) {
	case []map[string]any:
		if lanBypassPrepended {
			// LAN already there from AllowLAN; just inject hijack-dns after sniff.
			if len(existing) > 0 {
				route["rules"] = append([]map[string]any{existing[0], hijackDnsRule}, existing[1:]...)
			} else {
				route["rules"] = []map[string]any{hijackDnsRule}
			}
		} else {
			lanRule := lanBypassRule()
			if len(existing) > 0 {
				route["rules"] = append([]map[string]any{existing[0], hijackDnsRule, lanRule}, existing[1:]...)
			} else {
				route["rules"] = []map[string]any{hijackDnsRule, lanRule}
			}
		}
	case []any:
		if lanBypassPrepended {
			if len(existing) > 0 {
				route["rules"] = append([]any{existing[0], hijackDnsRule}, existing[1:]...)
			} else {
				route["rules"] = []any{hijackDnsRule}
			}
		} else {
			lanRule := lanBypassRule()
			if len(existing) > 0 {
				route["rules"] = append([]any{existing[0], hijackDnsRule, lanRule}, existing[1:]...)
			} else {
				route["rules"] = []any{hijackDnsRule, lanRule}
			}
		}
	default:
		if lanBypassPrepended {
			route["rules"] = []map[string]any{hijackDnsRule}
		} else {
			route["rules"] = []map[string]any{hijackDnsRule, lanBypassRule()}
		}
	}
	// Append a catch-all proxy rule at the END so default traffic that
	// didn't match LAN exception or user rules goes through the proxy
	// instead of falling through to final="block" (which would drop
	// every default packet, including DNS to the FakeIP layer).
	// final="block" stays as defense-in-depth for malformed configs;
	// killswitch behavior on proxy outbound failure is preserved by
	// sing-box's drop-on-outbound-failure semantics.
	//
	// When blockIPv6 is set ("disabled" mode: v6 is captured by the TUN but
	// must not be tunnelled), a ::/0 → block rule is inserted just before the
	// catch-all. LAN v6 (fc00::/7, fe80::/10) already matched the LAN-bypass
	// rule earlier, so only global v6 reaches this block.
	tail := make([]map[string]any, 0, 2)
	if blockIPv6 {
		tail = append(tail, map[string]any{"ip_cidr": []any{"::/0"}, "outbound": "block"})
	}
	tail = append(tail, map[string]any{"outbound": "proxy"})
	switch existing := route["rules"].(type) {
	case []map[string]any:
		route["rules"] = append(existing, tail...)
	case []any:
		for _, r := range tail {
			existing = append(existing, r)
		}
		route["rules"] = existing
	}
}

// BuildSingbox generates a sing-box config: mixed (HTTP+SOCKS5) inbound for
// local apps, three outbounds (proxy/direct/block), and the compiled rule
// engine driving routing decisions. The "proxy" outbound is a SOCKS5 client
// pointed at the embedded xray-core's local listener.
func BuildSingbox(in *SingboxInput) ([]byte, error) {
	ruleBytes, err := rules.Compile(in.Rules)
	if err != nil {
		slog.Error("singbox build failed", slog.String("scope", "configgen"),
			slog.String("stage", "compile rules"), slog.String("err", err.Error()))
		return nil, err
	}
	var route map[string]any
	if err := json.Unmarshal(ruleBytes, &route); err != nil {
		slog.Error("singbox build failed", slog.String("scope", "configgen"),
			slog.String("stage", "unmarshal route"), slog.String("err", err.Error()))
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

	lanBypassPrepended := false
	if in.AllowLAN {
		// After json.Unmarshal + the sniff prepend above, route["rules"] is
		// []any (sniff and existing rules are heterogeneous map types). Convert
		// back to []map[string]any so prependLanBypass can do its insertion,
		// then store back as the same shape — JSON marshal handles either.
		if rs, ok := route["rules"].([]any); ok {
			typed := make([]map[string]any, 0, len(rs))
			for _, r := range rs {
				if m, ok := r.(map[string]any); ok {
					typed = append(typed, m)
				}
			}
			route["rules"] = prependLanBypass(typed)
			lanBypassPrepended = true
		}
	}
	if in.Mode == ModeTun {
		blockIPv6 := in.TunIPv6 != "" && in.FakeIPv6Range == ""
		applyTunModeKillswitch(route, lanBypassPrepended, blockIPv6)
		// Bind outbound connections to the auto-detected default interface so
		// direct-routed traffic egresses the physical NIC instead of looping
		// back into the TUN (auto_route captures all destinations, including
		// sing-box's own direct outbound). Without this, any "direct" rule
		// (2ip.io, LAN bypass, etc.) times out. Cross-platform.
		route["auto_detect_interface"] = true
	}

	upstreams := in.DNSUpstreams
	if len(upstreams) == 0 {
		upstreams = []string{"1.1.1.1", "8.8.8.8"}
	}

	// 1.12+ schema requires route.default_domain_resolver (or per-outbound
	// domain_resolver) — the legacy {outbound:any, server:remote} DNS rule
	// is deprecated. Without this, sing-box logs a WARN and outbounds that
	// dial domain names fall back to system DNS, which can leak.
	route["default_domain_resolver"] = defaultDomainResolverFor(in)

	var inbound map[string]any
	switch in.Mode {
	case ModeTun:
		address := []string{in.TunIPv4}
		if in.TunIPv6 != "" {
			address = append(address, in.TunIPv6)
		}
		// Sniffing is configured via the route-rule action prepended above
		// (sing-box 1.13 removed legacy per-inbound sniff fields).
		inbound = map[string]any{
			"type":           "tun",
			"tag":            "in-tun",
			"interface_name": in.TunName,
			"address":        address,
			"auto_route":     true,
			"strict_route":   false,
		}
		if in.MTU > 0 {
			inbound["mtu"] = in.MTU
		}
		if len(in.RouteExcludeAddress) > 0 {
			inbound["route_exclude_address"] = in.RouteExcludeAddress
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
		"log": map[string]any{"level": logLevelOrDefault(in.LogLevel), "timestamp": true},
		"dns": buildDNSBlock(in, upstreams),
		"outbounds": []map[string]any{
			{
				"type":        "socks",
				"tag":         "proxy",
				"server":      in.XraySOCKSHost,
				"server_port": in.XraySOCKSPort,
				"version":     "5",
			},
			{"type": "direct", "tag": "direct", "domain_resolver": "local"},
			{"type": "block", "tag": "block"},
		},
		"route": route,
	}
	if tags := collectRuleSetTags(route); len(tags) > 0 {
		decls := make([]map[string]any, 0, len(tags))
		for _, tag := range tags {
			path, ok := in.GeoRuleSets[tag]
			if !ok {
				slog.Error("singbox build failed", slog.String("scope", "configgen"),
					slog.String("reason", "missing rule_set srs path"), slog.String("tag", tag))
				return nil, fmt.Errorf("BuildSingbox: no local .srs path for rule_set tag %q", tag)
			}
			decls = append(decls, map[string]any{
				"tag":    tag,
				"type":   "local",
				"format": "binary",
				"path":   path,
			})
		}
		route["rule_set"] = decls
	}
	var inbounds []map[string]any
	switch in.Mode {
	case ModeTun:
		inbounds = []map[string]any{inbound}
	case ModeSysProxy:
		if in.HTTPInboundPort > 0 && in.HTTPInboundPort != in.SocksInboundPort {
			inbounds = []map[string]any{
				{
					"type":        "socks",
					"tag":         "in-socks",
					"listen":      "127.0.0.1",
					"listen_port": in.SocksInboundPort,
				},
				{
					"type":        "http",
					"tag":         "in-http",
					"listen":      "127.0.0.1",
					"listen_port": in.HTTPInboundPort,
				},
			}
		} else {
			if in.HTTPInboundPort > 0 && in.HTTPInboundPort == in.SocksInboundPort {
				slog.Warn("configgen: SocksInboundPort==HTTPInboundPort, falling back to single mixed inbound",
					slog.String("scope", "configgen.singbox"),
					slog.Int("port", in.SocksInboundPort))
			}
			inbounds = []map[string]any{inbound} // mixed fallback
		}
	default:
		inbounds = []map[string]any{inbound}
	}
	doc["inbounds"] = inbounds
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	ruleSetCount := 0
	if rs, ok := route["rule_set"].([]map[string]any); ok {
		ruleSetCount = len(rs)
	}
	slog.Debug("singbox config built", slog.String("scope", "configgen"),
		slog.Int("bytes", len(out)), slog.Int("rule_sets", ruleSetCount))
	return out, nil
}
