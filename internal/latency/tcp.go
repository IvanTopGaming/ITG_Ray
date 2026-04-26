// Package latency measures reachability to VLESS servers.
package latency

import (
	"context"
	"net"
	"time"
)

// TCPConnect dials addr (host:port) and returns the time-to-establish.
// The dial is bounded by both the parent context and the timeout.
func TCPConnect(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	d := net.Dialer{}
	start := time.Now()
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	elapsed := time.Since(start)
	_ = c.Close()
	return elapsed, nil
}
