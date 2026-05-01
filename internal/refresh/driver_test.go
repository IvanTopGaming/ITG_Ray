package refresh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"
)

// fakeStore is an in-memory subscription.Store used in driver tests.
type fakeStore struct {
	subs    []subscription.Stored
	updates atomic.Int64
}

func (f *fakeStore) Load() ([]subscription.Stored, error) { return f.subs, nil }
func (f *fakeStore) Save(s []subscription.Stored) error   { f.subs = s; return nil }
func (f *fakeStore) UpdateMeta(_ string, _ time.Time, _, _ string, _ *subscription.Userinfo) error {
	f.updates.Add(1)
	return nil
}

// noopSync / noopProbe are placeholders for the skeleton tests; later tasks
// add tests that drive these through fake versions with side-effects.
func noopSync(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
	return existing, subscription.SyncMeta{Status: "ok", Message: "imported=0"}, nil
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

func TestDriver_SyncAndProbe_RaceFree(t *testing.T) {
	if testing.Short() {
		t.Skip("race-stress test skipped under -short")
	}
	dir := t.TempDir()
	serversPath := dir + "/servers.json"
	if err := server.Save(serversPath, nil); err != nil {
		t.Fatal(err)
	}

	st := &fakeStore{
		subs: []subscription.Stored{
			{ID: "s1", URL: "https://a.test", UpdateInterval: subscription.Duration(10 * time.Millisecond)},
			{ID: "s2", URL: "https://b.test", UpdateInterval: subscription.Duration(10 * time.Millisecond)},
		},
	}

	var counter atomic.Int64
	syncFn := func(_ context.Context, sub subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		// Each sync replaces servers with a small randomised set.
		n := int(counter.Add(1) % 5)
		out := make([]server.Server, n)
		for i := 0; i < n; i++ {
			out[i] = server.Server{
				ID:    fmt.Sprintf("%s-srv-%d", sub.ID, i),
				Name:  fmt.Sprintf("%s-srv-%d", sub.ID, i),
				Vless: vless.Config{Address: "x.test", Port: 443, UUID: "u"},
			}
		}
		return out, subscription.SyncMeta{Status: "ok", Message: fmt.Sprintf("imported=%d", n)}, nil
	}
	probeFn := func(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
		return time.Duration(counter.Load()%30) * time.Millisecond, nil
	}

	d := NewDriver(Config{
		Subs:               st,
		ServersPath:        serversPath,
		SyncFunc:           syncFn,
		ProbeFunc:          probeFn,
		DefaultSubInterval: 10 * time.Millisecond,
		ProbeInterval:      15 * time.Millisecond,
		FirstSubJitterMax:  1 * time.Millisecond,
		Rand:               rand.New(rand.NewSource(7)), //nolint:gosec // deterministic test seed
		Log:                slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := d.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run: %v", err)
	}

	// Final invariant: servers.json must be valid JSON parseable by server.Load.
	got, err := server.Load(serversPath)
	if err != nil {
		t.Fatalf("final servers.json corrupted: %v", err)
	}
	t.Logf("final state: %d servers, %d sync invocations", len(got), counter.Load())
}
