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
	h := NewServiceStatusHandler("1.2.3", startedAt, func() bool { return false })

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

func TestServiceStatusResult_DecodeBytes(t *testing.T) {
	raw := []byte(`{"version":"v1","uptime_secs":10,"up_bytes":1024,"down_bytes":2048}`)
	var r ServiceStatusResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.UpBytes != 1024 || r.DownBytes != 2048 {
		t.Fatalf("up=%d down=%d", r.UpBytes, r.DownBytes)
	}
}

func TestServiceStatusHandler_ChainActive(t *testing.T) {
	cases := []struct {
		name  string
		alive bool
	}{
		{"alive", true},
		{"idle", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewServiceStatusHandler("v", time.Now(), func() bool { return c.alive })
			res, err := h(context.Background(), nil)
			require.NoError(t, err)
			var got ServiceStatusResult
			require.NoError(t, json.Unmarshal(res, &got))
			require.Equal(t, c.alive, got.ChainActive)
		})
	}
}

func TestServiceStatusHandler_NilChainAliveTreatsAsIdle(t *testing.T) {
	h := NewServiceStatusHandler("v", time.Now(), nil)
	res, err := h(context.Background(), nil)
	require.NoError(t, err)
	var got ServiceStatusResult
	require.NoError(t, json.Unmarshal(res, &got))
	require.False(t, got.ChainActive)
}
