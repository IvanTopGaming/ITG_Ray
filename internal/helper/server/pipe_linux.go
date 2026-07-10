//go:build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
)

// Listen accepts unix-socket connections at path, gates each on the peer
// uid, and dispatches accepted connections to d. Returns when ctx is
// cancelled. The socket file is (re)created with mode 0660.
func Listen(ctx context.Context, path string, d *Dispatcher, allowedUID uint32) error {
	// Remove a stale socket from a prior crashed run so bind() succeeds.
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen unix %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o660); err != nil {
		_ = ln.Close()
		return fmt.Errorf("chmod %q: %w", path, err)
	}
	// The daemon runs as root but the socket must be reachable by the
	// unprivileged client (allowedUID); chown it so the connect() succeeds
	// before SO_PEERCRED is even consulted. gid is left unchanged (-1).
	if err := os.Chown(path, int(allowedUID), -1); err != nil {
		_ = ln.Close()
		return fmt.Errorf("chown %q: %w", path, err)
	}
	defer ln.Close() //nolint:errcheck // best-effort cleanup

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			slog.Error("helper: accept", slog.String("err", err.Error()))
			continue
		}
		if err := requirePeerUID(conn, allowedUID); err != nil {
			slog.Warn("helper: rejected connection", slog.String("err", err.Error()))
			_ = conn.Close()
			continue
		}
		go d.Serve(ctx, conn)
	}
}
