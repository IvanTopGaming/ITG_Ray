//go:build linux

package client

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Dial connects to the helper's unix socket with a 5 s budget.
func Dial(_ context.Context, socketPath string) (*Client, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial unix %q: %w", socketPath, err)
	}
	return NewWithConn(conn), nil
}
