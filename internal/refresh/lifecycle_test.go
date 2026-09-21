package refresh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/bindings"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"
)

type refreshRegressionServerStore struct{ path string }

func (s refreshRegressionServerStore) Load() ([]server.Server, error) { return server.Load(s.path) }
func (s refreshRegressionServerStore) Save(v []server.Server) error   { return server.Save(s.path, v) }

type refreshRegressionGateLocker struct {
	actual  *bindings.StoreLock
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (l *refreshRegressionGateLocker) Lock() {
	l.once.Do(func() { close(l.entered); <-l.release })
	l.actual.Lock()
}
func (l *refreshRegressionGateLocker) Unlock() { l.actual.Unlock() }

type refreshRegressionInitialStore struct {
	subscription.FileStore
	loaded  chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *refreshRegressionInitialStore) Load() ([]subscription.Stored, error) {
	values, err := s.FileStore.Load()
	s.once.Do(func() { close(s.loaded); <-s.release })
	return values, err
}

func TestRefreshRegressionNewSubscriptionGetsScheduler(t *testing.T) {
	dir := t.TempDir()
	st := &refreshRegressionInitialStore{FileStore: subscription.FileStore{Path: filepath.Join(dir, "subs.json")}, loaded: make(chan struct{}), release: make(chan struct{})}
	var calls atomic.Int64
	d := NewDriver(Config{Subs: st, ServersPath: filepath.Join(dir, "servers.json"), FirstSubJitterMax: time.Nanosecond, SyncFunc: func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		calls.Add(1)
		return existing, subscription.SyncMeta{Status: "ok"}, nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = d.Run(ctx); close(done) }()
	<-st.loaded
	if err := st.Save([]subscription.Stored{{ID: "new", URL: "https://demo.invalid", UpdateInterval: subscription.Duration(time.Millisecond)}}); err != nil {
		t.Fatal(err)
	}
	close(st.release)
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if calls.Load() == 0 {
		t.Fatal("subscription added after initial Load was never scheduled (0 refreshes before deadline)")
	}
}

func TestRefreshRegressionQueuedRefreshDoesNotResurrectRemovedSubscription(t *testing.T) {
	dir := t.TempDir()
	st := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	sub := subscription.Stored{ID: "s1", URL: "https://old.invalid", UpdateInterval: subscription.Duration(time.Millisecond)}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	servers := refreshRegressionServerStore{path: filepath.Join(dir, "servers.json")}
	shared := bindings.NewStoreLock()
	gate := &refreshRegressionGateLocker{actual: shared, entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewDriver(Config{Subs: st, ServersPath: servers.path, ServersLock: gate, FirstSubJitterMax: time.Nanosecond, OnSync: func(string) { cancel() }, SyncFunc: func(_ context.Context, in subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		return server.Merge(existing, []server.Server{server.New(vless.Config{Address: "old-node.invalid", Port: 443, UUID: "demo"}, server.OriginSubscription, in.ID)}, in.ID), subscription.SyncMeta{Status: "ok"}, nil
	}})
	done := make(chan struct{})
	go func() { d.runSub(ctx, sub); close(done) }()
	<-gate.entered
	svc := bindings.NewSubsService(bindings.SubsDeps{SubStore: st, ServerStore: servers, StoreLock: shared, Hub: hub.New()})
	if err := svc.Remove(sub.ID); err != nil {
		t.Fatal(err)
	}
	close(gate.release)
	<-done
	got, err := servers.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("removed subscription resurrected %d server(s), source=%q", len(got), got[0].SourceID)
	}
}

func TestRefreshRegressionSlowRefreshDoesNotBlockFavoriteEdit(t *testing.T) {
	dir := t.TempDir()
	st := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	sub := subscription.Stored{ID: "s1", URL: "https://demo.invalid"}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	servers := refreshRegressionServerStore{path: filepath.Join(dir, "servers.json")}
	if err := servers.Save([]server.Server{{ID: "manual", Origin: server.OriginManual}}); err != nil {
		t.Fatal(err)
	}
	shared := bindings.NewStoreLock()
	entered := make(chan struct{})
	release := make(chan struct{})
	d := NewDriver(Config{Subs: st, ServersPath: servers.path, ServersLock: shared, SyncFunc: func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		close(entered)
		<-release
		return existing, subscription.SyncMeta{Status: "ok"}, nil
	}})
	done := make(chan struct{})
	go func() { d.syncOne(context.Background(), sub); close(done) }()
	<-entered
	svc := bindings.NewServersService(bindings.ServersDeps{ServerStore: servers, StoreLock: shared, Hub: hub.New()})
	edited := make(chan error, 1)
	received := false
	go func() { edited <- svc.ToggleFavorite("manual") }()
	select {
	case err := <-edited:
		received = true
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("local favorite edit blocks for the entire upstream fetch")
	}
	close(release)
	<-done
	if !received {
		if err := <-edited; err != nil {
			t.Error(err)
		}
	}
}

func TestRefreshRegressionSchedulerRemoveReaddAndCancel(t *testing.T) {
	dir := t.TempDir()
	st := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	sub := subscription.Stored{ID: "s1", URL: "https://demo.invalid", UpdateInterval: subscription.Duration(time.Millisecond)}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 4)
	stopped := make(chan struct{}, 4)
	var active, maximum atomic.Int64
	d := NewDriver(Config{Subs: st, ServersPath: filepath.Join(dir, "servers.json"), FirstSubJitterMax: time.Nanosecond, SyncFunc: func(ctx context.Context, _ subscription.Subscription, _ []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		n := active.Add(1)
		if n > maximum.Load() {
			maximum.Store(n)
		}
		started <- struct{}{}
		<-ctx.Done()
		active.Add(-1)
		stopped <- struct{}{}
		return nil, subscription.SyncMeta{}, ctx.Err()
	}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = d.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	await := func(ch <-chan struct{}, message string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(4 * time.Second):
			t.Fatal(message)
		}
	}
	await(started, "initial subscription never started")
	if err := st.Save(nil); err != nil {
		t.Fatal(err)
	}
	await(stopped, "removed subscription fetch was not canceled")
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	await(started, "re-added subscription never started")
	select {
	case <-started:
		t.Error("duplicate worker started for existing subscription")
	case <-time.After(1200 * time.Millisecond):
	}
	cancel()
	await(stopped, "shutdown did not cancel active fetch")
	if got := maximum.Load(); got != 1 {
		t.Errorf("maximum concurrent workers for one subscription = %d", got)
	}
}

func TestRefreshRegressionMergeKeepsConcurrentServerChanges(t *testing.T) {
	dir := t.TempDir()
	st := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	sub := subscription.Stored{ID: "s1", URL: "https://demo.invalid"}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "servers.json")
	node := server.New(vless.Config{Address: "node.invalid", Port: 443, UUID: "demo"}, server.OriginSubscription, sub.ID)
	before := []server.Server{node, {ID: "removed", Origin: server.OriginManual, Vless: vless.Config{Address: "removed.invalid"}}}
	if err := server.Save(path, before); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	lock := &sync.Mutex{}
	d := NewDriver(Config{Subs: st, ServersPath: path, ServersLock: lock, SyncFunc: func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		close(entered)
		<-release
		return server.Merge(existing, []server.Server{node}, sub.ID), subscription.SyncMeta{Status: "ok"}, nil
	}})
	go func() { d.syncOne(context.Background(), sub); close(done) }()
	<-entered
	changed := make(chan error, 1)
	go func() {
		lock.Lock()
		defer lock.Unlock()
		node.Favorite = true
		changed <- server.Save(path, []server.Server{node, {ID: "added", Origin: server.OriginManual, Vless: vless.Config{Address: "added.invalid"}}})
	}()
	select {
	case err := <-changed:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		close(release)
		<-done
		<-changed
		t.Fatal("concurrent server edit blocked by fetch")
	}
	close(release)
	<-done
	got, err := server.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Favorite || got[1].ID != "added" {
		t.Fatalf("concurrent changes lost: %+v", got)
	}
}

type refreshRegressionMetaStore struct {
	subscription.FileStore
	updates atomic.Int64
}

func (s *refreshRegressionMetaStore) UpdateMeta(id, sourceURL string, at time.Time, status, message string, headers *subscription.Headers) error {
	s.updates.Add(1)
	return s.FileStore.UpdateMeta(id, sourceURL, at, status, message, headers)
}

func TestRefreshRegressionRemovedDuringFetchDoesNotCommit(t *testing.T) {
	dir := t.TempDir()
	st := &refreshRegressionMetaStore{FileStore: subscription.FileStore{Path: filepath.Join(dir, "subs.json")}}
	sub := subscription.Stored{ID: "s1", URL: "https://demo.invalid"}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	servers := refreshRegressionServerStore{path: filepath.Join(dir, "servers.json")}
	node := server.New(vless.Config{Address: "node.invalid", Port: 443, UUID: "demo"}, server.OriginSubscription, sub.ID)
	if err := servers.Save([]server.Server{node}); err != nil {
		t.Fatal(err)
	}
	lock := bindings.NewStoreLock()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var notifications atomic.Int64
	d := NewDriver(Config{Subs: st, ServersPath: servers.path, ServersLock: lock, OnSync: func(string) { notifications.Add(1) }, SyncFunc: func(_ context.Context, _ subscription.Subscription, existing []server.Server, _ time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		close(entered)
		<-release
		return server.Merge(existing, []server.Server{node}, sub.ID), subscription.SyncMeta{Status: "ok", Headers: subscription.Headers{ProfileTitle: "stale"}}, nil
	}})
	go func() { d.syncOne(context.Background(), sub); close(done) }()
	<-entered
	svc := bindings.NewSubsService(bindings.SubsDeps{SubStore: st, ServerStore: servers, StoreLock: lock, Hub: hub.New()})
	removed := make(chan error, 1)
	go func() { removed <- svc.Remove(sub.ID) }()
	select {
	case err := <-removed:
		if err != nil {
			close(release)
			<-done
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		close(release)
		<-done
		<-removed
		t.Fatal("subscription removal blocked by in-flight fetch")
	}
	close(release)
	<-done
	got, err := servers.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("removed subscription restored servers: %+v", got)
	}
	subs, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Errorf("removed subscription restored: %+v", subs)
	}
	if got := st.updates.Load(); got != 0 {
		t.Errorf("stale response attempted %d metadata updates", got)
	}
	if got := notifications.Load(); got != 0 {
		t.Errorf("stale response emitted %d sync notifications", got)
	}
}

func TestRefreshRegressionRepeatedSyncPreservesEndpointVariants(t *testing.T) {
	var reverse atomic.Bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		first := "vless://u@node.example:443?type=ws&path=%2Fa#First\n"
		second := "vless://u@node.example:443?type=ws&path=%2Fb#Second\n"
		if reverse.Load() {
			_, _ = w.Write([]byte(second + first))
			return
		}
		_, _ = w.Write([]byte(first + second))
	}))
	defer ts.Close()
	dir := t.TempDir()
	st := subscription.FileStore{Path: filepath.Join(dir, "subs.json")}
	sub := subscription.Stored{ID: "s1", URL: ts.URL}
	if err := st.Save([]subscription.Stored{sub}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "servers.json")
	d := NewDriver(Config{Subs: st, ServersPath: path})
	if retry := d.syncOne(context.Background(), sub); retry {
		t.Fatal("initial sync failed")
	}
	before, err := server.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || before[0].ID == before[1].ID {
		t.Fatalf("endpoint variants not distinct: %+v", before)
	}
	ids := make(map[string]string)
	for i := range before {
		ids[before[i].Vless.Path] = before[i].ID
		if before[i].Vless.Path == "/a" {
			before[i].Favorite = true
			before[i].Tags = []string{"keep"}
			latency := 17
			before[i].LatencyMS = &latency
		}
	}
	if ids["/a"] == "" || ids["/b"] == "" {
		t.Fatalf("unexpected variants: %+v", before)
	}
	if err := server.Save(path, before); err != nil {
		t.Fatal(err)
	}
	reverse.Store(true)
	if retry := d.syncOne(context.Background(), sub); retry {
		t.Fatal("second sync failed")
	}
	after, err := server.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("lost or duplicated endpoint variants: %+v", after)
	}
	for _, s := range after {
		if s.ID != ids[s.Vless.Path] {
			t.Errorf("variant %q changed ID from %q to %q", s.Vless.Path, ids[s.Vless.Path], s.ID)
		}
		if s.Vless.Path == "/a" && (!s.Favorite || len(s.Tags) != 1 || s.Tags[0] != "keep" || s.LatencyMS == nil || *s.LatencyMS != 17) {
			t.Errorf("variant local fields lost: %+v", s)
		}
		if s.Vless.Path == "/b" && (s.Favorite || len(s.Tags) != 0 || s.LatencyMS != nil) {
			t.Errorf("local fields leaked to other variant: %+v", s)
		}
	}
}
