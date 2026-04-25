package supervisor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sleepBin() string {
	if runtime.GOOS == "windows" {
		return "C:\\Windows\\System32\\timeout.exe"
	}
	return "/bin/sleep"
}

func sleepArgs() []string {
	if runtime.GOOS == "windows" {
		return []string{"/T", "60", "/NOBREAK"}
	}
	return []string{"60"}
}

func TestSpawnAndStop(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "child.log")
	c, err := Spawn("sleeper", sleepBin(), sleepArgs(), logPath)
	require.NoError(t, err)
	require.NotZero(t, c.Pid())
	require.NoError(t, c.Stop(2*time.Second))
	// Log file should exist (may be empty or contain stderr).
	_, err = os.Stat(logPath)
	require.NoError(t, err)
}

func TestStopOnExitedProcessIsNoop(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "child.log")
	bin := "/bin/true"
	if runtime.GOOS == "windows" {
		bin = "C:\\Windows\\System32\\cmd.exe"
	}
	args := []string{}
	if runtime.GOOS == "windows" {
		args = []string{"/C", "exit 0"}
	}
	c, err := Spawn("quick", bin, args, logPath)
	require.NoError(t, err)
	// Wait for natural exit.
	time.Sleep(500 * time.Millisecond)
	// Stop on an already-exited process should not error.
	require.NoError(t, c.Stop(time.Second))
}
