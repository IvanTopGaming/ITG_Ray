package vless

import (
	"errors"
	"fmt"
)

var ErrIncompatible = errors.New("incompatible vless config")

func (c *Config) Normalize() ([]string, error) {
	var msgs []string

	if c.Security == SecurityReality {
		switch c.Transport {
		case TransportTCP, TransportGRPC, TransportXHTTP:
		default:
			return msgs, fmt.Errorf("%w: reality is not supported with %s transport", ErrIncompatible, c.Transport)
		}
		if c.RealityPublicKey == "" {
			return msgs, fmt.Errorf("%w: reality requires a public key (pbk)", ErrIncompatible)
		}
		if c.SNI == "" {
			return msgs, fmt.Errorf("%w: reality requires an SNI (sni)", ErrIncompatible)
		}
	}

	if c.Flow != "" {
		ok := c.Transport == TransportTCP &&
			(c.Security == SecurityTLS || c.Security == SecurityReality) &&
			c.Flow == "xtls-rprx-vision"
		if !ok {
			msgs = append(msgs, fmt.Sprintf("dropped flow %q (requires TCP + TLS/Reality)", c.Flow))
			c.Flow = ""
		}
	}

	if c.Security == SecurityTLS && c.SNI == "" {
		msgs = append(msgs, "no SNI set; xray will use the server address")
	}
	if c.Security == SecurityNone && c.Transport == TransportQUIC {
		msgs = append(msgs, "QUIC without TLS is insecure")
	}

	return msgs, nil
}
