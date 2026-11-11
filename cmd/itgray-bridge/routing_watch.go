package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/itg-team/itg-ray/internal/clashapi"
	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logging"
)

type modeReader interface {
	Mode(ctx context.Context) (string, error)
}

func newRoutingClient(dataDir string) (*clashapi.Client, error) {
	secret, err := clashapi.LoadOrCreateSecret(dataDir)
	if err != nil {
		return nil, err
	}
	return clashapi.New(configgen.ClashAPIPort, secret), nil
}

func watchRouting(h *hub.Hub, api modeReader) {
	events := h.Subscribe(64)
	go func() {
		for e := range events {
			onRoutingEvent(e, api, h.Publish)
		}
	}()
}

func onRoutingEvent(e hub.Event, api modeReader, publish func(hub.Event)) {
	if e.Name != hub.EventVPNStatus || e.Payload["status"] != string(hub.StatusConnected) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	mode, err := api.Mode(ctx)
	if err != nil {
		slog.Warn("routing mode read failed", slog.String("scope", "run"), slog.String("err", logging.RedactError(err)))
		mode = "rule"
	}
	publish(hub.Event{Name: hub.EventRoutingMode, Payload: map[string]any{"mode": mode}})
}
