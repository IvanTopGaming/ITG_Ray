package clashapi

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOrCreateSecret_Stable(t *testing.T) {
	dir := t.TempDir()
	a, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)
	require.Len(t, a, 64)
	b, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)
	require.Equal(t, a, b)
	st, err := os.Stat(filepath.Join(dir, "clash-secret"))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	}
}

func TestLoadOrCreateSecret_RegeneratesGarbage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clash-secret"), []byte("short"), 0o600))
	s, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)
	require.Len(t, s, 64)
}
