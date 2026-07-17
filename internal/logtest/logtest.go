package logtest

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/itg-team/itg-ray/internal/logging"
)

// Capture redirects slog.Default() to an in-memory human handler (LevelDebug)
// for the duration of the test and returns the buffer to assert against.
// The previous default logger is restored on test cleanup.
func Capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(logging.NewHandler(&buf, slog.LevelDebug)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}
