package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceStatusHandler(t *testing.T) {
	startedAt := time.Now().Add(-3 * time.Second)
	h := NewServiceStatusHandler("1.2.3", startedAt)

	res, err := h(context.Background(), nil)
	require.NoError(t, err)

	var got struct {
		Version    string `json:"version"`
		UptimeSecs int    `json:"uptime_secs"`
	}
	require.NoError(t, json.Unmarshal(res, &got))
	require.Equal(t, "1.2.3", got.Version)
	require.GreaterOrEqual(t, got.UptimeSecs, 3)
	require.Less(t, got.UptimeSecs, 10)
}
