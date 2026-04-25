package refresh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

func mkServer(id, host string, port int) server.Server {
	return server.Server{ID: id, Name: id, Vless: vless.Config{Address: host, Port: uint16(port), UUID: "u"}}
}

func TestProbeOnce_Success_SetsLatencyMS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	if err := server.Save(path, []server.Server{mkServer("a", "a.test", 443), mkServer("b", "b.test", 443)}); err != nil {
		t.Fatal(err)
	}
	probeFn := func(_ context.Context, addr string, _ time.Duration) (time.Duration, error) {
		if addr == "a.test:443" {
			return 12 * time.Millisecond, nil
		}
		return 25 * time.Millisecond, nil
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   probeFn,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	d.probeOnce(context.Background())

	got, _ := server.Load(path)
	want := map[string]int{"a": 12, "b": 25}
	for _, s := range got {
		if s.LatencyMS == nil {
			t.Fatalf("%s: LatencyMS is nil after success", s.ID)
		}
		if *s.LatencyMS != want[s.ID] {
			t.Fatalf("%s: got %dms, want %dms", s.ID, *s.LatencyMS, want[s.ID])
		}
	}
}

func TestProbeOnce_Failure_ResetsLatencyToNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	five := 5
	seed := []server.Server{mkServer("a", "a.test", 443)}
	seed[0].LatencyMS = &five // previously known to be 5ms
	if err := server.Save(path, seed); err != nil {
		t.Fatal(err)
	}
	probeFn := func(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
		return 0, errors.New("connection refused")
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   probeFn,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	d.probeOnce(context.Background())

	got, _ := server.Load(path)
	if got[0].LatencyMS != nil {
		t.Fatalf("LatencyMS should be nil after probe failure, got %d", *got[0].LatencyMS)
	}
}

func TestProbeOnce_ConcurrencyCappedAt16(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	const N = 50
	servers := make([]server.Server, N)
	for i := 0; i < N; i++ {
		servers[i] = mkServer(fmt.Sprintf("s%02d", i), fmt.Sprintf("h%d.test", i), 443)
	}
	if err := server.Save(path, servers); err != nil {
		t.Fatal(err)
	}

	var inFlight, maxObserved atomic.Int32
	probeFn := func(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
		cur := inFlight.Add(1)
		for {
			old := maxObserved.Load()
			if cur <= old || maxObserved.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return 5 * time.Millisecond, nil
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   probeFn,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	d.probeOnce(context.Background())

	if got := maxObserved.Load(); got > 16 {
		t.Fatalf("max in-flight = %d, want ≤ 16", got)
	}
	if got := maxObserved.Load(); got < 8 {
		t.Logf("max in-flight = %d (informational — too low can mean serial execution)", got)
	}
}

func TestProbeOnce_SnapshotApply_NewServersAddedDuringProbeAreNotTouched(t *testing.T) {
	// Reload-merge sanity: probeOnce snapshots, releases lock, probes, then
	// re-loads + merges. A server that appeared between snapshot and apply
	// should remain in the file unchanged (LatencyMS stays nil).
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	if err := server.Save(path, []server.Server{mkServer("a", "a.test", 443)}); err != nil {
		t.Fatal(err)
	}

	startProbing := make(chan struct{})
	probeFn := func(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
		<-startProbing // hold inside probe so we can mutate file outside
		return 10 * time.Millisecond, nil
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   probeFn,
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); d.probeOnce(context.Background()) }()

	// While probe is paused (between snapshot and apply), append a new server
	// directly to the file. The probe loop holds no mutex during the actual
	// probe, so this write succeeds.
	time.Sleep(50 * time.Millisecond)
	d.serversMu.Lock()
	cur, _ := server.Load(path)
	cur = append(cur, mkServer("late", "late.test", 443))
	_ = server.Save(path, cur)
	d.serversMu.Unlock()

	close(startProbing)
	wg.Wait()

	final, _ := server.Load(path)
	var aLatency, lateLatency *int
	for _, s := range final {
		if s.ID == "a" {
			aLatency = s.LatencyMS
		}
		if s.ID == "late" {
			lateLatency = s.LatencyMS
		}
	}
	if aLatency == nil || *aLatency != 10 {
		t.Fatalf("server a should have LatencyMS=10, got %v", aLatency)
	}
	if lateLatency != nil {
		t.Fatalf("server late was added after snapshot — its LatencyMS should remain nil, got %d", *lateLatency)
	}
}

func TestProbeOnce_EmptyServers_NoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	if err := server.Save(path, nil); err != nil {
		t.Fatal(err)
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   func(_ context.Context, _ string, _ time.Duration) (time.Duration, error) { t.Fatal("probe must not be called"); return 0, nil },
		Rand:        rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
		Log:         slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	d.probeOnce(context.Background())
}
