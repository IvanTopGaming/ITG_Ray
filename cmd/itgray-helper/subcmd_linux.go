//go:build linux

package main

import (
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the systemd helper (run as root via pkexec)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uidStr, _ := cmd.Flags().GetString("uid")
			uid, err := strconv.Atoi(uidStr)
			if err != nil {
				return err
			}
			return install(uid)
		},
	}
	installCmd.Flags().String("uid", "", "owning uid")

	extraCommands = append(extraCommands,
		installCmd,
		&cobra.Command{
			Use:   "uninstall",
			Short: "Remove the systemd helper (run as root via pkexec)",
			RunE:  func(*cobra.Command, []string) error { return uninstall() },
		},
	)
}
