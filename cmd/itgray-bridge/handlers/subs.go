package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// Subs is the surface SubsHandlers needs from bindings.SubsService.
// The real type satisfies it directly (Add/Edit accept a userAgent that
// the bridge always passes as "" — bindings persists empty UA, and
// SyncOne falls back to the settings.subscriptions.userAgent default).
type Subs interface {
	List() ([]hub.SubView, error)
	Add(url, name, userAgent string) (hub.SubView, error)
	Edit(id, url, name, userAgent string) (hub.SubView, error)
	Remove(id string) error
	SyncOne(id string) error
	SyncAll() error
}

// SubsHandlers groups methods under the "subs." namespace.
type SubsHandlers struct {
	Svc Subs
}

type subsAddParams struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type subsEditParams struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type subsRemoveParams struct {
	ID string `json:"id"`
}

type subsSyncOneParams struct {
	ID string `json:"id"`
}

// List returns every persisted subscription. Nil-safe.
func (s SubsHandlers) List(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return []hub.SubView(nil), nil
	}
	return s.Svc.List()
}

// Add creates a new subscription. userAgent is always passed as "" — the
// bindings layer persists it as such, and SyncOne falls back to the
// per-call settings.subscriptions.userAgent default.
func (s SubsHandlers) Add(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.SubView{}, nil
	}
	var p subsAddParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return s.Svc.Add(p.URL, p.Name, "")
}

// Edit updates name and/or URL of an existing subscription. userAgent
// is always passed as "" for the same reason as Add.
func (s SubsHandlers) Edit(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.SubView{}, nil
	}
	var p subsEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return s.Svc.Edit(p.ID, p.URL, p.Name, "")
}

// Remove deletes the subscription with the given id.
func (s SubsHandlers) Remove(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	var p subsRemoveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return struct{}{}, s.Svc.Remove(p.ID)
}

// SyncOne fetches one subscription, merges its servers, and returns the
// updated SubView. bindings.SubsService.SyncOne returns only an error;
// the protocol declares hub.SubView as the result, so we re-fetch via
// List() and locate the row by id. Nil-safe: returns zero SubView when
// Svc is unset.
func (s SubsHandlers) SyncOne(_ context.Context, params json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.SubView{}, nil
	}
	var p subsSyncOneParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := s.Svc.SyncOne(p.ID); err != nil {
		return nil, err
	}
	views, err := s.Svc.List()
	if err != nil {
		return nil, err
	}
	for _, v := range views {
		if v.ID == p.ID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("subs.syncOne: id %q not found after sync", p.ID)
}

// SyncAll iterates over every subscription and refreshes each.
func (s SubsHandlers) SyncAll(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, s.Svc.SyncAll()
}
