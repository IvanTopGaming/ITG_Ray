package main

import (
	"context"
	"log/slog"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// App is the singleton bound to the Wails frontend at startup. Subsequent
// tasks attach more bindings (subs, servers, run, ...) by adding fields and
// methods.
type App struct {
	ctx     context.Context
	version string
	hub     *hub.Hub
}

// NewApp constructs a fresh App. Bindings are wired via Bind in main.go.
func NewApp(version string) *App {
	return &App{version: version, hub: hub.New()}
}

// Startup is invoked by Wails after the frontend has loaded.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	hub.StartWailsEmitter(ctx, a.hub)
	slog.Info("itgray-gui startup", "version", a.version)
}

// Shutdown is invoked by Wails before the window closes.
func (a *App) Shutdown(_ context.Context) {
	a.hub.Close()
	slog.Info("itgray-gui shutdown")
}

// GetVersion returns the build-time version string. This is the ONLY binding
// in C.T1 — it gives the frontend a smoke-test surface.
func (a *App) GetVersion() string { return a.version }

// Hub exposes the hub to other binding packages registered later.
func (a *App) Hub() *hub.Hub { return a.hub }
