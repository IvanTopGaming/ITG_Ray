package hub

import (
	"sync"
	"time"
)

// Hub is a fan-out pub-sub. Publish is non-blocking: if a subscriber's buffer
// is full, the oldest event is discarded and the new one queued.
type Hub struct {
	mu     sync.Mutex
	subs   map[chan Event]struct{}
	closed bool
}

// New returns a fresh Hub.
func New() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a buffered channel that will receive every event
// published until Unsubscribe or Close is called.
func (h *Hub) Subscribe(buffer int) <-chan Event {
	if buffer <= 0 {
		buffer = 1
	}
	c := make(chan Event, buffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(c)
		return c
	}
	h.subs[c] = struct{}{}
	return c
}

// Unsubscribe removes the channel from the hub and closes it.
func (h *Hub) Unsubscribe(c <-chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs {
		if (<-chan Event)(sub) == c { //nolint:gocritic // parens required: <-chan Event(sub) parses as <-(chan Event(sub))
			delete(h.subs, sub)
			close(sub)
			return
		}
	}
}

// Publish stamps the event with current time and fans it out. Non-blocking:
// a full subscriber buffer drops its oldest item to admit the new one.
func (h *Hub) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for sub := range h.subs {
		select {
		case sub <- e:
		default:
			// drop oldest, push newest
			select {
			case <-sub:
			default:
			}
			select {
			case sub <- e:
			default:
				// shouldn't happen — ignore
			}
		}
	}
}

// Close closes the hub and all subscriber channels. Subsequent Publish is a
// no-op.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for sub := range h.subs {
		close(sub)
	}
	h.subs = nil
}
