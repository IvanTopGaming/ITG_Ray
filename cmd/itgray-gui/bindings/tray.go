package bindings

import (
	"context"
	"sync"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/icons"
)

// Tray icon bytes are owned by cmd/itgray-gui/icons (which sits next to
// the PNGs because //go:embed cannot escape its own package). Wails
// v2.11 has no first-party SystemTray runtime API — the
// `pkg/menu/tray.go` TrayMenu struct is wired through the build asset
// pipeline only on the v3 branch and is "on hold" in v2 — so the bytes
// ride along the binary waiting for an adapter (fyne.io/systray or the
// v3 SystemTray when it lands).
var (
	iconConnected  = icons.TrayConnected
	iconConnecting = icons.TrayConnecting
	iconError      = icons.TrayError
	iconIdle       = icons.TrayIdle
)

// TrayIconName is the symbolic name for the active tray icon. It maps
// 1:1 to the embedded PNGs above; callers that render an actual OS tray
// translate it to bytes via TrayIconBytes.
type TrayIconName string

// TrayIconName values surfaced on every status change.
const (
	TrayIconIdle       TrayIconName = "idle"
	TrayIconConnecting TrayIconName = "connecting"
	TrayIconConnected  TrayIconName = "connected"
	TrayIconError      TrayIconName = "error"
)

// TrayMenuItem is a UI-agnostic menu entry. Click is nil for separators
// and disabled informational rows; tray-rendering adapters skip those.
type TrayMenuItem struct {
	Label     string
	Separator bool
	Disabled  bool
	Click     func()
}

// IconSetter is invoked when the tray icon should change. In production
// this is wired to whatever tray library main.go selects; in tests we
// inject a recorder. Nil means "do nothing" — useful when the GUI runs
// headless (e.g. integration test harness) and there is no tray to
// drive.
type IconSetter func(name TrayIconName, png []byte)

// MenuSetter is invoked when the tray context menu should be rebuilt.
// Same nil-allowed contract as IconSetter.
type MenuSetter func(items []TrayMenuItem)

// TrayDeps groups the dependencies for TrayService.
//
// Chain is required for the Connect/Disconnect actions; Hub is required
// to subscribe to vpn:status events that drive icon + menu rebuilds.
// OnShow / OnQuit are invoked by the corresponding menu items — main.go
// wires these to runtime.WindowShow / runtime.Quit. SetIcon and SetMenu
// are nil-tolerant adapters; see IconSetter / MenuSetter docs.
type TrayDeps struct {
	Hub     *hub.Hub
	Chain   *chainctl.Controller
	OnShow  func()
	OnQuit  func()
	SetIcon IconSetter
	SetMenu MenuSetter
}

// TrayService subscribes to vpn:status hub events and rebuilds the tray
// icon + context menu on every change. The actual OS-level tray
// rendering is delegated to the IconSetter / MenuSetter adapters
// supplied via TrayDeps; the architectural piece (event-driven menu
// rebuild) lives here so we can ship a real adapter later without
// touching the event flow.
//
// The current icon name and menu are also exposed via Snapshot for
// tests and for any future Wails binding that wants to render a custom
// HTML tray (e.g. a borderless detached window).
type TrayService struct {
	d TrayDeps

	mu      sync.Mutex
	ctx     context.Context
	icon    TrayIconName
	menu    []TrayMenuItem
	cancelF func()
}

// NewTrayService constructs a TrayService. Init must be called from
// app.Startup before events arrive.
func NewTrayService(d TrayDeps) *TrayService {
	return &TrayService{d: d, icon: TrayIconIdle}
}

// Init subscribes to vpn:status events and seeds the initial idle
// menu. Returns immediately; the subscriber goroutine runs until ctx is
// cancelled or Shutdown is called.
func (t *TrayService) Init(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	t.mu.Unlock()

	c := t.d.Hub.Subscribe(8)
	done := make(chan struct{})
	t.mu.Lock()
	t.cancelF = func() {
		t.d.Hub.Unsubscribe(c)
		<-done
	}
	t.mu.Unlock()

	go func() {
		defer close(done)
		for e := range c {
			if e.Name != hub.EventVPNStatus {
				continue
			}
			status, _ := e.Payload["status"].(string)
			t.refresh(status)
		}
	}()

	t.refresh(string(hub.StatusIdle))
}

// Shutdown unsubscribes from the hub and waits for the subscriber
// goroutine to drain. Safe to call multiple times.
func (t *TrayService) Shutdown() {
	t.mu.Lock()
	cancel := t.cancelF
	t.cancelF = nil
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Snapshot returns the current icon name and a copy of the current menu
// items. Used by tests and by future bindings that want to surface tray
// state to the frontend.
func (t *TrayService) Snapshot() (TrayIconName, []TrayMenuItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TrayMenuItem, len(t.menu))
	copy(out, t.menu)
	return t.icon, out
}

// refresh rebuilds the icon + menu for the supplied status. The status
// string matches hub.ChainStatus values; unknown values fall back to
// idle (defensive: a forward-compatible backend may emit a status the
// GUI does not yet know).
func (t *TrayService) refresh(status string) {
	icon, png := iconForStatus(status)
	items := t.buildMenu(status)

	t.mu.Lock()
	t.icon = icon
	t.menu = items
	setIcon := t.d.SetIcon
	setMenu := t.d.SetMenu
	t.mu.Unlock()

	if setIcon != nil {
		setIcon(icon, png)
	}
	if setMenu != nil {
		setMenu(items)
	}
}

// iconForStatus maps a hub.ChainStatus string to the matching icon name
// and embedded PNG bytes. Exported names use TrayIconName; callers that
// only need the bytes can use TrayIconBytes.
func iconForStatus(status string) (name TrayIconName, png []byte) {
	switch status {
	case string(hub.StatusConnected):
		return TrayIconConnected, iconConnected
	case string(hub.StatusConnecting), string(hub.StatusDisconnecting):
		return TrayIconConnecting, iconConnecting
	case string(hub.StatusError):
		return TrayIconError, iconError
	default:
		return TrayIconIdle, iconIdle
	}
}

// TrayIconBytes returns the embedded PNG bytes for the given icon name.
// Returns the idle icon for unknown names. Exposed so a future tray
// adapter in main.go can resolve names to bytes without importing the
// embed-private vars.
func TrayIconBytes(name TrayIconName) []byte {
	switch name {
	case TrayIconConnected:
		return iconConnected
	case TrayIconConnecting:
		return iconConnecting
	case TrayIconError:
		return iconError
	default:
		return iconIdle
	}
}

// buildMenu produces the context-menu item list for the given status.
// The shape is fixed: status header (disabled), one connect/disconnect
// action, separator, Show window, Quit. Tests cover each branch.
func (t *TrayService) buildMenu(status string) []TrayMenuItem {
	items := []TrayMenuItem{
		{Label: statusLabel(status), Disabled: true},
	}

	if status == string(hub.StatusConnected) {
		items = append(items, TrayMenuItem{
			Label: "Disconnect",
			Click: t.actionDisconnect,
		})
	} else {
		items = append(items, TrayMenuItem{
			Label:    "Connect (last server)",
			Disabled: t.lastSessionEmpty(),
			Click:    t.actionConnectLast,
		})
	}

	items = append(items,
		TrayMenuItem{Separator: true},
		TrayMenuItem{Label: "Show window", Click: t.actionShow},
		TrayMenuItem{Label: "Quit", Click: t.actionQuit},
	)
	return items
}

// statusLabel returns the human-readable header line for the given
// status. Unknown statuses display the raw value so a desync between
// frontend and backend is still visible to the user.
func statusLabel(status string) string {
	switch status {
	case string(hub.StatusConnected):
		return "Connected"
	case string(hub.StatusConnecting):
		return "Connecting…"
	case string(hub.StatusDisconnecting):
		return "Disconnecting…"
	case string(hub.StatusError):
		return "Error"
	case string(hub.StatusIdle), "":
		return "Disconnected"
	default:
		return status
	}
}

func (t *TrayService) lastSessionEmpty() bool {
	if t.d.Chain == nil {
		return true
	}
	id, _ := t.d.Chain.LastSession()
	return id == ""
}

func (t *TrayService) actionDisconnect() {
	if t.d.Chain == nil {
		return
	}
	ctx := t.contextOrBackground()
	_ = t.d.Chain.Stop(ctx)
}

func (t *TrayService) actionConnectLast() {
	if t.d.Chain == nil {
		return
	}
	id, mode := t.d.Chain.LastSession()
	if id == "" {
		return
	}
	if mode == "" {
		mode = string(chainctl.ModeTUN)
	}
	ctx := t.contextOrBackground()
	_ = t.d.Chain.Start(ctx, id, chainctl.Mode(mode))
}

func (t *TrayService) actionShow() {
	if t.d.OnShow != nil {
		t.d.OnShow()
	}
}

func (t *TrayService) actionQuit() {
	if t.d.OnQuit != nil {
		t.d.OnQuit()
	}
}

func (t *TrayService) contextOrBackground() context.Context {
	t.mu.Lock()
	ctx := t.ctx
	t.mu.Unlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
