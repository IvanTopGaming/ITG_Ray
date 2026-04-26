package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/stretchr/testify/require"
)

func TestDispatcher_DispatchOK(t *testing.T) {
	d := NewDispatcher()
	d.Register(protocol.OpServiceStatus, func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"version":"x"}`), nil
	})

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck // test cleanup
	defer serverConn.Close() //nolint:errcheck // test cleanup

	go d.Serve(context.Background(), serverConn)

	req := protocol.Request{ID: 1, Op: protocol.OpServiceStatus}
	body, _ := json.Marshal(req)
	require.NoError(t, protocol.WriteFrame(clientConn, body))

	respBody, err := protocol.ReadFrame(clientConn, protocol.MaxFrame)
	require.NoError(t, err)
	var resp protocol.Response
	require.NoError(t, json.Unmarshal(respBody, &resp))
	require.True(t, resp.OK)
	require.Equal(t, uint64(1), resp.ID)
	require.Equal(t, json.RawMessage(`{"version":"x"}`), resp.Result)
}

func TestDispatcher_UnknownOp(t *testing.T) {
	d := NewDispatcher()
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck // test cleanup
	defer serverConn.Close() //nolint:errcheck // test cleanup

	go d.Serve(context.Background(), serverConn)

	req := protocol.Request{ID: 9, Op: protocol.Op("Bogus")}
	body, _ := json.Marshal(req)
	require.NoError(t, protocol.WriteFrame(clientConn, body))

	respBody, err := protocol.ReadFrame(clientConn, protocol.MaxFrame)
	require.NoError(t, err)
	var resp protocol.Response
	require.NoError(t, json.Unmarshal(respBody, &resp))
	require.False(t, resp.OK)
	require.Contains(t, resp.Error, "unknown op")
}

func TestDispatcher_HandlerError(t *testing.T) {
	d := NewDispatcher()
	d.Register(protocol.OpTunCreate, func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		return nil, &handlerErr{msg: "no admin"}
	})
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck // test cleanup
	defer serverConn.Close() //nolint:errcheck // test cleanup

	go d.Serve(context.Background(), serverConn)

	req := protocol.Request{ID: 3, Op: protocol.OpTunCreate}
	body, _ := json.Marshal(req)
	require.NoError(t, protocol.WriteFrame(clientConn, body))

	// give it 200 ms to respond
	clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)) //nolint:errcheck,gosec // test setup; deadline failure surfaces via subsequent ReadFrame error
	respBody, err := protocol.ReadFrame(clientConn, protocol.MaxFrame)
	require.NoError(t, err)
	var resp protocol.Response
	require.NoError(t, json.Unmarshal(respBody, &resp))
	require.False(t, resp.OK)
	require.Equal(t, "no admin", resp.Error)
}

type handlerErr struct{ msg string }

func (e *handlerErr) Error() string { return e.msg }
