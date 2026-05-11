package server

import (
	"context"
	"encoding/json"
	"time"
)

// ServiceStatusResult is the typed payload returned by OpServiceStatus.
type ServiceStatusResult struct {
	Version    string `json:"version"`
	UptimeSecs int    `json:"uptime_secs"`
	UpBytes    uint64 `json:"up_bytes"`
	DownBytes  uint64 `json:"down_bytes"`
	// ChainActive reports whether the helper has a running chain session.
	// Used by chainctl.Reconcile on GUI/bridge startup to adopt an
	// already-running chain (the helper is a long-lived Windows service
	// while the GUI/bridge are short-lived; without this flag a GUI
	// restart loses connected-state and forces a re-Connect).
	ChainActive bool `json:"chain_active"`
}

// NewServiceStatusHandler returns a Handler that reports the helper's
// version, uptime, chain liveness, and (when a chain is active)
// cumulative outbound proxy byte counters from xray-core's StatsService.
// A transient gRPC failure surfaces last-cached counters rather than
// failing the call. chainAlive is consulted under the helper's
// chain-state lock; nil is treated as "no chain active".
func NewServiceStatusHandler(version string, startedAt time.Time, chainAlive func() bool) Handler {
	return func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		out := ServiceStatusResult{
			Version:    version,
			UptimeSecs: int(time.Since(startedAt).Seconds()),
		}
		if chainAlive != nil {
			out.ChainActive = chainAlive()
		}
		up, down, ok := readChainCounters(ctx)
		if ok {
			out.UpBytes = up
			out.DownBytes = down
		}
		return json.Marshal(out)
	}
}
