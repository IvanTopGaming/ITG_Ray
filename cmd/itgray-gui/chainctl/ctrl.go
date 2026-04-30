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
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/sysproxy"
)

// Mode is the connection mode requested by the user.
type Mode string

// Mode values surfaced to the rest of the GUI.
const (
	ModeTUN      Mode = "tun"
	ModeSysProxy Mode = "sysproxy"
)

// defaultTunName is the fixed TUN adapter name. Not user-configurable — the
// name is baked into the helper binary's wintun driver registration.
const defaultTunName = "ITGRay-TUN"

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

// ConfigBuilder produces the singbox+xray JSON pair for a given server,
// mode, and live network config. Injected so tests can stub it out without
// dragging the full configgen / vless stack into fixtures.
type ConfigBuilder func(srv *server.Server, mode Mode, net config.Network) (singboxJSON, xrayJSON []byte, err error)

// Deps is the constructor input.
type Deps struct {
	DataDir      string
	ServerStore  ServerStore
	Helper       HelperClient
	Sysproxy     sysproxy.Manager
	Hub          *hub.Hub
	BuildConfigs ConfigBuilder // optional; nil means "skip config generation" (tests + reconcile)
	// Network reads the user's persisted config.Network on every Connect
	// cycle. nil falls back to DefaultNetworkLoader (config.Defaults().Network),
	// which keeps existing tests / non-GUI consumers working.
	//
	// Concurrency contract: Network MUST be safe for concurrent calls.
	// bringUp invokes it from the worker goroutine launched in Start, while
	// a caller may concurrently mutate the underlying store via
	// SettingsService.Update / config.FileStore writes. Implementations
	// backed by config.Load(path) are safe by virtue of the FileStore's
	// internal locking; in-memory test loaders should likewise avoid
	// shared mutable state without a lock.
	Network func() (config.Network, error)
}

// networkSettingsView projects a config.Network into the camelCase shape
// the frontend expects on the vpn:status connected payload. Mirrors
// bindings.ConfigStore.toView's Network projection — duplicated here to
// avoid a chainctl → bindings import cycle. Tier 2b: payload reflects
// what the runtime ACTUALLY used at this Connect (avoids edit-during-
// connect race in the frontend reconnect-required pill).
func networkSettingsView(n config.Network) map[string]any {
	return map[string]any{
		"defaultMode": n.EffectiveMode(),
		"tunCidr":     n.TUN.IPv4CIDR,
		"tunMtu":      n.TUN.MTU,
		"tunName":     defaultTunName,
		"socksPort":   n.SysProxy.SOCKSPort,
		"httpPort":    n.SysProxy.HTTPPort,
		"allowLan":    n.AllowLAN,
		"ipv6Mode":    n.IPv6Mode,
		"dns": map[string]any{
			"mode":    n.DNS.Mode,
			"servers": n.DNS.Servers,
		},
	}
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

// New constructs a Controller. Defaults: when Deps.Network is nil, it is
// replaced with DefaultNetworkLoader so chainctl works against the
// stock config out of the box (tests, CLI use cases). Deps is taken by
// pointer because it is heavy enough that gocritic flags pass-by-value,
// and chainctl mirrors the bindings package convention where Deps live
// for the process lifetime anyway.
func New(d *Deps) *Controller {
	if d.Network == nil {
		d.Network = DefaultNetworkLoader()
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
		effectiveMode, net, err := c.bringUp(ctx, srv, mode)
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
				"network":  networkSettingsView(net),
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
// (preserved as a return for symmetry with future fall-back logic; today
// the effective mode always equals the requested mode) and the
// config.Network the runtime actually used — Start propagates the
// latter into the vpn:status connected payload so the frontend can
// snapshot exactly what landed (avoids edit-during-connect race).
//
//nolint:gocyclo,gocognit // orchestration sequence requires linear control flow so the rollback chain stays obvious
func (c *Controller) bringUp(ctx context.Context, srv *server.Server, mode Mode) (Mode, config.Network, error) {
	net, err := c.d.Network()
	if err != nil {
		c.d.Hub.Publish(hub.Event{
			Name:    hub.EventChainError,
			Payload: map[string]any{"message": fmt.Sprintf("config.Load: %v", err)},
		})
		return mode, config.Network{}, fmt.Errorf("config.Load: %w", err)
	}
	tunName := defaultTunName // package-level constant; not user-configurable
	tunCIDR := net.TUN.IPv4CIDR
	socksAddr := fmt.Sprintf("127.0.0.1:%d", net.SysProxy.SOCKSPort)
	httpAddr := fmt.Sprintf("127.0.0.1:%d", net.SysProxy.HTTPPort)
	dnsServers := ResolveDNS(net.DNS)

	var singboxJSON, xrayJSON []byte
	if c.d.BuildConfigs != nil {
		singboxJSON, xrayJSON, err = c.d.BuildConfigs(srv, mode, net)
		if err != nil {
			return mode, config.Network{}, fmt.Errorf("configgen: %w", err)
		}
	}

	if mode == ModeTUN {
		if err := c.d.Helper.RouteSnapshot(ctx); err != nil {
			return mode, config.Network{}, fmt.Errorf("RouteSnapshot: %w", err)
		}
		if err := c.d.Helper.TunCreate(ctx, tunName, tunCIDR); err != nil {
			_ = c.d.Helper.RouteRestore(ctx)
			return mode, config.Network{}, fmt.Errorf("TunCreate: %w", err)
		}
	}

	if err := c.d.Helper.StartChain(ctx, singboxJSON, xrayJSON); err != nil {
		if mode == ModeTUN {
			_ = c.d.Helper.TunDestroy(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
		}
		return mode, config.Network{}, fmt.Errorf("StartChain: %w", err)
	}

	if mode == ModeTUN {
		if err := c.d.Helper.RouteAdd(ctx, srv.Vless.Address); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			_ = c.d.Helper.TunDestroy(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
			return mode, config.Network{}, fmt.Errorf("RouteAdd: %w", err)
		}
		if err := c.d.Helper.DnsSet(ctx, dnsServers); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			_ = c.d.Helper.RouteRestore(ctx)
			_ = c.d.Helper.TunDestroy(ctx)
			return mode, config.Network{}, fmt.Errorf("DnsSet: %w", err)
		}
	} else {
		if err := c.d.Sysproxy.Set(sysproxy.Settings{Socks: socksAddr, HTTP: httpAddr}); err != nil {
			_ = c.d.Helper.StopChain(ctx)
			return mode, config.Network{}, fmt.Errorf("sysproxy.Set: %w", err)
		}
	}
	c.mu.Lock()
	c.mode = mode
	c.mu.Unlock()
	return mode, net, nil
}

// tearDown is best-effort: every step is independent and errors are
// swallowed so a partial bringup can still be unwound.
func (c *Controller) tearDown(ctx context.Context, mode Mode) {
	if mode == ModeSysProxy {
		_ = c.d.Sysproxy.Clear()
	}
	if mode == ModeTUN {
		_ = c.d.Helper.DnsRestore(ctx)
		_ = c.d.Helper.RouteRestore(ctx)
	}
	_ = c.d.Helper.StopChain(ctx)
	if mode == ModeTUN {
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
