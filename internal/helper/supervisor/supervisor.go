// Package supervisor manages a single long-running child process — the
// sing-box or xray binary that the Helper spawns on OpStartChain. The
// child inherits the Helper's SYSTEM token (or whatever the Helper is
// running under), so it has the privileges needed to create WinTUN
// adapters and bind privileged sockets.
package supervisor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Child wraps a running child process plus its log file.
type Child struct {
	name string
	cmd  *exec.Cmd
	log  *os.File

	mu       sync.Mutex
	exited   bool
	exitErr  error
	exitDone chan struct{}
}

// Spawn launches `exe args...`, redirects stdout+stderr to logPath,
// and returns a *Child that can be Stop()ped later. The child runs
// asynchronously; Spawn returns as soon as exec.Start succeeds.
func Spawn(name, exe string, args []string, logPath string) (*Child, error) {
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640) //nolint:gosec // %ProgramData%, admin-only
	if err != nil {
		return nil, fmt.Errorf("open log %q: %w", logPath, err)
	}

	cmd := exec.Command(exe, args...) //nolint:gosec // exe and args fully controlled by Helper
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	c := &Child{
		name:     name,
		cmd:      cmd,
		log:      logFile,
		exitDone: make(chan struct{}),
	}

	// Reap in a goroutine so Stop can wait deterministically.
	go func() {
		err := cmd.Wait()
		c.mu.Lock()
		c.exited = true
		c.exitErr = err
		c.mu.Unlock()
		_ = c.log.Close()
		close(c.exitDone)
	}()

	return c, nil
}

// Pid returns the OS process id, or 0 if not yet started.
func (c *Child) Pid() int {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

// Stop tries a graceful shutdown (os.Interrupt on Unix; on Windows the
// caller is expected to wrap with platform-specific signaling — for
// now we send Process.Kill if not exited within `grace`). Returns nil
// if the child is already exited or exits within `grace`.
func (c *Child) Stop(grace time.Duration) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.exited {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Best-effort graceful: os.Interrupt on Unix; on Windows
	// the os/exec API treats this as Kill (windows has no SIGINT).
	// The Windows-specific WM_CLOSE / taskkill /T path lives in the
	// caller (chain_windows.go) for this v0.1 — supervisor's job is
	// just to ensure the process is gone after `grace`.
	_ = c.cmd.Process.Signal(os.Interrupt)

	select {
	case <-c.exitDone:
		c.mu.Lock()
		err := c.exitErr
		c.mu.Unlock()
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			// exit-with-non-zero is normal for an interrupted process
			return nil
		}
		return nil
	case <-time.After(grace):
		_ = c.cmd.Process.Kill()
		<-c.exitDone
		return nil
	}
}

// Done returns a channel that is closed when the child has exited.
func (c *Child) Done() <-chan struct{} {
	if c == nil {
		// Already-finished sentinel.
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return c.exitDone
}
