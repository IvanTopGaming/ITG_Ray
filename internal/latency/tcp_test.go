package latency

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTCPConnect_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	d, err := TCPConnect(context.Background(), ln.Addr().String(), 2*time.Second)
	require.NoError(t, err)
	require.Greater(t, d, time.Duration(0))
	require.Less(t, d, 2*time.Second)
}

func TestTCPConnect_Unreachable(t *testing.T) {
	_, err := TCPConnect(context.Background(), "127.0.0.1:1", 200*time.Millisecond)
	require.Error(t, err)
}
