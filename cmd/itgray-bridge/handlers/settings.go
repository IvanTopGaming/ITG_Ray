package handlers

import (
	"context"
	"encoding/json"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// Settings is the surface SettingsHandlers needs from
// bindings.SettingsService. The real type satisfies it directly.
type Settings interface {
	Get() (hub.SettingsView, error)
	Update(section string, patch map[string]any) (hub.SettingsView, error)
}

// SettingsHandlers groups methods under the "settings." namespace.
type SettingsHandlers struct {
	Svc Settings
}

// Get returns the current persisted settings. Nil-safe: returns a zero
// SettingsView (no error) when Svc is unset, useful in tests.
func (s SettingsHandlers) Get(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.SettingsView{}, nil
	}
	return s.Svc.Get()
}

// settingsUpdateParams mirrors protocol.SettingsUpdateParams. It is owned
// here to avoid an import cycle into the protocol/codegen package.
type settingsUpdateParams struct {
	Section string         `json:"section"`
	Patch   map[string]any `json:"patch"`
}

// Update merges patch into the named section. Malformed params return
// an unmarshal error (mapped by the dispatcher to JSON-RPC -32603).
// Nil-safe: returns a zero SettingsView (no error) when Svc is unset.
func (s SettingsHandlers) Update(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.SettingsView{}, nil
	}
	var p settingsUpdateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return s.Svc.Update(p.Section, p.Patch)
}
