package bindings

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

// trayRecorder captures IconSetter / MenuSetter calls so tests can
// assert on the sequence emitted by TrayService.refresh.
type trayRecorder struct {
	mu    sync.Mutex
	icons []TrayIconName
	menus [][]TrayMenuItem
}

func (r *trayRecorder) setIcon(name TrayIconName, _ []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.icons = append(r.icons, name)
}

func (r *trayRecorder) setMenu(items []TrayMenuItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]TrayMenuItem, len(items))
	copy(cp, items)
	r.menus = append(r.menus, cp)
}

func (r *trayRecorder) lastIcon(t *testing.T) TrayIconName {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.icons, "no icon recorded")
	return r.icons[len(r.icons)-1]
}

func (r *trayRecorder) lastMenu(t *testing.T) []TrayMenuItem {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.menus, "no menu recorded")
	return r.menus[len(r.menus)-1]
}

// setupTray builds a TrayService against a real (lightweight)
// chainctl.Controller — same fakes as run_test.go — plus a recorder for
// the icon / menu setters. Returns the controller's DataDir so tests
// can seed session records (last-session.json) directly.
func setupTray(t *testing.T) (ts *TrayService, rec *trayRecorder, h *hub.Hub, ctrl *chainctl.Controller, dir string) {
	t.Helper()
	dir = t.TempDir()
	srv := &server.Server{
		ID:     "a",
		Origin: server.OriginManual,
		Name:   "DE",
		Vless: vless.Config{
			Address:   "127.0.0.1",
			Port:      443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
			Security:  vless.SecurityNone,
		},
	}
	store := runMemStore{m: map[string]*server.Server{"a": srv}}
	h = hub.New()
	t.Cleanup(h.Close)
	ctrl = chainctl.New(&chainctl.Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      &runFakeHelper{},
		Sysproxy:    runFakeSysproxy{},
		Hub:         h,
	})
	rec = &trayRecorder{}
	ts = NewTrayService(TrayDeps{
		Hub:     h,
		Chain:   ctrl,
		OnShow:  func() {},
		OnQuit:  func() {},
		SetIcon: rec.setIcon,
		SetMenu: rec.setMenu,
	})
	t.Cleanup(ts.Shutdown)
	return ts, rec, h, ctrl, dir
}

// labels extracts the Label / Separator marker for each item; useful
// for compact menu-shape assertions.
func labels(items []TrayMenuItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.Separator {
			out = append(out, "---")
			continue
		}
		out = append(out, it.Label)
	}
	return out
}

// TestTray_InitSeedsIdle asserts Init publishes an idle icon + the
// "Disconnected" header even before the first hub event arrives.
func TestTray_InitSeedsIdle(t *testing.T) {
	ts, rec, _, _, _ := setupTray(t)
	ts.Init(context.Background())

	require.Equal(t, TrayIconIdle, rec.lastIcon(t))
	got := labels(rec.lastMenu(t))
	require.Equal(t,
		[]string{"Disconnected", "Connect (last server)", "---", "Show window", "Quit"},
		got,
	)
}

// TestTray_MenuPerStatus walks each ChainStatus and asserts the icon +
// header + first action label match.
func TestTray_MenuPerStatus(t *testing.T) {
	cases := []struct {
		status string
		icon   TrayIconName
		header string
		action string
	}{
		{string(hub.StatusIdle), TrayIconIdle, "Disconnected", "Connect (last server)"},
		{string(hub.StatusConnecting), TrayIconConnecting, "Connecting…", "Connect (last server)"},
		{string(hub.StatusConnected), TrayIconConnected, "Connected", "Disconnect"},
		{string(hub.StatusDisconnecting), TrayIconConnecting, "Disconnecting…", "Connect (last server)"},
		{string(hub.StatusError), TrayIconError, "Error", "Connect (last server)"},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			ts, _, _, _, _ := setupTray(t)
			ts.refresh(tc.status)

			gotIcon, gotItems := ts.Snapshot()
			require.Equal(t, tc.icon, gotIcon)
			require.Equal(t, tc.header, gotItems[0].Label)
			require.True(t, gotItems[0].Disabled, "header item must be disabled")
			require.Equal(t, tc.action, gotItems[1].Label)
		})
	}
}

// TestTray_ConnectLastDisabledWithoutSession asserts the
// "Connect (last server)" item is disabled when no last-session.json
// exists — the click would no-op anyway, but disabling surfaces the
// state to the user.
func TestTray_ConnectLastDisabledWithoutSession(t *testing.T) {
	ts, _, _, _, _ := setupTray(t)
	ts.refresh(string(hub.StatusIdle))

	_, items := ts.Snapshot()
	require.Equal(t, "Connect (last server)", items[1].Label)
	require.True(t, items[1].Disabled, "expected disabled when LastSession is empty")
}

// TestTray_ConnectLastEnabledAfterStart asserts the connect item
// becomes enabled once chainctl has persisted a last-session record.
// We assert against the connecting state (status=connecting still
// surfaces "Connect (last server)" because we are not yet "connected"
// — same code path as idle, with a session present so the item is
// enabled). Stop() clears the session record by design, so we cannot
// assert against the post-disconnect state.
func TestTray_ConnectLastEnabledAfterStart(t *testing.T) {
	ts, _, _, ctrl, _ := setupTray(t)
	require.NoError(t, ctrl.Start(context.Background(), "a", chainctl.ModeSysProxy))
	require.Eventually(t, func() bool {
		id, _ := ctrl.LastSession()
		return id == "a"
	}, time.Second, 10*time.Millisecond)

	ts.refresh(string(hub.StatusError)) // not connected -> Connect item exposed
	_, items := ts.Snapshot()
	require.Equal(t, "Connect (last server)", items[1].Label)
	require.False(t, items[1].Disabled, "expected enabled connect after Start persisted session")
}

// TestTray_ConnectLast_NormalizesLegacyAutoMode asserts a session
// record persisted by a pre-ModeAuto-removal build (mode="auto") is
// normalized to TUN before being passed to Chain.Start. Without this,
// chainctl.Start receives an unknown Mode and the click is a no-op.
func TestTray_ConnectLast_NormalizesLegacyAutoMode(t *testing.T) {
	ts, _, _, ctrl, dir := setupTray(t)

	sessJSON := `{"serverId":"a","mode":"auto","at":"2026-04-01T00:00:00Z"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "last-session.json"), []byte(sessJSON), 0o600))

	id, mode := ctrl.LastSession()
	require.Equal(t, "a", id)
	require.Equal(t, "auto", mode, "sanity: legacy mode persisted")

	ts.actionConnectLast()

	require.Eventually(t, func() bool {
		s, _, m := ctrl.Status()
		return s == hub.StatusConnected && m == "tun"
	}, time.Second, 10*time.Millisecond, "legacy auto must normalize to TUN")
}

// TestTray_HubDrivesRefresh asserts a published vpn:status event flows
// through the subscriber and rebuilds the menu.
func TestTray_HubDrivesRefresh(t *testing.T) {
	ts, rec, h, _, _ := setupTray(t)
	ts.Init(context.Background())

	h.Publish(hub.Event{
		Name:    hub.EventVPNStatus,
		Payload: map[string]any{"status": string(hub.StatusConnected)},
	})

	require.Eventually(t, func() bool {
		return rec.lastIcon(t) == TrayIconConnected
	}, time.Second, 10*time.Millisecond)

	got := labels(rec.lastMenu(t))
	require.Equal(t,
		[]string{"Connected", "Disconnect", "---", "Show window", "Quit"},
		got,
	)
}

// TestTray_ShowAndQuitInvokeCallbacks asserts the Show / Quit items
// invoke the OnShow / OnQuit hooks supplied by main.go.
func TestTray_ShowAndQuitInvokeCallbacks(t *testing.T) {
	var shows, quits atomic.Int32
	h := hub.New()
	t.Cleanup(h.Close)
	ts := NewTrayService(TrayDeps{
		Hub:    h,
		OnShow: func() { shows.Add(1) },
		OnQuit: func() { quits.Add(1) },
	})
	ts.refresh(string(hub.StatusIdle))

	_, items := ts.Snapshot()
	// items[2] is the separator; items[3] = Show, items[4] = Quit.
	require.Equal(t, "Show window", items[3].Label)
	require.Equal(t, "Quit", items[4].Label)
	items[3].Click()
	items[4].Click()
	require.Equal(t, int32(1), shows.Load())
	require.Equal(t, int32(1), quits.Load())
}

// TestTray_UnknownStatusFallsBackToIdle asserts a forward-compatible
// hub event with an unrecognised status renders as the raw value (so
// the desync is visible) but still picks the idle icon.
func TestTray_UnknownStatusFallsBackToIdle(t *testing.T) {
	ts, _, _, _, _ := setupTray(t)
	ts.refresh("future-state")

	icon, items := ts.Snapshot()
	require.Equal(t, TrayIconIdle, icon)
	require.Equal(t, "future-state", items[0].Label)
}

// TestTrayIconBytes asserts the public byte resolver returns the
// embedded PNG for each known name and falls back to idle for unknown.
func TestTrayIconBytes(t *testing.T) {
	require.NotEmpty(t, TrayIconBytes(TrayIconConnected))
	require.NotEmpty(t, TrayIconBytes(TrayIconConnecting))
	require.NotEmpty(t, TrayIconBytes(TrayIconError))
	require.NotEmpty(t, TrayIconBytes(TrayIconIdle))
	require.Equal(t, TrayIconBytes(TrayIconIdle), TrayIconBytes(TrayIconName("bogus")))
}
