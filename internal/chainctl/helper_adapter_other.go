//go:build !windows

package chainctl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/core"
	"github.com/itg-team/itg-ray/internal/helper/xrayapi"
)

var errTunUnsupported = errors.New("TUN mode is not yet available on Linux (coming in Phase B); use SysProxy mode")

type coreRunner interface {
	Start(ctx context.Context, singboxJSON, xrayJSON []byte) error
	Stop() error
}

type counterSource interface {
	Counters(ctx context.Context) (up, down uint64, err error)
	Close() error
}

type coreHelperClient struct {
	newRunner func() coreRunner
	newStats  func() counterSource

	mu      sync.Mutex
	runner  coreRunner
	stats   counterSource
	running bool
}

func NewCoreHelperClient() HelperClient {
	return &coreHelperClient{
		newRunner: func() coreRunner { return core.NewManager() },
		newStats: func() counterSource {
			return xrayapi.New(fmt.Sprintf("127.0.0.1:%d", configgen.XrayAPIPort))
		},
	}
}

func (h *coreHelperClient) StartChain(ctx context.Context, singboxJSON, xrayJSON []byte, mode Mode) error {
	if mode == ModeTUN {
		return errTunUnsupported
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return fmt.Errorf("chainctl: chain already running")
	}
	runner := h.newRunner()
	if err := runner.Start(ctx, singboxJSON, xrayJSON); err != nil {
		return fmt.Errorf("core start: %w", err)
	}
	h.runner = runner
	h.stats = h.newStats()
	h.running = true
	return nil
}

func (h *coreHelperClient) StopChain(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return nil
	}
	if h.stats != nil {
		_ = h.stats.Close()
		h.stats = nil
	}
	var err error
	if h.runner != nil {
		err = h.runner.Stop()
		h.runner = nil
	}
	h.running = false
	return err
}

func (h *coreHelperClient) ServiceStatus(ctx context.Context) (ChainState, error) {
	h.mu.Lock()
	running := h.running
	stats := h.stats
	h.mu.Unlock()

	st := ChainState{Running: running}
	if running && stats != nil {
		if up, down, err := stats.Counters(ctx); err == nil {
			st.UpBytes = up
			st.DownBytes = down
		}
	}
	return st, nil
}

func (h *coreHelperClient) TunCreate(context.Context, string, string) error { return errTunUnsupported }
func (h *coreHelperClient) TunDestroy(context.Context) error                { return nil }
func (h *coreHelperClient) RouteSnapshot(context.Context) error             { return errTunUnsupported }
func (h *coreHelperClient) RouteAdd(context.Context, string) error          { return errTunUnsupported }
func (h *coreHelperClient) RouteRestore(context.Context) error              { return nil }
func (h *coreHelperClient) DnsSet(context.Context, []string) error          { return errTunUnsupported }
func (h *coreHelperClient) DnsRestore(context.Context) error                { return nil }

// modeRoutingHelperClient multiplexes chainctl's single HelperClient seam
// across two backends: the in-process core (SysProxy mode, no root) and the
// privileged unix-socket daemon (TUN mode, needs root for auto_route). The
// chosen backend is latched into `active` at StartChain time from the Mode
// argument; every subsequent lifecycle call (StopChain/ServiceStatus) then
// addresses that same backend so a TUN chain is never torn down through the
// core, or vice-versa.
//
// This mirrors the Windows single-adapter model — there OpStartChain bundles
// route/TUN/DNS work, so the granular per-op methods are no-ops. Here the
// same holds: bringUp (see ctrl.go) calls RouteSnapshot/TunCreate BEFORE
// StartChain in TUN mode, but both backends fold the real privileged work
// into StartChain, so those pre-start ops are unconditional no-ops. The
// remaining granular ops delegate to the active backend (falling back to the
// core before any StartChain latches `active`).
type modeRoutingHelperClient struct {
	core   HelperClient
	daemon HelperClient
	active HelperClient
}

func newModeRoutingHelperClient(core, daemon HelperClient) *modeRoutingHelperClient {
	return &modeRoutingHelperClient{core: core, daemon: daemon}
}

// StartChain latches the active backend from the requested mode, then
// delegates. TUN → daemon (privileged), everything else → core (in-process).
func (m *modeRoutingHelperClient) StartChain(ctx context.Context, sb, xr []byte, mode Mode) error {
	if mode == ModeTUN {
		m.active = m.daemon
	} else {
		m.active = m.core
	}
	return m.active.StartChain(ctx, sb, xr, mode)
}

// StopChain delegates to whichever backend StartChain latched. Nil-safe: a
// StopChain before any StartChain (idle teardown) is a no-op.
func (m *modeRoutingHelperClient) StopChain(ctx context.Context) error {
	if m.active == nil {
		return nil
	}
	return m.active.StopChain(ctx)
}

// ServiceStatus delegates to the active backend once one is latched. Before
// any StartChain (fresh boot, active==nil) it probes the daemon — the only
// backend whose TUN chain survives a bridge restart — so Reconcile can adopt
// a chain that outlived the GUI (Phase B acceptance criterion 5). If the
// daemon reports a running chain we latch it as active so subsequent
// StopChain/status reach it (adoption). A daemon that isn't installed or
// reachable (dial error) is reported as idle, NOT an error: a SysProxy-only
// machine must never see a Reconcile error from the boot probe.
func (m *modeRoutingHelperClient) ServiceStatus(ctx context.Context) (ChainState, error) {
	if m.active != nil {
		return m.active.ServiceStatus(ctx)
	}
	st, err := m.daemon.ServiceStatus(ctx)
	if err != nil {
		return ChainState{}, nil
	}
	if st.Running {
		m.active = m.daemon
	}
	return st, nil
}

// RouteSnapshot / TunCreate are no-ops: bringUp calls them before StartChain
// in TUN mode, but both backends bundle the real work into StartChain
// (mirrors helper_adapter_windows.go).
func (m *modeRoutingHelperClient) RouteSnapshot(context.Context) error             { return nil }
func (m *modeRoutingHelperClient) TunCreate(context.Context, string, string) error { return nil }

// delegate resolves the backend for the remaining granular ops: the latched
// active backend once StartChain has run, else the core (pre-start default).
func (m *modeRoutingHelperClient) delegate() HelperClient {
	if m.active != nil {
		return m.active
	}
	return m.core
}
func (m *modeRoutingHelperClient) TunDestroy(ctx context.Context) error         { return m.delegate().TunDestroy(ctx) }
func (m *modeRoutingHelperClient) RouteAdd(ctx context.Context, d string) error { return m.delegate().RouteAdd(ctx, d) }
func (m *modeRoutingHelperClient) RouteRestore(ctx context.Context) error       { return m.delegate().RouteRestore(ctx) }
func (m *modeRoutingHelperClient) DnsSet(ctx context.Context, s []string) error { return m.delegate().DnsSet(ctx, s) }
func (m *modeRoutingHelperClient) DnsRestore(ctx context.Context) error         { return m.delegate().DnsRestore(ctx) }

// NewRoutingHelperClient is the bridge entrypoint: SysProxy → in-process
// core, TUN → unix-socket daemon.
func NewRoutingHelperClient() HelperClient {
	return newModeRoutingHelperClient(NewCoreHelperClient(), newDaemonHelperClient())
}
