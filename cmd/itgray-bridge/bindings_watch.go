package main

import (
	"log/slog"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func watchBindings(h *hub.Hub, store *rules.Store, serversPath string, issues *routebind.Issues) {
	events := h.Subscribe(64)
	go func() {
		for e := range events {
			onBindingEvent(e, store, serversPath, issues, h.Publish)
		}
	}()
}

func onBindingEvent(e hub.Event, store *rules.Store, serversPath string, issues *routebind.Issues, publish func(hub.Event)) {
	switch e.Name {
	case hub.EventSubSynced:
		servers, err := server.Load(serversPath)
		if err != nil {
			return
		}
		changed, err := routebind.Apply(store, servers)
		if err != nil {
			slog.Warn("rule rebind failed", slog.String("scope", "rules"), slog.String("err", logging.RedactError(err)))
			return
		}
		if changed {
			slog.Info("rules rebound after subscription sync", slog.String("scope", "rules"))
			publish(hub.Event{Name: hub.EventRulesChanged})
		}
	case hub.EventVPNStatus:
		if e.Payload["status"] != string(hub.StatusConnected) {
			return
		}
		for _, b := range issues.Get() {
			if b.State == routebind.StateUnusable {
				publish(hub.Event{Name: hub.EventRulesChanged})
				return
			}
		}
	}
}
