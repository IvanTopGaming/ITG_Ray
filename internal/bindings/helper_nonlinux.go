//go:build !linux

package bindings

import (
	"errors"

	"github.com/itg-team/itg-ray/internal/helper/svcmgr"
)

// realHelperStatus on non-Linux delegates to svcmgr.Status (the Windows SCM
// query; a stub error elsewhere). Linux overrides this with a socket probe.
func realHelperStatus(n string) (svcmgr.State, error) { return svcmgr.Status(n) }

// InstallLinux / UninstallLinux are Linux-only (pkexec + systemd). On other
// platforms they return an error so an accidental cross-platform call is
// visible rather than silently succeeding.

func (h *HelperService) InstallLinux() error {
	return errors.New("InstallLinux: only supported on Linux")
}

func (h *HelperService) UninstallLinux() error {
	return errors.New("UninstallLinux: only supported on Linux")
}
