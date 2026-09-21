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

func TestControllerStopReportsDaemonPartialCleanup(t *testing.T) {
	c, _, _, _ := setup(t)
	cc, sc := net.Pipe()
	defer func() { _ = cc.Close() }()
	defer func() { _ = sc.Close() }()
	adapter := &daemonHelperClient{c: client.NewWithConn(cc), sessionID: "synthetic"}
	c.d.Helper = adapter
	c.mode = ModeTUN
	c.cleanupMode = ModeTUN
	c.cleanupPending = true
	require.NoError(t, saveSession(c.d.DataDir, sessionRecord{ServerID: "a", Mode: string(ModeTUN), At: time.Now()}))
	done := make(chan error, 1)
	go func() { done <- c.Stop(context.Background()) }()
	require.NoError(t, sc.SetDeadline(time.Now().Add(time.Second)))
	raw, err := protocol.ReadFrame(sc, protocol.MaxFrame)
	require.NoError(t, err)
	var req protocol.Request
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Equal(t, protocol.OpStopChain, req.Op)
	body, err := json.Marshal(protocol.NewOK(req.ID, json.RawMessage(`{"status":"stopped","partial_errors":["singbox.Stop: process still alive"]}`)))
	require.NoError(t, err)
	require.NoError(t, protocol.WriteFrame(sc, body))
	err = <-done
	status, _, _ := c.Status()
	id, _ := c.LastSession()
	t.Logf("Stop error=%v status=%s cleanupPending=%v last-session=%q", err, status, c.cleanupPending, id)
	require.Error(t, err, "partial cleanup failure must not be discarded by the adapter")
	require.Equal(t, "synthetic", adapter.sessionID)
	require.Equal(t, "a", id)
	require.True(t, c.cleanupPending)
	go func() { done <- c.Stop(context.Background()) }()
	raw, err = protocol.ReadFrame(sc, protocol.MaxFrame)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &req))
	var args struct {
		SessionID string `json:"session_id"`
	}
	require.NoError(t, json.Unmarshal(req.Args, &args))
	require.Equal(t, "synthetic", args.SessionID)
	body, err = json.Marshal(protocol.NewOK(req.ID, json.RawMessage(`{"status":"stopped"}`)))
	require.NoError(t, err)
	require.NoError(t, protocol.WriteFrame(sc, body))
	require.NoError(t, <-done)
	require.Empty(t, adapter.sessionID)
	require.False(t, c.cleanupPending)
}

func TestReconcilePendingDaemonCleanupCanDisconnect(t *testing.T) {
	for _, name := range []string{"present-server", "missing-server"} {
		t.Run(name, func(t *testing.T) {
			c, _, _, _ := setup(t)
			if name == "missing-server" {
				c.d.ServerStore = newMemStore()
			}
			cc, sc := net.Pipe()
			defer func() { _ = cc.Close() }()
			defer func() { _ = sc.Close() }()
			daemon := &daemonHelperClient{c: client.NewWithConn(cc)}
			c.d.Helper = newModeRoutingHelperClient(newFake(), daemon)
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

		})
	}
}
