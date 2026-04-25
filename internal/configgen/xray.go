package configgen

import (
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/vless"
)

// XrayInput collects what BuildXray needs: the chosen server and the local
// SOCKS5 port that sing-box's "proxy" outbound will dial.
type XrayInput struct {
	Server    vless.Config
	SocksPort int
}

// BuildXray emits an xray-core config: SOCKS5 inbound on 127.0.0.1:SocksPort,
// VLESS outbound configured for the given server (Reality/TLS/None + transport).
func BuildXray(in *XrayInput) ([]byte, error) {
	stream := buildStream(&in.Server)

	out := map[string]any{
		"protocol": "vless",
		"tag":      "proxy",
		"settings": map[string]any{
			"vnext": []map[string]any{
				{
					"address": in.Server.Address,
					"port":    int(in.Server.Port),
					"users": []map[string]any{
						{
							"id":         in.Server.UUID,
							"flow":       in.Server.Flow,
							"encryption": orDefaultStr(in.Server.Encryption, "none"),
						},
					},
				},
			},
		},
		"streamSettings": stream,
	}

	doc := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []map[string]any{
			{
				"tag":      "socks-in",
				"listen":   "127.0.0.1",
				"port":     in.SocksPort,
				"protocol": "socks",
				"settings": map[string]any{"udp": true, "auth": "noauth"},
			},
		},
		"outbounds": []map[string]any{out},
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
			"serverName":    c.SNI,
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
		ss["wsSettings"] = map[string]any{
			"path":    c.Path,
			"headers": map[string]any{"Host": c.WSHost},
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
		ss["xhttpSettings"] = map[string]any{
			"path": c.Path,
			"mode": c.XHTTPMode,
		}
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
