package forwarder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

type recordingEmitter struct {
	mu     sync.Mutex
	events []emittedEvent
}

type emittedEvent struct {
	Topic   string
	Payload any
}

func (r *recordingEmitter) Emit(topic string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, emittedEvent{Topic: topic, Payload: payload})
}

func (r *recordingEmitter) snapshot() []emittedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]emittedEvent, len(r.events))
	copy(out, r.events)
	return out
}

func waitForEvents(t *testing.T, e *recordingEmitter, n int) []emittedEvent {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := e.snapshot(); len(got) >= n {
			return got
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %d events; got %d", n, len(e.snapshot()))
	return nil
}

func TestForwarderTranslatesAllSevenTopics(t *testing.T) {
	cases := []struct {
		hubName  string
		dotTopic string
		payload  map[string]any
	}{
		{hub.EventVPNStatus, "vpn.status", map[string]any{"status": "connected"}},
		{hub.EventVPNSpeed, "vpn.speed", map[string]any{"upBps": 1000.0, "downBps": 2000.0}},
		{hub.EventChainError, "chain.error", map[string]any{"err": "dial failed"}},
		{hub.EventHelperState, "helper.state", map[string]any{"state": "running"}},
		{hub.EventSubSynced, "sub.synced", map[string]any{"id": "u1"}},
		{hub.EventProbeResult, "probe.result", map[string]any{"id": "s1", "ms": 42.0}},
		{hub.EventServersChanged, "servers.changed", map[string]any{}},
	}

	h := hub.New()
	em := &recordingEmitter{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go (Forwarder{Hub: h, Bus: em}).Run(ctx)

	time.Sleep(10 * time.Millisecond)

	for _, c := range cases {
		h.Publish(hub.Event{Name: c.hubName, Payload: c.payload})
	}

	got := waitForEvents(t, em, len(cases))
	if len(got) < len(cases) {
		t.Fatalf("expected %d events, got %d", len(cases), len(got))
	}

	gotTopics := map[string]any{}
	for _, e := range got[:len(cases)] {
		gotTopics[e.Topic] = e.Payload
	}
	for _, c := range cases {
		payload, ok := gotTopics[c.dotTopic]
		if !ok {
			t.Errorf("missing topic %q in emitted events", c.dotTopic)
			continue
		}
		pm, ok := payload.(map[string]any)
		if !ok {
			t.Errorf("topic %q payload type: got %T, want map[string]any", c.dotTopic, payload)
			continue
		}
		for k, v := range c.payload {
			if pm[k] != v {
				t.Errorf("topic %q payload[%q]: got %v, want %v", c.dotTopic, k, pm[k], v)
			}
		}
	}
}

func TestForwarderDropsUnknownTopic(t *testing.T) {
	h := hub.New()
	em := &recordingEmitter{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go (Forwarder{Hub: h, Bus: em}).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	h.Publish(hub.Event{Name: "future:unmapped", Payload: map[string]any{"x": 1}})
	h.Publish(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "idle"}})

	got := waitForEvents(t, em, 1)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 event (the known one), got %d", len(got))
	}
	if got[0].Topic != "vpn.status" {
		t.Fatalf("expected vpn.status, got %q", got[0].Topic)
	}
}

func TestForwarderExitsOnContextCancel(t *testing.T) {
	h := hub.New()
	em := &recordingEmitter{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		(Forwarder{Hub: h, Bus: em}).Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Run did not exit within 1s of ctx cancel")
	}
}

func TestForwarderNilHubOrBusIsNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		(Forwarder{Hub: nil, Bus: &recordingEmitter{}}).Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Run with nil Hub did not return immediately")
	}

	done2 := make(chan struct{})
	go func() {
		(Forwarder{Hub: hub.New(), Bus: nil}).Run(ctx)
		close(done2)
	}()
	select {
	case <-done2:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Run with nil Bus did not return immediately")
	}
}

func TestStartSubscribesBeforeReturn(t *testing.T) {
	h := hub.New()
	em := &recordingEmitter{}
	ctx, cancel := context.WithCancel(context.Background())

	// Start MUST call Hub.Subscribe(...) synchronously before returning,
	// so the very next Publish cannot race the subscribe-in-goroutine
	// pattern that "go Run(ctx)" would have. No sleep allowed between
	// Start and Publish — that's the contract being tested.
	wait := (Forwarder{Hub: h, Bus: em}).Start(ctx)
	// Cancel must happen before wait, otherwise wait blocks forever
	// (LIFO defer order would deadlock if both were deferred).
	defer func() {
		cancel()
		wait()
	}()

	h.Publish(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "connecting"}})

	got := waitForEvents(t, em, 1)
	if len(got) != 1 || got[0].Topic != "vpn.status" {
		t.Fatalf("expected single vpn.status event, got %v", got)
	}
}

type slowEmitter struct {
	mu       sync.Mutex
	events   []emittedEvent
	released chan struct{} // closed once test releases the emitter
	gate     chan struct{} // signaled per Emit before recording
}

func (s *slowEmitter) Emit(topic string, payload any) {
	// Signal that an Emit is in progress, then block until released so
	// the test can publish more events into the hub channel buffer
	// before drain proceeds to read them.
	select {
	case s.gate <- struct{}{}:
	default:
	}
	<-s.released
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, emittedEvent{Topic: topic, Payload: payload})
}

func TestStartFlushesBufferedEventsAfterCancel(t *testing.T) {
	h := hub.New()
	em := &slowEmitter{released: make(chan struct{}), gate: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())

	wait := (Forwarder{Hub: h, Bus: em}).Start(ctx)

	// Publish first event — drain picks it up and blocks inside Emit.
	h.Publish(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"i": 1}})
	// Wait until drain is actually inside the first Emit (so the next
	// publishes definitely land in the channel buffer, not a fast-pickup).
	select {
	case <-em.gate:
	case <-time.After(time.Second):
		t.Fatalf("drain never entered Emit")
	}

	// Now buffer up additional events while drain is blocked.
	h.Publish(hub.Event{Name: hub.EventVPNSpeed, Payload: map[string]any{"i": 2}})
	h.Publish(hub.Event{Name: hub.EventChainError, Payload: map[string]any{"i": 3}})

	// Cancel and release — drain should flush the buffered events
	// before exiting (NOT silently drop them).
	cancel()
	close(em.released)
	wait()

	em.mu.Lock()
	got := append([]emittedEvent(nil), em.events...)
	em.mu.Unlock()

	if len(got) < 3 {
		t.Fatalf("expected at least 3 emitted events after flush, got %d: %v", len(got), got)
	}
	wantTopics := map[string]bool{"vpn.status": false, "vpn.speed": false, "chain.error": false}
	for _, e := range got {
		if _, ok := wantTopics[e.Topic]; ok {
			wantTopics[e.Topic] = true
		}
	}
	for topic, seen := range wantTopics {
		if !seen {
			t.Errorf("missing flushed topic %q", topic)
		}
	}
}
