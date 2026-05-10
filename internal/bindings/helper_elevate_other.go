//go:build !windows

package bindings

import "errors"

// elevateCLI is a non-Windows stub. The Helper service is Windows-only,
// so elevated CLI calls have no meaning on Linux/macOS dev hosts.
// Returning an error keeps tests honest if they accidentally exercise
// the elevation path on a non-Windows runner.
func elevateCLI(_ ...string) error {
	return errors.New("elevateCLI: helper management is Windows-only")
}
