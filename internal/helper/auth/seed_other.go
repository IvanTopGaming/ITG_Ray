//go:build !windows

package auth

import "errors"

// Seed is a stub on non-Windows.
func Seed(_ string) error { return errors.New("auth: Windows-only") }

// Load is a stub on non-Windows.
func Load() ([]string, error) { return nil, errors.New("auth: Windows-only") }
