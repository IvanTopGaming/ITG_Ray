package hub

import (
	"context"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartWailsEmitter spawns a goroutine that drains the hub and forwards every
// event to the Wails frontend via runtime.EventsEmit. Returns when ctx is
// done or the hub closes.
func StartWailsEmitter(ctx context.Context, h *Hub) {
	c := h.Subscribe(64)
	go func() {
		defer h.Unsubscribe(c)
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-c:
				if !ok {
					return
				}
				runtime.EventsEmit(ctx, e.Name, e.Payload)
			}
		}
	}()
	slog.Debug("hub: wails emitter started")
}
