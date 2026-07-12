package bindings

import (
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logstream"
)

func timeZero() time.Time { return time.Unix(0, 0) }

func TestLogServiceStartReturnsSnapshotAndTogglesPoller(t *testing.T) {
	buf := logstream.New(hub.New(), 10)
	buf.Add("bridge", "INFO", "hello", timeZero())
	started, stopped := 0, 0
	svc := NewLogService(LogDeps{
		Buffer:      buf,
		StartPoller: func() { started++ },
		StopPoller:  func() { stopped++ },
		LogDir:      t.TempDir(),
	})

	res, err := svc.Start()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Message != "hello" {
		t.Fatalf("snapshot wrong: %+v", res.Entries)
	}
	if started != 1 {
		t.Fatalf("poller should start on first subscriber, started=%d", started)
	}
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	if stopped != 1 {
		t.Fatalf("poller should stop on last unsubscribe, stopped=%d", stopped)
	}
}
