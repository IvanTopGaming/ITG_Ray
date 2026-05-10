// Command itgray-bridge is the JSON-RPC backend for the Electron-based
// ITG Ray GUI. It reads requests from stdin, writes responses to stdout.
// Phase 0: app.ping. Phase 3.A: app.getSnapshot + onboarding.*.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/itg-team/itg-ray/cmd/itgray-bridge/bus"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/dispatcher"
	"github.com/itg-team/itg-ray/cmd/itgray-bridge/handlers"
	"github.com/itg-team/itg-ray/cmd/itgray-gui/bindings"
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

	d := dispatcher.New()

	app := handlers.AppHandlers{Snap: appSvc}
	d.Register("app.ping", app.Ping)
	d.Register("app.getSnapshot", app.GetSnapshot)

	onboarding := handlers.OnboardingHandlers{Svc: onboardingSvc}
	d.Register("onboarding.getState", onboarding.GetState)
	d.Register("onboarding.complete", onboarding.Complete)
	d.Register("onboarding.skip", onboarding.Skip)

	// Bus is held in scope so later phases (4) can attach it to chainctl,
	// hub, etc., for outbound notifications.
	_ = bus.New(out)

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
