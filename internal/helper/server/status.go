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
}

// NewServiceStatusHandler returns a Handler that reports the helper's
// version and uptime. Pass the start-time the helper picked up (e.g. at SCM
// boot).
func NewServiceStatusHandler(version string, startedAt time.Time) Handler {
	return func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		out := ServiceStatusResult{
			Version:    version,
			UptimeSecs: int(time.Since(startedAt).Seconds()),
		}
		return json.Marshal(out)
	}
}
