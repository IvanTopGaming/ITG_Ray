package handlers

import (
	"context"
	"encoding/json"
)

// Run is the surface RunHandlers needs from bindings.RunService. The
// real *bindings.RunService satisfies it directly. Reconnect /
// SwitchMode are deferred — the Electron renderer (dashStore.ts) only
// calls Connect / Disconnect today; reconnect is composed in the
// renderer as Disconnect+Connect.
type Run interface {
	Connect(serverID, mode string) error
	Disconnect() error
}

// RunHandlers groups methods under the "run." namespace.
type RunHandlers struct {
	Svc Run
}

type runConnectParams struct {
	ServerID string `json:"serverId"`
	Mode     string `json:"mode"`
}

// Connect kicks off a connect attempt for the given server in the given
// mode ("tun" | "sysproxy"). Non-blocking on the chain side: progress is
// observed via vpn:status / chain:error hub events (Phase 4 forwarder).
// Nil-safe: returns {} (no error) when Svc is unset.
func (r RunHandlers) Connect(_ context.Context, params json.RawMessage) (any, error) {
	if r.Svc == nil {
		return struct{}{}, nil
	}
	var p runConnectParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return struct{}{}, r.Svc.Connect(p.ServerID, p.Mode)
}

// Disconnect tears down the active chain. Idempotent at the binding
// layer: calling on an already-idle controller is a no-op.
func (r RunHandlers) Disconnect(_ context.Context, _ json.RawMessage) (any, error) {
	if r.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, r.Svc.Disconnect()
}
