package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func TestBuildSingbox_TunMode(t *testing.T) {
	in := SingboxInput{
		Mode:          ModeTun,
		TunName:       "ITGRay-TUN",
		TunIPv4:       "198.18.0.1/15",
		XraySOCKSHost: "127.0.0.1",
		XraySOCKSPort: 1081,
		Rules:         rules.Model{DefaultAction: rules.ActionProxy},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	in0 := inbounds[0].(map[string]any)
	require.Equal(t, "tun", in0["type"])
	require.Equal(t, "ITGRay-TUN", in0["interface_name"])
	require.Equal(t, true, in0["auto_route"])
	addr := in0["address"].([]any)
	require.Equal(t, "198.18.0.1/15", addr[0])
}
