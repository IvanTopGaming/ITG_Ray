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
	require.Equal(t, vless.TransportWS, c.Transport)
	require.Equal(t, "/ws", c.Path)
	require.Equal(t, "cdn.example.com", c.WSHost)
	require.Equal(t, "NL-AMS-1", c.Remark)
}

func TestParseSingboxJSON_NotJSON(t *testing.T) {
	_, err := ParseSingbox("not json")
	require.Error(t, err)
}
