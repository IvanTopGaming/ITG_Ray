package chainctl

import (
	"context"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
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
				c.mu.Lock()
				c.cancel = nil
				c.current = nil
				c.mu.Unlock()
				c.d.Hub.Publish(hub.Event{
					Name:    hub.EventVPNStatus,
					Payload: map[string]any{"status": string(hub.StatusError)},
				})
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
