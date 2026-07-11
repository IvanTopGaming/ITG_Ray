package handlers

import (
	"context"
	"encoding/json"
)

// Geo is the surface GeoHandlers needs.
type Geo interface {
	Refresh(ctx context.Context) error
}

// GeoHandlers groups methods under the "geo." namespace.
type GeoHandlers struct {
	Svc Geo
}

// Refresh re-downloads the geo rule-sets for the active source.
func (g GeoHandlers) Refresh(ctx context.Context, _ json.RawMessage) (any, error) {
	if g.Svc == nil {
		return struct{}{}, nil
	}
	if err := g.Svc.Refresh(ctx); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}
