//go:build !windows

// Package wintun is a Windows-only thin wrapper around the WinTUN driver.
package wintun

import "errors"

// Adapter is a stub on non-Windows.
type Adapter struct{}

// Close is a no-op on non-Windows.
func (*Adapter) Close() error { return nil }

// LUID returns 0 on non-Windows.
func (*Adapter) LUID() uint64 { return 0 }

// Create returns an unsupported error on non-Windows.
func Create(_ string) (*Adapter, error) {
	return nil, errors.New("wintun: Windows-only")
}
