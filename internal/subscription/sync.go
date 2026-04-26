package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
)

// Subscription is the in-memory shape used by Sync. It is not the on-disk
// representation: AuthFunc cannot be JSON-serialized. The CLI / Wails layer
// owns the persistent DTO that converts auth-method-name + credentials into
// an AuthFunc at load time.
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
// origin-aware merge.
//
// On success: returns (merged, meta, nil) with meta.Status == "OK".
// On failure: returns (nil, meta, err) — meta is still populated with
// LastUpdate, Status="ERROR: <message>", and Headers if Fetch succeeded.
// Callers should always persist meta regardless of err.
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
