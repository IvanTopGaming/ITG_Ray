package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/server"
)

// ErrUnrecognizedFormat is returned by Sync when Parse succeeds without an
// error but recognized nothing at all: zero configs, zero invalid entries,
// and zero recognized-but-unsupported ("skipped") URIs. Parse's format
// detection (sing-box JSON -> base64 -> plaintext) falls through every
// stage silently for this input — an HTML error page, a WAF interstitial,
// a bare "{}", a truncated response — and returns ParseResult{}, nil rather
// than an error (see detector.go). Left unchecked, Sync would hand that
// empty result to server.Merge, which deletes every previously-synced
// server for this subscription while Sync still reported "ok".
//
// Trade-off: Parse cannot reliably tell "nothing recognized" apart from "a
// legitimately empty subscription" — e.g. a sing-box document with an
// explicit empty "outbounds" array parses to the exact same zero-signal
// ParseResult as a bare "{}", since sbDoc has no required fields. Rather
// than guess, Sync treats every zero-signal parse as a hard failure. A
// subscription panel that genuinely has zero servers assigned will now
// surface as a sync error instead of silently clearing the local list —
// which is the safer default: existing servers are never dropped as a
// side effect of a response the app didn't understand.
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
func Sync(ctx context.Context, sub Subscription, existing []server.Server, timeout time.Duration) ([]server.Server, SyncMeta, error) { //nolint:gocritic // sub is a value type; caller convenience outweighs copy cost
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
	if len(parsed.Configs) == 0 && parsed.Invalid == 0 && sumSkipped(parsed.Skipped) == 0 {
		err = ErrUnrecognizedFormat
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
	merged := server.Merge(existing, incoming, sub.ID)

	meta.Status = "ok"
	meta.Message = fmt.Sprintf("imported=%d invalid=%d skipped=%d", len(parsed.Configs), parsed.Invalid, sumSkipped(parsed.Skipped))
	slog.Info("sub synced", slog.String("scope", "sub"), slog.String("id", sub.ID),
		slog.Int("servers", len(merged)), slog.Int("skipped", sumSkipped(parsed.Skipped)))
	return merged, meta, nil
}

func sumSkipped(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
