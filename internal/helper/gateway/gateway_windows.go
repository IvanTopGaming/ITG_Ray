//go:build windows

// Package gateway resolves the host's current default gateway by
// scanning the IPv4 route table for 0.0.0.0/0 entries and selecting
// the one with the lowest metric.
package gateway

import (
	"errors"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/route"
)

// Entry describes one default-gateway candidate.
type Entry struct {
	NextHop       string `json:"next_hop"`
	InterfaceLUID uint64 `json:"interface_luid"`
	Metric        uint32 `json:"metric"`
}

// Default returns the lowest-metric IPv4 default-gateway entry.
func Default() (Entry, error) {
	rows, err := route.Snapshot()
	if err != nil {
		return Entry{}, fmt.Errorf("route.Snapshot: %w", err)
	}
	var best *Entry
	for i := range rows {
		r := &rows[i]
		if r.DestCIDR != "0.0.0.0/0" {
			continue
		}
		cand := Entry{
			NextHop:       r.NextHop,
			InterfaceLUID: r.InterfaceLUID,
			Metric:        r.Metric,
		}
		if best == nil || cand.Metric < best.Metric {
			best = &cand
		}
	}
	if best == nil {
		return Entry{}, errors.New("no IPv4 default route present")
	}
	return *best, nil
}
