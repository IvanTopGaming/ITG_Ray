package refresh

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"
)

// metaCapture stores all UpdateMeta calls.
type metaCall struct {
	ID      string
	At      time.Time
	Status  string
	Message string
	UI      *subscription.Userinfo
}

type metaCaptureStore struct {
	subs []subscription.Stored
	mu   sync.Mutex
	log  []metaCall
}

func (m *metaCaptureStore) Load() ([]subscription.Stored, error) { return m.subs, nil }
func (m *metaCaptureStore) Save(s []subscription.Stored) error   { m.subs = s; return nil }
func (m *metaCaptureStore) UpdateMeta(id string, at time.Time, status, message string, ui *subscription.Userinfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, metaCall{ID: id, At: at, Status: status, Message: message, UI: ui})
	return nil
}

func writeSeedServers(t *testing.T, dir string, ss []server.Server) string {
	t.Helper()
	p := filepath.Join(dir, "servers.json")
	if err := server.Save(p, ss); err != nil {
		t.Fatalf("seed servers.json: %v", err)
	}
	return p
}

func mkDriver(t *testing.T, st subscription.Store, serversPath string, syncFn SyncFn) *Driver {
	t.Helper()
	return NewDriver(Config{
		Subs:        st,
		ServersPath: serversPath,
		SyncFunc:    syncFn,
		ProbeFunc:   noopProbe,
		Now:         func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
}

func TestSyncOne_Success_WritesServersAndOKMeta(t *testing.T) {
	dir := t.TempDir()
	serversPath := writeSeedServers(t, dir, nil)
	st := &metaCaptureStore{}

	merged := []server.Server{{ID: "srv1", Name: "X", Vless: vless.Config{Address: "a.test", Port: 443, UUID: "u"}}}
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return merged, subscription.SyncMeta{Status: "OK", Message: "imported=1 invalid=0 skipped=0"}, nil
	}
	d := mkDriver(t, st, serversPath, syncFn)

	d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	saved, err := server.Load(serversPath)
	if err != nil {
		t.Fatalf("Load servers: %v", err)
	}
	if len(saved) != 1 || saved[0].ID != "srv1" {
		t.Fatalf("servers.json not updated: %+v", saved)
	}
	if len(st.log) != 1 {
		t.Fatalf("UpdateMeta calls: %d, want 1", len(st.log))
	}
	if got := st.log[0].Status; got != "OK imported=1 invalid=0 skipped=0" {
		t.Fatalf("status: %q", got)
	}
	if !st.log[0].At.Equal(time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("at: %v", st.log[0].At)
	}
}

func TestSyncOne_Failure_DoesNotTouchServers_RecordsError(t *testing.T) {
	dir := t.TempDir()
	seed := []server.Server{{ID: "preexisting", Name: "Pre", Vless: vless.Config{Address: "p.test", Port: 443, UUID: "u"}}}
	serversPath := writeSeedServers(t, dir, seed)
	st := &metaCaptureStore{}

	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return nil, subscription.SyncMeta{}, errors.New("network unreachable")
	}
	d := mkDriver(t, st, serversPath, syncFn)

	d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	saved, _ := server.Load(serversPath)
	if len(saved) != 1 || saved[0].ID != "preexisting" {
		t.Fatalf("servers.json should be untouched on sync failure: %+v", saved)
	}
	if len(st.log) != 1 || st.log[0].Status != "ERROR: network unreachable" {
		t.Fatalf("status capture: %+v", st.log)
	}
}

func TestSyncOne_FailureMessage_TruncatedTo120Chars(t *testing.T) {
	st := &metaCaptureStore{}
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return nil, subscription.SyncMeta{}, errors.New(string(long))
	}
	d := mkDriver(t, st, t.TempDir()+"/servers.json", syncFn)
	d.syncOne(context.Background(), subscription.Stored{ID: "s1"})
	if len(st.log) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.log))
	}
	got := st.log[0].Status
	// "ERROR: " + truncated body.  Truncation should keep the whole message ≤ 120+len("ERROR: ").
	const maxBody = 120
	if len(got) > len("ERROR: ")+maxBody {
		t.Fatalf("status not truncated: len=%d, body=%q", len(got), got)
	}
}

func TestSyncOne_CtxCanceledMidSync_NoMetaUpdate(t *testing.T) {
	st := &metaCaptureStore{}
	syncFn := func(ctx context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		<-ctx.Done()
		return nil, subscription.SyncMeta{}, ctx.Err()
	}
	d := mkDriver(t, st, t.TempDir()+"/servers.json", syncFn)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	d.syncOne(ctx, subscription.Stored{ID: "s1"})

	if got := len(st.log); got != 0 {
		t.Fatalf("UpdateMeta should not be called on shutdown; got %d calls", got)
	}
}

func TestRunSub_FirstTickWithinJitterWindow(t *testing.T) {
	st := &metaCaptureStore{}
	st.subs = []subscription.Stored{{ID: "s1", URL: "https://a.test", UpdateInterval: subscription.Duration(time.Hour)}}

	syncCh := make(chan time.Time, 4)
	syncFn := func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		syncCh <- time.Now()
		return existing, subscription.SyncMeta{Status: "OK", Message: "imported=0"}, nil
	}
	const testJitterMax = 50 * time.Millisecond
	d := NewDriver(Config{
		Subs:              st,
		ServersPath:       t.TempDir() + "/servers.json",
		SyncFunc:          syncFn,
		ProbeFunc:         noopProbe,
		FirstSubJitterMax: testJitterMax,
		Rand:              rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:               slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.wg.Add(1)
	start := time.Now()
	go d.runSub(ctx, st.subs[0])

	select {
	case fired := <-syncCh:
		elapsed := fired.Sub(start)
		if elapsed > testJitterMax+200*time.Millisecond {
			t.Fatalf("first tick after %v, want ≤ %v", elapsed, testJitterMax)
		}
	case <-time.After(testJitterMax + 2*time.Second):
		t.Fatal("syncOne never fired within jitter window")
	}
	cancel()
	d.wg.Wait()
}

func TestRunSub_ZeroIntervalUsesDriverDefault(t *testing.T) {
	st := &metaCaptureStore{}
	// UpdateInterval not set → driver default applies. We set DefaultSubInterval
	// to a small value so we can observe a SECOND tick within the test window.
	st.subs = []subscription.Stored{{ID: "s1", URL: "https://a.test"}}

	syncCh := make(chan struct{}, 8)
	syncFn := func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		syncCh <- struct{}{}
		return existing, subscription.SyncMeta{Status: "OK", Message: "imported=0"}, nil
	}
	d := NewDriver(Config{
		Subs:               st,
		ServersPath:        t.TempDir() + "/servers.json",
		SyncFunc:           syncFn,
		ProbeFunc:          noopProbe,
		DefaultSubInterval: 100 * time.Millisecond,
		// Seed picked so Int63n(30s) yields ≈3ms — first tick fires almost
		// immediately so the test can observe a 2nd tick within the deadline
		// without waiting up to 30s of jitter.
		Rand: rand.New(rand.NewSource(1516)), //nolint:gosec // deterministic test seed
		Log:  slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.wg.Add(1)
	go d.runSub(ctx, st.subs[0])

	// First tick fires after ~3ms (seed-controlled jitter), second after
	// ~100ms ±10%. 3 seconds of slop covers slow CI/race-detector overhead.
	deadline := time.After(3 * time.Second)
	count := 0
	for count < 2 {
		select {
		case <-syncCh:
			count++
		case <-deadline:
			t.Fatalf("only saw %d ticks in 3s, want ≥ 2", count)
		}
	}
	cancel()
	d.wg.Wait()
}

func TestRunSub_CtxCancel_ExitsPromptly(t *testing.T) {
	st := &metaCaptureStore{}
	st.subs = []subscription.Stored{{ID: "s1", URL: "https://a.test", UpdateInterval: subscription.Duration(time.Hour)}}
	d := NewDriver(Config{
		Subs:        st,
		ServersPath: t.TempDir() + "/servers.json",
		SyncFunc:    noopSync,
		ProbeFunc:   noopProbe,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	d.wg.Add(1)
	go d.runSub(ctx, st.subs[0])

	cancel()
	gotDone := make(chan struct{})
	go func() { d.wg.Wait(); close(gotDone) }()
	select {
	case <-gotDone:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runSub did not exit within 500ms of cancel")
	}
}
