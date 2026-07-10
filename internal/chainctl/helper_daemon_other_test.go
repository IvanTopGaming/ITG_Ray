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

func (r *recordingClient) StartChain(context.Context, []byte, []byte, Mode) error { r.started = true; return nil }
func (r *recordingClient) StopChain(context.Context) error                        { r.stopped = true; return nil }
func (r *recordingClient) TunCreate(context.Context, string, string) error        { return errors.New("x") }
func (r *recordingClient) TunDestroy(context.Context) error                       { return nil }
func (r *recordingClient) RouteSnapshot(context.Context) error                    { return errors.New("x") }
func (r *recordingClient) RouteAdd(context.Context, string) error                 { return errors.New("x") }
func (r *recordingClient) RouteRestore(context.Context) error                     { return nil }
func (r *recordingClient) DnsSet(context.Context, []string) error                 { return errors.New("x") }
func (r *recordingClient) DnsRestore(context.Context) error                       { return nil }
func (r *recordingClient) ServiceStatus(context.Context) (ChainState, error)      { return ChainState{Running: r.started && !r.stopped}, nil }

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

func TestModeRouting_PreStartOpsAreNoops(t *testing.T) {
	rc := newModeRoutingHelperClient(&recordingClient{}, &recordingClient{})
	if err := rc.RouteSnapshot(context.Background()); err != nil {
		t.Fatalf("RouteSnapshot pre-StartChain must be a no-op, got %v", err)
	}
	if err := rc.TunCreate(context.Background(), "n", "c"); err != nil {
		t.Fatalf("TunCreate pre-StartChain must be a no-op, got %v", err)
	}
}
