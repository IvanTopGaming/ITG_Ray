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

// URL serializes the Config back into a vless:// URL.
func (c Config) URL() string { //nolint:gocritic // Config is a value type; caller convenience outweighs copy cost
	q := url.Values{}
	if c.Flow != "" {
		q.Set("flow", c.Flow)
	}
	if c.Encryption != "" {
		q.Set("encryption", c.Encryption)
	}
	q.Set("security", c.Security.String())
	if c.SNI != "" {
		q.Set("sni", c.SNI)
	}
	if len(c.ALPN) > 0 {
		q.Set("alpn", strings.Join(c.ALPN, ","))
	}
	if c.Fingerprint != "" {
		q.Set("fp", c.Fingerprint)
	}
	if c.AllowInsecure {
		q.Set("allowInsecure", "1")
	}
	if c.Security == SecurityReality {
		if c.RealityPublicKey != "" {
			q.Set("pbk", c.RealityPublicKey)
		}
		if c.RealityShortID != "" {
			q.Set("sid", c.RealityShortID)
		}
		if c.RealitySpiderX != "" {
			q.Set("spx", c.RealitySpiderX)
		}
	}
	q.Set("type", c.Transport.String())
	switch c.Transport {
	case TransportWS, TransportHTTPUpgrade:
		if c.Path != "" {
			q.Set("path", c.Path)
		}
		if c.WSHost != "" {
			q.Set("host", c.WSHost)
		}
	case TransportGRPC:
		if c.GRPCServiceName != "" {
			q.Set("serviceName", c.GRPCServiceName)
		}
		if c.GRPCMode != "" {
			q.Set("mode", c.GRPCMode)
		}
	case TransportXHTTP:
		if c.Path != "" {
			q.Set("path", c.Path)
		}
		if c.XHTTPMode != "" {
			q.Set("mode", c.XHTTPMode)
		}
	case TransportTCP:
		if c.HeaderType != "" {
			q.Set("headerType", c.HeaderType)
		}
	case TransportMKCP:
		if c.HeaderType != "" {
			q.Set("headerType", c.HeaderType)
		}
		if c.Seed != "" {
			q.Set("seed", c.Seed)
		}
	case TransportQUIC:
		if c.QUICSec != "" {
			q.Set("quicSecurity", c.QUICSec)
		}
		if c.QUICKey != "" {
			q.Set("key", c.QUICKey)
		}
	}
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(c.UUID),
		Host:     fmt.Sprintf("%s:%d", c.Address, c.Port),
		RawQuery: q.Encode(),
		Fragment: c.Remark,
	}
	return u.String()
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
