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
