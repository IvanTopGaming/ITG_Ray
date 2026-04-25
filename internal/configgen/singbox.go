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
			"auto_route":     false,
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
		"dns": map[string]any{
			"servers": []map[string]any{
				{"tag": "default", "address": upstreams[0]},
			},
			"strategy": "prefer_ipv4",
		},
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
