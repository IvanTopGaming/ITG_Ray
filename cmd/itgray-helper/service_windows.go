//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/helper/server"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "ITGRayHelper"

type handler struct {
	dispatcher *server.Dispatcher
	startedAt  time.Time
}

func runService() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("svc.IsWindowsService: %w", err)
	}
	h := &handler{
		dispatcher: buildDispatcher(),
		startedAt:  time.Now(),
	}
	if !isService {
		// Interactive: spin up the dispatcher anyway so the developer can
		// `itgray-cli helper status` against a manually-launched binary.
		fmt.Println("itgray-helper " + Version + " (interactive run)")
		return runInteractive(h)
	}
	return svc.Run(serviceName, h)
}

// Execute satisfies svc.Handler. Loops on SCM control requests; the actual
// pipe listener runs in runListener (Phase B5 wires the listener; for B4 we
// only handle SCM control).
//
//nolint:gocritic // Return signature (bool, uint32) is dictated by svc.Handler; cannot be renamed.
func (h *handler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listenerErr := make(chan error, 1)
	go func() { listenerErr <- runListener(ctx, h.dispatcher) }()

	s <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				cancel()
				<-listenerErr
				return false, 0
			default:
				slog.Warn("helper: unexpected SCM control", slog.Int("cmd", int(c.Cmd)))
			}
		case err := <-listenerErr:
			slog.Error("helper: listener exited", slog.String("err", fmt.Sprintf("%v", err)))
			return false, 1
		}
	}
}

func runInteractive(h *handler) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return runListener(ctx, h.dispatcher)
}

func buildDispatcher() *server.Dispatcher {
	d := server.NewDispatcher()
	d.Register(protocol.OpServiceStatus, server.NewServiceStatusHandler(Version, time.Now()))
	d.Register(protocol.OpTunCreate, server.NewTunCreateHandler())
	d.Register(protocol.OpTunDestroy, server.NewTunDestroyHandler())
	d.Register(protocol.OpDnsRestore, server.NewDnsRestoreHandler())
	d.Register(protocol.OpDnsSet, server.NewDnsSetHandler())
	d.Register(protocol.OpRouteAdd, server.NewRouteAddHandler())
	d.Register(protocol.OpRouteRemove, server.NewRouteRemoveHandler())
	d.Register(protocol.OpRouteRestore, server.NewRouteRestoreHandler())
	d.Register(protocol.OpRouteSnapshot, server.NewRouteSnapshotHandler())
	d.Register(protocol.OpStartChain, server.NewStartChainHandler())
	d.Register(protocol.OpStopChain, server.NewStopChainHandler())
	return d
}

// runListener is the per-connection dispatcher loop.
func runListener(ctx context.Context, d *server.Dispatcher) error {
	return server.Listen(ctx, PipeName, d)
}
