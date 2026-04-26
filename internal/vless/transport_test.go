package vless

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTransport(t *testing.T) {
	cases := []struct {
		in   string
		want Transport
		ok   bool
	}{
		{"tcp", TransportTCP, true},
		{"ws", TransportWS, true},
		{"grpc", TransportGRPC, true},
		{"httpupgrade", TransportHTTPUpgrade, true},
		{"xhttp", TransportXHTTP, true},
		{"mkcp", TransportMKCP, true},
		{"kcp", TransportMKCP, true},
		{"quic", TransportQUIC, true},
		{"", TransportTCP, true}, // empty defaults to tcp
		{"bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseTransport(c.in)
		require.Equal(t, c.ok, ok, "input=%q", c.in)
		if c.ok {
			require.Equal(t, c.want, got, "input=%q", c.in)
		}
	}
}

func TestParseSecurity(t *testing.T) {
	cases := []struct {
		in   string
		want Security
		ok   bool
	}{
		{"none", SecurityNone, true},
		{"", SecurityNone, true},
		{"tls", SecurityTLS, true},
		{"reality", SecurityReality, true},
		{"xtls", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseSecurity(c.in)
		require.Equal(t, c.ok, ok, "input=%q", c.in)
		if c.ok {
			require.Equal(t, c.want, got)
		}
	}
}
