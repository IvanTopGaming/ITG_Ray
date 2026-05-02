package hwid

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet_FreshDir_ComputesAndPersists(t *testing.T) {
	dir := t.TempDir()
	v, err := Get(dir)
	require.NoError(t, err)
	require.Len(t, v, expectedHexLen, "must be 64-char hex")
	_, err = hex.DecodeString(v)
	require.NoError(t, err)

	on, err := os.ReadFile(filepath.Join(dir, cacheFilename))
	require.NoError(t, err)
	require.Equal(t, v, string(on), "cache file must contain the same value")
}

func TestGet_ExistingValidCache_Reuses(t *testing.T) {
	dir := t.TempDir()
	cached := strings.Repeat("ab", 32) // valid 64-char hex
	require.NoError(t, os.WriteFile(filepath.Join(dir, cacheFilename), []byte(cached), 0o600))

	v, err := Get(dir)
	require.NoError(t, err)
	require.Equal(t, cached, v, "must reuse valid cache")
}

func TestGet_CorruptedCache_Recomputes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, cacheFilename), []byte("not-hex"), 0o600))

	v, err := Get(dir)
	require.NoError(t, err)
	require.Len(t, v, expectedHexLen)
	require.NotEqual(t, "not-hex", v)
}

func TestGet_TooShortCache_Recomputes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, cacheFilename), []byte("abcd"), 0o600))

	v, err := Get(dir)
	require.NoError(t, err)
	require.Len(t, v, expectedHexLen)
}

func TestRandomHex_ReturnsValidLengthHex(t *testing.T) {
	v := randomHex()
	require.Len(t, v, expectedHexLen)
	_, err := hex.DecodeString(v)
	require.NoError(t, err)
}

func TestGet_CacheWriteFails_StillReturnsValue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	dir := t.TempDir()
	// Make configDir un-writable so writeCache cannot create the file.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	v, err := Get(dir)
	require.Error(t, err, "should surface write error")
	require.Len(t, v, expectedHexLen, "but still return a usable hex value")
}
