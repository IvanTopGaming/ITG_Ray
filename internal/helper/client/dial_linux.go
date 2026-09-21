//go:build linux

package client

import (
	"context"
	"fmt"
	"net"
	"time"
)

func Dial(ctx context.Context, socketPath string) (*Client, error) {
	dial := func(ctx context.Context) (net.Conn, error) {
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(ctx, "unix", socketPath)
		if err != nil {
			return nil, fmt.Errorf("dial unix %q: %w", socketPath, err)
		}
		return conn, nil
	}
	conn, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	c := NewWithConn(conn)
	c.dial = dial
	return c, nil
}
