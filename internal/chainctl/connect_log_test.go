package chainctl

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logtest"
	"github.com/itg-team/itg-ray/internal/sysproxy"
	"github.com/stretchr/testify/require"
)

// TestStart_ConnectFailure_LogsScopedError pins that a bringUp failure is
// written to app.log (scope=chainctl), not only published to the UI hub.
// Start returns nil and runs bringUp on a goroutine, so the connect error
// never reaches the RPC Observer — without a source log the exported log
// file has no record of why a connect failed.
func TestStart_ConnectFailure_LogsScopedError(t *testing.T) {
	buf := logtest.Capture(t)
	dir := t.TempDir()
	store := newMemStore(fixtureServer())
	h := hub.New()
	t.Cleanup(h.Close)

	c := New(&Deps{
		DataDir:     dir,
		ServerStore: store,
		Helper:      newFake(),
		Sysproxy:    sysproxy.New(),
		Hub:         h,
		Network:     errNetwork(errors.New("disk corrupt")),
	})
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)

	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))
	// Wait for the terminal idle status: it is published at the END of the
	// goroutine's error path, AFTER the source log line, so its channel
	// receive both orders our read after the write and provides the
	// happens-before that makes reading the buffer race-free.
	waitForVpnStatus(t, rcv, string(hub.StatusIdle), 2*time.Second)

	out := buf.String()
	if !strings.Contains(out, "[chainctl]") || !strings.Contains(out, "chain connect failed") {
		t.Fatalf("connect failure not logged to app.log: %q", out)
	}
	if !strings.Contains(out, "config.Load") {
		t.Fatalf("connect log missing the failing-step detail: %q", out)
	}
}
