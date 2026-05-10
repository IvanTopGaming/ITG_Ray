package chainctl

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/sysproxy"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// memStore is a tiny in-memory ServerStore for tests. The bindings package
// uses Load/Save free functions; chainctl needs only Get(id), so we don't
// reuse that adapter here.
type memStore struct {
	mu sync.Mutex
	m  map[string]*server.Server
}

func newMemStore(seed ...*server.Server) *memStore {
	s := &memStore{m: make(map[string]*server.Server, len(seed))}
	for _, srv := range seed {
		s.m[srv.ID] = srv
	}
	return s
}

func (s *memStore) Get(id string) (*server.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id], nil
}

// staticNetwork builds a chainctl.Deps.Network closure that returns the
// supplied config.Network on every call. Used by Tier 2b tests to feed
// non-default values into bringUp without spinning up a config.FileStore.
func staticNetwork(net config.Network) func() (config.Network, error) {
	return func() (config.Network, error) { return net, nil }
}

// errNetwork builds a Network closure that always returns err. Pinned
// to the spec wording so chain:error assertions can match on
// "config.Load: <err>" without coupling to the closure's call site.
func errNetwork(err error) func() (config.Network, error) {
	return func() (config.Network, error) { return config.Network{}, err }
}

// fixtureServer is the canonical seed used across the suite.
func fixtureServer() *server.Server {
	return &server.Server{
		ID:     "a",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless: vless.Config{
			Address:   "127.0.0.1",
			Port:      443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
			Security:  vless.SecurityNone,
		},
	}
}

// setup wires a Controller against the fake helper and an in-memory
// store seeded with one server (id "a").
func setup(t *testing.T) (*Controller, *fakeHelper, *hub.Hub, *memStore) {
	t.Helper()
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake()
	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      fh,
		Sysproxy:    sysproxy.New(),
		Hub:         h,
	})
	return c, fh, h, store
}

// waitFor polls fn until it returns true or the deadline elapses.
func waitFor(t *testing.T, d time.Duration, fn func() bool) {
	t.Helper()
	require.Eventually(t, fn, d, 10*time.Millisecond)
}

// waitForEvent drains rcv until it sees an event with name == want or
// the deadline elapses. Returns the matching event or fails the test.
func waitForEvent(t *testing.T, rcv <-chan hub.Event, want string, d time.Duration) hub.Event {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %q event", want)
			return hub.Event{}
		case e, ok := <-rcv:
			if !ok {
				t.Fatalf("hub closed before %q arrived", want)
				return hub.Event{}
			}
			if e.Name == want {
				return e
			}
		}
	}
}

// waitForVpnStatus waits up to timeout for a vpn:status event whose
// "status" payload equals want. It drains intermediate states
// (e.g. connecting -> connected) under a single overall deadline,
// avoiding the re-arming-per-iteration flake risk of looping
// waitForEvent calls with fresh deadlines.
func waitForVpnStatus(t *testing.T, rcv <-chan hub.Event, want string, timeout time.Duration) hub.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Until(deadline) > 0 {
		ev := waitForEvent(t, rcv, hub.EventVPNStatus, time.Until(deadline))
		if ev.Payload["status"] == want {
			return ev
		}
	}
	t.Fatalf("vpn:status %q not seen within %v", want, timeout)
	return hub.Event{}
}

func TestController_Start_Stop_TUN_HappyPath(t *testing.T) {
	c, fh, h, _ := setup(t)
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))
	// Wait for the connected status event — it's published after bringUp
	// + saveSession, so observing it implies last-session.json is on disk.
	e := waitForEvent(t, rcv, hub.EventVPNStatus, time.Second)
	for e.Payload["status"] != string(hub.StatusConnected) {
		e = waitForEvent(t, rcv, hub.EventVPNStatus, time.Second)
	}

	// last-session.json should now exist.
	rec, err := loadSession(c.d.DataDir)
	require.NoError(t, err)
	require.Equal(t, "a", rec.ServerID)
	require.Equal(t, string(ModeTUN), rec.Mode)

	// Status reports connected.
	st, srv, mode := c.Status()
	require.Equal(t, hub.StatusConnected, st)
	require.NotNil(t, srv)
	require.Equal(t, "a", srv.ID)
	require.Equal(t, ModeTUN, mode)

	// Verify the bringup ordering: snapshot → tun → start → route → dns.
	fh.mu.Lock()
	calls := append([]string(nil), fh.calls...)
	fh.mu.Unlock()
	require.Contains(t, calls, "RouteSnapshot")
	require.Contains(t, calls, "TunCreate")
	require.Contains(t, calls, "StartChain")
	require.Contains(t, calls, "RouteAdd")
	require.Contains(t, calls, "DnsSet")

	require.NoError(t, c.Stop(context.Background()))
	waitFor(t, time.Second, func() bool {
		fh.mu.Lock()
		defer fh.mu.Unlock()
		return !fh.running
	})

	// Session cleared.
	rec, err = loadSession(c.d.DataDir)
	require.NoError(t, err)
	require.Empty(t, rec.ServerID)
}

func TestController_Start_StartChainFails_Rollback(t *testing.T) {
	c, fh, h, _ := setup(t)
	fh.failOn = "StartChain"
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))

	e := waitForEvent(t, rcv, hub.EventChainError, time.Second)
	require.Equal(t, "bringup_failed", e.Payload["kind"])

	// After the bringup-failed event, controller should reset to idle.
	waitFor(t, time.Second, func() bool {
		st, _, _ := c.Status()
		return st == hub.StatusIdle
	})

	fh.mu.Lock()
	defer fh.mu.Unlock()
	require.Contains(t, fh.calls, "RouteRestore", "rollback should restore routes")
	require.Contains(t, fh.calls, "TunDestroy", "rollback should destroy TUN")
	require.False(t, fh.running, "chain should not be running after rollback")
}

func TestController_Reconcile_AfterCrash(t *testing.T) {
	c, fh, h, _ := setup(t)
	// Pretend the helper survived a GUI crash and is still running.
	fh.mu.Lock()
	fh.running = true
	fh.mu.Unlock()
	// Seed a last-session record so Reconcile can rebind the picker.
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{
		ServerID: "a",
		Mode:     string(ModeTUN),
		At:       time.Now(),
	}))

	rcv := h.Subscribe(8)
	defer h.Unsubscribe(rcv)

	c.Reconcile(context.Background())

	// Reconcile only pre-fills the picker; it does NOT emit a connected
	// event because helper.OpServiceStatus cannot reliably distinguish
	// "service alive" from "chain alive". The user must reconnect explicitly.
	c.mu.Lock()
	srv := c.current
	mode := c.mode
	cancel := c.cancel
	c.mu.Unlock()
	require.NotNil(t, srv, "Reconcile should pre-fill current server from session")
	require.Equal(t, "a", srv.ID)
	require.Equal(t, ModeTUN, mode)
	require.Nil(t, cancel, "Reconcile must not claim chain ownership")

	// No status event should fire from Reconcile; drain briefly to confirm.
	select {
	case e := <-rcv:
		if e.Name == hub.EventVPNStatus {
			t.Fatalf("unexpected vpn:status event from Reconcile: %v", e.Payload)
		}
	case <-time.After(100 * time.Millisecond):
		// quiet — expected
	}
}

func TestController_Stop_IsIdempotent(t *testing.T) {
	c, _, _, _ := setup(t)
	// Calling Stop on a never-started controller must not panic and
	// must not emit a transition.
	require.NoError(t, c.Stop(context.Background()))
	require.NoError(t, c.Stop(context.Background()))
	st, srv, mode := c.Status()
	require.Equal(t, hub.StatusIdle, st)
	require.Nil(t, srv)
	require.Equal(t, Mode(""), mode)
}

// TestController_ActiveServerID_IdleAndConnected pins the contract used
// by bindings.ServersService.Remove (Tier 6 Task 5): "" while idle, the
// active server's id while connected.
func TestController_ActiveServerID_IdleAndConnected(t *testing.T) {
	c, _, h, _ := setup(t)
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	if got := c.ActiveServerID(); got != "" {
		t.Fatalf("idle ActiveServerID = %q, want \"\"", got)
	}

	if err := c.Start(context.Background(), "a", ModeTUN); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)

	if got := c.ActiveServerID(); got != "a" {
		t.Fatalf("connected ActiveServerID = %q, want %q", got, "a")
	}
}

func TestController_Start_Stop_SysProxy_HappyPath(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake()
	fsp := &fakeSysproxy{}
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

	// Wait for the connected status — bringUp emits it after sysproxy.Set.
	e := waitForEvent(t, rcv, hub.EventVPNStatus, time.Second)
	for e.Payload["status"] != string(hub.StatusConnected) {
		e = waitForEvent(t, rcv, hub.EventVPNStatus, time.Second)
	}

	// Sysproxy bringup invariants: Set called at least once, helper started.
	require.GreaterOrEqual(t, fsp.SetCalls(), 1, "sysproxy.Set must be called during bringUp")
	fh.mu.Lock()
	calls := append([]string(nil), fh.calls...)
	fh.mu.Unlock()
	require.Contains(t, calls, "StartChain", "helper.StartChain must run in sysproxy mode")
	// Sysproxy mode must NOT touch TUN/route ops.
	require.NotContains(t, calls, "RouteSnapshot")
	require.NotContains(t, calls, "TunCreate")
	require.NotContains(t, calls, "RouteAdd")
	require.NotContains(t, calls, "DnsSet")

	// Stop must clear the sysproxy and stop the chain.
	require.NoError(t, c.Stop(context.Background()))
	waitFor(t, time.Second, func() bool {
		fh.mu.Lock()
		defer fh.mu.Unlock()
		return !fh.running
	})
	require.GreaterOrEqual(t, fsp.ClearCalls(), 1, "sysproxy.Clear must be called during tearDown")
}

// TestBringUpPassesSysProxyModeToHelper pins that bringUp threads the
// requested Mode through to HelperClient.StartChain — Task 3 of Tier 4.5
// extended the interface so the helper can skip TUN-only steps when the
// GUI requests sysproxy mode. Without this assertion, a future refactor
// could drop the mode argument at the bringUp call site and the helper
// would silently fall back to TUN behavior.
func TestBringUpPassesSysProxyModeToHelper(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	fh := newFake()
	fsp := &fakeSysproxy{}
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
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)

	fh.mu.Lock()
	got := fh.gotMode
	fh.mu.Unlock()
	require.Equal(t, string(ModeSysProxy), got, "StartChain must receive ModeSysProxy when bringUp runs in sysproxy mode")
}

// TestStart_PassesNetworkValuesToSysproxy pins that the config-driven
// SOCKS/HTTP ports on Network.SysProxy flow into the sysproxy.Manager
// argument during a sysproxy-mode bringup. This is the Tier 2b
// runtime-wiring smoke test — without it, the new accessor could be
// silently ignored and the sysproxy would still get the old hardcoded
// "127.0.0.1:1080" / no-HTTP literal pair.
func TestStart_PassesNetworkValuesToSysproxy(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	spy := &fakeSysproxy{}
	net := config.Defaults().Network
	net.SysProxy.SOCKSPort = 1090
	net.SysProxy.HTTPPort = 8889

	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      newFake(),
		Sysproxy:    spy,
		Hub:         h,
		Network:     staticNetwork(net),
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	// bringUp is async; wait for the connected event so we know
	// sysproxy.Set has run before reading spy.last.
	_ = waitForVpnStatus(t, rcv, string(hub.StatusConnected), 2*time.Second)

	spy.mu.Lock()
	got := spy.last
	spy.mu.Unlock()
	require.Equal(t, sysproxy.Settings{Socks: "127.0.0.1:1090", HTTP: "127.0.0.1:8889"}, got)
}

// TestStart_NetworkLoaderError_PublishesChainError pins the failure
// surface for a corrupt config.json: bringUp must emit chain:error with
// "config.Load: <err>" and bounce back to idle. Without this contract
// a partially-init Controller could wedge the UI on connecting.
func TestStart_NetworkLoaderError_PublishesChainError(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)

	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      newFake(),
		Sysproxy:    sysproxy.New(),
		Hub:         h,
		Network:     errNetwork(errors.New("disk corrupt")),
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	// Start kicks off bringUp on a goroutine; the loader error surfaces
	// via chain:error rather than the synchronous Start return.
	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))
	ev := waitForEvent(t, rcv, hub.EventChainError, 2*time.Second)
	require.Contains(t, ev.Payload["message"].(string), "config.Load")
}

// TestStart_ConnectedEvent_PayloadIncludesNetwork pins that the
// vpn:status connected event carries a "network" key projecting the
// config.Network the runtime ACTUALLY used during this Connect cycle.
// Frontend (Tier 2b Task 7) snapshots from this payload to drive the
// reconnect-required pill, avoiding the edit-during-connect race
// where a user could change a Network field between chainctl's
// Network() read and the publish.
//
// Subscribe BEFORE Start: the connected event is published from the
// bringUp goroutine, and a late subscribe could miss it.
func TestStart_ConnectedEvent_PayloadIncludesNetwork(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	net := config.Defaults().Network
	net.SysProxy.SOCKSPort = 1090

	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      newFake(),
		Sysproxy:    &fakeSysproxy{},
		Hub:         h,
		Network:     staticNetwork(net),
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))

	ev := waitForVpnStatus(t, rcv, string(hub.StatusConnected), 2*time.Second)
	netView, ok := ev.Payload["network"]
	require.True(t, ok, "connected event must carry network payload")
	require.NotNil(t, netView)
	view, ok := netView.(map[string]any)
	require.True(t, ok, "network payload must be map[string]any")
	require.EqualValues(t, 1090, view["socksPort"])
}

// TestStart_MTUOutOfRange_PassesRawToBuildConfigs pins the chainctl/
// configbuilder split: chainctl forwards the raw Network to the
// builder; clamping on the builder side lands in Task 5. Today the
// chainctl-side ClampMTU helper is exercised directly in
// network_test.go and is unused at the helper boundary because
// HelperAdapter.TunCreate is a no-op in production.
func TestStart_MTUOutOfRange_PassesRawToBuildConfigs(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)
	captured := config.Network{}
	builder := func(_ *server.Server, _ Mode, net config.Network) ([]byte, []byte, error) {
		captured = net
		return []byte("{}"), []byte("{}"), nil
	}
	net := config.Defaults().Network
	net.TUN.MTU = 100 // out of [576, 9000]; chainctl passes it through

	c := New(&Deps{
		DataDir:      dir,
		ServerStore:  store,
		Helper:       newFake(),
		Sysproxy:     sysproxy.New(),
		Hub:          h,
		BuildConfigs: builder,
		Network:      staticNetwork(net),
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))
	_ = waitForVpnStatus(t, rcv, string(hub.StatusConnected), 2*time.Second)
	require.Equal(t, 100, captured.TUN.MTU)
}

// fakeSysproxy records Set/Clear/IsSet invocations so tests can assert
// the bringUp/tearDown sequence drove the sysproxy.Manager. The last
// Settings argument is captured for Tier 2b assertions that pin the
// config-driven SOCKS/HTTP ports landing on the manager. Safe for
// concurrent use because the Controller's bringUp runs on a worker
// goroutine while the test asserts from the main goroutine.
type fakeSysproxy struct {
	mu         sync.Mutex
	setCalls   int
	clearCalls int
	isSetCalls int
	on         bool
	last       sysproxy.Settings
}

func (f *fakeSysproxy) Set(s sysproxy.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCalls++
	f.on = true
	f.last = s
	return nil
}

func (f *fakeSysproxy) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearCalls++
	f.on = false
	return nil
}

func (f *fakeSysproxy) IsSet() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isSetCalls++
	return f.on, nil
}

func (f *fakeSysproxy) SetCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.setCalls
}

func (f *fakeSysproxy) ClearCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clearCalls
}
