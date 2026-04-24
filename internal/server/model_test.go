package server

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func TestStableID_Determinism(t *testing.T) {
	c := vless.Config{
		Address: "example.com",
		Port:    443,
		UUID:    "550e8400-e29b-41d4-a716-446655440000",
	}
	require.Equal(t, StableID(c), StableID(c))
}

func TestStableID_DependsOnlyOnAddressPortUUID(t *testing.T) {
	a := vless.Config{Address: "x", Port: 1, UUID: "u"}
	b := a
	b.Remark = "different"
	b.SNI = "sni.example"
	require.Equal(t, StableID(a), StableID(b))
}

func TestStableID_Differs(t *testing.T) {
	a := vless.Config{Address: "x", Port: 1, UUID: "u"}
	b := vless.Config{Address: "x", Port: 2, UUID: "u"}
	require.NotEqual(t, StableID(a), StableID(b))
}

func TestServer_Defaults(t *testing.T) {
	c := vless.Config{Address: "h", Port: 443, UUID: "u", Remark: "NL-1"}
	s := New(c, OriginManual, "")
	require.Equal(t, StableID(c), s.ID)
	require.Equal(t, "NL-1", s.Name)
	require.Equal(t, OriginManual, s.Origin)
	require.False(t, s.Favorite)
	require.False(t, s.Disabled)
	require.Nil(t, s.LatencyMS)
}

func TestNew_FallbackNameWhenNoRemark(t *testing.T) {
	c := vless.Config{Address: "h", Port: 443, UUID: "u"}
	s := New(c, OriginManual, "")
	require.Equal(t, "h:443", s.Name)
}
