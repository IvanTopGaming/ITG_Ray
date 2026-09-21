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
	Title   string
}

type metaCaptureStore struct {
	subs []subscription.Stored
	mu   sync.Mutex
	log  []metaCall
}

func (m *metaCaptureStore) Load() ([]subscription.Stored, error) { return m.subs, nil }
func (m *metaCaptureStore) Save(s []subscription.Stored) error   { m.subs = s; return nil }
func (m *metaCaptureStore) UpdateMeta(id, sourceURL string, at time.Time, status, message string, headers *subscription.Headers) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ui *subscription.Userinfo
	var title string
	if headers != nil {
		ui = headers.Userinfo
		title = headers.ProfileTitle
	}
	m.log = append(m.log, metaCall{ID: id, At: at, Status: status, Message: message, UI: ui, Title: title})
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

func TestSyncOne_PersistsProviderName(t *testing.T) {
	dir := t.TempDir()
	store := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	sub := subscription.Stored{ID: "s1", Name: "Old Provider", URL: "https://provider.example/sub"}
	if err := store.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	d := mkDriver(t, store, writeSeedServers(t, dir, nil), func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return existing, subscription.SyncMeta{Status: "ok", Headers: subscription.Headers{ProfileTitle: "New Provider"}}, nil
	})
	d.syncOne(context.Background(), sub)
	stored, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Name != "New Provider" {
		t.Fatalf("provider name not persisted: %+v", stored)
	}
}

func TestSyncOne_IgnoresProviderNameFromReplacedURL(t *testing.T) {
	dir := t.TempDir()
	store := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	old := subscription.Stored{ID: "s1", Name: "Old Provider", URL: "https://old.example/sub"}
	current := subscription.Stored{ID: "s1", Name: "new.example", URL: "https://new.example/sub"}
	if err := store.Save([]subscription.Stored{old}); err != nil {
		t.Fatal(err)
	}
	d := mkDriver(t, store, writeSeedServers(t, dir, nil), func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		if err := store.Save([]subscription.Stored{current}); err != nil {
			return nil, subscription.SyncMeta{}, err
		}
		return []server.Server{server.New(vless.Config{Address: "old-node.example", Port: 443, UUID: "demo"}, server.OriginSubscription, old.ID)}, subscription.SyncMeta{Status: "ok", Headers: subscription.Headers{ProfileTitle: "Old Provider"}}, nil
	})
	d.syncOne(context.Background(), old)
	stored, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored[0].Name != current.Name || !stored[0].LastSyncAt.IsZero() {
		t.Fatalf("outdated response changed current subscription: %+v", stored[0])
	}
	saved, err := server.Load(d.serversPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 0 {
		t.Fatalf("outdated response restored old URL servers: %+v", saved)
	}
}

func TestRunSub_UsesCurrentSubscriptionURL(t *testing.T) {
	dir := t.TempDir()
	store := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	old := subscription.Stored{ID: "s1", URL: "https://old.example/sub"}
	current := subscription.Stored{ID: "s1", URL: "https://new.example/sub"}
	if err := store.Save([]subscription.Stored{current}); err != nil {
		t.Fatal(err)
	}
	requested := make(chan string, 1)
	d := mkDriver(t, store, writeSeedServers(t, dir, nil), func(_ context.Context, sub subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		requested <- sub.URL
		return existing, subscription.SyncMeta{Status: "ok"}, nil
	})
	d.firstSubJitterMax = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); d.wg.Wait() })
	d.wg.Go(func() { d.runSub(ctx, old) })
	select {
	case got := <-requested:
		if got != current.URL {
			t.Fatalf("requested %q, want current URL %q", got, current.URL)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("scheduled sync did not run")
	}
}

func TestSyncOne_Success_WritesServersAndOKMeta(t *testing.T) {
	dir := t.TempDir()
	serversPath := writeSeedServers(t, dir, nil)
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}

	merged := []server.Server{{ID: "srv1", Name: "X", Vless: vless.Config{Address: "a.test", Port: 443, UUID: "u"}}}
	ui := &subscription.Userinfo{
		Upload: 100, HasUpload: true,
		Download: 200, HasDownload: true,
		Total: 1000, HasTotal: true,
	}
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return merged, subscription.SyncMeta{
			Status:  "ok",
			Message: "imported=1 invalid=0 skipped=0",
			Headers: subscription.Headers{Userinfo: ui},
		}, nil
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
	if got := st.log[0].Status; got != "ok" {
		t.Fatalf("status: %q, want %q", got, "ok")
	}
	if got := st.log[0].Message; got != "imported=1 invalid=0 skipped=0" {
		t.Fatalf("message: %q", got)
	}
	if st.log[0].UI == nil {
		t.Fatalf("UI: nil, want non-nil with Upload/Download/Total")
	}
	if st.log[0].UI.Upload != 100 || st.log[0].UI.Download != 200 || st.log[0].UI.Total != 1000 {
		t.Fatalf("UI fields: %+v", st.log[0].UI)
	}
	if !st.log[0].At.Equal(time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("at: %v", st.log[0].At)
	}
}

func TestSyncOne_Failure_DoesNotTouchServers_RecordsError(t *testing.T) {
	dir := t.TempDir()
	seed := []server.Server{{ID: "preexisting", Name: "Pre", Vless: vless.Config{Address: "p.test", Port: 443, UUID: "u"}}}
	serversPath := writeSeedServers(t, dir, seed)
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}

	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return nil, subscription.SyncMeta{Status: "error", Message: "network unreachable"}, errors.New("network unreachable")
	}
	d := mkDriver(t, st, serversPath, syncFn)

	d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	saved, _ := server.Load(serversPath)
	if len(saved) != 1 || saved[0].ID != "preexisting" {
		t.Fatalf("servers.json should be untouched on sync failure: %+v", saved)
	}
	if len(st.log) != 1 {
		t.Fatalf("UpdateMeta calls: %d, want 1", len(st.log))
	}
	if got := st.log[0].Status; got != "error" {
		t.Fatalf("status: %q, want %q", got, "error")
	}
	if got := st.log[0].Message; got != "network unreachable" {
		t.Fatalf("message: %q", got)
	}
	if st.log[0].UI != nil {
		t.Fatalf("UI: %+v, want nil on failure", st.log[0].UI)
	}
}

func TestSyncOne_FailureMessage_TruncatedTo120Chars(t *testing.T) {
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	syncFn := func(_ context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return nil, subscription.SyncMeta{Status: "error", Message: string(long)}, errors.New(string(long))
	}
	d := mkDriver(t, st, t.TempDir()+"/servers.json", syncFn)
	d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})
	if len(st.log) != 1 {
		t.Fatalf("expected 1 update, got %d", len(st.log))
	}
	if got := st.log[0].Status; got != "error" {
		t.Fatalf("status: %q, want %q", got, "error")
	}
	// Message should be truncated to ≤ 120 bytes on disk.
	const maxBody = 120
	if got := st.log[0].Message; len(got) > maxBody {
		t.Fatalf("message not truncated: len=%d, body=%q", len(got), got)
	}
}

func TestSyncOne_CtxCanceledMidSync_NoMetaUpdate(t *testing.T) {
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}
	syncFn := func(ctx context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		<-ctx.Done()
		return nil, subscription.SyncMeta{}, ctx.Err()
	}
	d := mkDriver(t, st, t.TempDir()+"/servers.json", syncFn)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	d.syncOne(ctx, subscription.Stored{ID: "s1", URL: "https://x.test"})

	if got := len(st.log); got != 0 {
		t.Fatalf("UpdateMeta should not be called on shutdown; got %d calls", got)
	}
}

func TestRunSub_FirstTickWithinJitterWindow(t *testing.T) {
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}
	st.subs = []subscription.Stored{{ID: "s1", URL: "https://a.test", UpdateInterval: subscription.Duration(time.Hour)}}

	syncCh := make(chan time.Time, 4)
	syncFn := func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		syncCh <- time.Now()
		return existing, subscription.SyncMeta{Status: "ok", Message: "imported=0"}, nil
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
	start := time.Now()
	d.wg.Go(func() { d.runSub(ctx, st.subs[0]) })

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
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}
	// UpdateInterval not set → driver default applies. We set DefaultSubInterval
	// to a small value so we can observe a SECOND tick within the test window.
	st.subs = []subscription.Stored{{ID: "s1", URL: "https://a.test"}}

	syncCh := make(chan struct{}, 8)
	syncFn := func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		syncCh <- struct{}{}
		return existing, subscription.SyncMeta{Status: "ok", Message: "imported=0"}, nil
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
	d.wg.Go(func() { d.runSub(ctx, st.subs[0]) })

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
	st := &metaCaptureStore{subs: []subscription.Stored{{ID: "s1", URL: "https://x.test"}}}
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
	d.wg.Go(func() { d.runSub(ctx, st.subs[0]) })

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
