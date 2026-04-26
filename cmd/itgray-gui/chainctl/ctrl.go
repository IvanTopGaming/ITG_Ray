// Package chainctl orchestrates Connect/Disconnect via the helper service.
// It owns the chain lifecycle on the GUI side: drives the helper RPC
// sequence, manages sysproxy registry, persists last-session.json for
// snapshot recovery, and runs the 1-Hz status poller.
//
// The HelperClient interface is intentionally narrow so unit tests can
// swap in a fake. The real helper-RPC client (internal/helper/client) is
// adapted by helper_adapter.go to satisfy this surface.
package chainctl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/sysproxy"
)

// Mode is the connection mode requested by the user.
type Mode string

// Mode values surfaced to the rest of the GUI.
const (
	ModeTUN      Mode = "tun"
	ModeSysProxy Mode = "sysproxy"
	ModeAuto     Mode = "auto"
)

// HelperClient is the small surface chainctl needs from the helper-RPC
// client. The real helper-RPC client (per Plan B) bundles many of these
// steps inside a single OpStartChain — the per-op methods here exist so
// tests can verify ordering and rollback. helper_adapter.go translates
// this surface to whatever the real client offers.
type HelperClient interface {
	StartChain(ctx context.Context, singboxJSON, xrayJSON []byte) error
	StopChain(ctx context.Context) error
	TunCreate(ctx context.Context, name, cidr string) error
	TunDestroy(ctx context.Context) error
	RouteSnapshot(ctx context.Context) error
	RouteAdd(ctx context.Context, dest string) error
	RouteRestore(ctx context.Context) error
	DnsSet(ctx context.Context, dns []string) error
	DnsRestore(ctx context.Context) error
	ServiceStatus(ctx context.Context) (ChainState, error)
}

// ChainState mirrors the helper's status response, expressed in the
// fields chainctl actually consumes (running flag for crash detection,
// byte counters for speed computation, last error message).
type ChainState struct {
	Running   bool
	UpBytes   uint64
	DownBytes uint64
	LastError string
}

// ServerStore is the narrow lookup contract chainctl needs. The runtime
// adapter (defined in main.go in a later task) reuses the existing
// bindings.ServerStore Load() shim; tests use a tiny in-memory map.
type ServerStore interface {
	Get(id string) (*server.Server, error)
}

// ConfigBuilder produces the singbox+xray JSON pair for a given server
// and mode. Injected so tests can stub it out without dragging the full
// configgen / vless stack into fixtures.
type ConfigBuilder func(srv *server.Server, mode Mode) (singboxJSON, xrayJSON []byte, err error)

// Deps is the constructor input.
type Deps struct {
	DataDir      string
	ServerStore  ServerStore
	Helper       HelperClient
	Sysproxy     sysproxy.Manager
	Hub          *hub.Hub
	BuildConfigs ConfigBuilder // optional; nil means "skip config generation" (tests + reconcile)
	SocksProxy   string        // sysproxy mode target, e.g. "127.0.0.1:1080"
	TunName      string        // e.g. "ITGRay-TUN"
	TunCIDR      string        // e.g. "198.18.0.1/15"
	DNSServers   []string      // e.g. {"1.1.1.1", "8.8.8.8"}
}

// Controller is the public type owning the chain lifecycle.
type Controller struct {
	d        Deps
	mu       sync.Mutex
	cancel   context.CancelFunc
	current  *server.Server
	mode     Mode
	prevUp   uint64
	prevDown uint64
	prevAt   time.Time
}

// New constructs a Controller. Defaults are filled for fields the caller
// left blank so chainctl works against the standard helper layout out of
// the box. Deps is taken by pointer because it is heavy enough (152 bytes)
// that gocritic flags pass-by-value, and chainctl mirrors the bindings
// package convention where Deps live for the process lifetime anyway.
func New(d *Deps) *Controller {
	if d.SocksProxy == "" {
		d.SocksProxy = "127.0.0.1:1080"
	}
	if d.TunName == "" {
		d.TunName = "ITGRay-TUN"
	}
	if d.TunCIDR == "" {
		d.TunCIDR = "198.18.0.1/15"
	}
	if len(d.DNSServers) == 0 {
		d.DNSServers = []string{"1.1.1.1", "8.8.8.8"}
	}
	return &Controller{d: *d}
}

// Start launches a connect attempt asynchronously. State changes flow via
// hub events. Returns immediately after the helper accepts; further state
// arrives through events. Calling Start while already connected returns an
// error — caller should Stop first.
func (c *Controller) Start(ctx context.Context, serverID string, mode Mode) error {
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return fmt.Errorf("chainctl: already connected")
	}
	srv, err := c.d.ServerStore.Get(serverID)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	if srv == nil {
		c.mu.Unlock()
		return fmt.Errorf("chainctl: server %q not found", serverID)
	}

	pollCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.current = srv
	c.mode = mode
	c.prevAt = time.Now()
	c.prevUp = 0
	c.prevDown = 0
	c.mu.Unlock()

	c.d.Hub.Publish(hub.Event{
		Name:    hub.EventVPNStatus,
		Payload: map[string]any{"status": string(hub.StatusConnecting)},
	})

	go func() {
		effectiveMode, err := c.bringUp(ctx, srv, mode)
		if err != nil {
			c.d.Hub.Publish(hub.Event{
				Name: hub.EventChainError,
				Payload: map[string]any{
					"kind":    "bringup_failed",
					"message": err.Error(),
				},
			})
			cancel()
			c.mu.Lock()
			c.cancel = nil
			c.current = nil
			c.mu.Unlock()
			c.d.Hub.Publish(hub.Event{
				Name:    hub.EventVPNStatus,
				Payload: map[string]any{"status": string(hub.StatusIdle)},
			})
			return
		}
		_ = saveSession(c.d.DataDir, sessionRecord{
			ServerID: srv.ID,
			Mode:     string(effectiveMode),
			At:       time.Now(),
		})
		c.d.Hub.Publish(hub.Event{
			Name: hub.EventVPNStatus,
			Payload: map[string]any{
				"status":   string(hub.StatusConnected),
				"serverId": srv.ID,
				"mode":     string(effectiveMode),
			},
		})
		c.runPoller(pollCtx)
	}()
	return nil
}

// Stop tears down the chain. Idempotent — safe to call when already idle.
func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	mode := c.mode
	c.current = nil
	c.mu.Unlock()
	if cancel == nil {
		// Already idle. Don't emit transitions, don't touch session.
		return nil
	}
	cancel()
	c.d.Hub.Publish(hub.Event{
		Name:    hub.EventVPNStatus,
		Payload: map[string]any{"status": string(hub.StatusDisconnecting)},
	})
	c.tearDown(ctx, mode)
	c.d.Hub.Publish(hub.Event{
		Name:    hub.EventVPNStatus,
		Payload: map[string]any{"status": string(hub.StatusIdle)},
	})
	_ = clearSession(c.d.DataDir)
	return nil
}

// Status returns the cached current state derived from the last poll.
func (c *Controller) Status() (hub.ChainStatus, *server.Server, Mode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel == nil {
		return hub.StatusIdle, nil, ""
	}
	return hub.StatusConnected, c.current, c.mode
}

// LastSession returns the last persisted (serverID, mode) pair if any.
// Used by tray "Connect (last server)" actions in C.T13.
func (c *Controller) LastSession() (serverID, mode string) {
	rec, err := loadSession(c.d.DataDir)
	if err != nil {
		return "", ""
	}
	return rec.ServerID, rec.Mode
}

// bringUp performs the helper-RPC sequence. Returns the effective mode
// (which can differ from the requested mode if ModeAuto fell back from
// TUN to sysproxy after a TunCreate failure).
//
//nolint:gocyclo,gocognit // orchestration sequence requires linear control flow so the rollback chain stays obvious
func (c *Controller) bringUp(ctx context.Context, srv *server.Server, mode Mode) (Mode, error) {
	var singboxJSON, xrayJSON []byte
	if c.d.BuildConfigs != nil {
		var err error
		singboxJSON, xrayJSON, err = c.d.BuildConfigs(srv, mode)
		if err != nil {
			return mode, fmt.Errorf("configgen: %w", err)
		}
	}

	if mode == ModeTUN || mode == ModeAuto {
		if err := c.d.Helper.RouteSnapshot(ctx); err != nil {
			return mode, fmt.Errorf("RouteSnapshot: %w", err)
		}
		if err := c.d.Helper.TunCreate(ctx, c.d.TunName, c.d.TunCIDR); err != nil {
			if mode == ModeTUN {
				_ = c.d.Helper.RouteRestore(ctx)
				return mode, fmt.Errorf("TunCreate: %w", err)
			}
			// Auto: fall back to sysproxy. Roll back the route snapshot
			// since we won't be using TUN after all.
			_ = c.d.Helper.RouteRestore(ctx)
			mode = ModeSysProxy
		}
	}

	if err := c.d.Helper.StartChain(ctx, singboxJSON, xrayJSON); err != nil {
		if mode == ModeTUN || mode == ModeAuto {
			_ = c.d.Helper.TunDestroy(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
		}
		return mode, fmt.Errorf("StartChain: %w", err)
	}

	if mode == ModeTUN || mode == ModeAuto {
		if err := c.d.Helper.RouteAdd(ctx, srv.Vless.Address); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			_ = c.d.Helper.TunDestroy(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
			return mode, fmt.Errorf("RouteAdd: %w", err)
		}
		if err := c.d.Helper.DnsSet(ctx, c.d.DNSServers); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
			_ = c.d.Helper.TunDestroy(ctx)
			return mode, fmt.Errorf("DnsSet: %w", err)
		}
	} else {
		if err := c.d.Sysproxy.Set(c.d.SocksProxy); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			return mode, fmt.Errorf("sysproxy.Set: %w", err)
		}
	}
	c.mu.Lock()
	c.mode = mode
	c.mu.Unlock()
	return mode, nil
}

// tearDown is best-effort: every step is independent and errors are
// swallowed so a partial bringup can still be unwound.
func (c *Controller) tearDown(ctx context.Context, mode Mode) {
	if mode == ModeSysProxy {
		_ = c.d.Sysproxy.Clear()
	}
	if mode == ModeTUN || mode == ModeAuto {
		_ = c.d.Helper.DnsRestore(ctx)
		_ = c.d.Helper.RouteRestore(ctx)
	}
	_ = c.d.Helper.StopChain(ctx)
	if mode == ModeTUN || mode == ModeAuto {
		_ = c.d.Helper.TunDestroy(ctx)
	}
}

// Reconcile is called at app boot. It stays idle by default — the
// helper-side OpServiceStatus only reports whether the SERVICE is alive
// (not the chain inside it), so we cannot reliably detect an orphaned
// chain from a previous GUI/CLI session. Until the helper exposes a
// real "chain alive" probe (TODO: plan-c-helper-chainstatus), every GUI
// boot starts in the idle state and the user reconnects explicitly.
//
// We still consult last-session.json to PRE-FILL the picker selection —
// useful UX continuity — but we never set c.cancel or emit a connected
// event from this path.
func (c *Controller) Reconcile(_ context.Context) {
	rec, err := loadSession(c.d.DataDir)
	if err != nil || rec.ServerID == "" {
		return
	}
	srv, err := c.d.ServerStore.Get(rec.ServerID)
	if err != nil || srv == nil {
		_ = clearSession(c.d.DataDir)
		return
	}
	c.mu.Lock()
	c.current = srv
	c.mode = Mode(rec.Mode)
	c.mu.Unlock()
}
