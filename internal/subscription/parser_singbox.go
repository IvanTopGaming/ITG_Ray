package subscription

import (
	"encoding/json"
	"errors"

	"github.com/itg-team/itg-ray/internal/vless"
)

// ErrNotSingboxJSON is returned when the input cannot be parsed as a sing-box document.
var ErrNotSingboxJSON = errors.New("not a sing-box JSON document")

type sbDoc struct {
	Outbounds []sbOutbound `json:"outbounds"`
}

type sbOutbound struct {
	Type       string   `json:"type"`
	Tag        string   `json:"tag"`
	Server     string   `json:"server"`
	ServerPort int      `json:"server_port"`
	UUID       string   `json:"uuid"`
	Flow       string   `json:"flow"`
	TLS        *sbTLS   `json:"tls"`
	Transport  *sbTrans `json:"transport"`
}

type sbTLS struct {
	Enabled    bool       `json:"enabled"`
	ServerName string     `json:"server_name"`
	ALPN       []string   `json:"alpn"`
	Reality    *sbReality `json:"reality"`
	UTLS       *sbUTLS    `json:"utls"`
	Insecure   bool       `json:"insecure"`
}

type sbReality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id"`
}

type sbUTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint"`
}

type sbTrans struct {
	Type        string            `json:"type"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	ServiceName string            `json:"service_name"`
}

// ParseSingbox parses a sing-box JSON config, extracting vless outbounds.
// Non-vless outbounds are counted in ParseResult.Skipped by their "type" field.
func ParseSingbox(s string) (ParseResult, error) {
	var d sbDoc
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return ParseResult{}, ErrNotSingboxJSON
	}
	r := ParseResult{Skipped: map[string]int{}}
	for _, o := range d.Outbounds {
		if o.Type != "vless" {
			if o.Type != "" {
				r.Skipped[o.Type]++
			}
			continue
		}
		port := o.ServerPort
		if port < 1 || port > 65535 {
			r.Invalid++
			continue
		}
		c := vless.Config{
			Address:    o.Server,
			Port:       uint16(port),
			UUID:       o.UUID,
			Flow:       o.Flow,
			Encryption: "none",
			Remark:     o.Tag,
			Security:   vless.SecurityNone,
			Transport:  vless.TransportTCP,
		}
		if o.TLS != nil && o.TLS.Enabled {
			c.Security = vless.SecurityTLS
			c.SNI = o.TLS.ServerName
			c.ALPN = o.TLS.ALPN
			c.AllowInsecure = o.TLS.Insecure
			if o.TLS.UTLS != nil && o.TLS.UTLS.Enabled {
				c.Fingerprint = o.TLS.UTLS.Fingerprint
			}
			if o.TLS.Reality != nil && o.TLS.Reality.Enabled {
				c.Security = vless.SecurityReality
				c.RealityPublicKey = o.TLS.Reality.PublicKey
				c.RealityShortID = o.TLS.Reality.ShortID
			}
		}
		if o.Transport != nil && o.Transport.Type != "" {
			tr, ok := vless.ParseTransport(o.Transport.Type)
			if !ok {
				r.Invalid++
				continue
			}
			c.Transport = tr
			switch tr {
			case vless.TransportWS, vless.TransportHTTPUpgrade:
				c.Path = o.Transport.Path
				c.WSHost = o.Transport.Headers["Host"]
			case vless.TransportGRPC:
				c.GRPCServiceName = o.Transport.ServiceName
			case vless.TransportXHTTP:
				c.Path = o.Transport.Path
				c.WSHost = o.Transport.Headers["Host"]
			}
		}
		if c.Address == "" || c.UUID == "" {
			r.Invalid++
			continue
		}
		if _, nerr := c.Normalize(); nerr != nil {
			r.Invalid++
			continue
		}
		r.Configs = append(r.Configs, c)
	}
	return r, nil
}
