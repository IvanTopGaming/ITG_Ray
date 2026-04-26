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
)

// Version is injected at build time via -ldflags -X main.Version=<git-rev>.
var Version = "dev"

// BuildDate is injected at build time via -ldflags -X main.BuildDate=<iso-8601>.
var BuildDate = ""

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	slog.SetDefault(slog.New(logging.NewHandler(os.Stderr, slog.LevelInfo)))

	app := NewApp(Version)

	dataDir := defaultDataDir()
	serverStore := serversFileStore{path: filepath.Join(dataDir, "servers.json")}
	subStore := subscription.FileStore{Path: filepath.Join(dataDir, "subscriptions.json")}

	appSvc := bindings.NewAppService(&bindings.AppDeps{
		DataDir:      dataDir,
		Hub:          app.Hub(),
		Version:      Version,
		BuildDate:    BuildDate,
		ServerStore:  serverStore,
		SubStore:     subStore,
		HelperProber: probeHelperState,
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
		SocksProxy:   "127.0.0.1:1080",
		TunName:      defaultTunName,
		TunCIDR:      defaultTunCIDR,
		DNSServers:   []string{"1.1.1.1", "8.8.8.8"},
	})
	runSvc := bindings.NewRunService(bindings.RunDeps{
		Chain: chainCtrl,
		Hub:   app.Hub(),
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
			// Reconcile after the frontend has subscribed so the
			// "still-running" status event is delivered to the live UI
			// rather than dropped into a closed hub.
			chainCtrl.Reconcile(ctx)
		},
		OnShutdown: func(ctx context.Context) { app.Shutdown(ctx) },
		Bind:       []any{app, appSvc, serversSvc, subsSvc, runSvc},
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

// probeHelperState is a placeholder until C.T14 wires real helper IPC. The
// frontend treats "missing" as "onboarding required", which is the safe
// default for the Wails build before the helper is bundled.
func probeHelperState() string { return "missing" }

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
