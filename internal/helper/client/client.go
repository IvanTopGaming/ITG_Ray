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

// Client is a one-connection-at-a-time client. Concurrent callers serialize
// on a mutex so request/response IDs don't tangle on the wire.
type Client struct {
	mu     sync.Mutex
	conn   net.Conn
	nextID atomic.Uint64
}

// NewWithConn wraps an already-dialed connection. Used by tests with net.Pipe
// and by the Windows-side helper to wrap a winio pipe.
func NewWithConn(conn net.Conn) *Client {
	return &Client{conn: conn}
}

// Close releases the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

// Call sends one request and waits for the matching response. Pipe traffic is
// strictly serial per connection — no need for a response demuxer.
func (c *Client) Call(_ context.Context, op protocol.Op, args json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	req := protocol.Request{ID: id, Op: op, Args: args}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := protocol.WriteFrame(c.conn, body); err != nil {
		return nil, fmt.Errorf("write frame: %w", err)
	}

	respBody, err := protocol.ReadFrame(c.conn, protocol.MaxFrame)
	if err != nil {
		return nil, fmt.Errorf("read frame: %w", err)
	}
	var resp protocol.Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.ID != id {
		return nil, fmt.Errorf("response id mismatch: got %d want %d", resp.ID, id)
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	return resp.Result, nil
}
