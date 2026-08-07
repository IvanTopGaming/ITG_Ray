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

type Buffer struct {
	mu     sync.Mutex
	cap    int
	perSrc map[string][]Entry
	seq    uint64
	subs   int
	hub    *hub.Hub
}

func New(h *hub.Hub, capPerSource int) *Buffer {
	if capPerSource <= 0 {
		capPerSource = 2000
	}
	return &Buffer{cap: capPerSource, perSrc: map[string][]Entry{}, hub: h}
}

func (b *Buffer) Add(source, level, message string, t time.Time) {
	b.mu.Lock()
	b.seq++
	e := Entry{Seq: b.seq, Time: t, Level: level, Source: source, Message: logging.Redact(message)}
	ring := append(b.perSrc[source], e) //nolint:gocritic // intentional: bounded append into a temp, capped, then stored back
	if len(ring) > b.cap {
		ring = ring[len(ring)-b.cap:]
	}
	b.perSrc[source] = ring
	publish := b.subs > 0
	b.mu.Unlock()

	if publish && b.hub != nil {
		b.hub.Publish(hub.Event{Name: hub.EventLogLine, Payload: e.toMap()})
	}
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
	return b.subs
}
