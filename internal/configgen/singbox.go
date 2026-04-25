// Package configgen generates sing-box and xray JSON from the single UI model.
package configgen

import (
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/rules"
)

// SingboxInput collects everything BuildSingbox needs to emit a sing-box config.
type SingboxInput struct {
	SocksInboundPort int
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

	upstreams := in.DNSUpstreams
	if len(upstreams) == 0 {
		upstreams = []string{"1.1.1.1", "8.8.8.8"}
	}

	doc := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": map[string]any{
			"servers": []map[string]any{
				{"tag": "default", "address": upstreams[0]},
			},
			"strategy": "prefer_ipv4",
		},
		"inbounds": []map[string]any{
			{
				"type":                       "mixed",
				"tag":                        "in-local",
				"listen":                     "127.0.0.1",
				"listen_port":                in.SocksInboundPort,
				"sniff":                      true,
				"sniff_override_destination": true,
			},
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
	return json.MarshalIndent(doc, "", "  ")
}
