//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/dns"
)

// NewDnsSetHandler replaces the DNS server list on the named interface,
// returning a snapshot of the prior settings so the user-level CLI can
// stash it for restore.
func NewDnsSetHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var s dns.Settings
		if err := json.Unmarshal(args, &s); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		prior, err := dns.Snapshot(s.InterfaceAlias)
		if err != nil {
			return nil, err
		}
		if err := dns.Set(s); err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Prior dns.Settings `json:"prior"`
		}{Prior: prior})
	}
}

// NewDnsRestoreHandler replays a previously-captured Settings snapshot.
func NewDnsRestoreHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var s dns.Settings
		if err := json.Unmarshal(args, &s); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if err := dns.Restore(s); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	}
}
