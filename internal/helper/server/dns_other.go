//go:build !windows

package server

import (
	"context"
	"encoding/json"
	"errors"
)

// NewDnsSetHandler is a stub on non-Windows.
func NewDnsSetHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("dns: Windows-only")
	}
}

// NewDnsRestoreHandler is a stub on non-Windows.
func NewDnsRestoreHandler() Handler {
	return func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("dns: Windows-only")
	}
}
