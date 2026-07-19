package refresh

import (
	"context"
	"log/slog"
	"math/rand"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func mkRetryDriver(t *testing.T, st subscription.Store, serversPath string, syncFn SyncFn, fetchBackoff, schedBackoff []time.Duration) *Driver {
	t.Helper()
	return NewDriver(Config{
		Subs:                 st,
		ServersPath:          serversPath,
		SyncFunc:             syncFn,
		ProbeFunc:            noopProbe,
		SubFetchRetryBackoff: fetchBackoff,
		SubRetryBackoff:      schedBackoff,
		Now:                  func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
		Rand:                 rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:                  slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
}

func okMeta() subscription.SyncMeta {
	return subscription.SyncMeta{Status: "ok", Message: "imported=1 invalid=0 skipped=0"}
}

func TestSyncOne_RetriesTransientThenSucceeds(t *testing.T) {
	dir := t.TempDir()
	serversPath := writeSeedServers(t, dir, nil)
	st := &metaCaptureStore{}
	merged := []server.Server{{ID: "srv1", Name: "X", Vless: vless.Config{Address: "a.test", Port: 443, UUID: "u"}}}

	calls := 0
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		calls++
		if calls < 3 {
			return nil, subscription.SyncMeta{Status: "error", Message: "flaky"}, &subscription.FetchError{Transient: true}
		}
		return merged, okMeta(), nil
	}
	d := mkRetryDriver(t, st, serversPath, syncFn, []time.Duration{time.Millisecond, time.Millisecond}, nil)

	retryable := d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	require.Equal(t, 3, calls, "expected 2 transient failures then a success")
	require.False(t, retryable, "final success must not be retryable")
	saved, _ := server.Load(serversPath)
	require.Len(t, saved, 1)
	require.Equal(t, "ok", st.log[len(st.log)-1].Status)
}

func TestSyncOne_TransientExhausted_Retryable(t *testing.T) {
	dir := t.TempDir()
	serversPath := writeSeedServers(t, dir, nil)
	st := &metaCaptureStore{}

	calls := 0
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		calls++
		return nil, subscription.SyncMeta{Status: "error", Message: "down"}, &subscription.FetchError{Transient: true}
	}
	d := mkRetryDriver(t, st, serversPath, syncFn, []time.Duration{time.Millisecond, time.Millisecond}, nil)

	retryable := d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	require.Equal(t, 3, calls, "1 initial + 2 retries")
	require.True(t, retryable, "exhausted transient failure must be retryable")
}

func TestSyncOne_PermanentNotRetried(t *testing.T) {
	dir := t.TempDir()
	serversPath := writeSeedServers(t, dir, nil)
	st := &metaCaptureStore{}

	calls := 0
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		calls++
		return nil, subscription.SyncMeta{Status: "error", Message: "forbidden"}, &subscription.FetchError{StatusCode: 403, Transient: false}
	}
	d := mkRetryDriver(t, st, serversPath, syncFn, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}, nil)

	retryable := d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	require.Equal(t, 1, calls, "permanent failure must not be retried")
	require.False(t, retryable, "permanent failure must not trigger scheduler backoff")
}

func TestNextTick_BackoffProgressionAndReset(t *testing.T) {
	interval := 1000 * time.Millisecond
	sched := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	d := mkRetryDriver(t, &metaCaptureStore{}, "", nil, nil, sched)

	inBand := func(got, base time.Duration) bool {
		return got >= time.Duration(float64(base)*0.9) && got <= time.Duration(float64(base)*1.1)
	}
	fails := 0
	for _, want := range []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond} {
		got := d.nextTick(interval, true, &fails)
		require.Truef(t, inBand(got, want), "transient tick: got %v, want ~%v", got, want)
	}
	require.Equal(t, 4, fails)

	got := d.nextTick(interval, false, &fails)
	require.Truef(t, inBand(got, interval), "success tick: got %v, want ~%v", got, interval)
	require.Equal(t, 0, fails, "success resets consecutive-failure count")
}

func TestNextTick_CapsAtInterval(t *testing.T) {
	interval := 5 * time.Millisecond
	sched := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	d := mkRetryDriver(t, &metaCaptureStore{}, "", nil, nil, sched)

	fails := 0
	got := d.nextTick(interval, true, &fails)
	require.LessOrEqual(t, got, time.Duration(float64(interval)*1.1), "backoff must not exceed interval (+jitter)")
}
