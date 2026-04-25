//go:build !windows

package main

import (
	"context"
	"errors"

	"github.com/itg-team/itg-ray/internal/server"
)

func startHelperSession(_ context.Context, _ *server.Server, _, _ string) (cleanup func(), luid uint64, err error) {
	return func() {}, 0, errors.New("--use-helper is Windows-only")
}
