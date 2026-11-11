package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/clashapi"
	"github.com/itg-team/itg-ray/internal/hub"
)

type staticMode string

func (s staticMode) Mode(context.Context) (string, error) { return string(s), nil }

func TestOnRoutingEvent_ConnectedPublishesLiveMode(t *testing.T) {
	var got []hub.Event
	publish := func(e hub.Event) { got = append(got, e) }
	onRoutingEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "connected"}}, staticMode("global"), publish)
	require.Len(t, got, 1)
	require.Equal(t, hub.EventRoutingMode, got[0].Name)
	require.Equal(t, "global", got[0].Payload["mode"])
	onRoutingEvent(hub.Event{Name: hub.EventVPNStatus, Payload: map[string]any{"status": "idle"}}, staticMode("global"), publish)
	require.Len(t, got, 1)
}

func TestRoutingClient_SurvivesBridgeRestart(t *testing.T) {
	dir := t.TempDir()
	a, err := newRoutingClient(dir)
	require.NoError(t, err)
	b, err := newRoutingClient(dir)
	require.NoError(t, err)
	secret, err := clashapi.LoadOrCreateSecret(dir)
	require.NoError(t, err)
	require.Equal(t, a.Secret(), secret)
	require.Equal(t, b.Secret(), secret)
}
