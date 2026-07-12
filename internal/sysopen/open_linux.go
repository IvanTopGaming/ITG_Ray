//go:build linux

package sysopen

import "os/exec"

// Dir opens path in the user's file manager via xdg-open.
func Dir(path string) error {
	return exec.Command("xdg-open", path).Start()
}
