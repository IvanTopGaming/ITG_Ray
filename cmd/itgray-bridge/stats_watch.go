package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/stats"
)

const statsRetention = 30 * 24 * time.Hour

func onStatsEvent(e hub.Event, col *stats.Collector, now time.Time) {
	if e.Name != hub.EventVPNStatus {
		return
	}
	switch e.Payload["status"] {
	case string(hub.StatusConnected):
		col.SessionStart(now)
	case string(hub.StatusIdle), string(hub.StatusError):
		col.SessionEnd(now)
		_ = col.Flush()
	}
}

func runStats(ctx context.Context, h *hub.Hub, col *stats.Collector, store *stats.Store, connected func() bool) (stop func()) {
	events := h.Subscribe(64)
	if connected() {
		col.SessionStart(time.Now())
	}
	_ = store.Prune(time.Now(), statsRetention)
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Second)
		flush := time.NewTicker(time.Minute)
		prune := time.NewTicker(24 * time.Hour)
		defer tick.Stop()
		defer flush.Stop()
		defer prune.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-events:
				if !ok {
					return
				}
				onStatsEvent(e, col, time.Now())
			case <-tick.C:
				if connected() {
					col.Tick(ctx)
				}
			case <-flush.C:
				if err := col.Flush(); err != nil {
					slog.Warn("stats flush failed", slog.String("scope", "stats"), slog.String("err", err.Error()))
				}
			case <-prune.C:
				_ = store.Prune(time.Now(), statsRetention)
			}
		}
	}()
	return func() {
		h.Unsubscribe(events)
		<-done
		col.SessionEnd(time.Now())
		_ = col.Flush()
		_ = store.Close()
	}
}
