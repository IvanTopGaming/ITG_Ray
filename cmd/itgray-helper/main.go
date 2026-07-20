// Command itgray-helper is the privileged Windows Service that exposes
// TUN/route/DNS operations to the user-level itgray-cli over a SID-gated
// named pipe.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/helper/server"
	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/spf13/cobra"
)

const (
	// Version is overridden at build time via -ldflags "-X main.Version=...".
	Version = "0.0.0-dev"
	// PipeName is the canonical pipe path the user-side client connects to.
	PipeName = `\\.\pipe\ITGRay.Helper.v1`
)

// extraCommands holds platform-specific subcommands registered via init() in
// build-tagged files (e.g. install/uninstall on Linux). Empty on Windows.
var extraCommands []*cobra.Command

func main() {
	root := &cobra.Command{
		Use:     "itgray-helper",
		Short:   "ITG Ray helper service (privileged TUN/route/DNS operations)",
		Version: Version,
		RunE: func(*cobra.Command, []string) error {
			var w io.Writer = os.Stderr
			logPath := filepath.Join(server.RuntimeDir(), "helper.log")
			if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err == nil {
				if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640); err == nil { //nolint:gosec // path is app-controlled, not attacker-supplied
					_ = f.Close()
					w = io.MultiWriter(os.Stderr, logging.NewRotatingWriter(logPath, 5*1024*1024, 3))
				}
			}
			slog.SetDefault(slog.New(logging.NewHandler(w, slog.LevelInfo)))
			return runService()
		},
	}
	root.AddCommand(extraCommands...)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
