package configgen

import (
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/vless"
)

// XrayAPIPort is the localhost port xray-core's StatsService listens on.
// Hardcoded for now; lift to config.Network if a conflict ever surfaces.
const XrayAPIPort = 13355

// XrayInput collects what BuildXray needs: the chosen server and the local
// SOCKS5 port that sing-box's "proxy" outbound will dial.
type XrayInput struct {
	Server    vless.Config
	SocksPort int
	// ServerIP, when non-empty, is a pre-resolved IPv4 literal that overrides
	// Server.Address in the vnext outbound. Use this when the caller wants to
	// bypass xray's runtime DNS resolution — specifically in TUN/FakeIP mode,
	// where xray would otherwise resolve Server.Address through Windows
	// DnsClient → TUN → sing-box DNS engine → FakeIP, get back a synthetic
	// 198.18.x.x address, and then dial THAT for VLESS, creating a
	// self-referential routing loop. See B6.7.18 in the smoke log.
	//
	// The override only swaps the dial target. Reality/TLS SNI is unaffected
	// because buildStream populates serverName from c.SNI, not c.Address.
	ServerIP string
	Extra    []XrayServer
}

type XrayServer struct {
	Tag       string
	Server    vless.Config
	ServerIP  string
	SocksPort int
}

const ClashAPIPort = 13356

func ExtraTag(serverID string) string {
	return "proxy-" + serverID
}

// BuildXray emits an xray-core config: SOCKS5 inbound on 127.0.0.1:SocksPort,
// VLESS outbound configured for the given server (Reality/TLS/None + transport).
func BuildXray(in *XrayInput) ([]byte, error) {
	inbounds := []map[string]any{
		{
			"tag":      "socks-in",
			"listen":   "127.0.0.1",
			"port":     in.SocksPort,
			"protocol": "socks",
			"settings": map[string]any{"udp": true, "auth": "noauth"},
		},
		{
			"tag":      "api",
			"listen":   "127.0.0.1",
			"port":     XrayAPIPort,
			"protocol": "dokodemo-door",
			"settings": map[string]any{"address": "127.0.0.1"},
		},
	}
	outbounds := []map[string]any{vlessOutbound("proxy", &in.Server, in.ServerIP)}
	routingRules := []map[string]any{
		{
			"type":        "field",
			"inboundTag":  []string{"api"},
			"outboundTag": "api",
		},
	}
	for i := range in.Extra {
		x := &in.Extra[i]
		inTag := "socks-in-" + x.Tag
		inbounds = append(inbounds, map[string]any{
			"tag":      inTag,
			"listen":   "127.0.0.1",
			"port":     x.SocksPort,
			"protocol": "socks",
			"settings": map[string]any{"udp": true, "auth": "noauth"},
		})
		outbounds = append(outbounds, vlessOutbound(x.Tag, &x.Server, x.ServerIP))
		routingRules = append(routingRules, map[string]any{
			"type":        "field",
			"inboundTag":  []string{inTag},
			"outboundTag": x.Tag,
		})
	}

	doc := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"StatsService"},
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   map[string]any{"rules": routingRules},
		"stats":     map[string]any{},
		"policy": map[string]any{
			"system": map[string]any{
				"statsInboundUplink":    true,
				"statsInboundDownlink":  true,
				"statsOutboundUplink":   true,
				"statsOutboundDownlink": true,
			},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func vlessOutbound(tag string, server *vless.Config, serverIP string) map[string]any {
	stream := buildStream(server)

	// Pre-resolved IP literal wins over hostname when caller supplies one
	// (TUN/FakeIP-loop mitigation; see ServerIP doc above).
	address := server.Address
	if serverIP != "" {
		address = serverIP
	}

	return map[string]any{
		"protocol": "vless",
		"tag":      tag,
		"settings": map[string]any{
			"vnext": []map[string]any{
				{
					"address": address,
					"port":    int(server.Port),
					"users": []map[string]any{
						{
							"id":         server.UUID,
							"flow":       server.Flow,
							"encryption": orDefaultStr(server.Encryption, "none"),
						},
					},
				},
			},
		},
		"streamSettings": stream,
	}
}

func buildStream(c *vless.Config) map[string]any {
	ss := map[string]any{"network": c.Transport.String()}
	switch c.Security {
	case vless.SecurityReality:
		ss["security"] = "reality"
		ss["realitySettings"] = map[string]any{
			"serverName":  c.SNI,
			"fingerprint": c.Fingerprint,
			"publicKey":   c.RealityPublicKey,
			"shortId":     c.RealityShortID,
			"spiderX":     c.RealitySpiderX,
		}
	case vless.SecurityTLS:
		ss["security"] = "tls"
		tls := map[string]any{
			"serverName":    orDefaultStr(c.SNI, c.Address),
			"allowInsecure": c.AllowInsecure,
		}
		if c.Fingerprint != "" {
			tls["fingerprint"] = c.Fingerprint
		}
		if len(c.ALPN) > 0 {
			tls["alpn"] = toAnySlice(c.ALPN)
		}
		ss["tlsSettings"] = tls
	default:
		ss["security"] = "none"
	}
	switch c.Transport {
	case vless.TransportWS:
		// Host goes in its own field: xray deprecated headers.Host for
		// WebSocket and warns that it will be removed.
		ss["wsSettings"] = map[string]any{
			"path": c.Path,
			"host": c.WSHost,
		}
	case vless.TransportGRPC:
		ss["grpcSettings"] = map[string]any{
			"serviceName": c.GRPCServiceName,
			"multiMode":   c.GRPCMode == "multi",
		}
	case vless.TransportHTTPUpgrade:
		ss["httpupgradeSettings"] = map[string]any{
			"path": c.Path,
			"host": c.WSHost,
		}
	case vless.TransportXHTTP:
		xs := map[string]any{"path": c.Path, "mode": c.XHTTPMode}
		if c.WSHost != "" {
			xs["host"] = c.WSHost
		}
		ss["xhttpSettings"] = xs
	case vless.TransportMKCP:
		ss["kcpSettings"] = map[string]any{
			"header": map[string]any{"type": orDefaultStr(c.HeaderType, "none")},
			"seed":   c.Seed,
		}
	case vless.TransportQUIC:
		ss["quicSettings"] = map[string]any{
			"security": c.QUICSec,
			"key":      c.QUICKey,
			"header":   map[string]any{"type": orDefaultStr(c.HeaderType, "none")},
		}
	case vless.TransportTCP:
		if c.HeaderType == "http" {
			ss["tcpSettings"] = map[string]any{
				"header": map[string]any{"type": "http"},
			}
		}
	}
	return ss
}

func orDefaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func toAnySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
