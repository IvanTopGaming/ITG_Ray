//go:build !windows

// Package adapter enumerates Windows network adapters by LUID. Stub on
// non-Windows.
package adapter

import "errors"

var errPlatform = errors.New("adapter: Windows-only")

// Snapshot returns all current adapters. Stub on non-Windows.
func Snapshot() ([]Adapter, error) { return nil, errPlatform }
