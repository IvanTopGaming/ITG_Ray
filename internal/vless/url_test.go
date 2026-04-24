package vless

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseURL_RealityXHTTP(t *testing.T) {
	u := "vless://550e8400-e29b-41d4-a716-446655440000@example.com:443" +
		"?encryption=none&flow=xtls-rprx-vision&security=reality" +
		"&sni=www.cloudflare.com&fp=chrome" +
		"&pbk=PUBKEY123&sid=0011&spx=%2F" +
		"&type=xhttp&mode=packet-up&path=%2Fabc" +
		"#NL-AMS-1"
	c, err := ParseURL(u)
	require.NoError(t, err)
	require.Equal(t, "example.com", c.Address)
	require.Equal(t, uint16(443), c.Port)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", c.UUID)
	require.Equal(t, "xtls-rprx-vision", c.Flow)
	require.Equal(t, "none", c.Encryption)
	require.Equal(t, SecurityReality, c.Security)
	require.Equal(t, "www.cloudflare.com", c.SNI)
	require.Equal(t, "chrome", c.Fingerprint)
	require.Equal(t, "PUBKEY123", c.RealityPublicKey)
	require.Equal(t, "0011", c.RealityShortID)
	require.Equal(t, "/", c.RealitySpiderX)
	require.Equal(t, TransportXHTTP, c.Transport)
	require.Equal(t, "packet-up", c.XHTTPMode)
	require.Equal(t, "/abc", c.Path)
	require.Equal(t, "NL-AMS-1", c.Remark)
}

func TestParseURL_TLSWebSocket(t *testing.T) {
	u := "vless://abc-123@host:443?type=ws&security=tls&sni=h.example.com&path=/ws&host=cdn.example.com&alpn=h2%2Chttp%2F1.1#tag"
	c, err := ParseURL(u)
	require.NoError(t, err)
	require.Equal(t, TransportWS, c.Transport)
	require.Equal(t, SecurityTLS, c.Security)
	require.Equal(t, "h.example.com", c.SNI)
	require.Equal(t, "/ws", c.Path)
	require.Equal(t, "cdn.example.com", c.WSHost)
	require.Equal(t, []string{"h2", "http/1.1"}, c.ALPN)
	require.Equal(t, "tag", c.Remark)
}

func TestParseURL_NoneTCP(t *testing.T) {
	u := "vless://u@h:80#plain"
	c, err := ParseURL(u)
	require.NoError(t, err)
	require.Equal(t, TransportTCP, c.Transport)
	require.Equal(t, SecurityNone, c.Security)
	require.Equal(t, "plain", c.Remark)
}

func TestParseURL_gRPC(t *testing.T) {
	u := "vless://u@h:443?type=grpc&security=tls&serviceName=route%2Fv1&mode=multi#g"
	c, err := ParseURL(u)
	require.NoError(t, err)
	require.Equal(t, TransportGRPC, c.Transport)
	require.Equal(t, "route/v1", c.GRPCServiceName)
	require.Equal(t, "multi", c.GRPCMode)
}

func TestParseURL_Errors(t *testing.T) {
	bad := []string{
		"",
		"http://x@y:1",
		"vless://no-at-sign",
		"vless://u@h",          // missing :port
		"vless://u@h:notaport", // bad port
		"vless://u@h:0",        // port 0
		"vless://u@h:70000",    // port overflow
		"vless://u@h:443?security=bogus",
		"vless://u@h:443?type=bogus",
	}
	for _, u := range bad {
		_, err := ParseURL(u)
		require.Error(t, err, "input=%q", u)
	}
}

func TestSerializeURL_RoundTrip(t *testing.T) {
	inputs := []string{
		"vless://550e8400-e29b-41d4-a716-446655440000@example.com:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.cloudflare.com&fp=chrome&pbk=PUBKEY123&sid=0011&spx=%2F&type=xhttp&mode=packet-up&path=%2Fabc#NL-AMS-1",
		"vless://abc@h:443?type=ws&security=tls&sni=h.example.com&path=%2Fws&host=cdn.example.com&alpn=h2%2Chttp%2F1.1#tag",
		"vless://u@h:80#plain",
	}
	for _, in := range inputs {
		c, err := ParseURL(in)
		require.NoError(t, err)
		out := c.URL()
		c2, err := ParseURL(out)
		require.NoError(t, err, "re-parse failed for %q", out)
		require.Equal(t, c, c2, "round-trip mismatch:\nin=%s\nout=%s", in, out)
	}
}
