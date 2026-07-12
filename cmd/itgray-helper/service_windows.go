//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/helper/route"
	"github.com/itg-team/itg-ray/internal/helper/runtime"
	"github.com/itg-team/itg-ray/internal/helper/server"
	"github.com/itg-team/itg-ray/internal/helper/undo"

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

	// Replay any leftover undo journal from a prior crashed session before
	// the listener begins accepting requests.
	recoverFromUndo(slog.Default())

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
				// Tear the active chain down BEFORE cancelling the
				// listener. If cancel runs first, the listener goroutine
				// exits and any in-flight OpStopChain races with us;
				// holding chainMu inside StopActiveChain serializes
				// correctly. Without this hook, sing-box.exe / xray.exe
				// outlive the helper as orphans (Plan B.5 papercut).
				if err := server.StopActiveChain(); err != nil {
					slog.Warn("helper: chain teardown reported partial errors during SCM stop",
						slog.String("err", err.Error()))
				}
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
	// Same crash-recovery sweep as the SCM path so manual debug runs also
	// repair host state if a prior session left an undo journal behind.
	recoverFromUndo(slog.Default())
	return runListener(ctx, h.dispatcher)
}

func buildDispatcher() *server.Dispatcher {
	d := server.NewDispatcher()
	d.Register(protocol.OpServiceStatus, server.NewServiceStatusHandler(Version, time.Now(), server.IsChainActive))
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
	d.Register(protocol.OpReadLogs, server.NewReadLogsHandler())
	return d
}

// runListener is the per-connection dispatcher loop.
func runListener(ctx context.Context, d *server.Dispatcher) error {
	return server.Listen(ctx, PipeName, d)
}

// recoverFromUndo replays the undo journal on Helper startup. Idempotent —
// safe to call when no journal exists. The path matches undoPath() in
// chain_windows.go (%ProgramData%\ITG Ray\Helper\undo.json).
func recoverFromUndo(logger *slog.Logger) {
	path := runtime.ConfigPath("..\\undo.json")
	j, err := undo.Load(path)
	if err != nil {
		logger.Error("undo.Load", "err", err)
		return
	}
	if len(j.Routes) == 0 && len(j.DNSPrior) == 0 && j.TunName == "" {
		// No prior session.
		return
	}
	logger.Warn("recovering from prior crash",
		"tun_name", j.TunName,
		"route_entries", len(j.Routes),
		"dns_entries", len(j.DNSPrior),
	)
	// Best-effort restore: re-add routes that disappeared since the prior
	// snapshot. Key shape matches B7.2 RouteRestore and B5.4.1 OpStopChain
	// byte-for-byte (Metric excluded so we do not falsely diff on metric
	// drift between snapshots).
	if cur, err := route.Snapshot(); err == nil {
		want := map[string]route.Entry{}
		for _, e := range j.Routes {
			key := fmt.Sprintf("%s|%s|%d", e.DestCIDR, e.NextHop, e.InterfaceLUID)
			want[key] = e
		}
		have := map[string]route.Entry{}
		for _, e := range cur {
			key := fmt.Sprintf("%s|%s|%d", e.DestCIDR, e.NextHop, e.InterfaceLUID)
			have[key] = e
		}
		for k, e := range want {
			if _, ok := have[k]; !ok {
				if err := route.Add(e); err != nil {
					logger.Error("recovery route.Add", "err", err)
				}
			}
		}
	} else {
		logger.Error("recovery route.Snapshot", "err", err)
	}
	for _, s := range j.DNSPrior {
		if err := dns.Restore(s); err != nil {
			logger.Error("recovery dns.Restore", "err", err)
		}
	}
	if err := undo.Clear(path); err != nil {
		logger.Error("recovery undo.Clear", "err", err)
	}
	logger.Info("recovery complete")
}
