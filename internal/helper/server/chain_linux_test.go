//go:build linux

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/itg-team/itg-ray/internal/logtest"
)

// useTempRuntimeDir points the package-level runtimeDir at a writable temp
// directory for the duration of a test, so StartChain can write its core
// configs without needing the root-owned /run path (which fails on CI runners
// and any non-root machine).
func useTempRuntimeDir(t *testing.T) {
	t.Helper()
	orig := runtimeDir
	runtimeDir = t.TempDir()
	t.Cleanup(func() { runtimeDir = orig })
}

func TestStartChain_RejectsMissingServer(t *testing.T) {
	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{TunName: "t", Mode: "tun"}))
	if err == nil {
		t.Fatal("expected error when server_host/server_port missing")
	}
}

func TestStartChain_SpawnFailureRollsBack(t *testing.T) {
	useTempRuntimeDir(t)
	orig := spawnCore
	spawnCore = func(name, exe string, args []string, logPath string) (*supervisor.Child, error) {
		return nil, errors.New("boom")
	}
	defer func() { spawnCore = orig }()

	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{
		SingboxConfig: json.RawMessage(`{}`), XrayConfig: json.RawMessage(`{}`),
		ServerHost: "203.0.113.7", ServerPort: 443, TunName: "ITGRay-TUN", Mode: "tun",
	}))
	if err == nil {
		t.Fatal("expected StartChain to fail when spawn fails")
	}
	if IsChainActive() {
		t.Fatal("no chain should be active after a failed StartChain")
	}
}

func TestStartChain_RollbackWarnsOnSwallowedCoreStopFailure(t *testing.T) {
	useTempRuntimeDir(t)
	buf := logtest.Capture(t)

	origSpawn := spawnCore
	spawnCore = func(name, exe string, args []string, logPath string) (*supervisor.Child, error) {
		if name == "sing-box" {
			return &supervisor.Child{}, nil
		}
		return nil, errors.New("xray spawn boom")
	}
	defer func() { spawnCore = origSpawn }()

	origStop := stopChildBestEffort
	stopChildBestEffort = func(c *supervisor.Child, grace time.Duration) error {
		return errors.New("stop boom")
	}
	defer func() { stopChildBestEffort = origStop }()

	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{
		SingboxConfig: json.RawMessage(`{}`), XrayConfig: json.RawMessage(`{}`),
		ServerHost: "203.0.113.7", ServerPort: 443, TunName: "ITGRay-TUN", Mode: "tun",
	}))
	if err == nil {
		t.Fatal("expected StartChain to fail when xray spawn fails")
	}
	if IsChainActive() {
		t.Fatal("no chain should be active after a failed StartChain")
	}

	out := buf.String()
	if !strings.Contains(out, "[helper]") {
		t.Fatalf("missing helper scope in log: %q", out)
	}
	if !strings.Contains(out, "chain teardown: singbox stop failed") {
		t.Fatalf("swallowed rollback stop failure not logged: %q", out)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ctxBlockingStatsClient is a fake statsClient whose Counters call blocks
// until the given ctx is cancelled/times out, then returns ctx.Err(). This
// stands in for a wedged xray-core StatsService without needing a real gRPC
// server: readChainCounters's own bounded context (once fixed) is what
// eventually frees it, exactly as it would for a genuinely hung xray.
type ctxBlockingStatsClient struct {
	started chan struct{}
	once    sync.Once
}

func (f *ctxBlockingStatsClient) Counters(ctx context.Context) (up, down uint64, err error) {
	if f.started != nil {
		f.once.Do(func() { close(f.started) })
	}
	<-ctx.Done()
	return 0, 0, ctx.Err()
}

func (f *ctxBlockingStatsClient) Close() error { return nil }

// withActiveSess installs sess as activeSess for the duration of the test,
// restoring whatever was there before on cleanup. Direct package-var
// manipulation is legitimate here (test runs in-package): it lets the H4
// tests exercise readChainCounters/OpStopChain's chainMu contract without
// spawning real sing-box/xray processes.
func withActiveSess(t *testing.T, sess *chainState) {
	t.Helper()
	chainMu.Lock()
	prev := activeSess
	activeSess = sess
	chainMu.Unlock()
	t.Cleanup(func() {
		chainMu.Lock()
		activeSess = prev
		chainMu.Unlock()
	})
}

// TestReadChainCounters_DoesNotWedgeStopChain is the H4 regression test.
// readChainCounters must not hold chainMu across the xray stats RPC: while
// one goroutine is wedged inside a hung Counters() call, OpStopChain must
// still be able to acquire chainMu and complete. Before the fix,
// readChainCounters holds chainMu for the entire (here: unbounded, since we
// pass context.Background() exactly as status.go's real caller chain does
// upstream of any deadline) Counters call, so StopChain queues behind it
// forever and this test times out.
func TestReadChainCounters_DoesNotWedgeStopChain(t *testing.T) {
	fake := &ctxBlockingStatsClient{started: make(chan struct{})}
	withActiveSess(t, &chainState{sessionID: "wedge-test", xrayAPI: fake})

	go func() { _, _, _ = readChainCounters(context.Background()) }()

	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("fake Counters was never invoked")
	}

	stop := NewStopChainHandler()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := stop(context.Background(), nil); err != nil {
			t.Errorf("StopChain: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OpStopChain wedged behind an in-flight counters read (chainMu held across the gRPC call)")
	}
}

// TestReadChainCounters_BoundsCountersCallAt1s is the H4 timeout-bound
// assertion: readChainCounters must wrap the caller's ctx (which, per
// status.go's real call chain, carries no deadline of its own) in a bounded
// context.WithTimeout so a wedged xray can't block a single status poll
// forever either. Falls back to the cached counters on timeout.
func TestReadChainCounters_BoundsCountersCallAt1s(t *testing.T) {
	fake := &ctxBlockingStatsClient{}
	withActiveSess(t, &chainState{sessionID: "timeout-test", xrayAPI: fake, cachedUp: 111, cachedDown: 222})

	start := time.Now()
	up, down, ok := readChainCounters(context.Background())
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("ok=false, want true (chain active)")
	}
	if up != 111 || down != 222 {
		t.Fatalf("up=%d down=%d, want cached 111/222 on timeout", up, down)
	}
	if elapsed < 900*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("elapsed=%v, want ~1s (bounded by context.WithTimeout, not unbounded)", elapsed)
	}
}
