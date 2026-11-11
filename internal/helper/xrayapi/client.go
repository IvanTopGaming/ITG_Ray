// Package xrayapi is a thin client over xray-core's StatsService gRPC API.
// The helper uses it on each OpServiceStatus poll to populate UpBytes /
// DownBytes for the GUI's speed card. Connection is lazy; a failed dial or
// transient RPC error is reported as (0, 0, error) and the helper falls
// back to last-cached values — never failing the status response.
package xrayapi

import (
	"context"
	"fmt"
	"strings"
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

// Counters returns the cumulative uplink/downlink bytes summed over every
// proxy outbound (the main "proxy" and each per-rule "proxy-<id>").
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

	resp, err := c.client.QueryStats(ctx, &statsservice.QueryStatsRequest{Pattern: "outbound>>>proxy", Reset_: false})
	if err != nil {
		_ = c.resetLocked()
		return 0, 0, fmt.Errorf("QueryStats(outbound>>>proxy): %w", err)
	}
	for _, st := range resp.GetStat() {
		if st == nil || st.Value < 0 {
			continue
		}
		switch {
		case strings.HasSuffix(st.Name, ">>>traffic>>>uplink"):
			up += uint64(st.Value)
		case strings.HasSuffix(st.Name, ">>>traffic>>>downlink"):
			down += uint64(st.Value)
		}
	}
	return up, down, nil
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
