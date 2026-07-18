//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/itg-team/itg-ray/internal/helper/wintun"
	"github.com/itg-team/itg-ray/internal/logging"
)

// TunCreateArgs is the JSON payload of OpTunCreate.
type TunCreateArgs struct {
	Name string `json:"name"`
}

// TunCreateResult is the JSON payload returned by OpTunCreate.
type TunCreateResult struct {
	LUID uint64 `json:"luid"`
}

// TunDestroyArgs is the JSON payload of OpTunDestroy.
type TunDestroyArgs struct {
	Name string `json:"name"`
}

var (
	tunMu       sync.Mutex
	tunAdapters = map[string]*wintun.Adapter{}
)

// NewTunCreateHandler creates a WinTUN adapter and stores it under its name.
func NewTunCreateHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a TunCreateArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if a.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		slog.Info("tun create", slog.String("scope", "helper"), slog.String("name", a.Name))
		tunMu.Lock()
		defer tunMu.Unlock()
		if _, exists := tunAdapters[a.Name]; exists {
			return nil, fmt.Errorf("adapter %q already exists", a.Name)
		}
		ad, err := wintun.Create(a.Name)
		if err != nil {
			slog.Error("tun create failed", slog.String("scope", "helper"),
				slog.String("name", a.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		tunAdapters[a.Name] = ad
		return json.Marshal(TunCreateResult{LUID: ad.LUID()})
	}
}

// NewTunDestroyHandler removes a previously-created adapter.
func NewTunDestroyHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a TunDestroyArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		slog.Info("tun destroy", slog.String("scope", "helper"), slog.String("name", a.Name))
		tunMu.Lock()
		defer tunMu.Unlock()
		ad, ok := tunAdapters[a.Name]
		if !ok {
			return nil, fmt.Errorf("adapter %q not found", a.Name)
		}
		if err := ad.Close(); err != nil {
			slog.Error("tun destroy failed", slog.String("scope", "helper"),
				slog.String("name", a.Name), slog.String("err", logging.RedactError(err)))
			return nil, err
		}
		delete(tunAdapters, a.Name)
		return json.RawMessage(`{}`), nil
	}
}
