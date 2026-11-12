package handlers

import (
	"context"
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/hub"
)

type Stats interface {
	Overview(period string) (hub.StatsOverview, error)
	Connections() (hub.StatsConnections, error)
	Clear() error
}

type StatsHandlers struct {
	Svc Stats
}

type statsOverviewParams struct {
	Period string `json:"period"`
}

func (s StatsHandlers) Overview(_ context.Context, params json.RawMessage) (any, error) {
	var p statsOverviewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if s.Svc == nil {
		return hub.StatsOverview{Period: p.Period}, nil
	}
	return s.Svc.Overview(p.Period)
}

func (s StatsHandlers) Connections(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return hub.StatsConnections{Conns: []hub.StatsConn{}}, nil
	}
	return s.Svc.Connections()
}

func (s StatsHandlers) Clear(_ context.Context, _ json.RawMessage) (any, error) {
	if s.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, s.Svc.Clear()
}
