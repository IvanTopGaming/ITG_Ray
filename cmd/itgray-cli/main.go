// Command itgray-cli is the headless test harness for the ITG Ray core.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/spf13/cobra"
)

var (
	dataDir string
	verbose bool
)

func main() {
	root := &cobra.Command{
		Use:   "itgray-cli",
		Short: "ITG Ray headless VPN core — CLI test harness",
	}
	root.PersistentFlags().StringVar(&dataDir, "data-dir", defaultDataDir(), "location of servers.json / rules.json / config.json / subscriptions.json / stats.db")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable DEBUG logs")

	cobra.OnInitialize(func() {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(logging.NewHandler(os.Stderr, level)))
	})

	root.AddCommand(newSubCmd())
	root.AddCommand(newServerCmd())
	root.AddCommand(newRuleCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newHelperCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultDataDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(d, "ITG Ray")
}
