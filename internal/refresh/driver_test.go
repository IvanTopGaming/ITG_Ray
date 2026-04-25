package refresh

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// fakeStore is an in-memory subscription.Store used in driver tests.
type fakeStore struct {
	subs    []subscription.Stored
	updates atomic.Int64
}

func (f *fakeStore) Load() ([]subscription.Stored, error)             { return f.subs, nil }
func (f *fakeStore) Save(s []subscription.Stored) error               { f.subs = s; return nil }
func (f *fakeStore) UpdateMeta(_ string, _ time.Time, _ string) error { f.updates.Add(1); return nil }

// noopSync / noopProbe are placeholders for the skeleton tests; later tasks
// add tests that drive these through fake versions with side-effects.
func noopSync(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
	return existing, subscription.SyncMeta{Status: "OK", Summary: "imported=0"}, nil
}
func noopProbe(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
	return 0, nil
}

func newTestDriver(t *testing.T, st subscription.Store, serversPath string) *Driver {
	t.Helper()
	d := NewDriver(Config{
		Subs:        st,
		ServersPath: serversPath,
		SyncFunc:    noopSync,
		ProbeFunc:   noopProbe,
		Now:         time.Now,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	return d
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func TestDriver_Run_ReturnsOnContextCancel(t *testing.T) {
	st := &fakeStore{}
	d := newTestDriver(t, st, t.TempDir()+"/servers.json")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want nil or context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of cancel")
	}
}

func TestDriver_Run_NoSubs_ProbesNothing_ReturnsCleanly(t *testing.T) {
	st := &fakeStore{}
	d := newTestDriver(t, st, t.TempDir()+"/servers.json")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := d.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run: %v", err)
	}
	if st.updates.Load() != 0 {
		t.Fatalf("UpdateMeta should not be called with no subs, got %d", st.updates.Load())
	}
}

func TestNewDriver_AppliesDefaults(t *testing.T) {
	st := &fakeStore{}
	d := NewDriver(Config{Subs: st, ServersPath: "/tmp/x"})
	if d.defaultSubInterval != 12*time.Hour {
		t.Fatalf("defaultSubInterval=%v", d.defaultSubInterval)
	}
	if d.probeInterval != 5*time.Minute {
		t.Fatalf("probeInterval=%v", d.probeInterval)
	}
	if d.probeTimeout != 5*time.Second {
		t.Fatalf("probeTimeout=%v", d.probeTimeout)
	}
	if d.probeConcurrency != 16 {
		t.Fatalf("probeConcurrency=%d", d.probeConcurrency)
	}
	if d.now == nil || d.rand == nil || d.log == nil {
		t.Fatal("now/rand/log must default to non-nil")
	}
	if d.syncFunc == nil || d.probeFunc == nil {
		t.Fatal("syncFunc/probeFunc must default to non-nil (subscription.Sync / latency.TCPConnect)")
	}
}
