package logstream

import (
	"strconv"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
)

func TestBufferEvictsPerSourceAndSnapshotsOrdered(t *testing.T) {
	b := New(hub.New(), 2)
	ts := time.Unix(0, 0)
	b.Add("bridge", "INFO", "b1", ts)
	b.Add("sing-box", "INFO", "s1", ts)
	b.Add("bridge", "INFO", "b2", ts)
	b.Add("bridge", "INFO", "b3", ts)

	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("want 3 entries (bridge capped to 2 + 1 sing-box), got %d", len(snap))
	}
	for i := 1; i < len(snap); i++ {
		if snap[i].Seq <= snap[i-1].Seq {
			t.Fatalf("snapshot not Seq-ordered: %v", snap)
		}
	}
	if snap[len(snap)-1].Message != "b3" {
		t.Fatalf("newest bridge line should survive, got %q", snap[len(snap)-1].Message)
	}
}

// Opening the Logs tab must not ship the whole buffer at once — with four
// sources at 2000 lines each that is up to 8000 entries over the bridge, all
// of them held in the renderer. Tail returns just the newest slice.
func TestBufferTailReturnsNewestInOrder(t *testing.T) {
	b := New(hub.New(), 100)
	ts := time.Unix(0, 0)
	for i := range 50 {
		b.Add("bridge", "INFO", "b"+strconv.Itoa(i), ts)
		b.Add("sing-box", "INFO", "s"+strconv.Itoa(i), ts)
	}

	tail := b.Tail(10)
	if len(tail) != 10 {
		t.Fatalf("want 10 entries, got %d", len(tail))
	}
	for i := 1; i < len(tail); i++ {
		if tail[i].Seq <= tail[i-1].Seq {
			t.Fatalf("tail not Seq-ordered: %v", tail)
		}
	}
	// The newest line overall is the last one added.
	full := b.Snapshot()
	if tail[len(tail)-1].Seq != full[len(full)-1].Seq {
		t.Fatalf("tail must end at the newest entry: got seq %d, want %d",
			tail[len(tail)-1].Seq, full[len(full)-1].Seq)
	}
	// And it must be the *newest* 10, not the oldest.
	if tail[0].Seq != full[len(full)-10].Seq {
		t.Fatalf("tail must start 10 back from the newest: got seq %d, want %d",
			tail[0].Seq, full[len(full)-10].Seq)
	}
}

func TestBufferTailShorterThanRequested(t *testing.T) {
	b := New(hub.New(), 100)
	b.Add("bridge", "INFO", "only", time.Unix(0, 0))
	if got := b.Tail(500); len(got) != 1 {
		t.Fatalf("want the 1 available entry, got %d", len(got))
	}
}

func TestBufferTailNonPositiveReturnsEverything(t *testing.T) {
	b := New(hub.New(), 100)
	ts := time.Unix(0, 0)
	b.Add("bridge", "INFO", "a", ts)
	b.Add("bridge", "INFO", "b", ts)
	if got := b.Tail(0); len(got) != 2 {
		t.Fatalf("Tail(0) should not truncate, got %d", len(got))
	}
}

func TestBufferRedactsMessage(t *testing.T) {
	b := New(hub.New(), 10)
	b.Add("bridge", "INFO", "password=supersecret ok", time.Unix(0, 0))
	got := b.Snapshot()[0].Message
	if got == "password=supersecret ok" {
		t.Fatalf("message not redacted: %q", got)
	}
}

func TestBufferPublishesOnlyWithSubscribers(t *testing.T) {
	h := hub.New()
	ch := h.Subscribe(8)
	b := New(h, 10)

	b.Add("bridge", "INFO", "silent", time.Unix(0, 0))
	select {
	case <-ch:
		t.Fatal("published with zero subscribers")
	default:
	}

	if n := b.Subscribe(); n != 1 {
		t.Fatalf("Subscribe count = %d, want 1", n)
	}
	b.Add("bridge", "INFO", "loud", time.Unix(0, 0))
	select {
	case e := <-ch:
		if e.Name != "log:line" {
			t.Fatalf("event name = %q", e.Name)
		}
	default:
		t.Fatal("expected publish with an active subscriber")
	}

	if n := b.Unsubscribe(); n != 0 {
		t.Fatalf("Unsubscribe count = %d, want 0", n)
	}
}
