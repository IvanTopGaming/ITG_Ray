//go:build darwin

package sysopen

import "os/exec"

// Dir opens path in the macOS Finder.
func Dir(path string) error {
	return exec.Command("open", path).Start()
}
