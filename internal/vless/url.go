package vless

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Config holds every field decoded from a vless:// URL.
type Config struct {
	Address    string
	Port       uint16
	UUID       string
	Flow       string
	Encryption string
	Remark     string

	Security         Security
	SNI              string
	ALPN             []string
	Fingerprint      string
	AllowInsecure    bool
	RealityPublicKey string
	RealityShortID   string
	RealitySpiderX   string

	Transport Transport

	Path   string
	WSHost string

	GRPCServiceName string
	GRPCMode        string

	XHTTPMode string

	HeaderType string // for tcp + httpupgrade
	Seed       string // mkcp
	QUICSec    string // quic security
	QUICKey    string // quic key
}

// ErrInvalidURL is returned by ParseURL when the input is not a well-formed vless:// URL.
var ErrInvalidURL = errors.New("invalid vless url")

// ParseURL decodes a vless:// URL into a Config. Unknown query parameters are ignored.
func ParseURL(raw string) (Config, error) {
	if !strings.HasPrefix(raw, "vless://") {
		return Config{}, fmt.Errorf("%w: scheme must be vless://", ErrInvalidURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return Config{}, fmt.Errorf("%w: missing uuid", ErrInvalidURL)
	}
	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return Config{}, fmt.Errorf("%w: missing host:port", ErrInvalidURL)
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum < 1 || portNum > 65535 {
		return Config{}, fmt.Errorf("%w: invalid port %q", ErrInvalidURL, portStr)
	}

	q := u.Query()
	sec, ok := ParseSecurity(q.Get("security"))
	if !ok {
		return Config{}, fmt.Errorf("%w: unknown security %q", ErrInvalidURL, q.Get("security"))
	}
	tr, ok := ParseTransport(q.Get("type"))
	if !ok {
		return Config{}, fmt.Errorf("%w: unknown transport %q", ErrInvalidURL, q.Get("type"))
	}

	c := Config{
		Address:     host,
		Port:        uint16(portNum),
		UUID:        u.User.Username(),
		Flow:        q.Get("flow"),
		Encryption:  orDefault(q.Get("encryption"), "none"),
		Remark:      u.Fragment,
		Security:    sec,
		SNI:         q.Get("sni"),
		Fingerprint: q.Get("fp"),
		Transport:   tr,
	}
	if alpn := q.Get("alpn"); alpn != "" {
		c.ALPN = splitCSV(alpn)
	}
	if v := q.Get("allowInsecure"); v == "1" || strings.EqualFold(v, "true") {
		c.AllowInsecure = true
	}

	if sec == SecurityReality {
		c.RealityPublicKey = q.Get("pbk")
		c.RealityShortID = q.Get("sid")
		c.RealitySpiderX = q.Get("spx")
	}

	switch tr {
	case TransportWS, TransportHTTPUpgrade:
		c.Path = q.Get("path")
		c.WSHost = q.Get("host")
	case TransportGRPC:
		c.GRPCServiceName = q.Get("serviceName")
		c.GRPCMode = q.Get("mode")
	case TransportXHTTP:
		c.Path = q.Get("path")
		c.XHTTPMode = q.Get("mode")
	case TransportTCP:
		c.HeaderType = q.Get("headerType")
	case TransportMKCP:
		c.HeaderType = q.Get("headerType")
		c.Seed = q.Get("seed")
	case TransportQUIC:
		c.QUICSec = q.Get("quicSecurity")
		c.QUICKey = q.Get("key")
	}

	return c, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
