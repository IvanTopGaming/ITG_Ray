//go:build linux

package handlers

import "os"

// packagedUnitPath is where a distribution package installs the helper unit.
// The in-app installer writes to /etc/systemd/system instead, so the presence
// of this file is the signal that pacman (or another package manager) owns the
// helper and the GUI's install/uninstall actions must stay out of the way.
const packagedUnitPath = "/usr/lib/systemd/system/itgray-helper.service"

func detectPackageManagedHelper() bool {
	_, err := os.Stat(packagedUnitPath)
	return err == nil
}
