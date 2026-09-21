//go:build windows

package client

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func Dial(ctx context.Context, pipeName string) (*Client, error) {
	dial := func(ctx context.Context) (net.Conn, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return winio.DialPipeContext(ctx, pipeName)
	}
	conn, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	c := NewWithConn(conn)
	c.dial = dial
	return c, nil
}
