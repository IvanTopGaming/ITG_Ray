package handlers

import (
	"context"
	"encoding/json"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// Servers is the surface ServersHandlers needs from
// bindings.ServersService. The real type satisfies it directly.
type Servers interface {
	List() ([]hub.ServerView, error)
	Add(uri, name string) (hub.ServerView, error)
	Edit(id, uri, name string) (hub.ServerView, bool, error)
	Remove(id string) error
	ToggleFavorite(id string) error
	TestLatency(id string) error
}

// ServersHandlers groups methods under the "servers." namespace.
type ServersHandlers struct {
	Svc Servers
}

// Local mirrors of protocol.* param/result structs to avoid an import
// cycle into the protocol/codegen package. Kept in sync by the codegen
// drift gate (scripts/check-codegen.sh).
type serversAddParams struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type serversEditParams struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type serversRemoveParams struct {
	ID string `json:"id"`
}

type serversToggleFavoriteParams struct {
	ID string `json:"id"`
}

type serversTestLatencyParams struct {
	ID string `json:"id"`
}

type serversEditResult struct {
	View         hub.ServerView `json:"view"`
	VlessChanged bool           `json:"vlessChanged"`
}

// List returns every known server. Nil-safe: returns nil slice (no error)
// when Svc is unset. Renderer treats nil as empty list.
func (s ServersHandlers) List(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return []hub.ServerView(nil), nil
	}
	return s.Svc.List()
}

// Add creates a manual server from the supplied VLESS URI. Validation,
// ID generation, and persistence happen inside the binding.
func (s ServersHandlers) Add(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.ServerView{}, nil
	}
	var p serversAddParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return s.Svc.Add(p.URI, p.Name)
}

// Edit updates name and/or URI of an existing server. The vlessChanged
// flag tells the renderer whether to show the "Reconnect to apply" banner.
func (s ServersHandlers) Edit(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return serversEditResult{}, nil
	}
	var p serversEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	view, changed, err := s.Svc.Edit(p.ID, p.URI, p.Name)
	if err != nil {
		return nil, err
	}
	return serversEditResult{View: view, VlessChanged: changed}, nil
}

// Remove deletes a manual server. Refuses (returns error) when the server
// is the active connection target — the binding owns that policy.
func (s ServersHandlers) Remove(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	var p serversRemoveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return struct{}{}, s.Svc.Remove(p.ID)
}

// ToggleFavorite flips the favorite flag for the given server id.
func (s ServersHandlers) ToggleFavorite(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	var p serversToggleFavoriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return struct{}{}, s.Svc.ToggleFavorite(p.ID)
}

// TestLatency probes one server (id != "") or every server (id == "").
// Results are published to the hub as probe.result events.
func (s ServersHandlers) TestLatency(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	var p serversTestLatencyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return struct{}{}, s.Svc.TestLatency(p.ID)
}
