package refresh

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// syncOne executes one sync attempt for sub (with level-1 in-attempt retries
// on transient failures). servers.json is updated only on success. Status meta
// is written on success and on failure, but skipped when ctx is cancelled
// mid-call (shutdown is not a sync result). It returns retryable=true when the
// attempt failed transiently, so the scheduler can back off to a sooner retry
// instead of waiting the full update interval.
func (d *Driver) syncOne(ctx context.Context, sub subscription.Stored) (retryable bool) {
	start := d.now()
	d.serversMu.Lock()
	existing, err := server.Load(d.serversPath)
	if err != nil {
		d.serversMu.Unlock()
		d.recordMeta(ctx, sub.ID, "error", "load servers: "+truncate(err.Error(), lastStatusMaxLen))
		d.log.Error("refresh sync: load servers failed",
			slog.String("scope", "refresh"),
			slog.String("id", sub.ID),
			slog.String("err", logging.RedactError(err)),
		)
		return true // local IO hiccup — worth a sooner retry
	}

	merged, meta, syncErr := d.syncWithRetry(ctx, sub, existing)
	if syncErr == nil {
		if saveErr := server.Save(d.serversPath, merged); saveErr != nil {
			d.serversMu.Unlock()
			d.recordMeta(ctx, sub.ID, "error", "save servers: "+truncate(saveErr.Error(), lastStatusMaxLen))
			d.log.Error("refresh sync: save servers failed",
				slog.String("scope", "refresh"),
				slog.String("id", sub.ID),
				slog.String("err", logging.RedactError(saveErr)),
			)
			return true // local IO hiccup — worth a sooner retry
		}
	}
	d.serversMu.Unlock()

	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// Shutdown — do not record this attempt.
		return false
	}

	var ui *subscription.Userinfo
	if syncErr == nil {
		ui = meta.Headers.Userinfo
	}
	if err := d.subs.UpdateMeta(sub.ID, d.now(), meta.Status, truncate(meta.Message, lastStatusMaxLen), ui); err != nil {
		d.log.Error("refresh sync: update meta failed",
			slog.String("scope", "refresh"),
			slog.String("id", sub.ID),
			slog.String("err", logging.RedactError(err)),
		)
	}
	d.log.Info("refresh sync done",
		slog.String("scope", "refresh"),
		slog.String("id", sub.ID),
		slog.String("status", meta.Status),
		slog.String("message", truncate(meta.Message, 80)),
		slog.Duration("duration", d.now().Sub(start)),
	)
	if d.onSync != nil {
		d.onSync(sub.ID)
	}
	return subscription.IsTransient(syncErr)
}

// syncWithRetry runs syncFunc, re-attempting on transient failures per the
// level-1 backoff schedule, and returns the last attempt's result. The caller
// holds serversMu across this call — consistent with the pre-existing "fetch
// under lock" behavior — so a flaky sub can delay others by at most the
// bounded retry window. A server Retry-After hint is honored when longer than
// the scheduled wait, unless it exceeds maxInAttemptRetryWait (then the attempt
// gives up and the scheduler backs off instead). Respects ctx cancellation.
func (d *Driver) syncWithRetry(ctx context.Context, sub subscription.Stored, existing []server.Server) ([]server.Server, subscription.SyncMeta, error) {
	var (
		merged  []server.Server
		meta    subscription.SyncMeta
		syncErr error
	)
	for attempt := 0; ; attempt++ {
		merged, meta, syncErr = d.syncFunc(ctx, sub.ToSyncInput(), existing, syncFetchTimeout)
		if syncErr == nil || !subscription.IsTransient(syncErr) || attempt >= len(d.subFetchRetryBackoff) {
			return merged, meta, syncErr
		}
		wait := d.subFetchRetryBackoff[attempt]
		if ra := subscription.RetryAfterHint(syncErr); ra > wait {
			if ra > maxInAttemptRetryWait {
				return merged, meta, syncErr
			}
			wait = ra
		}
		d.log.Warn("refresh sync: transient failure, retrying",
			slog.String("scope", "refresh"),
			slog.String("id", sub.ID),
			slog.Int("attempt", attempt+1),
			slog.Duration("wait", wait),
			slog.String("err", logging.RedactError(syncErr)),
		)
		select {
		case <-ctx.Done():
			return merged, meta, syncErr
		case <-time.After(wait):
		}
	}
}

// recordMeta is a small helper that respects ctx-cancel for shutdown.
// ui is intentionally omitted — recordMeta is only used for pre-sync
// failures where there is no fresh Userinfo to write.
func (d *Driver) recordMeta(ctx context.Context, id, status, message string) {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return
	}
	if err := d.subs.UpdateMeta(id, d.now(), status, message, nil); err != nil {
		d.log.Error("refresh sync: update meta failed",
			slog.String("scope", "refresh"),
			slog.String("id", id),
			slog.String("err", logging.RedactError(err)),
		)
	}
}

// truncate clips s to at most n bytes; appending "…" (3 bytes in UTF-8) if cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	const ellipsis = "…"
	if n <= len(ellipsis) {
		return s[:n]
	}
	return s[:n-len(ellipsis)] + ellipsis
}

// runSub is the per-subscription scheduler. The first tick fires after a
// random delay in [0, firstSubJitterMax) to stagger startup with multiple
// subs; subsequent ticks fire after the effective interval (with ±10% jitter)
// elapsed since the previous tick completed.
func (d *Driver) runSub(ctx context.Context, s subscription.Stored) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("refresh sync panic",
				slog.String("scope", "refresh"),
				slog.String("id", s.ID),
				slog.Any("panic", r),
			)
		}
	}()

	interval := time.Duration(s.UpdateInterval)
	if interval <= 0 {
		interval = d.defaultSubInterval
	}

	d.randMu.Lock()
	first := time.Duration(d.rand.Int63n(int64(d.firstSubJitterMax)))
	d.randMu.Unlock()
	timer := time.NewTimer(first)
	defer timer.Stop()

	fails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			retryable := d.syncOne(ctx, s)
			timer.Reset(d.nextTick(interval, retryable, &fails))
		}
	}
}

// nextTick computes the delay until the next sync attempt. On a transient
// failure it applies the level-2 backoff schedule — the delay grows with the
// consecutive-failure count (*fails) but never exceeds interval — so a sub that
// failed recovers in minutes rather than after the full update interval. On
// success or a permanent failure it resets *fails and returns the full interval.
// Both cases carry the usual ±tickJitterPct jitter.
func (d *Driver) nextTick(interval time.Duration, retryable bool, fails *int) time.Duration {
	base := interval
	if retryable && len(d.subRetryBackoff) > 0 {
		base = min(d.subRetryBackoff[min(*fails, len(d.subRetryBackoff)-1)], interval)
		*fails++
	} else {
		*fails = 0
	}
	next := d.jittered(base, tickJitterPct)
	if next <= 0 {
		next = base
	}
	return next
}
