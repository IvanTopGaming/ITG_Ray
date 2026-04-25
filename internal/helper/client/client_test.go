package client

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/stretchr/testify/require"
)

func TestClient_Call(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck // test cleanup
	defer serverConn.Close() //nolint:errcheck // test cleanup

	// fake server: read one frame, echo a fixed OK response.
	go func() {
		body, err := protocol.ReadFrame(serverConn, protocol.MaxFrame)
		if err != nil {
			return
		}
		var req protocol.Request
		_ = json.Unmarshal(body, &req)
		resp := protocol.NewOK(req.ID, json.RawMessage(`{"version":"x","uptime_secs":1}`))
		out, _ := json.Marshal(resp)
		_ = protocol.WriteFrame(serverConn, out)
	}()

	c := NewWithConn(clientConn)
	res, err := c.Call(context.Background(), protocol.OpServiceStatus, nil)
	require.NoError(t, err)
	require.Equal(t, json.RawMessage(`{"version":"x","uptime_secs":1}`), res)
}

func TestClient_ServerError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close() //nolint:errcheck // test cleanup
	defer serverConn.Close() //nolint:errcheck // test cleanup

	go func() {
		body, err := protocol.ReadFrame(serverConn, protocol.MaxFrame)
		if err != nil {
			return
		}
		var req protocol.Request
		_ = json.Unmarshal(body, &req)
		out, _ := json.Marshal(protocol.NewError(req.ID, "no admin"))
		_ = protocol.WriteFrame(serverConn, out)
	}()

	c := NewWithConn(clientConn)
	_, err := c.Call(context.Background(), protocol.OpTunCreate, json.RawMessage(`{}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no admin")
}
