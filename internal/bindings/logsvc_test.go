package bindings

import (
	"strconv"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logstream"
)

// Opening the Logs tab used to return the entire buffer — thousands of lines
// pushed over the bridge and held in the renderer, when the view only ever
// shows the newest screenful. Start must return a bounded tail.
func TestLogServiceStartReturnsBoundedTail(t *testing.T) {
	buf := logstream.New(hub.New(), 5000)
	ts := time.Unix(0, 0)
	total := startTailLimit + 250
	for i := range total {
		buf.Add("bridge", "INFO", "line"+strconv.Itoa(i), ts)
	}

	svc := NewLogService(LogDeps{Buffer: buf})
	res, err := svc.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(res.Entries) != startTailLimit {
		t.Fatalf("want %d entries, got %d", startTailLimit, len(res.Entries))
	}
	// The newest lines are the ones worth keeping.
	last := res.Entries[len(res.Entries)-1]
	if last.Message != "line"+strconv.Itoa(total-1) {
		t.Fatalf("tail must end at the newest line, got %q", last.Message)
	}
}

func TestLogServiceStartReturnsEverythingBelowTheLimit(t *testing.T) {
	buf := logstream.New(hub.New(), 5000)
	buf.Add("bridge", "INFO", "only", time.Unix(0, 0))

	svc := NewLogService(LogDeps{Buffer: buf})
	res, err := svc.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("want the 1 available entry, got %d", len(res.Entries))
	}
}

func TestLogServiceStartStartsPollerOnFirstSubscriberOnly(t *testing.T) {
	buf := logstream.New(hub.New(), 10)
	starts := 0
	svc := NewLogService(LogDeps{Buffer: buf, StartPoller: func() { starts++ }})

	if _, err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if starts != 1 {
		t.Fatalf("poller should start once, started %d times", starts)
	}
}
