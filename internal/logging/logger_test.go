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
	require.NotContains(t, line, "scope=") // scope lives in the prefix, not the trailing attrs
}

func TestNewHandler_WithAttrsNoAliasing(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(NewHandler(&buf, slog.LevelInfo)).With(slog.String("a", "1"))

	l1 := base.With(slog.String("b", "2"))
	l2 := base.With(slog.String("c", "3"))

	buf.Reset()
	l1.Info("m1")
	l2.Info("m2")

	out := buf.String()
	// l1 should see a=1 b=2, not c=3 from the sibling.
	require.Contains(t, out, "m1 a=1 b=2")
	require.NotContains(t, out, "m1 a=1 b=2 c=3")
	require.NotContains(t, out, "m1 a=1 c=3")
	// l2 should see a=1 c=3, not b=2 from the sibling.
	require.Contains(t, out, "m2 a=1 c=3")
	require.NotContains(t, out, "m2 a=1 b=2")
}

func TestNewHandler_InlineScope(t *testing.T) {
	// Scope attr set inline (not via .With) should still be promoted to the prefix.
	var buf bytes.Buffer
	h := NewHandler(&buf, slog.LevelInfo)
	log := slog.New(h)

	log.Info("msg", slog.String("scope", "ad-hoc"))

	line := strings.TrimSpace(buf.String())
	require.Contains(t, line, "[ad-hoc]")
	require.NotContains(t, line, "scope=ad-hoc")
}
