package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/bindings"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/hwid"
	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/sysproxy"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is injected at build time via -ldflags -X main.Version=<git-rev>.
var Version = "dev"

// BuildDate is injected at build time via -ldflags -X main.BuildDate=<iso-8601>.
var BuildDate = ""

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	slog.SetDefault(slog.New(logging.NewHandler(os.Stderr, slog.LevelInfo)))

	// Optional Chromium remote-debugging port for the embedded WebView2.
	// Activated when ITGRAY_GUI_DEBUG_PORT is set in the environment, so
	// release builds default to no exposed debug surface. Used by the
	// dev-loop CDP driver in scripts/cdp-driver.js.
	if port := os.Getenv("ITGRAY_GUI_DEBUG_PORT"); port != "" {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
			"--remote-debugging-port="+port+" --remote-allow-origins=*")
	}

	app := NewApp(Version)

	dataDir := defaultDataDir()
	serverStore := serversFileStore{path: filepath.Join(dataDir, "servers.json")}
	subStore := subscription.FileStore{Path: filepath.Join(dataDir, "subscriptions.json")}

	helperSvc := bindings.NewHelperService()
	onboardingSvc := bindings.NewOnboardingService(bindings.OnboardingDeps{DataDir: dataDir})

	// HelperProber wraps HelperService.Status: GetSnapshot needs a
	// best-effort string ("running"/"stopped"/"missing"). Any svcmgr
	// error collapses to "missing" so the wizard fires on a fresh box
	// even when the SCM rejects us — the user's path forward is the
	// same: install the helper.
	helperProber := func() string {
		state, err := helperSvc.Status()
		if err != nil {
			return "missing"
		}
		return state
	}

	serversSvc := bindings.NewServersService(bindings.ServersDeps{
		ServerStore: serverStore,
		Hub:         app.Hub(),
	})

	// settingsStore is constructed early so SubsService can read live
	// SubscriptionSettings (HWID toggles, default UA) on each SyncOne
	// without restart — the closure resolves at call time, not init time.
	configPath := filepath.Join(dataDir, "config.json")
	settingsStore := bindings.NewConfigStore(configPath, Version, BuildDate)

	// hwid.Get caches a stable per-machine identifier in dataDir; on
	// machineid unavailability or cache-write failure it still returns a
	// usable value (random fallback) alongside the error. Log and keep
	// going — the Sync path tolerates an empty HWID by skipping the
	// header.
	hwidValue, err := hwid.Get(dataDir)
	if err != nil {
		slog.Warn("hwid.Get returned error; using fallback value", "err", err)
	}
	deviceInfo := hwid.Info()

	subsSvc := bindings.NewSubsService(bindings.SubsDeps{
		SubStore:    subStore,
		ServerStore: serverStore,
		Hub:         app.Hub(),
		SettingsView: func() hub.SettingsView {
			view, verr := settingsStore.View()
			if verr != nil {
				// Fall back to zero value: SyncOne will see HWIDEnabled=false
				// and emit no identity headers, which is the safe default.
				return hub.SettingsView{}
			}
			return view
		},
		HWID:       hwidValue,
		DeviceInfo: deviceInfo,
	})

	// chainctl needs a Get-by-id surface; wrap the existing Load/Save
	// shim. helperBootCtx is a short-lived context used only to dial the
	// helper pipe — the helper client itself does not capture it, so we
	// cancel inline rather than via defer (gocritic exitAfterDefer would
	// flag a deferred cancel paired with the wails.Run os.Exit branch
	// below; the dial completes synchronously inside newHelperClient).
	helperBootCtx, cancelHelperBoot := context.WithCancel(context.Background())
	helperClient := newHelperClient(helperBootCtx)
	cancelHelperBoot()
	// networkLoader reads the user's persisted config.json on every
	// Connect cycle. The path mirrors bindings.NewConfigStore (declared
	// above for SubsService) so chainctl reads exactly what
	// SettingsService.Update writes — no process restart required for
	// Network edits to land on the next connection. config.Load overlays
	// defaults onto missing/partial fields, so a fresh box (no
	// config.json yet) yields the same Network values
	// DefaultNetworkLoader used to.
	networkLoader := func() (config.Network, error) {
		c, err := config.Load(configPath)
		if err != nil {
			return config.Network{}, err
		}
		return c.Network, nil
	}
	chainCtrl := chainctl.New(&chainctl.Deps{
		DataDir:      dataDir,
		ServerStore:  serverStoreGetter{serverStore},
		Helper:       helperClient,
		Sysproxy:     sysproxy.New(),
		Hub:          app.Hub(),
		BuildConfigs: buildConfigs(dataDir),
		Network:      networkLoader,
	})
	appSvc := bindings.NewAppService(&bindings.AppDeps{
		DataDir:      dataDir,
		Hub:          app.Hub(),
		Version:      Version,
		ServerStore:  serverStore,
		SubStore:     subStore,
		HelperProber: helperProber,
		// AppCtx returns the Wails app context as set by App.Startup. The
		// closure indirects so AppService captures the *future* ctx (still
		// nil at this point) — Quit dereferences only when invoked, by
		// which time Startup has populated app.ctx.
		AppCtx:        func() context.Context { return app.ctx },
		Chain:         chainCtrl,
		ConfigViewer:  settingsStore,
		NetworkLoader: networkLoader,
	})
	runSvc := bindings.NewRunService(bindings.RunDeps{
		Chain: chainCtrl,
		Hub:   app.Hub(),
	})
	settingsSvc := bindings.NewSettingsService(bindings.SettingsDeps{
		Store: settingsStore,
		Hub:   app.Hub(),
	})

	// TrayService records icon + menu state on every vpn:status event.
	// IconSetter / MenuSetter are nil today: Wails v2.11 has no
	// first-party SystemTray runtime API, and adding fyne.io/systray
	// pulls native build deps we are not ready to commit to. The
	// architectural plumbing (event-driven menu rebuild) lands here so
	// a v0.2 task can wire a concrete adapter without touching the
	// event flow. See bindings/tray.go for the full rationale.
	traySvc := bindings.NewTrayService(bindings.TrayDeps{
		Hub:    app.Hub(),
		Chain:  chainCtrl,
		OnShow: func() { wailsruntime.WindowShow(app.ctx) },
		OnQuit: func() { wailsruntime.Quit(app.ctx) },
	})

	err = wails.Run(&options.App{
		Title:            "ITG Ray",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 10, G: 13, B: 23, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			traySvc.Init(ctx)
			// Reconcile after the frontend has subscribed so the
			// "still-running" status event is delivered to the live UI
			// rather than dropped into a closed hub.
			chainCtrl.Reconcile(ctx)
		},
		// OnBeforeClose intentionally omitted: a system-tray adapter
		// is a v0.2 task (see TrayService comment above) and Wails
		// v2.11 has no first-party tray UI to drive a hide-to-tray
		// flow. Until that ships, close = quit so users have a
		// reachable exit.
		OnShutdown: func(ctx context.Context) {
			traySvc.Shutdown()
			app.Shutdown(ctx)
		},
		Bind: []any{app, appSvc, serversSvc, subsSvc, runSvc, settingsSvc, helperSvc, onboardingSvc},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			DisableWindowIcon:    false,
		},
	})
	if err != nil {
		slog.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}

// defaultDataDir mirrors cmd/itgray-cli/main.go: prefer os.UserConfigDir/ITG Ray,
// fall back to ./data when the user-config path is unavailable.
func defaultDataDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(d, "ITG Ray")
}

// serversFileStore adapts the package-level server.Load / server.Save
// free functions to the bindings.ServerStore interface. internal/server
// does not (yet) export a FileStore type — the binding layer owns this
// trivial shim.
type serversFileStore struct{ path string }

// Load reads the configured servers.json path. Missing file → empty slice.
func (s serversFileStore) Load() ([]server.Server, error) { return server.Load(s.path) }

// Save writes the full server list back to the configured path atomically
// via tmp + rename (see internal/server.Save).
func (s serversFileStore) Save(list []server.Server) error { return server.Save(s.path, list) }

// serverStoreGetter adapts the bindings.ServerStore (Load/Save) shape to
// the chainctl.ServerStore (Get-by-id) surface. We re-Load on every Get;
// the file is small and Connect happens at most once per user click, so
// caching adds no measurable benefit and would complicate cache
// invalidation when servers.go mutators rewrite the file.
type serverStoreGetter struct{ inner serversFileStore }

// Get returns the server matching id, or (nil, nil) if not found. Errors
// come from the underlying Load (file unreadable / corrupt JSON).
func (g serverStoreGetter) Get(id string) (*server.Server, error) {
	all, err := g.inner.Load()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, nil
}
