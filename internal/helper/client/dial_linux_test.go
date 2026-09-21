package client

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/stretchr/testify/require"
)

func TestDialReconnectsAfterCanceledCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helper.sock")
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	c, err := Dial(context.Background(), path)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	sc, err := listener.Accept()
	require.NoError(t, err)
	defer func() { _ = sc.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() { _, err := c.Call(ctx, protocol.OpServiceStatus, nil); first <- err }()
	readRequest(t, sc)
	cancel()
	require.ErrorIs(t, waitCall(t, first), context.Canceled)
	second := make(chan error, 1)
	go func() { _, err := c.Call(context.Background(), protocol.OpServiceStatus, nil); second <- err }()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	var reconnected net.Conn
	select {
	case reconnected = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("client did not reconnect")
	}
	defer func() { _ = reconnected.Close() }()
	req := readRequest(t, reconnected)
	writeResponse(t, reconnected, req.ID)
	require.NoError(t, waitCall(t, second))
}
