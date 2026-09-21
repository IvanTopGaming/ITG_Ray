package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/server"
)

var ErrUnrecognizedFormat = errors.New("unrecognized subscription format / empty response")

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

	// Identity headers (Remnawave x-hwid contract). Pre-resolved by the
	// bindings layer (resolveIdentity in cmd/itgray-gui/bindings/identity.go).
	// Empty values cause Fetch to skip the corresponding header.
	HWID        string
	DeviceOS    string
	OSVersion   string
	DeviceModel string
}

// SyncMeta describes the outcome of a Sync call.
//
// Status is a clean enum: "ok" or "error". Message carries human-readable
// detail — on success, an "imported=N invalid=M skipped=K" summary; on
// failure, the error string. Headers contains the parsed standard
// subscription headers (quota/expiry/title) when Fetch succeeded.
type SyncMeta struct {
	LastUpdate time.Time
	Status     string
	Message    string
	Headers    Headers
}

// Sync fetches the subscription, parses its body in any supported format,
// and reconciles the resulting servers against the existing list using
// origin-aware merge.
//
// On success: returns (merged, meta, nil) with meta.Status == "ok" and
// meta.Message == "imported=N invalid=M skipped=K".
// On failure: returns (nil, meta, err) — meta is still populated with
// LastUpdate, Status="error", Message=logging.RedactError(err), and Headers if Fetch
// succeeded. Callers should always persist meta regardless of err.
//
// Callers that must not hold a lock across the network round-trip should use
// FetchParse instead and call server.Merge themselves once the fetch is done.
func Sync(ctx context.Context, sub Subscription, existing []server.Server, timeout time.Duration) ([]server.Server, SyncMeta, error) { //nolint:gocritic // sub is a value type; caller convenience outweighs copy cost
	incoming, meta, err := FetchParse(ctx, sub, timeout)
	if err != nil {
		return nil, meta, err
	}
	merged := server.Merge(existing, incoming, sub.ID)
	slog.Info("sub synced", slog.String("scope", "sub"), slog.String("id", sub.ID),
		slog.Int("servers", len(merged)))
	return merged, meta, nil
}

// FetchParse fetches the subscription and parses its body into the servers it
// declares, without touching the local list. Splitting this out of Sync lets a
// caller run the slow network half concurrently and take its store lock only
// for the merge-and-save that follows — holding a lock across a 30s fetch
// would serialize every subscription refresh behind the slowest provider.
//
// Returns the incoming servers plus the same SyncMeta contract as Sync: meta
// is populated (including Headers when the fetch itself succeeded) even on
// error, so callers can persist it regardless.
func FetchParse(ctx context.Context, sub Subscription, timeout time.Duration) ([]server.Server, SyncMeta, error) { //nolint:gocritic // sub is a value type; caller convenience outweighs copy cost
	slog.Info("sub sync start", slog.String("scope", "sub"), slog.String("id", sub.ID))

	meta := SyncMeta{LastUpdate: time.Now()}
	res, err := Fetch(ctx, FetchOptions{
		URL:         sub.URL,
		UserAgent:   sub.UserAgent,
		Auth:        sub.Auth,
		Timeout:     timeout,
		HWID:        sub.HWID,
		DeviceOS:    sub.DeviceOS,
		OSVersion:   sub.OSVersion,
		DeviceModel: sub.DeviceModel,
	})
	if err != nil {
		meta.Status = "error"
		meta.Message = logging.RedactError(err)
		slog.Error("sub sync failed", slog.String("scope", "sub"), slog.String("id", sub.ID),
			slog.String("stage", "fetch"), slog.String("err", logging.RedactError(err)))
		return nil, meta, err
	}
	meta.Headers = res.Headers

	parsed, err := Parse(res.Body)
	if err != nil {
		meta.Status = "error"
		meta.Message = logging.RedactError(err)
		slog.Error("sub sync failed", slog.String("scope", "sub"), slog.String("id", sub.ID),
			slog.String("stage", "parse"), slog.String("err", logging.RedactError(err)))
		return nil, meta, err
	}
	if len(parsed.Configs) == 0 {
		err = noServersError(parsed)
		meta.Status = "error"
		meta.Message = logging.RedactError(err)
		slog.Error("sub sync failed", slog.String("scope", "sub"), slog.String("id", sub.ID),
			slog.String("stage", "parse"), slog.String("err", logging.RedactError(err)))
		return nil, meta, err
	}

	incoming := make([]server.Server, 0, len(parsed.Configs))
	for i := range parsed.Configs {
		incoming = append(incoming, server.New(parsed.Configs[i], server.OriginSubscription, sub.ID))
	}

	meta.Status = "ok"
	meta.Message = fmt.Sprintf("imported=%d invalid=%d skipped=%d", len(parsed.Configs), parsed.Invalid, sumSkipped(parsed.Skipped))
	slog.Info("sub fetched", slog.String("scope", "sub"), slog.String("id", sub.ID),
		slog.Int("servers", len(incoming)), slog.Int("skipped", sumSkipped(parsed.Skipped)))
	return incoming, meta, nil
}

func sumSkipped(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func noServersError(parsed ParseResult) error {
	if parsed.Invalid == 0 && sumSkipped(parsed.Skipped) == 0 {
		return ErrUnrecognizedFormat
	}
	protocols := make([]string, 0, len(parsed.Skipped))
	for protocol := range parsed.Skipped {
		protocols = append(protocols, safeProtocol(protocol))
	}
	sort.Strings(protocols)
	return fmt.Errorf("no usable VLESS servers: invalid=%d skipped=%d protocols=%s", parsed.Invalid, sumSkipped(parsed.Skipped), strings.Join(protocols, ","))
}
