package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/sysproxy"
	"github.com/stretchr/testify/require"
)

type stopTestStore struct{}

func (stopTestStore) Get(string) (*server.Server, error) { return &server.Server{ID: "synthetic"}, nil }
func TestControllerStopReportsUnavailableLazyHelper(t *testing.T) {
	online := true
	backend := &fakeHelper{}
	lazy := newLazyHelperClient(func(context.Context) (chainctl.HelperClient, error) {
		if !online {
			return nil, errors.New("helper unreachable")
		}
		return backend, nil
	})
	h := hub.New()
	defer h.Close()
	events := h.Subscribe(64)
	defer h.Unsubscribe(events)
	c := chainctl.New(&chainctl.Deps{DataDir: t.TempDir(), ServerStore: stopTestStore{}, Helper: lazy, Sysproxy: sysproxy.New(), Hub: h})
	require.NoError(t, c.Start(context.Background(), "synthetic", chainctl.ModeTUN))
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	waiting := true
	for waiting {
		select {
		case e := <-events:
			waiting = e.Name != hub.EventVPNStatus || e.Payload["status"] != "connected"
		case <-timer.C:
			t.Fatal("connect timeout")
		}
	}
	lazy.invalidate()
	online = false
	err := c.Stop(context.Background())
	status, _, _ := c.Status()
	id, _ := c.LastSession()
	t.Logf("Stop error=%v status=%s last-session=%q backend calls=%v", err, status, id, backend.calls)
	require.Error(t, err, "unreachable helper must not be treated as confirmed successful cleanup")
	require.Equal(t, hub.StatusError, status)
	require.Equal(t, "synthetic", id)
	online = true
	require.NoError(t, c.Stop(context.Background()))
	status, _, _ = c.Status()
	require.Equal(t, hub.StatusIdle, status)
	id, _ = c.LastSession()
	require.Empty(t, id)
}
