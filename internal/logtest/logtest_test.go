package logtest

import (
	"log/slog"
	"strings"
	"testing"
)

func TestCapture_RecordsAndRestores(t *testing.T) {
	prev := slog.Default()
	buf := Capture(t)
	slog.Info("hello", slog.String("scope", "test"), slog.String("k", "v"))
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "[test]") || !strings.Contains(out, "k=v") {
		t.Fatalf("captured output missing fields: %q", out)
	}
	if slog.Default() == prev {
		t.Fatal("expected Capture to have swapped the default logger during the test")
	}
}
