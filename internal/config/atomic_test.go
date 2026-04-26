package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAtomic_Creates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "file.json")
	require.NoError(t, WriteAtomic(p, []byte("hello"), 0o600))
	b, err := os.ReadFile(p) //nolint:gosec // path is a temp dir constructed in test
	require.NoError(t, err)
	require.Equal(t, "hello", string(b))
}

func TestWriteAtomic_DoesNotLeaveTmpOnSuccess(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")
	require.NoError(t, WriteAtomic(p, []byte("x"), 0o600))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "f.json", entries[0].Name())
}
