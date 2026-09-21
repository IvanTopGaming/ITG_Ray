package chainctl

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/stretchr/testify/require"
)

func TestWindowsAdapterRetainsSessionOnPartialStop(t *testing.T) {
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	a := NewHelperAdapter(client.NewWithConn(cc), "test")
	a.sessionID = "owned"
	for _, response := range []string{`{"status":"stopped","partial_errors":["dns.Restore: unavailable"]}`, `{"status":"stopped"}`} {
		done := make(chan error, 1)
		go func() { done <- a.StopChain(context.Background()) }()
		require.NoError(t, sc.SetDeadline(time.Now().Add(time.Second)))
		raw, err := protocol.ReadFrame(sc, protocol.MaxFrame)
		require.NoError(t, err)
		var req protocol.Request
		require.NoError(t, json.Unmarshal(raw, &req))
		var args struct {
			SessionID string `json:"session_id"`
		}
		require.NoError(t, json.Unmarshal(req.Args, &args))
		require.Equal(t, "owned", args.SessionID)
		raw, err = json.Marshal(protocol.NewOK(req.ID, json.RawMessage(response)))
		require.NoError(t, err)
		require.NoError(t, protocol.WriteFrame(sc, raw))
		err = <-done
		if response == `{"status":"stopped"}` {
			require.NoError(t, err)
			require.Empty(t, a.sessionID)
		} else {
			require.ErrorContains(t, err, "dns.Restore")
			require.Equal(t, "owned", a.sessionID)
		}
	}
}

func TestReconcilePendingWindowsCleanupCanDisconnect(t *testing.T) {
	c, _, _, _ := setup(t)
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	adapter := NewHelperAdapter(client.NewWithConn(cc), "test")
	c.d.Helper = adapter
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{ServerID: "a", Mode: string(ModeTUN), At: time.Now()}))
	done := make(chan struct{})
	go func() { c.Reconcile(context.Background()); close(done) }()
	require.NoError(t, sc.SetDeadline(time.Now().Add(time.Second)))
	raw, err := protocol.ReadFrame(sc, protocol.MaxFrame)
	require.NoError(t, err)
	var req protocol.Request
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Equal(t, protocol.OpServiceStatus, req.Op)
	raw, err = json.Marshal(protocol.NewOK(req.ID, json.RawMessage(`{"chain_active":false,"cleanup_pending":true}`)))
	require.NoError(t, err)
	require.NoError(t, protocol.WriteFrame(sc, raw))
	<-done
	status, _, _ := c.Status()
	require.Equal(t, hub.StatusError, status, "pending cleanup must be recoverable without adopting a running VPN")
	stopped := make(chan error, 1)
	go func() { stopped <- c.Stop(context.Background()) }()
	raw, err = protocol.ReadFrame(sc, protocol.MaxFrame)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Equal(t, protocol.OpStopChain, req.Op)
	raw, err = json.Marshal(protocol.NewOK(req.ID, json.RawMessage(`{"status":"stopped"}`)))
	require.NoError(t, err)
	require.NoError(t, protocol.WriteFrame(sc, raw))
	require.NoError(t, <-stopped)
	status, _, _ = c.Status()
	require.Equal(t, hub.StatusIdle, status)
	id, _ := c.LastSession()
	require.Empty(t, id)
}
