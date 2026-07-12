package subscription

import (
	"os"
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func TestParseSingboxJSON_Basic(t *testing.T) {
	b, err := os.ReadFile("../../testdata/subscriptions/vless-singbox.json")
	require.NoError(t, err)

	r, err := ParseSingbox(string(b))
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Equal(t, 1, r.Skipped["shadowsocks"])

	c := r.Configs[0]
	require.Equal(t, "example.com", c.Address)
	require.Equal(t, uint16(443), c.Port)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", c.UUID)
	require.Equal(t, "xtls-rprx-vision", c.Flow)
	require.Equal(t, vless.SecurityReality, c.Security)
	require.Equal(t, "www.cloudflare.com", c.SNI)
	require.Equal(t, "PUBKEY", c.RealityPublicKey)
	require.Equal(t, "0011", c.RealityShortID)
	require.Equal(t, "chrome", c.Fingerprint)
	require.Equal(t, vless.TransportTCP, c.Transport)
	require.Equal(t, "NL-AMS-1", c.Remark)
}

func TestParseSingboxJSON_NotJSON(t *testing.T) {
	_, err := ParseSingbox("not json")
	require.Error(t, err)
}

func TestParseSingboxJSON_UnknownTransportIsInvalid(t *testing.T) {
	// An outbound with a transport type we don't support should count as Invalid,
	// not silently decay to a TCP config carrying the foreign transport's metadata.
	in := `{
      "outbounds": [{
        "type": "vless",
        "server": "h",
        "server_port": 443,
        "uuid": "u",
        "transport": {"type": "h2", "path": "/bad"}
      }]
    }`
	r, err := ParseSingbox(in)
	require.NoError(t, err)
	require.Empty(t, r.Configs)
	require.Equal(t, 1, r.Invalid)
}

func TestParseSingboxJSON_TLSWithoutReality(t *testing.T) {
	in := `{
      "outbounds": [{
        "type": "vless",
        "server": "h",
        "server_port": 443,
        "uuid": "u",
        "tls": {"enabled": true, "server_name": "h.example.com"}
      }]
    }`
	r, err := ParseSingbox(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Equal(t, vless.SecurityTLS, r.Configs[0].Security)
	require.Equal(t, "h.example.com", r.Configs[0].SNI)
}

func TestParseSingboxJSON_PortOutOfRange(t *testing.T) {
	in := `{
      "outbounds": [{
        "type": "vless",
        "server": "h",
        "server_port": 70000,
        "uuid": "u"
      }]
    }`
	r, err := ParseSingbox(in)
	require.NoError(t, err)
	require.Empty(t, r.Configs)
	require.Equal(t, 1, r.Invalid)
}
