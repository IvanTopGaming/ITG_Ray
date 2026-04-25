package core

import (
	"context"
	"testing"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func TestManager_Validates_ConfigsAreWellFormed(t *testing.T) {
	srv := vless.Config{
		Address: "example.com", Port: 443, UUID: "550e8400-e29b-41d4-a716-446655440000",
		Security: vless.SecurityReality, SNI: "www.cloudflare.com", Fingerprint: "chrome",
		// RealityPublicKey must be a valid base64url-encoded 32-byte Curve25519 key.
		RealityPublicKey: "6_rb2oEmDjmMrOrPzozWY2DTl3_rgHG4kNRHAFYBSQ8", RealityShortID: "01",
		Transport: vless.TransportTCP,
	}
	sbCfg, err := configgen.BuildSingbox(&configgen.SingboxInput{
		SocksInboundPort: 1080, XraySOCKSHost: "127.0.0.1", XraySOCKSPort: 1081,
		Rules: rules.Model{DefaultAction: rules.ActionProxy},
	})
	require.NoError(t, err)
	xrCfg, err := configgen.BuildXray(&configgen.XrayInput{Server: srv, SocksPort: 1081})
	require.NoError(t, err)

	m := NewManager()
	require.NoError(t, m.DryValidate(context.Background(), sbCfg, xrCfg))
}
