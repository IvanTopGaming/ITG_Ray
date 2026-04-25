//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/Microsoft/go-winio"
)

// pipeSecurityDescriptor restricts the named pipe to the current user (the
// SID that the SCM started us under, which for LocalSystem is S-1-5-18, but
// we ALSO want to accept connections from interactive members of the local
// Administrators group). For MVP we open it to all authenticated users and
// rely on the per-connection SID check (auth_windows.go) for the actual
// authorisation decision.
const pipeSecurityDescriptor = "D:P(A;;GA;;;AU)" // grant all-access to Authenticated Users

// Listen accepts named-pipe connections at name and dispatches each to d.
// Returns when ctx is cancelled.
func Listen(ctx context.Context, name string, d *Dispatcher) error {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}
	ln, err := winio.ListenPipe(name, cfg)
	if err != nil {
		return fmt.Errorf("listen pipe %q: %w", name, err)
	}
	defer ln.Close() //nolint:errcheck // best-effort cleanup

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, winio.ErrPipeListenerClosed) || ctx.Err() != nil {
				return nil
			}
			slog.Error("helper: accept", slog.String("err", err.Error()))
			continue
		}
		// Per-connection SID check (Phase B5.2).
		if err := requireOwnerSID(conn); err != nil {
			slog.Warn("helper: rejected connection", slog.String("err", err.Error()))
			_ = conn.Close()
			continue
		}
		go d.Serve(ctx, conn)
	}
}

// requireOwnerSID verifies the peer SID matches the user that installed the
// service. For B5.1 we accept any SID; B5.2 tightens this.
func requireOwnerSID(_ net.Conn) error { return nil } //nolint:unparam // real check arrives in Phase B5.2
