package bindings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/hub"
)

type fakeRouting struct {
	mode   string
	calls  []string
	setErr error
}

func (f *fakeRouting) Mode(context.Context) (string, error) { return f.mode, nil }
func (f *fakeRouting) SetMode(_ context.Context, m string) error {
	f.calls = append(f.calls, "set:"+m)
	if f.setErr != nil {
		return f.setErr
	}
	f.mode = m
	return nil
}
func (f *fakeRouting) CloseConnections(context.Context) error {
	f.calls = append(f.calls, "close")
	return nil
}

func routingSvc(t *testing.T, connected bool, api RoutingAPI) (*RunService, <-chan hub.Event) {
	t.Helper()
	h := hub.New()
	t.Cleanup(h.Close)
	events := h.Subscribe(8)
	svc := NewRunService(RunDeps{Hub: h, Routing: api, Connected: func() bool { return connected }})
	return svc, events
}

func TestSetRoutingMode_RejectsUnknownAndIdle(t *testing.T) {
	api := &fakeRouting{mode: "rule"}
	svc, _ := routingSvc(t, true, api)
	require.Error(t, svc.SetRoutingMode("turbo"))
	idle, _ := routingSvc(t, false, api)
	require.Error(t, idle.SetRoutingMode("global"))
	require.Empty(t, api.calls)
}

func TestSetRoutingMode_ClosesConnectionsAndPublishes(t *testing.T) {
	api := &fakeRouting{mode: "rule"}
	svc, events := routingSvc(t, true, api)
	require.NoError(t, svc.SetRoutingMode("direct"))
	require.Equal(t, []string{"set:direct", "close"}, api.calls)
	select {
	case e := <-events:
		require.Equal(t, hub.EventRoutingMode, e.Name)
		require.Equal(t, "direct", e.Payload["mode"])
	case <-time.After(2 * time.Second):
		t.Fatal("routing:mode not published")
	}
}

func TestSetRoutingMode_FailureDoesNotCloseOrPublish(t *testing.T) {
	api := &fakeRouting{mode: "rule", setErr: errors.New("boom")}
	svc, events := routingSvc(t, true, api)
	require.Error(t, svc.SetRoutingMode("global"))
	require.Equal(t, []string{"set:global"}, api.calls)
	select {
	case e := <-events:
		t.Fatalf("unexpected event %s", e.Name)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestGetRoutingMode(t *testing.T) {
	api := &fakeRouting{mode: "global"}
	svc, _ := routingSvc(t, true, api)
	m, err := svc.GetRoutingMode()
	require.NoError(t, err)
	require.Equal(t, "global", m)
	idle, _ := routingSvc(t, false, api)
	m, err = idle.GetRoutingMode()
	require.NoError(t, err)
	require.Equal(t, "rule", m)
}
