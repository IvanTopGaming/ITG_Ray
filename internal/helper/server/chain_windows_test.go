//go:build windows

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartChainArgs_DecodeIncludesMode(t *testing.T) {
	raw := []byte(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"tun_name":"t","mode":"sysproxy"}`)
	var a StartChainArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Mode != "sysproxy" {
		t.Fatalf("Mode=%q, want sysproxy", a.Mode)
	}
}

func TestStartChainHandler_InvalidMode(t *testing.T) {
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"tun_name":"t","mode":"bogus"}`)
	_, err := h(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Fatalf("err=%v, want 'invalid mode'", err)
	}
}

func TestStartChainHandler_TunModeRequiresTunName(t *testing.T) {
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"mode":"tun"}`)
	_, err := h(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "tun_name required") {
		t.Fatalf("err=%v, want tun_name required", err)
	}
}

func TestStartChainHandler_SysProxyAcceptsEmptyTunName(t *testing.T) {
	// Validation must accept sysproxy mode without a tun_name. The handler
	// will fail later (no real binaries on test box), but the validator gate
	// must pass.
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"mode":"sysproxy"}`)
	_, err := h(context.Background(), args)
	if err == nil {
		return // unexpected success but not the failure we're testing
	}
	if strings.Contains(err.Error(), "tun_name required") {
		t.Fatalf("err=%v: validator should accept sysproxy without tun_name", err)
	}
}

type slowStopper struct {
	delay time.Duration
	err   error
}

func (s *slowStopper) Stop(_ time.Duration) error {
	time.Sleep(s.delay)
	return s.err
}

func TestStopBoth_RunsInParallel(t *testing.T) {
	a := &slowStopper{delay: 800 * time.Millisecond}
	b := &slowStopper{delay: 800 * time.Millisecond}
	start := time.Now()
	xerr, serr := stopBoth(2*time.Second, a, b)
	elapsed := time.Since(start)
	if xerr != nil || serr != nil {
		t.Fatalf("errs: %v %v", xerr, serr)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("elapsed=%v, want < 1.5s (parallel, not sequential)", elapsed)
	}
}

func TestStopBoth_NilSafe(t *testing.T) {
	xerr, serr := stopBoth(time.Second, nil, nil)
	if xerr != nil || serr != nil {
		t.Fatalf("nil cores: %v %v", xerr, serr)
	}
}

func TestStopBoth_PropagatesErrors(t *testing.T) {
	a := &slowStopper{err: errors.New("xray-fail")}
	b := &slowStopper{err: errors.New("sb-fail")}
	xerr, serr := stopBoth(time.Second, a, b)
	if xerr == nil || xerr.Error() != "xray-fail" {
		t.Fatalf("xerr=%v", xerr)
	}
	if serr == nil || serr.Error() != "sb-fail" {
		t.Fatalf("serr=%v", serr)
	}
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
