//go:build linux

package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

// TestListen_ChownsSocketToAllowedUID verifies that Listen chowns the socket to
// allowedUID so an unprivileged client can connect before SO_PEERCRED is
// consulted. Running as a normal user, chown-to-self is a no-op that still
// exercises the code path; the assertions confirm ownership matches allowedUID
// and that a same-process dial + framed round-trip actually works.
func TestListen_ChownsSocketToAllowedUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "itgray-helper-test.sock")
	uid := uint32(os.Getuid())

	d := NewDispatcher()
	const opEcho = protocol.Op("test.echo")
	d.Register(opEcho, func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		return args, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- Listen(ctx, path, d, uid) }()

	// Poll for the socket to appear rather than sleeping arbitrarily.
	var appeared bool
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err == nil {
			appeared = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !appeared {
		t.Fatalf("socket %q never appeared", path)
	}

	// The chown target must be allowedUID.
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if st.Uid != uid {
		t.Fatalf("socket uid = %d, want allowedUID %d", st.Uid, uid)
	}

	// Stronger: a same-process dial and framed round-trip succeeds.
	c, err := client.Dial(ctx, path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close() //nolint:errcheck
	payload := json.RawMessage(`{"hello":"world"}`)
	got, err := c.Call(ctx, opEcho, payload)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo mismatch: got %s want %s", got, payload)
	}

	// Shut the listener down cleanly.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Listen returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Listen did not return after ctx cancel")
	}
}
