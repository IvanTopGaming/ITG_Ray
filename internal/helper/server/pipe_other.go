//go:build !windows

package server

import (
	"context"
	"errors"
)

// Listen is a stub on non-Windows so callers compile.
func Listen(_ context.Context, _ string, _ *Dispatcher) error {
	return errors.New("server.Listen is Windows-only")
}
