//go:build !windows

// Package route configures the Windows IPv4 route table on behalf of the
// Helper. This file is a non-Windows stub so the package compiles on Linux/Mac
// build hosts; the real implementation lives in route_windows.go.
package route

import "errors"

// Entry mirrors a single route table row. The InterfaceLUID identifies which
// adapter owns the route — typically the WinTUN LUID returned from
// internal/helper/wintun.Adapter.LUID.
type Entry struct {
	DestCIDR      string `json:"dest_cidr"`
	NextHop       string `json:"next_hop"`
	InterfaceLUID uint64 `json:"interface_luid"`
	Metric        uint32 `json:"metric"`
}

var errPlatform = errors.New("route: Windows-only")

// Snapshot returns all current IPv4 routes. Stub on non-Windows.
func Snapshot() ([]Entry, error) { return nil, errPlatform }

// Add inserts a route. Stub on non-Windows.
func Add(_ Entry) error { return errPlatform }

// Remove deletes a route. Stub on non-Windows.
func Remove(_ Entry) error { return errPlatform }
