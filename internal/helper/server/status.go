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
}

// NewServiceStatusHandler returns a Handler that reports the helper's
// version, uptime, and (when a chain is active) cumulative outbound
// proxy byte counters from xray-core's StatsService. A transient gRPC
// failure surfaces last-cached counters rather than failing the call.
func NewServiceStatusHandler(version string, startedAt time.Time) Handler {
	return func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		out := ServiceStatusResult{
			Version:    version,
			UptimeSecs: int(time.Since(startedAt).Seconds()),
		}
		up, down, ok := readChainCounters(ctx)
		if ok {
			out.UpBytes = up
			out.DownBytes = down
		}
		return json.Marshal(out)
	}
}
