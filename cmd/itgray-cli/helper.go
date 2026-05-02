package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/helper/auth"
	"github.com/itg-team/itg-ray/internal/helper/svcmgr"
	"github.com/spf13/cobra"
)

// helperServiceName matches the constant used by itgray-helper itself.
const helperServiceName = "ITGRayHelper"

func newHelperCmd() *cobra.Command {
	h := &cobra.Command{Use: "helper", Short: "manage the ITGRayHelper Windows service"}

	install := &cobra.Command{
		Use:   "install [path-to-helper.exe]",
		Short: "register the helper service in SCM",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			binPath := defaultHelperPath()
			if len(args) == 1 {
				binPath = args[0]
			}
			abs, err := filepath.Abs(binPath)
			if err != nil {
				return err
			}
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("helper binary not found at %s: %w", abs, err)
			}
			sid, err := currentUserSID()
			if err != nil {
				return fmt.Errorf("get current user sid: %w", err)
			}
			if err := auth.Seed(sid); err != nil {
				return fmt.Errorf("seed allow-list: %w", err)
			}
			if err := svcmgr.Install(helperServiceName, abs, "ITG Ray helper service"); err != nil {
				return err
			}
			fmt.Println("installed:", helperServiceName, "->", abs, "sid:", sid)
			return nil
		},
	}
	h.AddCommand(install)

	h.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "remove the helper service from SCM",
		RunE: func(*cobra.Command, []string) error {
			if err := svcmgr.Uninstall(helperServiceName); err != nil {
				return err
			}
			fmt.Println("uninstalled:", helperServiceName)
			return nil
		},
	})

	h.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "start the helper service",
		RunE: func(*cobra.Command, []string) error {
			if err := svcmgr.Start(helperServiceName); err != nil {
				return err
			}
			fmt.Println("started:", helperServiceName)
			return nil
		},
	})

	h.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "stop the helper service",
		RunE: func(*cobra.Command, []string) error {
			if err := svcmgr.Stop(helperServiceName); err != nil {
				return err
			}
			fmt.Println("stopped:", helperServiceName)
			return nil
		},
	})

	h.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "stop and start the helper service in one elevated invocation",
		RunE: func(*cobra.Command, []string) error {
			if err := svcmgr.Stop(helperServiceName); err != nil && !svcmgr.IsNotRunning(err) {
				return fmt.Errorf("stop: %w", err)
			}
			if err := svcmgr.Start(helperServiceName); err != nil {
				return fmt.Errorf("start: %w", err)
			}
			fmt.Println("restarted:", helperServiceName)
			return nil
		},
	})

	h.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "report current SCM state",
		RunE: func(*cobra.Command, []string) error {
			st, err := svcmgr.Status(helperServiceName)
			if err != nil {
				return err
			}
			fmt.Println(st)
			return nil
		},
	})
	return h
}

// defaultHelperPath returns the typical install location alongside itgray-cli.
func defaultHelperPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "itgray-helper.exe"
	}
	return filepath.Join(filepath.Dir(exe), "itgray-helper.exe")
}
