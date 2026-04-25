package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/stretchr/testify/require"
)

func TestBuildSingbox_Minimal(t *testing.T) {
	in := SingboxInput{
		SocksInboundPort: 1080,
		XraySOCKSHost:    "127.0.0.1",
		XraySOCKSPort:    1081,
		Rules: rules.Model{
			DefaultAction: rules.ActionProxy,
			Groups: []rules.Group{
				{
					ID: "g", Enabled: true, Rules: []rules.Rule{
						{
							ID: "r", Enabled: true, Action: rules.ActionDirect,
							Conditions: rules.Conditions{IPCIDRs: []string{"10.0.0.0/8"}},
						},
					},
				},
			},
		},
	}
	b, err := BuildSingbox(&in)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	require.Equal(t, "mixed", inbounds[0].(map[string]any)["type"])

	outbounds := doc["outbounds"].([]any)
	tags := map[string]bool{}
	for _, o := range outbounds {
		tags[o.(map[string]any)["tag"].(string)] = true
	}
	require.True(t, tags["proxy"])
	require.True(t, tags["direct"])
	require.True(t, tags["block"])

	rt := doc["route"].(map[string]any)
	require.Equal(t, "proxy", rt["final"])
}
