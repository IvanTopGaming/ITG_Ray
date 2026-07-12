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
	ring := append(b.perSrc[source], e)
	if len(ring) > b.cap {
		ring = ring[len(ring)-b.cap:]
	}
	b.perSrc[source] = ring
	publish := b.subs > 0
	b.mu.Unlock()

	if publish && b.hub != nil {
		b.hub.Publish(hub.Event{Name: "log:line", Payload: e.toMap()})
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
