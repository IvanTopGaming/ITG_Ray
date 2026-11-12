package logstream

import (
	"sort"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logging"
)

type Entry struct {
	Seq     uint64    `json:"seq"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}

// DefaultFlushInterval is how long the buffer coalesces new entries before
// publishing them as one log:batch event. Publishing per line used to turn a
// busy engine log into thousands of bridge → main → renderer messages a
// second; the renderer re-rendered on each one and froze the window, and the
// flood evicted unrelated events (vpn:status) from the hub's bounded queues.
const DefaultFlushInterval = 200 * time.Millisecond

type Buffer struct {
	mu     sync.Mutex
	cap    int
	perSrc map[string][]Entry
	seq    uint64
	subs   int
	hub    *hub.Hub
	// pending holds entries added while someone is subscribed that have not
	// been published yet; flushArmed records that a flush timer is running.
	pending    []Entry
	flushArmed bool
	flushEvery time.Duration
	taps       []func(source, message string)
}

func (b *Buffer) OnLine(fn func(source, message string)) {
	b.mu.Lock()
	b.taps = append(b.taps, fn)
	b.mu.Unlock()
}

func New(h *hub.Hub, capPerSource int) *Buffer {
	if capPerSource <= 0 {
		capPerSource = 2000
	}
	return &Buffer{cap: capPerSource, perSrc: map[string][]Entry{}, hub: h, flushEvery: DefaultFlushInterval}
}

func (b *Buffer) Add(source, level, message string, t time.Time) {
	b.mu.Lock()
	taps := b.taps
	defer func() {
		for _, fn := range taps {
			fn(source, message)
		}
	}()
	b.seq++
	e := Entry{Seq: b.seq, Time: t, Level: level, Source: source, Message: logging.Redact(message)}
	ring := append(b.perSrc[source], e) //nolint:gocritic // intentional: bounded append into a temp, capped, then stored back
	if len(ring) > b.cap {
		ring = ring[len(ring)-b.cap:]
	}
	b.perSrc[source] = ring
	if b.subs > 0 && b.hub != nil {
		b.pending = append(b.pending, e)
		// The renderer keeps only the newest cap lines anyway; anything older
		// in a single burst would be discarded on arrival.
		if len(b.pending) > b.cap {
			b.pending = b.pending[len(b.pending)-b.cap:]
		}
		if !b.flushArmed {
			b.flushArmed = true
			time.AfterFunc(b.flushEvery, b.flush)
		}
	}
	b.mu.Unlock()
}

// flush publishes everything pending as a single log:batch event.
func (b *Buffer) flush() {
	b.mu.Lock()
	batch := b.pending
	b.pending = nil
	b.flushArmed = false
	publish := b.subs > 0
	b.mu.Unlock()
	if !publish || len(batch) == 0 {
		return
	}
	entries := make([]map[string]any, len(batch))
	for i, e := range batch {
		entries[i] = e.toMap()
	}
	b.hub.Publish(hub.Event{Name: hub.EventLogBatch, Payload: map[string]any{"entries": entries}})
}

func (e Entry) toMap() map[string]any {
	return map[string]any{
		"seq":     e.Seq,
		"time":    e.Time.Format(time.RFC3339Nano),
		"level":   e.Level,
		"source":  e.Source,
		"message": e.Message,
	}
}

func (b *Buffer) Snapshot() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	var all []Entry
	for _, ring := range b.perSrc {
		all = append(all, ring...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Seq < all[j].Seq })
	return all
}

// Tail returns the n most recent entries across all sources, oldest first.
// A non-positive n means "everything", matching Snapshot.
//
// The Logs tab opens with a tail rather than the full buffer: four sources at
// capPerSource each add up to thousands of entries, and shipping all of them
// over the bridge on every open only to render the last screenful is wasted
// work on both ends.
func (b *Buffer) Tail(n int) []Entry {
	all := b.Snapshot()
	if n <= 0 || len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

func (b *Buffer) Subscribe() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs++
	return b.subs
}

func (b *Buffer) Unsubscribe() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs > 0 {
		b.subs--
	}
	if b.subs == 0 {
		b.pending = nil
	}
	return b.subs
}
