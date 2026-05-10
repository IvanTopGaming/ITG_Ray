// Command itgray-bridge is the JSON-RPC backend for the Electron-based
// ITG Ray GUI. It reads requests from stdin, writes responses to stdout.
// Phase 0: only the app.ping method is registered. Later phases register
// servers/subs/run/settings/helper/onboarding namespaces.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/itg-team/itg-ray/cmd/itgray-bridge/bus"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/dispatcher"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/handlers"
)

// lockedWriter serializes writes to an underlying io.Writer so dispatcher
// responses and bus notifications never interleave on stdout. Required
// because both subsystems use independent json.Encoder instances and
// Windows pipe writes are not atomic.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func main() {
	out := &lockedWriter{w: os.Stdout}

	d := dispatcher.New()
	app := handlers.AppHandlers{}
	d.Register("app.ping", app.Ping)

	// Bus is created here so later phases can wire it through to chainctl,
	// hub, etc., for outbound notifications.
	_ = bus.New(out)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	if err := d.Serve(ctx, os.Stdin, out); err != nil {
		fmt.Fprintln(os.Stderr, "bridge: serve:", err)
		os.Exit(1)
	}
}
