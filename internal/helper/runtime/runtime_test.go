package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasePathContainsITGRay(t *testing.T) {
	p := BasePath()
	require.True(t, strings.Contains(p, "ITG Ray"), "BasePath should mention ITG Ray, got %q", p)
	require.True(t, strings.Contains(p, "Helper"), "BasePath should mention Helper, got %q", p)
	require.True(t, strings.Contains(p, "runtime"), "BasePath should mention runtime, got %q", p)
}

func TestConfigPathDerived(t *testing.T) {
	cp := ConfigPath("sing-box.json")
	require.Equal(t, filepath.Join(BasePath(), "sing-box.json"), cp)
}

func TestLogPathDerived(t *testing.T) {
	lp := LogPath("sing-box.log")
	require.Equal(t, filepath.Join(BasePath(), "sing-box.log"), lp)
}

func TestEnsureCleanCreatesAndRemoves(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ITGRAY_RUNTIME_BASE", tmp) // override for tests; see implementation
	stub := filepath.Join(tmp, "leftover.log")
	require.NoError(t, os.WriteFile(stub, []byte("stale"), 0o644)) //nolint:gosec // test stub file
	require.NoError(t, EnsureClean())
	_, err := os.Stat(stub)
	require.True(t, os.IsNotExist(err), "stale file should be wiped, err=%v", err)
	_, err = os.Stat(BasePath())
	require.NoError(t, err, "BasePath should exist after EnsureClean")
}
