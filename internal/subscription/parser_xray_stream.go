package subscription

import (
	"encoding/json"
	"strings"

	"github.com/itg-team/itg-ray/internal/vless"
)

func applyXrayStream(c *vless.Config, s xrayStream) (bool, bool) {
	if s.Address != "" || s.Port != 0 || hasJSONValue(s.FinalMask) {
		return false, true
	}
	network := s.Network
	switch network {
	case "raw":
		network = "tcp"
	case "splithttp":
		network = "xhttp"
	}
	transport, ok := vless.ParseTransport(network)
	if !ok {
		return true, false
	}
	security, ok := vless.ParseSecurity(s.Security)
	if !ok {
		return true, false
	}
	c.Transport = transport
	c.Security = security
	switch security {
	case vless.SecurityTLS:
		if hasJSONValue(s.TLS.Certificates) || s.TLS.PinnedPeerCertSHA256 != "" || s.TLS.VerifyPeerCertByName != "" || s.TLS.ECHConfigList != "" {
			return false, true
		}
		c.SNI = s.TLS.ServerName
		c.ALPN = s.TLS.ALPN
		c.Fingerprint = s.TLS.Fingerprint
		c.AllowInsecure = s.TLS.AllowInsecure
	case vless.SecurityReality:
		if s.Reality.Mldsa65Verify != "" {
			return false, true
		}
		c.SNI = s.Reality.ServerName
		c.Fingerprint = s.Reality.Fingerprint
		c.RealityPublicKey = s.Reality.PublicKey
		if s.Reality.Password != "" {
			c.RealityPublicKey = s.Reality.Password
		}
		c.RealityShortID = s.Reality.ShortID
		c.RealitySpiderX = s.Reality.SpiderX
	}
	switch transport {
	case vless.TransportTCP:
		tcp := s.TCP
		if s.Raw != nil {
			tcp = s.Raw
		}
		if tcp == nil {
			break
		}
		if hasJSONValue(tcp.Header.Request) {
			return false, true
		}
		c.HeaderType = tcp.Header.Type
	case vless.TransportWS:
		return applyXrayHTTP(c, s.WS)
	case vless.TransportHTTPUpgrade:
		return applyXrayHTTP(c, s.HTTPUpgrade)
	case vless.TransportXHTTP:
		h := s.XHTTP
		if h == nil {
			h = s.SplitHTTP
		}
		if h == nil {
			break
		}
		if unsupportedXrayHTTP(*h) {
			return false, true
		}
		if hasJSONValue(h.Extra) {
			var extra xrayHTTP
			if json.Unmarshal(h.Extra, &extra) != nil {
				return true, false
			}
			if unsupportedXrayHTTP(extra) {
				return false, true
			}
			if len(extra.Headers) > 0 {
				return false, true
			}
		}
		c.XHTTPMode = h.Mode
		return applyXrayHTTP(c, *h)
	case vless.TransportGRPC:
		if s.GRPC.Authority != "" {
			return false, true
		}
		c.GRPCServiceName = s.GRPC.ServiceName
		if s.GRPC.MultiMode {
			c.GRPCMode = "multi"
		}
	case vless.TransportMKCP:
		c.Seed = s.KCP.Seed
		c.HeaderType = s.KCP.Header.Type
	case vless.TransportQUIC:
		c.QUICSec = s.QUIC.Security
		c.QUICKey = s.QUIC.Key
		c.HeaderType = s.QUIC.Header.Type
	}
	return true, true
}

func applyXrayHTTP(c *vless.Config, h xrayHTTP) (bool, bool) {
	c.Path = h.Path
	c.WSHost = h.Host
	for key, value := range h.Headers {
		if !strings.EqualFold(key, "Host") {
			return false, true
		}
		if c.WSHost == "" {
			c.WSHost = value
		}
	}
	return true, true
}

func unsupportedXrayHTTP(h xrayHTTP) bool {
	return hasJSONValue(h.DownloadSettings) || h.XPaddingObfsMode || h.NoGRPCHeader || h.NoSSEHeader ||
		h.XPaddingKey != "" || h.XPaddingHeader != "" || h.XPaddingPlacement != "" || h.XPaddingMethod != "" ||
		h.UplinkHTTPMethod != "" || h.SessionPlacement != "" || h.SessionKey != "" || h.SeqPlacement != "" || h.SeqKey != "" || h.UplinkDataPlacement != "" || h.UplinkDataKey != ""
}
