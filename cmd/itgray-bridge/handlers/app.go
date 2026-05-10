// Package handlers wires JSON-RPC method names to business logic.
// Phase 0: app.ping. Phase 3.A: app.getSnapshot + onboarding.*.
package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
)

// Version is overridden at build time via -ldflags "-X .../handlers.Version=...".
var Version = "dev"

// Snapshotter is the surface AppHandlers needs from bindings.AppService.
// bindings.AppService.GetSnapshot() satisfies it directly.
type Snapshotter interface {
	GetSnapshot() (hub.Snapshot, error)
	GetPublicIP() (string, error)
}

// AppHandlers groups methods under the "app." namespace.
type AppHandlers struct {
	Snap Snapshotter // optional; nil safe — GetSnapshot returns an empty snapshot if nil
}

// PingResult is the response shape for app.ping.
type PingResult struct {
	Pong    int64  `json:"pong"`
	Version string `json:"version"`
}

// Ping returns the current unix-millis and the build version. Liveness probe.
func (AppHandlers) Ping(_ context.Context, _ json.RawMessage) (any, error) {
	return PingResult{
		Pong:    time.Now().UnixMilli(),
		Version: Version,
	}, nil
}

// GetPublicIP returns the egress public IP as seen by an outside
// observer. The binding gates on chain status: when the chain is not
// connected it returns bindings.ErrNotConnected, which the dispatcher
// maps to JSON-RPC -32603. Nil-safe: returns "" (no error) when Snap
// is unset, matching the GetSnapshot pattern.
func (a AppHandlers) GetPublicIP(_ context.Context, _ json.RawMessage) (any, error) {
	if a.Snap == nil {
		return "", nil
	}
	return a.Snap.GetPublicIP()
}

// GetSnapshot returns the full app state for renderer bootstrap.
// Errors at the bindings layer (file I/O, etc.) propagate as JSON-RPC
// internal errors. Nil Snap (test/dev configurations) returns an empty
// Snapshot with no error.
func (a AppHandlers) GetSnapshot(_ context.Context, _ json.RawMessage) (any, error) {
	if a.Snap == nil {
		return hub.Snapshot{}, nil
	}
	return a.Snap.GetSnapshot()
}
