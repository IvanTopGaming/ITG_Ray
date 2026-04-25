//go:build !windows

package server

import (
	"context"
	"encoding/json"
	"errors"
)

// NewRouteSnapshotHandler is a stub on non-Windows.
func NewRouteSnapshotHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("route: Windows-only")
	}
}

// NewRouteAddHandler is a stub on non-Windows.
func NewRouteAddHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("route: Windows-only")
	}
}

// NewRouteRemoveHandler is a stub on non-Windows.
func NewRouteRemoveHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("route: Windows-only")
	}
}

// NewRouteRestoreHandler is a stub on non-Windows.
func NewRouteRestoreHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("route: Windows-only")
	}
}
