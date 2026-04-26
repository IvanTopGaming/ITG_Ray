package bindings

import (
	"context"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// RunDeps groups the dependencies for RunService. Hub is captured so
// future event-emitting helpers (e.g. preflight warnings) have a place
// to land without changing the public surface.
type RunDeps struct {
	Chain *chainctl.Controller
	Hub   *hub.Hub
}

// RunService implements the Run.* bindings (Connect / Disconnect /
// GetStatus). It is a thin translator between the JS surface and the
// chainctl.Controller orchestrator: status transitions and errors flow
// to the frontend via hub events that chainctl already publishes.
type RunService struct{ d RunDeps }

// NewRunService constructs a new RunService.
func NewRunService(d RunDeps) *RunService { return &RunService{d: d} }

// Connect kicks off a connect attempt for the given server in the given
// mode ("auto" | "tun" | "sysproxy"). The call is non-blocking — the
// frontend tracks progress via vpn:status / chain:error events.
func (r *RunService) Connect(ctx context.Context, serverID, mode string) error {
	return r.d.Chain.Start(ctx, serverID, chainctl.Mode(mode))
}

// Disconnect tears down the active chain. Idempotent: calling on an
// already-idle controller is a no-op and returns nil.
func (r *RunService) Disconnect(ctx context.Context) error {
	return r.d.Chain.Stop(ctx)
}

// GetStatus returns the cached chain state as a JSON-serialisable map.
// Reads from the Controller's in-memory snapshot — does not touch the
// helper synchronously.
func (r *RunService) GetStatus(_ context.Context) map[string]any {
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
