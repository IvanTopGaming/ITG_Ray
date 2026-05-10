// Command itgray-bridge is the JSON-RPC backend for the Electron-based
// ITG Ray GUI. It reads requests from stdin, writes responses to stdout.
// Phase 0: app.ping. Phase 3.A: app.getSnapshot + onboarding.*.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/itg-team/itg-ray/cmd/itgray-bridge/bus"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/dispatcher"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/handlers"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/bindings"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/hwid"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// BuildDate is overridden at build time via -ldflags "-X main.BuildDate=...".
var BuildDate = ""

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

// serversFileStore adapts the package-level server.Load / server.Save
// to bindings.ServerStore. Mirrors the same struct in cmd/itgray-gui/main.go.
type serversFileStore struct{ path string }

func (s serversFileStore) Load() ([]server.Server, error)  { return server.Load(s.path) }
func (s serversFileStore) Save(list []server.Server) error { return server.Save(s.path, list) }

func defaultDataDir() string {
	if v := os.Getenv("ITGRAY_DATA_DIR"); v != "" {
		return v
	}
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		// Fallback: home-dir/.config-style path. Only triggered if both
		// XDG and HOME are unset; in that case the bridge will fail at
		// store load time and surface an error to the renderer.
		return filepath.Join(".", "ITG Ray")
	}
	return filepath.Join(cfg, "ITG Ray")
}

func main() {
	out := &lockedWriter{w: os.Stdout}

	dataDir := defaultDataDir()
	configPath := filepath.Join(dataDir, "config.json")
	serverStore := serversFileStore{path: filepath.Join(dataDir, "servers.json")}
	subStore := subscription.FileStore{Path: filepath.Join(dataDir, "subscriptions.json")}

	configStore := bindings.NewConfigStore(configPath, handlers.Version, BuildDate)
	appSvc := bindings.NewAppService(&bindings.AppDeps{
		DataDir:      dataDir,
		Version:      handlers.Version,
		ServerStore:  serverStore,
		SubStore:     subStore,
		ConfigViewer: configStore,
		// Chain, HelperProber, AppCtx are nil for Phase 3.A — Phase 3.C
		// wires the live chain controller and helper prober. GetSnapshot
		// tolerates nil and returns StatusIdle / "tun" / "missing".
	})
	onboardingSvc := bindings.NewOnboardingService(bindings.OnboardingDeps{DataDir: dataDir})

	settingsSvc := bindings.NewSettingsService(bindings.SettingsDeps{
		Store: configStore,
		// Hub is nil — Phase 4 wires hub.EventSettings forwarding through
		// bus.Emit. Without a Hub, Update succeeds but no bridge → main
		// notification is published. The renderer either uses the returned
		// SettingsView or calls Get to refresh.
		Hub: nil,
	})
	helperSvc := bindings.NewHelperService()

	// Bridge owns its own hub. ServersService and SubsService publish
	// servers.changed / sub.synced events into it; Phase 4 will subscribe
	// a forwarder that emits them as JSON-RPC notifications over stdout.
	h := hub.New()

	// HWID + DeviceInfo for SubsService HWID-aware sync. Failure is
	// non-fatal: SubsService treats empty HWID as "HWID disabled".
	hwidValue, err := hwid.Get(dataDir)
	if err != nil {
		slog.Warn("hwid.Get returned error; using fallback value", "err", err)
	}
	deviceInfo := hwid.Info()

	serversSvc := bindings.NewServersService(bindings.ServersDeps{
		ServerStore: serverStore,
		Hub:         h,
		// ActiveServer is nil — Phase 3.C.2 wires chainctl. With nil,
		// Remove cannot block deletion of the active server; the renderer
		// disconnects first via run.disconnect (not yet wired) before
		// removing, so this is safe for Phase 3.C.1.
		ActiveServer: nil,
	})
	subsSvc := bindings.NewSubsService(bindings.SubsDeps{
		SubStore:    subStore,
		ServerStore: serverStore,
		Hub:         h,
		SettingsView: func() hub.SettingsView {
			view, verr := configStore.View()
			if verr != nil {
				return hub.SettingsView{}
			}
			return view
		},
		HWID:       hwidValue,
		DeviceInfo: deviceInfo,
	})

	d := dispatcher.New()

	app := handlers.AppHandlers{Snap: appSvc}
	d.Register("app.ping", app.Ping)
	d.Register("app.getSnapshot", app.GetSnapshot)

	onboarding := handlers.OnboardingHandlers{Svc: onboardingSvc}
	d.Register("onboarding.getState", onboarding.GetState)
	d.Register("onboarding.complete", onboarding.Complete)
	d.Register("onboarding.skip", onboarding.Skip)

	settings := handlers.SettingsHandlers{Svc: settingsSvc}
	d.Register("settings.get", settings.Get)
	d.Register("settings.update", settings.Update)

	helper := handlers.HelperHandlers{Svc: helperSvc}
	d.Register("helper.status", helper.Status)
	d.Register("helper.install", helper.Install)
	d.Register("helper.start", helper.Start)
	d.Register("helper.stop", helper.Stop)
	d.Register("helper.restart", helper.Restart)
	d.Register("helper.reinstall", helper.Reinstall)

	servers := handlers.ServersHandlers{Svc: serversSvc}
	d.Register("servers.list", servers.List)
	d.Register("servers.add", servers.Add)
	d.Register("servers.edit", servers.Edit)
	d.Register("servers.remove", servers.Remove)
	d.Register("servers.toggleFavorite", servers.ToggleFavorite)
	d.Register("servers.testLatency", servers.TestLatency)

	subs := handlers.SubsHandlers{Svc: subsSvc}
	d.Register("subs.list", subs.List)
	d.Register("subs.add", subs.Add)
	d.Register("subs.edit", subs.Edit)
	d.Register("subs.remove", subs.Remove)
	d.Register("subs.syncOne", subs.SyncOne)
	d.Register("subs.syncAll", subs.SyncAll)

	// Bus + hub are held in scope so Phase 4 can attach a forwarder that
	// subscribes to h's events and emits JSON-RPC notifications via bus.
	_ = bus.New(out)
	_ = h

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
