package bindings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/clashapi"
	"github.com/itg-team/itg-ray/internal/hub"
)

// RunDeps groups the dependencies for RunService. Hub is captured so
// future event-emitting helpers (e.g. preflight warnings) have a place
// to land without changing the public surface.
type RunDeps struct {
	Chain *chainctl.Controller
	Hub   *hub.Hub

	Routing   RoutingAPI
	Connected func() bool
}

type RoutingAPI interface {
	Mode(ctx context.Context) (string, error)
	SetMode(ctx context.Context, mode string) error
	CloseConnections(ctx context.Context) error
}

// RunService implements the Run.* bindings (Connect / Disconnect /
// GetStatus). It is a thin translator between the JS surface and the
// chainctl.Controller orchestrator: status transitions and errors flow
// to the frontend via hub events that chainctl already publishes.
type RunService struct{ d RunDeps }

// NewRunService constructs a new RunService.
func NewRunService(d RunDeps) *RunService { return &RunService{d: d} }

// Connect kicks off a connect attempt for the given server in the given
// mode ("tun" | "sysproxy"). The call is non-blocking — the
// frontend tracks progress via vpn:status / chain:error events.
//
// Wails v2.11 does not auto-inject ctx for service methods, so a fresh
// context.Background() is passed down to chainctl. The accepted tradeoff
// is no cancellation from the JS side; chainctl owns its own internal
// timeouts on each chain step.
func (r *RunService) Connect(serverID, mode string) error {
	return r.d.Chain.Start(context.Background(), serverID, chainctl.Mode(mode))
}

// Disconnect tears down the active chain. Idempotent: calling on an
// already-idle controller is a no-op and returns nil.
func (r *RunService) Disconnect() error {
	return r.d.Chain.Stop(context.Background())
}

// GetStatus returns the cached chain state as a JSON-serialisable map.
// Reads from the Controller's in-memory snapshot — does not touch the
// helper synchronously.
func (r *RunService) GetStatus() map[string]any {
	st, srv, mode := r.d.Chain.Status()
	out := map[string]any{
		"status": string(st),
		"mode":   string(mode),
	}
	if srv != nil {
		out["serverId"] = srv.ID
		out["serverName"] = srv.Name
	}
	return out
}

var errNotConnected = errors.New("routing mode: not connected")

func (r *RunService) connected() bool {
	if r.d.Connected != nil {
		return r.d.Connected()
	}
	if r.d.Chain == nil {
		return false
	}
	st, _, _ := r.d.Chain.Status()
	return st == hub.StatusConnected
}

func (r *RunService) SetRoutingMode(mode string) error {
	if !clashapi.ValidMode(mode) {
		return fmt.Errorf("routing mode: unknown mode %q", mode)
	}
	if !r.connected() || r.d.Routing == nil {
		return errNotConnected
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.d.Routing.SetMode(ctx, mode); err != nil {
		return err
	}
	_ = r.d.Routing.CloseConnections(ctx)
	r.d.Hub.Publish(hub.Event{Name: hub.EventRoutingMode, Payload: map[string]any{"mode": mode}})
	return nil
}

func (r *RunService) GetRoutingMode() (string, error) {
	if !r.connected() || r.d.Routing == nil {
		return "rule", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return r.d.Routing.Mode(ctx)
}
