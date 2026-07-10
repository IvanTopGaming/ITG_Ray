package bindings

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/sysproxy"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// runFakeHelper is a tiny in-memory chainctl.HelperClient. The bindings
// package can't import the chainctl test fake (test files don't export),
// so we redefine the minimum surface needed to drive Connect through
// bringUp without touching real OS resources.
type runFakeHelper struct {
	mu      sync.Mutex
	running bool
}

func (f *runFakeHelper) StartChain(_ context.Context, _, _ []byte, _ chainctl.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = true
	return nil
}

func (f *runFakeHelper) StopChain(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	return nil
}

func (f *runFakeHelper) TunCreate(_ context.Context, _, _ string) error { return nil }
func (f *runFakeHelper) TunDestroy(_ context.Context) error             { return nil }
func (f *runFakeHelper) RouteSnapshot(_ context.Context) error          { return nil }
func (f *runFakeHelper) RouteAdd(_ context.Context, _ string) error     { return nil }
func (f *runFakeHelper) RouteRestore(_ context.Context) error           { return nil }
func (f *runFakeHelper) DnsSet(_ context.Context, _ []string) error     { return nil }
func (f *runFakeHelper) DnsRestore(_ context.Context) error             { return nil }

func (f *runFakeHelper) ServiceStatus(_ context.Context) (chainctl.ChainState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return chainctl.ChainState{Running: f.running}, nil
}

// runMemStore is a minimal chainctl.ServerStore that resolves IDs from
// an in-memory map. Mirrors the chainctl_test memStore.
type runMemStore struct{ m map[string]*server.Server }

func (s runMemStore) Get(id string) (*server.Server, error) { return s.m[id], nil }

// runFakeSysproxy is a no-op sysproxy.Manager so bringUp's sysproxy.Set
// call succeeds in tests without touching real OS proxy settings.
type runFakeSysproxy struct{}

func (runFakeSysproxy) Set(sysproxy.Settings) error { return nil }
func (runFakeSysproxy) Clear() error                { return nil }
func (runFakeSysproxy) IsSet() (bool, error)        { return false, nil }

// setupRun wires a RunService against a fake helper + in-memory server
// store. Returns the service plus the underlying Controller (in case the
// caller wants to assert chainctl-level state).
func setupRun(t *testing.T) (*RunService, *chainctl.Controller, *hub.Hub) {
	t.Helper()
	dir := t.TempDir()
	srv := &server.Server{
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
	store := runMemStore{m: map[string]*server.Server{"a": srv}}
	h := hub.New()
	t.Cleanup(h.Close)
	ctrl := chainctl.New(&chainctl.Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      &runFakeHelper{},
		Sysproxy:    runFakeSysproxy{},
		Hub:         h,
	})
	rs := NewRunService(RunDeps{Chain: ctrl, Hub: h})
	return rs, ctrl, h
}

// TestRun_ConnectStart asserts Connect drives the chain to "connected"
// via the hub event stream.
func TestRun_ConnectStart(t *testing.T) {
	rs, _, h := setupRun(t)
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	require.NoError(t, rs.Connect("a", "sysproxy"))

	// Wait for the connected status — emitted after bringUp completes.
	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for vpn:status connected event")
		case e, ok := <-rcv:
			require.True(t, ok, "hub closed early")
			if e.Name == hub.EventVPNStatus && e.Payload["status"] == string(hub.StatusConnected) {
				return
			}
		}
	}
}

// TestRun_DisconnectIdempotent asserts Disconnect on an idle controller
// is a no-op (no panic, no error, no transitions).
func TestRun_DisconnectIdempotent(t *testing.T) {
	rs, _, _ := setupRun(t)
	require.NoError(t, rs.Disconnect())
	require.NoError(t, rs.Disconnect())
	st := rs.GetStatus()
	require.Equal(t, string(hub.StatusIdle), st["status"])
}

// TestRun_GetStatusIdle asserts the initial status snapshot reports
// "idle" with empty mode and no server fields.
func TestRun_GetStatusIdle(t *testing.T) {
	rs, _, _ := setupRun(t)
	st := rs.GetStatus()
	require.Equal(t, string(hub.StatusIdle), st["status"])
	require.Equal(t, "", st["mode"])
	_, hasID := st["serverId"]
	require.False(t, hasID, "idle status must not carry serverId")
	_, hasName := st["serverName"]
	require.False(t, hasName, "idle status must not carry serverName")
}
