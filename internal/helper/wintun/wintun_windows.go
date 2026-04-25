//go:build windows

// Package wintun is a thin wrapper around the wintun.dll driver. The DLL is
// loaded from the same directory as the running executable; the build script
// (scripts/build-windows.sh) places third_party/wintun/wintun.dll there.
package wintun

import (
	"fmt"

	"golang.zx2c4.com/wintun"
)

// Adapter is one WinTUN adapter — created by Helper, used by sing-box via
// interface name attach.
type Adapter struct {
	a *wintun.Adapter
}

// Create allocates a new WinTUN adapter with the given user-visible name and
// the canonical "ITG Ray" tunnel type.
func Create(name string) (*Adapter, error) {
	a, err := wintun.CreateAdapter(name, "ITG Ray", nil)
	if err != nil {
		return nil, fmt.Errorf("wintun.CreateAdapter: %w", err)
	}
	return &Adapter{a: a}, nil
}

// Close removes the adapter from the system. Idempotent.
func (a *Adapter) Close() error {
	if a == nil || a.a == nil {
		return nil
	}
	err := a.a.Close()
	a.a = nil
	return err
}

// LUID returns the adapter's locally-unique identifier; route APIs consume
// this to scope additions to the right adapter.
func (a *Adapter) LUID() uint64 {
	if a == nil || a.a == nil {
		return 0
	}
	return a.a.LUID()
}
