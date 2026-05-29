package chainctl

import (
	"context"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
)

// runPoller is the 1-Hz goroutine spawned after a successful bringUp (or
// after Reconcile detects an already-running helper). It publishes
// vpn:speed every tick from the byte-counter delta and emits chain:error
// + transitions status to "error" if the helper reports the chain has
// stopped running between ticks (crash detection).
func (c *Controller) runPoller(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			state, err := c.d.Helper.ServiceStatus(ctx)
			if err != nil {
				continue
			}
			if !state.Running {
				c.d.Hub.Publish(hub.Event{
					Name: hub.EventChainError,
					Payload: map[string]any{
						"kind":    "chain_crashed",
						"message": state.LastError,
					},
				})

				ks, kerr := c.d.KillSwitch()
				if kerr != nil {
					// Fail closed (stay blocked) but never silently — a load
					// error flipping the user's security posture must be visible.
					slog.Error("chainctl: kill-switch load failed, failing closed",
						slog.String("scope", "chainctl.poll"), slog.Any("err", kerr))
				}
				blocking := kerr != nil || ks.Enabled // fail closed on loader error

				c.mu.Lock()
				mode := c.mode
				c.cancel = nil
				c.current = nil
				// Note: unlike Stop(), we deliberately do NOT clearSession here —
				// an unsolicited drop is not a user disconnect, so the session is
				// preserved for next-boot Reconcile.
				c.mu.Unlock()

				if !blocking {
					c.tearDown(ctx, mode) // kill-switch OFF: restore direct networking
					c.d.Hub.Publish(hub.Event{
						Name:    hub.EventVPNStatus,
						Payload: map[string]any{"status": string(hub.StatusIdle)},
					})
				} else {
					c.d.Hub.Publish(hub.Event{
						Name:    hub.EventVPNStatus,
						Payload: map[string]any{"status": string(hub.StatusError)},
					})
				}
				return
			}
			c.mu.Lock()
			elapsed := now.Sub(c.prevAt).Seconds()
			var up, down uint64
			if elapsed > 0 && c.prevAt.Year() > 1 {
				if state.UpBytes >= c.prevUp {
					up = uint64(float64(state.UpBytes-c.prevUp) / elapsed)
				}
				if state.DownBytes >= c.prevDown {
					down = uint64(float64(state.DownBytes-c.prevDown) / elapsed)
				}
			}
			c.prevAt = now
			c.prevUp = state.UpBytes
			c.prevDown = state.DownBytes
			c.mu.Unlock()
			c.d.Hub.Publish(hub.Event{
				Name:    hub.EventVPNSpeed,
				Payload: map[string]any{"upBps": up, "downBps": down},
			})
		}
	}
}
