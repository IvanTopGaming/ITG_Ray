package bindings

import (
	"fmt"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/stats"
)

type StatsDeps struct {
	Store       *stats.Store
	Collector   *stats.Collector
	Rules       *rules.Store
	ServersPath string
	Now         func() time.Time
	Loc         *time.Location
}

type StatsService struct{ d StatsDeps }

func NewStatsService(d StatsDeps) *StatsService {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Loc == nil {
		d.Loc = time.Local
	}
	return &StatsService{d: d}
}

type ruleInfo struct {
	name, action, groupID string
	locked                bool
}

func (s *StatsService) lookups() (map[string]ruleInfo, map[string]string) {
	ruleByID := map[string]ruleInfo{}
	if s.d.Rules != nil {
		if m, err := s.d.Rules.Load(); err == nil {
			for _, g := range m.Groups {
				for _, r := range g.Rules {
					ruleByID[r.ID] = ruleInfo{name: r.Name, action: string(r.Action), groupID: g.ID, locked: g.Locked}
				}
			}
		}
	}
	serverName := map[string]string{}
	if s.d.ServersPath != "" {
		if list, err := server.Load(s.d.ServersPath); err == nil {
			for i := range list {
				serverName[list[i].ID] = list[i].Name
			}
		}
	}
	return ruleByID, serverName
}

func (s *StatsService) Overview(period string) (hub.StatsOverview, error) {
	p := stats.Period(period)
	if !stats.ValidPeriod(p) {
		return hub.StatsOverview{}, fmt.Errorf("stats: unknown period %q", period)
	}
	if s.d.Collector != nil {
		if err := s.d.Collector.Flush(); err != nil {
			return hub.StatsOverview{}, err
		}
	}
	now := s.d.Now()
	ov, err := s.d.Store.Overview(p, now, s.d.Loc)
	if err != nil {
		return hub.StatsOverview{}, err
	}
	if s.d.Collector != nil && len(ov.Series) > 0 {
		if since, ok := s.d.Collector.ActiveSince(); ok {
			begin := ov.Series[0].Start
			if since.Before(begin) {
				since = begin
			}
			if now.After(since) {
				ov.OnlineSeconds += int64(now.Sub(since).Seconds())
				ov.Sessions++
			}
		}
	}
	ruleByID, serverName := s.lookups()
	out := hub.StatsOverview{
		Period: period, Down: ov.Down, Up: ov.Up,
		DownProxy: ov.DownProxy, DownDirect: ov.DownDirect, UpProxy: ov.UpProxy, UpDirect: ov.UpDirect,
		Blocked: ov.Blocked, OnlineSeconds: ov.OnlineSeconds, Sessions: ov.Sessions,
		Series: make([]hub.StatsPoint, 0, len(ov.Series)),
		Apps:   make([]hub.StatsApp, 0, len(ov.Apps)),
		Rules:  make([]hub.StatsRule, 0, len(ov.Rules)),
	}
	for _, pt := range ov.Series {
		out.Series = append(out.Series, hub.StatsPoint{
			Start: pt.Start.Format(time.RFC3339), Down: pt.Down, Up: pt.Up,
			DownProxy: pt.DownProxy, DownDirect: pt.DownDirect, UpProxy: pt.UpProxy, UpDirect: pt.UpDirect,
		})
	}
	for _, a := range ov.Apps {
		out.Apps = append(out.Apps, hub.StatsApp{App: a.App, Down: a.Down, Up: a.Up, DownProxy: a.DownProxy, UpProxy: a.UpProxy})
	}
	for _, r := range ov.Rules {
		row := hub.StatsRule{RuleID: r.RuleID, Route: r.Route, ServerID: r.ServerID, ServerName: serverName[r.ServerID], Conns: r.Conns}
		if info, ok := ruleByID[r.RuleID]; ok {
			row.RuleName = info.name
			row.Action = info.action
		} else if r.RuleID != "final" {
			row.Deleted = true
		}
		if row.Action == "" {
			row.Action = r.Route
		}
		out.Rules = append(out.Rules, row)
	}
	return out, nil
}

func (s *StatsService) Connections() (hub.StatsConnections, error) {
	out := hub.StatsConnections{Conns: []hub.StatsConn{}}
	if s.d.Collector == nil {
		return out, nil
	}
	ruleByID, serverName := s.lookups()
	for _, c := range s.d.Collector.Live() {
		row := hub.StatsConn{
			ID: c.ID, App: c.App, Host: c.Host, Suffix: c.Suffix, DestIP: c.DestIP, Network: c.Network,
			RuleID: c.RuleID, Route: c.Route, ServerID: c.ServerID, ServerName: serverName[c.ServerID],
			DownBps: c.DownBps, UpBps: c.UpBps,
		}
		if !c.Start.IsZero() {
			row.Start = c.Start.Format(time.RFC3339)
		}
		if info, ok := ruleByID[c.RuleID]; ok {
			row.RuleName = info.name
			row.RuleGroupID = info.groupID
			row.RuleGroupLocked = info.locked
		}
		out.Conns = append(out.Conns, row)
	}
	return out, nil
}

func (s *StatsService) Clear() error {
	if s.d.Collector != nil {
		if err := s.d.Collector.Flush(); err != nil {
			return err
		}
	}
	return s.d.Store.Clear()
}
