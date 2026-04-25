//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "ITGRayHelper"

func runService() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("svc.IsWindowsService: %w", err)
	}
	if !isService {
		// Interactive run (developer mode) — print a notice and exit cleanly.
		fmt.Println("itgray-helper " + Version)
		fmt.Println("This binary is intended to be invoked by the Service Control Manager.")
		fmt.Println("Use `itgray-cli helper install` from the user-level CLI to register it.")
		return nil
	}
	// Real SCM-driven path is wired in Phase B4. For B0.2 the bare hook is enough.
	return errors.New("SCM run loop not yet implemented (Phase B4)")
}
