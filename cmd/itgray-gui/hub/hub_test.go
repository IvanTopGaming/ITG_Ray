package hub

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHub_PublishToOneSubscriber(t *testing.T) {
	h := New()
	defer h.Close()

	rcv := h.Subscribe(8)
	t.Cleanup(func() { h.Unsubscribe(rcv) })

	h.Publish(Event{Name: EventVPNStatus, Payload: map[string]any{"v": "connected"}})

	select {
	case e := <-rcv:
		require.Equal(t, EventVPNStatus, e.Name)
		require.Equal(t, "connected", e.Payload["v"])
	case <-time.After(time.Second):
		t.Fatal("no event delivered within 1s")
	}
}

func TestHub_PublishToManySubscribers(t *testing.T) {
	h := New()
	defer h.Close()

	const n = 8
	chans := make([]<-chan Event, n)
	for i := range chans {
		chans[i] = h.Subscribe(4)
	}
	h.Publish(Event{Name: EventVPNSpeed, Payload: map[string]any{"up": 100, "down": 200}})
	for i, c := range chans {
		select {
		case e := <-c:
			require.Equal(t, EventVPNSpeed, e.Name, "subscriber %d", i)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d didn't receive", i)
		}
	}
}

func TestHub_SlowSubscriberDropsOldEvents(t *testing.T) {
	h := New()
	defer h.Close()

	slow := h.Subscribe(2) // tiny buffer
	for i := 0; i < 10; i++ {
		h.Publish(Event{Name: EventVPNSpeed, Payload: map[string]any{"i": i}})
	}
	// Drain — slow subscriber must NOT block the publisher; it loses old events.
	count := 0
	deadline := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case <-slow:
			count++
		case <-deadline:
			break loop
		}
	}
	require.LessOrEqual(t, count, 2, "slow subscriber should keep at most buffer size")
	require.Greater(t, count, 0, "slow subscriber should still get something")
}

func TestHub_UnsubscribeStopsDelivery(t *testing.T) {
	h := New()
	defer h.Close()

	c := h.Subscribe(4)
	h.Publish(Event{Name: EventVPNStatus, Payload: map[string]any{"v": "idle"}})
	<-c
	h.Unsubscribe(c)
	h.Publish(Event{Name: EventVPNStatus, Payload: map[string]any{"v": "connecting"}})

	select {
	case _, ok := <-c:
		if ok {
			t.Fatalf("channel still receiving after Unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		// channel quiet — accepted (closed or empty)
	}
}

func TestHub_RaceFreeUnderConcurrentPublish(t *testing.T) {
	h := New()
	defer h.Close()

	rcv := h.Subscribe(64)
	t.Cleanup(func() { h.Unsubscribe(rcv) })

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				h.Publish(Event{Name: EventVPNSpeed, Payload: map[string]any{"w": id, "i": i}})
			}
		}(w)
	}
	wg.Wait()
	// Drain the buffered channel.
	drained := 0
	for {
		select {
		case <-rcv:
			drained++
		case <-time.After(100 * time.Millisecond):
			require.Greater(t, drained, 0)
			return
		}
	}
}

func TestHub_CloseUnblocksAllSubscribers(t *testing.T) {
	h := New()
	c := h.Subscribe(1)
	h.Close()
	select {
	case _, ok := <-c:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("Close didn't close subscriber channel")
	}
}
