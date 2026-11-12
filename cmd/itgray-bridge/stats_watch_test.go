package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/stats"
)

func TestOnStatsEvent_SessionsFollowVPNStatus(t *testing.T) {
	store, err := stats.OpenStore(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	defer store.Close()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	col := stats.NewCollector(nil, store, func() string { return "" }, func() time.Time { return now })
	onStatsEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "connected"}}, col, now)
	since, ok := col.ActiveSince()
	require.True(t, ok)
	require.True(t, since.Equal(now))
	onStatsEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "connecting"}}, col, now)
	_, ok = col.ActiveSince()
	require.True(t, ok)
	onStatsEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "idle"}}, col, now.Add(time.Hour))
	_, ok = col.ActiveSince()
	require.False(t, ok)
	ov, err := store.Overview(stats.PeriodToday, now.Add(2*time.Hour), time.UTC)
	require.NoError(t, err)
	require.Equal(t, int64(3600), ov.OnlineSeconds)
}
