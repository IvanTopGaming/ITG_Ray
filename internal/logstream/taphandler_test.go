package logstream

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logging"
)

func TestTapHandlerWritesStderrAndBuffer(t *testing.T) {
	var stderr bytes.Buffer
	buf := New(hub.New(), 10)
	h := NewTapHandler(logging.NewHandler(&stderr, slog.LevelInfo), buf)
	log := slog.New(h)

	log.Info("connect requested", "mode", "tun")

	if stderr.Len() == 0 {
		t.Fatal("expected stderr passthrough write")
	}
	snap := buf.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("want 1 buffered entry, got %d", len(snap))
	}
	if snap[0].Source != "bridge" || snap[0].Level != "INFO" {
		t.Fatalf("bad entry: %+v", snap[0])
	}
	if snap[0].Time.IsZero() {
		t.Fatal("entry time not set")
	}
}
