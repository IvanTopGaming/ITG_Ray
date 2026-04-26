//go:build !windows

package server

import (
	"context"
	"encoding/json"
	"errors"
)

// NewTunCreateHandler is a stub on non-Windows.
func NewTunCreateHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("tun: Windows-only")
	}
}

// NewTunDestroyHandler is a stub on non-Windows.
func NewTunDestroyHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("tun: Windows-only")
	}
}
