package hub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshot_MarshalsToCamelCase(t *testing.T) {
	s := Snapshot{
		Status:      StatusConnected,
		Mode:        "tun",
		HelperState: "running",
		Speeds:      SpeedSample{UpBps: 1, DownBps: 2},
	}
	b, err := json.Marshal(s)
	require.NoError(t, err)
	js := string(b)
	require.Contains(t, js, `"status":"connected"`)
	require.Contains(t, js, `"helperState":"running"`)
	require.Contains(t, js, `"upBps":1`)
}
