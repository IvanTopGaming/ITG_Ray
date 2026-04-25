package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRotateLog_NoInputIsNoOp(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "x.log")
	require.NoError(t, RotateLog(logPath))
	_, err := os.Stat(logPath)
	require.True(t, os.IsNotExist(err))
}

func TestRotateLog_FirstRotationProducesDot1(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "x.log")
	require.NoError(t, os.WriteFile(logPath, []byte("v1"), 0o644)) //nolint:gosec // test stub file
	require.NoError(t, RotateLog(logPath))
	_, err := os.Stat(logPath)
	require.True(t, os.IsNotExist(err), "current .log should be moved away")
	b, err := os.ReadFile(logPath + ".1") //nolint:gosec // test stub read
	require.NoError(t, err)
	require.Equal(t, "v1", string(b))
}

func TestRotateLog_CascadeOf3(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "x.log")
	// Build a stack: .log=v4, .log.1=v3, .log.2=v2, .log.3=v1
	for i, content := range []string{"v1", "v2", "v3", "v4"} {
		var p string
		if i == 3 {
			p = logPath
		} else {
			p = logPath + "." + intToStr(3-i) // .3, .2, .1
		}
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644)) //nolint:gosec // test stub file
	}
	require.NoError(t, RotateLog(logPath))
	// After: .log absent (rotated to .1), .log.1=v4, .log.2=v3, .log.3=v2, v1 evicted
	_, err := os.Stat(logPath)
	require.True(t, os.IsNotExist(err))
	b, _ := os.ReadFile(logPath + ".1") //nolint:gosec // test stub read
	require.Equal(t, "v4", string(b))
	b, _ = os.ReadFile(logPath + ".2") //nolint:gosec // test stub read
	require.Equal(t, "v3", string(b))
	b, _ = os.ReadFile(logPath + ".3") //nolint:gosec // test stub read
	require.Equal(t, "v2", string(b))
	// v1 evicted: there's no .log.4
	_, err = os.Stat(logPath + ".4")
	require.True(t, os.IsNotExist(err))
}

func intToStr(n int) string {
	return string(rune('0' + n))
}
