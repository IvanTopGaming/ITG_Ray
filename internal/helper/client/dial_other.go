//go:build !windows

package client

import (
	"context"
	"errors"
)

// Dial returns an explicit unsupported error on non-Windows platforms.
func Dial(_ context.Context, _ string) (*Client, error) {
	return nil, errors.New("helper named pipes are only supported on Windows")
}
