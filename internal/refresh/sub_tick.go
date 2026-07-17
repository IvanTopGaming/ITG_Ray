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

// syncOne executes one sync attempt for sub. servers.json is updated only
// on success. Status meta is written on success and on failure, but skipped
// when ctx is cancelled mid-call (shutdown is not a sync result).
func (d *Driver) syncOne(ctx context.Context, sub subscription.Stored) {
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
		return
	}

	merged, meta, syncErr := d.syncFunc(ctx, sub.ToSyncInput(), existing, syncFetchTimeout)
	if syncErr == nil {
		if saveErr := server.Save(d.serversPath, merged); saveErr != nil {
			d.serversMu.Unlock()
			d.recordMeta(ctx, sub.ID, "error", "save servers: "+truncate(saveErr.Error(), lastStatusMaxLen))
			d.log.Error("refresh sync: save servers failed",
				slog.String("scope", "refresh"),
				slog.String("id", sub.ID),
				slog.String("err", logging.RedactError(saveErr)),
			)
			return
		}
	}
	d.serversMu.Unlock()

	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// Shutdown — do not record this attempt.
		return
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			d.syncOne(ctx, s)
			next := d.jittered(interval, tickJitterPct)
			if next <= 0 {
				next = interval
			}
			timer.Reset(next)
		}
	}
}
