//go:build !windows

package chainctl

import (
	"context"
	"errors"
	"testing"
)

type recordingClient struct {
	started bool
	stopped bool
	label   string
}

func (r *recordingClient) StartChain(context.Context, []byte, []byte, Mode) error {
	r.started = true
	return nil
}
func (r *recordingClient) StopChain(context.Context) error                 { r.stopped = true; return nil }
func (r *recordingClient) TunCreate(context.Context, string, string) error { return errors.New("x") }
func (r *recordingClient) TunDestroy(context.Context) error                { return nil }
func (r *recordingClient) RouteSnapshot(context.Context) error             { return errors.New("x") }
func (r *recordingClient) RouteAdd(context.Context, string) error          { return errors.New("x") }
func (r *recordingClient) RouteRestore(context.Context) error              { return nil }
func (r *recordingClient) DnsSet(context.Context, []string) error          { return errors.New("x") }
func (r *recordingClient) DnsRestore(context.Context) error                { return nil }
func (r *recordingClient) ServiceStatus(context.Context) (ChainState, error) {
	return ChainState{Running: r.started && !r.stopped}, nil
}

func TestModeRouting_TUNGoesToDaemon(t *testing.T) {
	core := &recordingClient{label: "core"}
	daemon := &recordingClient{label: "daemon"}
	rc := newModeRoutingHelperClient(core, daemon)

	if err := rc.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeTUN); err != nil {
		t.Fatalf("StartChain: %v", err)
	}
	if !daemon.started || core.started {
		t.Fatal("TUN mode must route to daemon, not core")
	}
	_ = rc.StopChain(context.Background())
	if !daemon.stopped {
		t.Fatal("StopChain must reach the active (daemon) backend")
	}
}

func TestModeRouting_SysProxyGoesToCore(t *testing.T) {
	core := &recordingClient{label: "core"}
	daemon := &recordingClient{label: "daemon"}
	rc := newModeRoutingHelperClient(core, daemon)

	if err := rc.StartChain(context.Background(), []byte(`{}`), []byte(`{}`), ModeSysProxy); err != nil {
		t.Fatalf("StartChain: %v", err)
	}
	if !core.started || daemon.started {
		t.Fatal("SysProxy mode must route to core, not daemon")
	}
}

// statusFake is a HelperClient whose ServiceStatus is fully scripted so the
// boot-time daemon probe can be exercised without a live socket.
type statusFake struct {
	recordingClient
	statusRunning bool
	statusErr     error
}

func (s *statusFake) ServiceStatus(context.Context) (ChainState, error) {
	if s.statusErr != nil {
		return ChainState{}, s.statusErr
	}
	return ChainState{Running: s.statusRunning}, nil
}

// At fresh boot (active==nil) the routing client must probe the daemon and,
// if it reports a surviving TUN chain, adopt it — so a later StopChain
// reaches the daemon. Guards Phase B acceptance criterion 5 (reconnect after
// GUI/bridge restart).
func TestModeRouting_AdoptsDaemonOnBootWhenRunning(t *testing.T) {
	core := &statusFake{}
	daemon := &statusFake{statusRunning: true}
	rc := newModeRoutingHelperClient(core, daemon)

	st, err := rc.ServiceStatus(context.Background())
	if err != nil {
		t.Fatalf("ServiceStatus: %v", err)
	}
	if !st.Running {
		t.Fatal("boot probe must report the daemon's running chain")
	}
	if err := rc.StopChain(context.Background()); err != nil {
		t.Fatalf("StopChain: %v", err)
	}
	if !daemon.stopped {
		t.Fatal("adoption must latch active to the daemon so StopChain reaches it")
	}
	if core.stopped {
		t.Fatal("StopChain must not reach the core after daemon adoption")
	}
}

// A daemon that isn't installed/reachable (dial error) must surface as an
// idle ChainState with a nil error — a SysProxy-only machine must never see
// a Reconcile error from the boot probe.
func TestModeRouting_IdleWhenDaemonUnreachable(t *testing.T) {
	daemon := &statusFake{statusErr: errors.New("dial helper daemon: connection refused")}
	rc := newModeRoutingHelperClient(&statusFake{}, daemon)

	st, err := rc.ServiceStatus(context.Background())
	if err != nil {
		t.Fatalf("unreachable daemon must be idle, not an error: %v", err)
	}
	if st.Running {
		t.Fatal("unreachable daemon must report idle (not running)")
	}
}

func TestModeRouting_PreStartOpsAreNoops(t *testing.T) {
	rc := newModeRoutingHelperClient(&recordingClient{}, &recordingClient{})
	if err := rc.RouteSnapshot(context.Background()); err != nil {
		t.Fatalf("RouteSnapshot pre-StartChain must be a no-op, got %v", err)
	}
	if err := rc.TunCreate(context.Background(), "n", "c"); err != nil {
		t.Fatalf("TunCreate pre-StartChain must be a no-op, got %v", err)
	}
}
