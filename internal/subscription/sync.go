package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
)

// Subscription is a stored subscription source: URL, fetch UA/auth, refresh interval.
type Subscription struct {
	ID             string
	Name           string
	URL            string
	UserAgent      string
	Auth           AuthFunc
	UpdateInterval time.Duration
}

// SyncMeta describes the outcome of a Sync call: timestamp, status string,
// human-readable summary, and the parsed standard headers (quota/expiry/title).
type SyncMeta struct {
	LastUpdate time.Time
	Status     string
	Summary    string
	Headers    Headers
}

// Sync fetches the subscription, parses its body in any supported format,
// and reconciles the resulting servers against the existing list using
// origin-aware merge. Returns the new server list, sync metadata, and any
// transport/parse error.
func Sync(ctx context.Context, sub Subscription, existing []server.Server, timeout time.Duration) ([]server.Server, SyncMeta, error) { //nolint:gocritic // sub is a value type; caller convenience outweighs copy cost
	meta := SyncMeta{LastUpdate: time.Now()}
	ua := sub.UserAgent
	if ua == "" {
		ua = "ITG-Ray/0.1"
	}
	res, err := Fetch(ctx, FetchOptions{URL: sub.URL, UserAgent: ua, Auth: sub.Auth, Timeout: timeout})
	if err != nil {
		meta.Status = fmt.Sprintf("ERROR: %v", err)
		return nil, meta, err
	}
	meta.Headers = res.Headers

	parsed, err := Parse(res.Body)
	if err != nil {
		meta.Status = fmt.Sprintf("ERROR: %v", err)
		return nil, meta, err
	}

	incoming := make([]server.Server, 0, len(parsed.Configs))
	for i := range parsed.Configs {
		incoming = append(incoming, server.New(parsed.Configs[i], server.OriginSubscription, sub.ID))
	}
	merged := server.Merge(existing, incoming, sub.ID)

	meta.Status = "OK"
	meta.Summary = fmt.Sprintf("imported=%d invalid=%d skipped=%d", len(parsed.Configs), parsed.Invalid, sumSkipped(parsed.Skipped))
	return merged, meta, nil
}

func sumSkipped(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
