// Package client is the user-level wrapper around the Helper named pipe.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

type Client struct {
	mu     sync.Mutex
	gate   chan struct{}
	conn   net.Conn
	dial   func(context.Context) (net.Conn, error)
	closed bool
	nextID atomic.Uint64
}

// NewWithConn wraps an already-dialed connection. Used by tests with net.Pipe
// and by the Windows-side helper to wrap a winio pipe.
func NewWithConn(conn net.Conn) *Client {
	return &Client{conn: conn, gate: make(chan struct{}, 1)}
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (c *Client) connection(ctx context.Context) (net.Conn, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, net.ErrClosed
	}
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		return conn, nil
	}
	if c.dial == nil {
		return nil, net.ErrClosed
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close()
		return nil, net.ErrClosed
	}
	c.conn = conn
	c.mu.Unlock()
	return conn, nil
}

func (c *Client) discard(conn net.Conn) {
	c.mu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.mu.Unlock()
	_ = conn.Close()
}

func (c *Client) Call(ctx context.Context, op protocol.Op, args json.RawMessage) (result json.RawMessage, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	canceled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = conn.Close()
		close(canceled)
	})
	defer func() {
		if !stop() {
			<-canceled
			c.discard(conn)
			result = nil
			err = ctx.Err()
		}
	}()

	id := c.nextID.Add(1)
	req := protocol.Request{ID: id, Op: op, Args: args}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := protocol.WriteFrame(conn, body); err != nil {
		c.discard(conn)
		return nil, fmt.Errorf("write frame: %w", err)
	}
	respBody, err := protocol.ReadFrame(conn, protocol.MaxFrame)
	if err != nil {
		c.discard(conn)
		return nil, fmt.Errorf("read frame: %w", err)
	}
	var resp protocol.Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		c.discard(conn)
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.ID != id {
		c.discard(conn)
		return nil, fmt.Errorf("response id mismatch: got %d want %d", resp.ID, id)
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	return resp.Result, nil
}
