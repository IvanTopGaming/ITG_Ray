package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/bindings"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
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

	appSvc := bindings.NewAppService(&bindings.AppDeps{
		DataDir:      dataDir,
		Hub:          app.Hub(),
		Version:      Version,
		BuildDate:    BuildDate,
		ServerStore:  serverStore,
		SubStore:     subStore,
		HelperProber: helperProber,
		// AppCtx returns the Wails app context as set by App.Startup. The
		// closure indirects so AppService captures the *future* ctx (still
		// nil at this point) — Quit dereferences only when invoked, by
		// which time Startup has populated app.ctx.
		AppCtx: func() context.Context { return app.ctx },
	})
	serversSvc := bindings.NewServersService(bindings.ServersDeps{
		ServerStore: serverStore,
		Hub:         app.Hub(),
	})
	subsSvc := bindings.NewSubsService(bindings.SubsDeps{
		SubStore:    subStore,
		ServerStore: serverStore,
		Hub:         app.Hub(),
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
	chainCtrl := chainctl.New(&chainctl.Deps{
		DataDir:      dataDir,
		ServerStore:  serverStoreGetter{serverStore},
		Helper:       helperClient,
		Sysproxy:     sysproxy.New(),
		Hub:          app.Hub(),
		BuildConfigs: buildConfigs(dataDir),
		// Task 6 of Tier 2b will replace this with a real
		// config.Load(filepath.Join(dataDir, "config.json"))-backed
		// closure so user edits to Network land on the next Connect
		// without a process restart. DefaultNetworkLoader keeps the
		// existing stock-config behaviour until then.
		Network: chainctl.DefaultNetworkLoader(),
	})
	runSvc := bindings.NewRunService(bindings.RunDeps{
		Chain: chainCtrl,
		Hub:   app.Hub(),
	})
	settingsStore := bindings.NewConfigStore(filepath.Join(dataDir, "config.json"), Version, BuildDate)
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

	err := wails.Run(&options.App{
		Title:            "ITG Ray",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true, // drag region + window controls land in C.T4
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
