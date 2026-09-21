package chainctl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/stretchr/testify/require"
)

func TestController_DisconnectClearsProxyAfterCrash(t *testing.T) {
	c, fh, h, _ := setup(t)
	proxy := &fakeSysproxy{}
	c.d.Sysproxy = proxy
	c.d.KillSwitch = func() (config.KillSwitch, error) { return config.KillSwitch{Enabled: true}, nil }
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	fh.mu.Lock()
	fh.running = false
	fh.mu.Unlock()
	waitForVpnStatus(t, rcv, string(hub.StatusError), 2*time.Second)
	c.wg.Wait()
	require.NoError(t, c.Stop(context.Background()))
	enabled, err := proxy.IsSet()
	require.NoError(t, err)
	status, _, _ := c.Status()
	t.Logf("after explicit Disconnect: proxy enabled=%v clear calls=%d controller status=%s", enabled, proxy.ClearCalls(), status)
	require.False(t, enabled, "explicit disconnect must restore networking after crash")
}

func TestController_RestartAfterCrashCleansPreviousMode(t *testing.T) {
	c, fh, h, _ := setup(t)
	proxy := &fakeSysproxy{}
	c.d.Sysproxy = proxy
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	fh.mu.Lock()
	fh.running = false
	fh.mu.Unlock()
	waitForVpnStatus(t, rcv, string(hub.StatusError), 2*time.Second)
	c.wg.Wait()
	enabled, err := proxy.IsSet()
	require.NoError(t, err)
	require.True(t, enabled)
	require.NoError(t, c.Start(context.Background(), "a", ModeTUN))
	defer func() { _ = c.Stop(context.Background()) }()
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	enabled, err = proxy.IsSet()
	require.NoError(t, err)
	require.False(t, enabled, "switching to TUN after a crash must release the previous OS proxy")
}

func TestController_FailedRetryKeepsCrashProxyBlocked(t *testing.T) {
	for _, stage := range []string{"build", "start", "sysproxy"} {
		t.Run(stage, func(t *testing.T) {
			c, fh, h, _ := setup(t)
			proxy := &fakeSysproxy{}
			c.d.Sysproxy = proxy
			rcv := h.Subscribe(64)
			defer h.Unsubscribe(rcv)
			require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
			waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
			fh.mu.Lock()
			fh.running = false
			fh.mu.Unlock()
			waitForVpnStatus(t, rcv, string(hub.StatusError), 2*time.Second)
			c.wg.Wait()
			mode := ModeTUN
			switch stage {
			case "build":
				c.d.BuildConfigs = func(_ *server.Server, _ Mode, _ config.Network) ([]byte, []byte, error) {
					return nil, nil, errors.New("build failed")
				}
			case "start":
				fh.failOn = "StartChain"
			case "sysproxy":
				mode = ModeSysProxy
				proxy.setErr = errFail
			}
			require.NoError(t, c.Start(context.Background(), "a", mode))
			c.wg.Wait()
			enabled, err := proxy.IsSet()
			require.NoError(t, err)
			require.True(t, enabled, "failed reconnect must retain fail-closed proxy")
			require.Zero(t, proxy.ClearCalls())
			require.NoError(t, c.Stop(context.Background()))
			enabled, err = proxy.IsSet()
			require.NoError(t, err)
			require.False(t, enabled)
		})
	}
}

type deadlineHelper struct {
	*fakeHelper
	deadline time.Time
}

func (h *deadlineHelper) StopChain(ctx context.Context) error {
	h.deadline, _ = ctx.Deadline()
	return h.fakeHelper.StopChain(ctx)
}

func TestController_DisconnectBoundsHelperTeardown(t *testing.T) {
	c, fh, h, _ := setup(t)
	helper := &deadlineHelper{fakeHelper: fh}
	c.d.Helper = helper
	c.d.Sysproxy = &fakeSysproxy{}
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	require.NoError(t, c.Stop(context.Background()))
	require.False(t, helper.deadline.IsZero(), "Disconnect must not wait indefinitely for an unresponsive helper")
}

type blockedStopHelper struct {
	*fakeHelper
	blocked bool
}

func (h *blockedStopHelper) StopChain(ctx context.Context) error {
	if h.blocked {
		<-ctx.Done()
		return ctx.Err()
	}
	return h.fakeHelper.StopChain(ctx)
}

func TestController_DisconnectTimeoutRetainsCleanupForRetry(t *testing.T) {
	c, fh, h, _ := setup(t)
	helper := &blockedStopHelper{fakeHelper: fh, blocked: true}
	proxy := &fakeSysproxy{}
	c.d.Helper = helper
	c.d.Sysproxy = proxy
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, c.Stop(ctx), context.DeadlineExceeded)
	enabled, err := proxy.IsSet()
	require.NoError(t, err)
	require.False(t, enabled)
	status, _, _ := c.Status()
	require.Equal(t, hub.StatusError, status)
	id, _ := c.LastSession()
	require.Equal(t, "a", id)
	helper.blocked = false
	require.NoError(t, c.Stop(context.Background()))
	status, _, _ = c.Status()
	require.Equal(t, hub.StatusIdle, status)
	id, _ = c.LastSession()
	require.Empty(t, id)
}

func TestController_ConcurrentRetriesStartOnlyOneChain(t *testing.T) {
	c, fh, h, _ := setup(t)
	c.d.Sysproxy = &fakeSysproxy{}
	rcv := h.Subscribe(64)
	defer h.Unsubscribe(rcv)
	require.NoError(t, c.Start(context.Background(), "a", ModeSysProxy))
	waitForVpnStatus(t, rcv, string(hub.StatusConnected), time.Second)
	fh.mu.Lock()
	fh.running = false
	fh.mu.Unlock()
	waitForVpnStatus(t, rcv, string(hub.StatusError), 2*time.Second)
	c.wg.Wait()
	c.wg.Add(1)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-start; results <- c.Start(context.Background(), "a", ModeSysProxy) }()
	}
	close(start)
	time.Sleep(20 * time.Millisecond)
	c.wg.Done()
	first, second := <-results, <-results
	require.True(t, (first == nil) != (second == nil), "only one concurrent retry may acquire the chain: %v, %v", first, second)
	require.NoError(t, c.Stop(context.Background()))
}
