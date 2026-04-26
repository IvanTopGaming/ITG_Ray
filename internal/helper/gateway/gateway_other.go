//go:build !windows

// Package gateway resolves the host's current default gateway. Used by
// OpStartChain to compute the peer-route. Stub on non-Windows.
package gateway

import "errors"

// Entry describes one default-gateway candidate.
type Entry struct {
	NextHop       string `json:"next_hop"`
	InterfaceLUID uint64 `json:"interface_luid"`
	Metric        uint32 `json:"metric"`
}

// Default returns the lowest-metric IPv4 default-gateway entry.
// Stub on non-Windows.
func Default() (Entry, error) { return Entry{}, errors.New("gateway: Windows-only") }
