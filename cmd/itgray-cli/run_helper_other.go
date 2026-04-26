//go:build !windows

package main

import (
	"context"
	"errors"

	"github.com/itg-team/itg-ray/internal/server"
)

type helperSession struct {
	sessionID string
}

func (s *helperSession) cleanup(_ context.Context) {}

func startHelperSession(
	_ context.Context, _ *server.Server, _, _ []byte, _ string,
) (*helperSession, error) {
	return nil, errors.New("--use-helper is Windows-only")
}
