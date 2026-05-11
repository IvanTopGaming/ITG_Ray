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

// readChainCounters is the non-Windows stub. The helper is Windows-only,
// so non-Windows callers (tests on linux dev boxes) always see no chain.
func readChainCounters(_ context.Context) (uint64, uint64, bool) {
	return 0, 0, false
}

// IsChainActive is the non-Windows stub — returns false. The real
// implementation (chain_windows.go) reads activeSess under chainMu.
func IsChainActive() bool { return false }
