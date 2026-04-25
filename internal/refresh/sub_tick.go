package refresh

import (
	"context"
	"errors"
	"log/slog"
	"time"

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
		status := "ERROR: load servers: " + truncate(err.Error(), lastStatusMaxLen)
		d.recordMeta(ctx, sub.ID, status)
		d.log.Error("refresh.sync.load", "id", sub.ID, "err", err)
		return
	}

	merged, meta, syncErr := d.syncFunc(ctx, sub.ToSyncInput(), existing, syncFetchTimeout)
	if syncErr == nil {
		if saveErr := server.Save(d.serversPath, merged); saveErr != nil {
			d.serversMu.Unlock()
			status := "ERROR: save servers: " + truncate(saveErr.Error(), lastStatusMaxLen)
			d.recordMeta(ctx, sub.ID, status)
			d.log.Error("refresh.sync.save", "id", sub.ID, "err", saveErr)
			return
		}
	}
	d.serversMu.Unlock()

	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// Shutdown — do not record this attempt.
		return
	}

	var status string
	if syncErr != nil {
		status = "ERROR: " + truncate(syncErr.Error(), lastStatusMaxLen)
	} else {
		status = "OK " + meta.Summary
	}
	if err := d.subs.UpdateMeta(sub.ID, d.now(), status); err != nil {
		d.log.Error("refresh.sync.updateMeta", "id", sub.ID, "err", err)
	}
	d.log.Info("subscription.sync",
		slog.String("id", sub.ID),
		slog.String("status", status),
		slog.Duration("duration", d.now().Sub(start)),
	)
}

// recordMeta is a small helper that respects ctx-cancel for shutdown.
func (d *Driver) recordMeta(ctx context.Context, id, status string) {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return
	}
	if err := d.subs.UpdateMeta(id, d.now(), status); err != nil {
		d.log.Error("refresh.sync.updateMeta", "id", id, "err", err)
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
	defer d.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("refresh.runSub panic", "id", s.ID, "panic", r)
		}
	}()

	interval := time.Duration(s.UpdateInterval)
	if interval <= 0 {
		interval = d.defaultSubInterval
	}

	d.randMu.Lock()
	first := time.Duration(d.rand.Int63n(int64(firstSubJitterMax)))
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
