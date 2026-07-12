//go:build windows

package sysopen

import "os/exec"

// Dir opens path in Windows Explorer.
func Dir(path string) error {
	return exec.Command("explorer", path).Start()
}
