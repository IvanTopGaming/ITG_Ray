package logstream

import (
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
