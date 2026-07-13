package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLevelFromString(t *testing.T) {
	require.Equal(t, slog.LevelError, LevelFromString("error"))
	require.Equal(t, slog.LevelWarn, LevelFromString("warn"))
	require.Equal(t, slog.LevelInfo, LevelFromString("info"))
	require.Equal(t, slog.LevelDebug, LevelFromString("debug"))
	require.Equal(t, slog.LevelDebug, LevelFromString("trace"))
	require.Equal(t, slog.LevelInfo, LevelFromString("nonsense"))
}

func TestRotatingWriter_RotatesAndKeeps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	w := NewRotatingWriter(p, 10, 2)
	for i := 0; i < 5; i++ {
		_, err := w.Write([]byte("0123456789\n"))
		require.NoError(t, err)
	}
	require.FileExists(t, p)
	require.FileExists(t, p+".1")
	require.FileExists(t, p+".2")
	require.NoFileExists(t, p+".3")
}

func TestRotatingWriter_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "logs", "app.log")
	w := NewRotatingWriter(p, 1000, 1)
	_, err := w.Write([]byte("hello\n"))
	require.NoError(t, err)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(b), "hello"))
}
