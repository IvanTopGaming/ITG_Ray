package client

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/stretchr/testify/require"
)

func readRequest(t *testing.T, conn net.Conn) protocol.Request {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	raw, err := protocol.ReadFrame(conn, protocol.MaxFrame)
	require.NoError(t, err)
	var req protocol.Request
	require.NoError(t, json.Unmarshal(raw, &req))
	return req
}

func writeResponse(t *testing.T, conn net.Conn, id uint64) {
	t.Helper()
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(time.Second)))
	raw, err := json.Marshal(protocol.NewOK(id, json.RawMessage(`{"ok":true}`)))
	require.NoError(t, err)
	require.NoError(t, protocol.WriteFrame(conn, raw))
}

func waitCall(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("Call did not return after its context ended")
		return nil
	}
}

func TestCallCancellationInterruptsResponseRead(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			cc, sc := net.Pipe()
			defer func() { _ = cc.Close() }()
			defer func() { _ = sc.Close() }()
			c := NewWithConn(cc)
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if deadline {
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				want = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := c.Call(ctx, protocol.OpServiceStatus, nil); done <- err }()
			readRequest(t, sc)
			if !deadline {
				cancel()
			}
			require.ErrorIs(t, waitCall(t, done), want)
			_ = sc.SetReadDeadline(time.Now().Add(time.Second))
			_, err := protocol.ReadFrame(sc, protocol.MaxFrame)
			require.Error(t, err)
			retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
			defer retryCancel()
			_, err = c.Call(retryCtx, protocol.OpServiceStatus, nil)
			require.Error(t, err)
		})
	}
}

func TestQueuedCancellationPreservesActiveConnection(t *testing.T) {
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	c := NewWithConn(cc)
	first := make(chan error, 1)
	go func() { _, err := c.Call(context.Background(), protocol.OpServiceStatus, nil); first <- err }()
	req := readRequest(t, sc)
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() { _, err := c.Call(ctx, protocol.OpServiceStatus, nil); second <- err }()
	cancel()
	require.ErrorIs(t, waitCall(t, second), context.Canceled)
	writeResponse(t, sc, req.ID)
	require.NoError(t, waitCall(t, first))
	third := make(chan error, 1)
	go func() { _, err := c.Call(context.Background(), protocol.OpServiceStatus, nil); third <- err }()
	req = readRequest(t, sc)
	writeResponse(t, sc, req.ID)
	require.NoError(t, waitCall(t, third))
}

func TestCompletedCallCancellationDoesNotCloseNextCall(t *testing.T) {
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	c := NewWithConn(cc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() { _, err := c.Call(ctx, protocol.OpServiceStatus, nil); first <- err }()
	req := readRequest(t, sc)
	writeResponse(t, sc, req.ID)
	require.NoError(t, waitCall(t, first))
	next := make(chan error, 1)
	go func() { _, err := c.Call(context.Background(), protocol.OpServiceStatus, nil); next <- err }()
	req = readRequest(t, sc)
	cancel()
	writeResponse(t, sc, req.ID)
	require.NoError(t, waitCall(t, next))
}

func TestCloseInterruptsActiveCall(t *testing.T) {
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	c := NewWithConn(cc)
	done := make(chan error, 1)
	go func() { _, err := c.Call(context.Background(), protocol.OpServiceStatus, nil); done <- err }()
	readRequest(t, sc)
	require.NoError(t, c.Close())
	require.Error(t, waitCall(t, done))
	_, err := c.Call(context.Background(), protocol.OpServiceStatus, nil)
	require.ErrorIs(t, err, net.ErrClosed)
}

func TestCallCancellationInterruptsRequestWrite(t *testing.T) {
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	c := NewWithConn(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.Call(ctx, protocol.OpServiceStatus, nil); done <- err }()
	require.ErrorIs(t, waitCall(t, done), context.DeadlineExceeded)
}
