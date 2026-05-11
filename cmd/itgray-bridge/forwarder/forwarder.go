// Package forwarder bridges the Wails-style hub.Hub event stream to the
// JSON-RPC bus, translating colon-separated event names ("vpn:status")
// into the dotted protocol topics ("vpn.status") consumed by the
// Electron renderer through wails-shim/runtime.ts.
package forwarder

import (
	"context"

	"github.com/itg-team/itg-ray/internal/hub"
)

// Emitter is the surface Forwarder needs from cmd/itgray-bridge/bus.
// *bus.Bus satisfies it directly.
type Emitter interface {
	Emit(topic string, payload any)
}

// Forwarder subscribes to a hub.Hub and emits each known event as a
// JSON-RPC notification through Emitter. Unknown event names are
// silently dropped — defense against future hub additions the renderer
// has not subscribed to.
type Forwarder struct {
	Hub *hub.Hub
	Bus Emitter
}

// hubToProtocol maps the 8 Wails-style hub event names (with ":")
// to the dotted protocol topic names declared in
// internal/bridge/protocol/schema.go EventTopic constants.
var hubToProtocol = map[string]string{
	hub.EventVPNStatus:      "vpn.status",
	hub.EventVPNSpeed:       "vpn.speed",
	hub.EventChainError:     "chain.error",
	hub.EventHelperState:    "helper.state",
	hub.EventSubSynced:      "sub.synced",
	hub.EventProbeResult:    "probe.result",
	hub.EventServersChanged: "servers.changed",
	hub.EventRulesChanged:   "rules.changed",
}

// Run subscribes to the hub and forwards events until ctx is cancelled
// (drains any in-flight event then unsubscribes). Nil Hub or Bus → no-op
// return; safe to call from a goroutine in main.
func (f Forwarder) Run(ctx context.Context) {
	if f.Hub == nil || f.Bus == nil {
		return
	}
	// Buffer of 64 chosen empirically: chainctl bursts ~6 events at
	// connect (status=connecting, speed samples, status=connected) and
	// the dispatcher can stall for tens of ms during a Snapshot encode.
	// Hub.Publish drops oldest on overflow (intentional — events are
	// best-effort indicators, not a transactional log).
	ch := f.Hub.Subscribe(64)
	defer f.Hub.Unsubscribe(ch)

	f.drain(ctx, ch)
}

// Start subscribes to the hub synchronously (so no events are missed
// between Start returning and the first Publish), then launches a
// goroutine that forwards until ctx is cancelled. The returned function
// blocks until the forwarder has flushed all buffered events and exited;
// call it (e.g. defer fwd()) after d.Serve returns so in-flight events
// are not lost when main exits. Use this from main instead of
// "go f.Run(ctx)" to eliminate the subscribe-before-publish race.
func (f Forwarder) Start(ctx context.Context) (wait func()) {
	if f.Hub == nil || f.Bus == nil {
		return func() {}
	}
	ch := f.Hub.Subscribe(64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer f.Hub.Unsubscribe(ch)
		f.drain(ctx, ch)
	}()
	return func() { <-done }
}

// drain reads from ch until ctx is done or ch is closed. When ctx is
// cancelled, any events already buffered in ch are flushed before
// returning so in-flight hub publishes are not silently dropped.
func (f Forwarder) drain(ctx context.Context, ch <-chan hub.Event) {
	for {
		select {
		case <-ctx.Done():
			// Flush remaining buffered events before exiting.
			for {
				select {
				case ev, ok := <-ch:
					if !ok {
						return
					}
					topic, known := hubToProtocol[ev.Name]
					if !known {
						continue
					}
					f.Bus.Emit(topic, ev.Payload)
				default:
					return
				}
			}
		case ev, ok := <-ch:
			if !ok {
				return
			}
			topic, known := hubToProtocol[ev.Name]
			if !known {
				continue
			}
			f.Bus.Emit(topic, ev.Payload)
		}
	}
}
