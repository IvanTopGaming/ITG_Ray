// Package xrayapi is a thin client over xray-core's StatsService gRPC API.
// The helper uses it on each OpServiceStatus poll to populate UpBytes /
// DownBytes for the GUI's speed card. Connection is lazy; a failed dial or
// transient RPC error is reported as (0, 0, error) and the helper falls
// back to last-cached values — never failing the status response.
package xrayapi

import (
	"context"
	"fmt"
	"sync"

	statsservice "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a reusable handle to xray-core's StatsService.
type Client struct {
	addr string

	mu     sync.Mutex
	conn   *grpc.ClientConn
	client statsservice.StatsServiceClient
}

// New returns a Client for the given gRPC endpoint (e.g. "127.0.0.1:13355").
// No dial happens until the first Counters call.
func New(addr string) *Client { return &Client{addr: addr} }

// Counters returns the cumulative outbound proxy uplink/downlink bytes.
// Lazy-dials on first call; on any error returns (0, 0, err) and clears
// the cached connection so the next call retries.
func (c *Client) Counters(ctx context.Context) (up, down uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		conn, err := grpc.NewClient(c.addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return 0, 0, fmt.Errorf("create xray api client: %w", err)
		}
		c.conn = conn
		c.client = statsservice.NewStatsServiceClient(conn)
	}

	up, errU := c.queryCounter(ctx, "outbound>>>proxy>>>traffic>>>uplink")
	down, errD := c.queryCounter(ctx, "outbound>>>proxy>>>traffic>>>downlink")
	if errU != nil || errD != nil {
		_ = c.resetLocked()
		if errU != nil {
			return 0, 0, errU
		}
		return 0, 0, errD
	}
	return up, down, nil
}

func (c *Client) queryCounter(ctx context.Context, name string) (uint64, error) {
	resp, err := c.client.GetStats(ctx, &statsservice.GetStatsRequest{
		Name:   name,
		Reset_: false,
	})
	if err != nil {
		return 0, fmt.Errorf("GetStats(%s): %w", name, err)
	}
	if resp == nil || resp.Stat == nil {
		return 0, nil
	}
	v := resp.Stat.Value
	if v < 0 {
		return 0, nil
	}
	return uint64(v), nil
}

// Close releases the gRPC connection. Idempotent.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resetLocked()
}

// resetLocked closes and clears the conn. Caller MUST hold c.mu.
func (c *Client) resetLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.client = nil
	return err
}
