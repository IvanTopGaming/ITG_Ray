//go:build !windows

package server

import (
	"context"
	"encoding/json"
	"errors"
)

// NewStartChainHandler is a stub on non-Windows.
func NewStartChainHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("StartChain: Windows-only")
	}
}

// NewStopChainHandler is a stub on non-Windows.
func NewStopChainHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("StopChain: Windows-only")
	}
}
