package main

import (
	"context"
	"embed"
	"log/slog"
	"os"

	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Version is injected at build time via -ldflags -X main.Version=<git-rev>.
var Version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	slog.SetDefault(slog.New(logging.NewHandler(os.Stderr, slog.LevelInfo)))

	app := NewApp(Version)

	err := wails.Run(&options.App{
		Title:            "ITG Ray",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true, // drag region + window controls land in C.T4
		BackgroundColour: &options.RGBA{R: 10, G: 13, B: 23, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        func(ctx context.Context) { app.Startup(ctx) },
		OnShutdown:       func(ctx context.Context) { app.Shutdown(ctx) },
		Bind:             []any{app},
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
