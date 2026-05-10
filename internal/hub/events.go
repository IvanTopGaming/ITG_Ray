// Package hub is an in-process pub-sub aggregator. The GUI backend publishes
// state-change events to the hub; subscribers (the Wails event emitter, the
// tray updater, tests) consume them.
package hub

import "time"

// Event is the union shape of all hub events. Type discriminator is Name.
type Event struct {
	Name    string         // e.g., "vpn:status", "vpn:speed", "sub:synced"
	Payload map[string]any // JSON-serializable map; keys per event spec
	Time    time.Time      // wall-clock at emission
}

// Event-name constants. All events reach the frontend via the same name in
// runtime.EventsEmit, so JS subscribers use these strings verbatim.
const (
	EventVPNStatus   = "vpn:status"
	EventVPNSpeed    = "vpn:speed"
	EventSubSynced   = "sub:synced"
	EventProbeResult = "probe:result"
	EventChainError  = "chain:error"
	EventHelperState = "helper:state"
	EventSettings    = "settings:changed"
)

// EventServersChanged is published by ServersService after a successful
// Add / Edit / Remove. Payload is nil; consumers re-fetch via List or
// App.GetSnapshot.
const EventServersChanged = "servers:changed"
