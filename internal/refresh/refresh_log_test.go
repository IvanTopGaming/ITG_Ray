package refresh

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/logtest"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// newLoggingTestDriver builds a Driver with no explicit Log, so it falls
// back to slog.Default() inside NewDriver. Callers MUST invoke
// logtest.Capture(t) before this helper so the driver captures the swapped
// default logger.
func newLoggingTestDriver(t *testing.T) (*Driver, string) {
	t.Helper()
	st := &metaCaptureStore{}
	serversPath := t.TempDir() + "/servers.json"
	if err := server.Save(serversPath, nil); err != nil {
		t.Fatalf("seed servers.json: %v", err)
	}
	d := NewDriver(Config{
		Subs:        st,
		ServersPath: serversPath,
		SyncFunc:    noopSync,
		ProbeFunc:   noopProbe,
	})
	return d, "s1"
}

func TestSyncOne_LogsScopedSummary(t *testing.T) {
	buf := logtest.Capture(t) // MUST precede NewDriver so d.log == captured default
	d, subID := newLoggingTestDriver(t)

	d.syncOne(context.Background(), subscription.Stored{ID: subID, URL: "https://x.test"})

	out := buf.String()
	if !strings.Contains(out, "[refresh]") {
		t.Fatalf("refresh sync log not scoped: %q", out)
	}
	if !strings.Contains(out, "refresh sync done") {
		t.Fatalf("expected renamed sync-done message, got: %q", out)
	}
}

func TestSyncOne_LoadFailure_LogsScopedError(t *testing.T) {
	buf := logtest.Capture(t)
	st := &metaCaptureStore{}
	// serversPath points at a directory, so server.Load will fail to read it as a file.
	dir := t.TempDir()
	d := NewDriver(Config{
		Subs:        st,
		ServersPath: dir,
		SyncFunc:    noopSync,
		ProbeFunc:   noopProbe,
	})

	d.syncOne(context.Background(), subscription.Stored{ID: "s1", URL: "https://x.test"})

	out := buf.String()
	if !strings.Contains(out, "[refresh]") {
		t.Fatalf("refresh load-failure log not scoped: %q", out)
	}
	if !strings.Contains(out, "refresh sync: load servers failed") {
		t.Fatalf("expected renamed load-failure message, got: %q", out)
	}
}

func TestDriver_Run_LogsScopedStartupSummary(t *testing.T) {
	buf := logtest.Capture(t)
	st := &metaCaptureStore{}
	serversPath := t.TempDir() + "/servers.json"
	if err := server.Save(serversPath, nil); err != nil {
		t.Fatal(err)
	}
	d := NewDriver(Config{
		Subs:        st,
		ServersPath: serversPath,
		SyncFunc:    noopSync,
		ProbeFunc:   noopProbe,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := d.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[refresh]") {
		t.Fatalf("refresh startup log not scoped: %q", out)
	}
	if !strings.Contains(out, "refresh started") {
		t.Fatalf("expected \"refresh started\" summary, got: %q", out)
	}
}

func TestProbeOnce_LogsScopedSummary(t *testing.T) {
	buf := logtest.Capture(t)
	dir := t.TempDir()
	path := dir + "/servers.json"
	if err := server.Save(path, []server.Server{mkServer("a", "a.test", 443)}); err != nil {
		t.Fatal(err)
	}
	d := NewDriver(Config{
		Subs:        &fakeStore{},
		ServersPath: path,
		ProbeFunc:   noopProbe,
	})

	d.probeOnce(context.Background())

	out := buf.String()
	if !strings.Contains(out, "[refresh]") {
		t.Fatalf("refresh probe log not scoped: %q", out)
	}
	if !strings.Contains(out, "refresh probe done") {
		t.Fatalf("expected renamed probe-done message, got: %q", out)
	}
}
