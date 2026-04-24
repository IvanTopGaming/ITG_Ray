package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHandler_RedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(&buf, slog.LevelDebug)
	log := slog.New(h)

	log.Info("connected",
		slog.String("uuid", "550e8400-e29b-41d4-a716-446655440000"),
		slog.String("server", "NL-1"))

	out := buf.String()
	require.NotContains(t, out, "550e8400")
	require.Contains(t, out, "***redacted***")
	require.Contains(t, out, "NL-1")
}

func TestNewHandler_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(&buf, slog.LevelWarn)
	log := slog.New(h)

	log.Debug("debug-message")
	log.Info("info-message")
	log.Warn("warn-message")

	out := buf.String()
	require.NotContains(t, out, "debug-message")
	require.NotContains(t, out, "info-message")
	require.Contains(t, out, "warn-message")
}

func TestNewHandler_HumanReadableFormat(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(&buf, slog.LevelInfo)
	log := slog.New(h).With(slog.String("scope", "subscription.sync"))

	log.Info("finished", slog.Int("new", 3), slog.Int("updated", 2))

	line := strings.TrimSpace(buf.String())
	require.Contains(t, line, "INFO")
	require.Contains(t, line, "[subscription.sync]")
	require.Contains(t, line, "finished")
	require.Contains(t, line, "new=3")
	require.Contains(t, line, "updated=2")
}
