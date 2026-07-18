package chainctl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logtest"
	"github.com/stretchr/testify/require"
)

// newTeardownTestController wires a Controller against a fake helper whose
// DnsRestore/RouteRestore/StopChain/TunDestroy calls all fail, so tearDown's
// swallowed-error paths are exercised.
func newTeardownTestController(t *testing.T) *Controller {
	t.Helper()
	c, fh, _, _ := setup(t)
	fh.dnsRestoreErr = errFail
	fh.routeRestoreErr = errFail
	fh.stopErr = errFail
	fh.tunDestroyErr = errFail
	return c
}

func TestTearDown_LogsSwallowedFailures(t *testing.T) {
	buf := logtest.Capture(t)
	c := newTeardownTestController(t)
	c.tearDown(context.Background(), ModeTUN)
	out := buf.String()
	if !strings.Contains(out, "[chainctl]") {
		t.Fatalf("teardown failures not scoped: %q", out)
	}
	// at least the stop-chain swallow must be surfaced as a warn
	if !strings.Contains(out, "teardown") {
		t.Fatalf("no teardown warn emitted: %q", out)
	}
}

// TestTearDown_ContinuesPastEachSwallowedFailure pins the best-effort
// contract: tearDown must still attempt every subsequent step even though
// an earlier step in the sequence failed. This is the exact behavior the
// new logging must NOT regress — no early returns are allowed.
func TestTearDown_ContinuesPastEachSwallowedFailure(t *testing.T) {
	c, fh, _, _ := setup(t)
	fh.dnsRestoreErr = errFail
	fh.routeRestoreErr = errFail
	fh.stopErr = errFail
	fh.tunDestroyErr = errFail

	c.tearDown(context.Background(), ModeTUN)

	fh.mu.Lock()
	calls := append([]string(nil), fh.calls...)
	fh.mu.Unlock()
	require.Contains(t, calls, "DnsRestore")
	require.Contains(t, calls, "RouteRestore")
	require.Contains(t, calls, "StopChain")
	require.Contains(t, calls, "TunDestroy")
}

// TestTearDown_SysproxyClearFailureLogsWarn covers the ModeSysProxy branch
// (Sysproxy.Clear), the one tearDown swallow site not exercised by the TUN
// tests above.
func TestTearDown_SysproxyClearFailureLogsWarn(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	fsp := &fakeSysproxy{clearErr: errFail}
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      newFake(),
		Sysproxy:    fsp,
	})

	buf := logtest.Capture(t)
	c.tearDown(context.Background(), ModeSysProxy)
	out := buf.String()
	if !strings.Contains(out, "[chainctl]") || !strings.Contains(out, "sysproxy") {
		t.Fatalf("missing sysproxy clear warn: %q", out)
	}
	require.GreaterOrEqual(t, fsp.ClearCalls(), 1)
}

// TestBringUp_StartChainRollbackLogsSwallowedFailures pins one of the
// bringUp rollback branches: when StartChain fails in TUN mode, the
// TunDestroy+RouteRestore rollback calls are still best-effort (swallowed
// for control-flow purposes) but must now surface at Warn, and the
// original StartChain error returned to the caller must be unchanged.
func TestBringUp_StartChainRollbackLogsSwallowedFailures(t *testing.T) {
	c, fh, h, _ := setup(t)
	fh.failOn = "StartChain"
	fh.tunDestroyErr = errFail
	fh.routeRestoreErr = errFail
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	buf := logtest.Capture(t)
	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))

	e := waitForEvent(t, rcv, hub.EventChainError, time.Second)
	require.Equal(t, "bringup_failed", e.Payload["kind"])
	require.Contains(t, e.Payload["message"].(string), "StartChain: forced failure",
		"rollback logging must not change the error surfaced to the caller")

	fh.mu.Lock()
	calls := append([]string(nil), fh.calls...)
	fh.mu.Unlock()
	require.Contains(t, calls, "TunDestroy", "rollback must still attempt tun destroy despite the swallow")
	require.Contains(t, calls, "RouteRestore", "rollback must still attempt route restore despite the swallow")

	out := buf.String()
	if !strings.Contains(out, "[chainctl]") || !strings.Contains(out, "rollback") {
		t.Fatalf("missing rollback warn logs: %q", out)
	}
}
