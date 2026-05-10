// Package handlers wires JSON-RPC method names to business logic.
// Phase 0: only app.ping. Subsequent phases expand this package.
package handlers

import (
	"context"
	"encoding/json"
	"time"
)

// Version is overridden at build time via -ldflags "-X .../handlers.Version=...".
var Version = "dev"

// AppHandlers groups methods under the "app." namespace.
type AppHandlers struct{}

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
