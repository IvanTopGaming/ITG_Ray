//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/itg-team/itg-ray/internal/helper/wintun"
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
		tunMu.Lock()
		defer tunMu.Unlock()
		if _, exists := tunAdapters[a.Name]; exists {
			return nil, fmt.Errorf("adapter %q already exists", a.Name)
		}
		ad, err := wintun.Create(a.Name)
		if err != nil {
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
		tunMu.Lock()
		defer tunMu.Unlock()
		ad, ok := tunAdapters[a.Name]
		if !ok {
			return nil, fmt.Errorf("adapter %q not found", a.Name)
		}
		if err := ad.Close(); err != nil {
			return nil, err
		}
		delete(tunAdapters, a.Name)
		return json.RawMessage(`{}`), nil
	}
}
