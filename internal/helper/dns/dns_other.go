//go:build !windows

// Package dns configures Windows DNS settings on host adapters via netsh.
// This file is a non-Windows stub.
package dns

import "errors"

// Settings is the per-interface DNS state.
type Settings struct {
	InterfaceAlias string
	Addresses      []string
}

var errPlatform = errors.New("dns: Windows-only")

// Snapshot is a stub on non-Windows.
func Snapshot(_ string) (Settings, error) { return Settings{}, errPlatform }

// Set is a stub on non-Windows.
func Set(_ Settings) error { return errPlatform }

// Restore is a stub on non-Windows.
func Restore(_ Settings) error { return errPlatform }
