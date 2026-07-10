// Command itgray-helper is the privileged Windows Service that exposes
// TUN/route/DNS operations to the user-level itgray-cli over a SID-gated
// named pipe.
package main

import (
	"fmt"
	"log/slog"
	"os"

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
			slog.SetDefault(slog.New(logging.NewHandler(os.Stderr, slog.LevelInfo)))
			return runService()
		},
	}
	root.AddCommand(extraCommands...)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
