//go:build windows

package client

import (
	"context"
	"time"

	"github.com/Microsoft/go-winio"
)

// Dial connects to the helper's named pipe with a 5 s budget.
func Dial(_ context.Context, pipeName string) (*Client, error) {
	timeout := 5 * time.Second
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return nil, err
	}
	return NewWithConn(conn), nil
}
