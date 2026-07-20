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

// TestController_BringUp_SysproxySetFails_ClearsProxy pins backend-review
// finding 1 (CRITICAL): a failed sysproxy.Set during a sysproxy-mode
// Connect must not leave the OS proxy enabled and pointing at a port
// StopChain just killed. bringUp's sysproxy-mode error branch must call
// Sysproxy.Clear() alongside Helper.StopChain, mirroring every other
// rollback branch already in bringUp (TUN mode's TunCreate/RouteAdd/DnsSet
// failures all roll back their own side effects).
func TestController_BringUp_SysproxySetFails_ClearsProxy(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake()
	fsp := &fakeSysproxy{setErr: errFail}
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      fh,
		Sysproxy:    fsp,
		Hub:         h,
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))

	ev := waitForEvent(t, rcv, hub.EventChainError, time.Second)
	require.Equal(t, "bringup_failed", ev.Payload["kind"])
	require.Contains(t, ev.Payload["message"].(string), "sysproxy.Set",
		"rollback must not change the error surfaced to the caller")

	waitFor(t, time.Second, func() bool { return fsp.ClearCalls() >= 1 })

	fh.mu.Lock()
	calls := append([]string(nil), fh.calls...)
	fh.mu.Unlock()
	require.Contains(t, calls, "StopChain", "helper chain must still be stopped on sysproxy.Set failure")
}

// TestController_Reconcile_StaleSysproxy_NotRunning_Clears pins finding 2
// (HIGH): when Reconcile finds the helper chain is not running (e.g. the
// previous GUI process and the helper's managed chain both went down
// together while the last session was ModeSysProxy), it must clear any
// stale OS proxy left behind rather than silently no-op'ing.
func TestController_Reconcile_StaleSysproxy_NotRunning_Clears(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake() // running defaults false: "chain not running" path
	fsp := &fakeSysproxy{on: true}
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      fh,
		Sysproxy:    fsp,
		Hub:         h,
	})
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{
		ServerID: "a",
		Mode:     string(ModeSysProxy),
		At:       time.Now(),
	}))

	c.Reconcile(context.Background())

	require.GreaterOrEqual(t, fsp.ClearCalls(), 1,
		"Reconcile must clear a stale sysproxy left by a session whose chain died while the GUI was down")
}

// TestController_Reconcile_StaleTUNSession_NotRunning_DoesNotTouchSysproxy
// pins the negative case: a stale TUN-mode session has no sysproxy state to
// clean up, so the finding-2 fix must not call Sysproxy.Clear() at all for
// ModeTUN sessions.
func TestController_Reconcile_StaleTUNSession_NotRunning_DoesNotTouchSysproxy(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake() // running defaults false
	fsp := &fakeSysproxy{}
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      fh,
		Sysproxy:    fsp,
		Hub:         h,
	})
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{
		ServerID: "a",
		Mode:     string(ModeTUN),
		At:       time.Now(),
	}))

	c.Reconcile(context.Background())

	require.Equal(t, 0, fsp.ClearCalls(), "Reconcile must not touch sysproxy for a stale TUN-mode session")
}

// TestController_Reconcile_StaleSysproxy_StillSetAfterClear_LogsWarn pins
// finding 5 (LOW): after Clear() runs, the caller must re-verify with
// IsSet() and log a Warn if the OS proxy is still enabled — Clear() can
// itself swallow a registry-write failure and report success anyway (see
// sysproxy_windows.go's Clear), so nothing else in the app would otherwise
// ever notice.
func TestController_Reconcile_StaleSysproxy_StillSetAfterClear_LogsWarn(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake()
	fsp := &fakeSysproxy{on: true, clearIneffective: true}
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      fh,
		Sysproxy:    fsp,
		Hub:         h,
	})
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{
		ServerID: "a",
		Mode:     string(ModeSysProxy),
		At:       time.Now(),
	}))

	buf := logtest.Capture(t)
	c.Reconcile(context.Background())
	out := buf.String()

	require.GreaterOrEqual(t, fsp.ClearCalls(), 1)
	require.GreaterOrEqual(t, fsp.IsSetCalls(), 1, "must verify with IsSet() after Clear()")
	if !strings.Contains(out, "[chainctl]") || !strings.Contains(out, "still") {
		t.Fatalf("missing still-set warn after clear: %q", out)
	}
}
