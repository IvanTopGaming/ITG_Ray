//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/logging"
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
		slog.Info("dns set", slog.String("scope", "helper"), slog.String("interface", s.InterfaceAlias))
		prior, err := dns.Snapshot(s.InterfaceAlias)
		if err != nil {
			slog.Error("dns set failed", slog.String("scope", "helper"),
				slog.String("stage", "snapshot"), slog.String("interface", s.InterfaceAlias),
				slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		if err := dns.Set(s); err != nil {
			slog.Error("dns set failed", slog.String("scope", "helper"),
				slog.String("stage", "set"), slog.String("interface", s.InterfaceAlias),
				slog.String("err", logging.RedactError(err)))
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
		slog.Info("dns restore", slog.String("scope", "helper"), slog.String("interface", s.InterfaceAlias))
		if err := dns.Restore(s); err != nil {
			slog.Error("dns restore failed", slog.String("scope", "helper"),
				slog.String("interface", s.InterfaceAlias), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	}
}
