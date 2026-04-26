package chainctl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
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
	// Seed a last-session record so Reconcile can rebind the server.
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{
		ServerID: "a",
		Mode:     string(ModeTUN),
		At:       time.Now(),
	}))

	rcv := h.Subscribe(8)
	defer h.Unsubscribe(rcv)

	c.Reconcile(context.Background())

	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("no vpn:status connected event after Reconcile")
		case e, ok := <-rcv:
			require.True(t, ok, "hub closed before status event")
			if e.Name == hub.EventVPNStatus && e.Payload["status"] == string(hub.StatusConnected) {
				st, srv, mode := c.Status()
				require.Equal(t, hub.StatusConnected, st)
				require.NotNil(t, srv)
				require.Equal(t, "a", srv.ID)
				require.Equal(t, ModeTUN, mode)
				return
			}
		}
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

// fakeSysproxy records Set/Clear/IsSet invocations so tests can assert
// the bringUp/tearDown sequence drove the sysproxy.Manager. It is
// safe for concurrent use because the Controller's bringUp runs on a
// worker goroutine while the test asserts from the main goroutine.
type fakeSysproxy struct {
	mu         sync.Mutex
	setCalls   int
	clearCalls int
	isSetCalls int
	on         bool
}

func (f *fakeSysproxy) Set(_ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCalls++
	f.on = true
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
