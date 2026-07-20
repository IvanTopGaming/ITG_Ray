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

func TestSpawnRunsInLogDir(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "child.log")
	// The child writes a RELATIVE file; with cmd.Dir set to the log's dir it
	// must land in `dir`, not the test process CWD. Pick a shell per platform
	// so the test exercises cmd.Dir on both Linux and Windows.
	bin, args := "/bin/sh", []string{"-c", "echo ok > relout.txt"}
	if runtime.GOOS == "windows" {
		bin, args = "cmd.exe", []string{"/C", "echo ok > relout.txt"}
	}
	c, err := Spawn("writer", bin, args, logPath)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	<-c.Done()
	if _, err := os.Stat(filepath.Join(dir, "relout.txt")); err != nil {
		t.Fatalf("relative file did not land in log dir: %v", err)
	}
}
