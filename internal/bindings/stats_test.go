package bindings

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/stats"
)

type staticConns struct{ snap stats.Snapshot }

func (s staticConns) Connections(context.Context) (stats.Snapshot, error) { return s.snap, nil }

var statsTZ = time.FixedZone("MSK", 3*3600)

func newStatsFixture(t *testing.T) (*StatsService, *stats.Store, *stats.Collector, *time.Time) {
	t.Helper()
	dir := t.TempDir()
	store, err := stats.OpenStore(filepath.Join(dir, "stats.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, statsTZ)
	rs := rules.NewStore(dir)
	require.NoError(t, rs.Save(rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{
		{ID: "safety", Name: "Safety", Locked: true, Enabled: true, Rules: []rules.Rule{{ID: "lan", Name: "Private IPs", Enabled: true, Action: rules.ActionDirect, Conditions: rules.Conditions{IPCIDRs: []string{"10.0.0.0/8"}}}}},
		{ID: "g", Name: "Mine", Enabled: true, Rules: []rules.Rule{{ID: "r1", Name: "Video", Enabled: true, Action: rules.ActionProxy, ServerID: "x", Conditions: rules.Conditions{Processes: []string{"firefox"}}}}},
	}}))
	srvPath := filepath.Join(dir, "servers.json")
	require.NoError(t, server.Save(srvPath, []server.Server{{ID: "x", Name: "🇫🇮 Finland", Origin: server.OriginManual}}))
	src := staticConns{snap: stats.Snapshot{Conns: []stats.Conn{
		{ID: "1", Host: "video.example.com", Process: "/usr/bin/firefox", Rule: "process_name=firefox || inbound=itg-rule-r1 => route(proxy-x)", Chains: []string{"proxy-x"}, Down: 500},
		{ID: "2", DestIP: "10.0.0.5", Process: "/usr/bin/ssh", Rule: "ip_cidr=10.0.0.0/8 || inbound=itg-rule-lan => route(direct)", Chains: []string{"direct"}, Down: 5},
	}}}
	col := stats.NewCollector(src, store, func() string { return "main" }, func() time.Time { return now })
	svc := NewStatsService(StatsDeps{Store: store, Collector: col, Rules: rs, ServersPath: srvPath, Now: func() time.Time { return now }, Loc: statsTZ})
	return svc, store, col, &now
}

func TestStatsService_OverviewEnrichesAndFlagsDeleted(t *testing.T) {
	svc, store, col, now := newStatsFixture(t)
	col.Tick(context.Background())
	require.NoError(t, store.Add(*now, stats.Dims{RuleID: "gone", Route: "block"}, stats.Delta{Conns: 4}))
	ov, err := svc.Overview("today")
	require.NoError(t, err)
	require.Equal(t, uint64(505), ov.Down)
	byID := map[string]int{}
	for i, r := range ov.Rules {
		byID[r.RuleID] = i
	}
	video := ov.Rules[byID["r1"]]
	require.Equal(t, "Video", video.RuleName)
	require.Equal(t, "proxy", video.Action)
	require.Equal(t, "🇫🇮 Finland", video.ServerName)
	gone := ov.Rules[byID["gone"]]
	require.True(t, gone.Deleted)
	require.Equal(t, "", gone.RuleName)
	require.Equal(t, 4, ov.Blocked)
	require.Len(t, ov.Series, 24)
}

func TestStatsService_OverviewIncludesActiveSession(t *testing.T) {
	svc, _, col, now := newStatsFixture(t)
	col.SessionStart(now.Add(-30 * time.Minute))
	ov, err := svc.Overview("today")
	require.NoError(t, err)
	require.Equal(t, int64(1800), ov.OnlineSeconds)
	require.Equal(t, 1, ov.Sessions)
}

func TestStatsService_OverviewRejectsUnknownPeriod(t *testing.T) {
	svc, _, _, _ := newStatsFixture(t)
	_, err := svc.Overview("forever")
	require.Error(t, err)
}

func TestStatsService_ConnectionsCarryRuleContext(t *testing.T) {
	svc, _, col, _ := newStatsFixture(t)
	col.Tick(context.Background())
	res, err := svc.Connections()
	require.NoError(t, err)
	require.Len(t, res.Conns, 2)
	byID := map[string]int{}
	for i, c := range res.Conns {
		byID[c.ID] = i
	}
	v := res.Conns[byID["1"]]
	require.Equal(t, "video.example.com", v.Host)
	require.Equal(t, "example.com", v.Suffix)
	require.Equal(t, "Video", v.RuleName)
	require.Equal(t, "g", v.RuleGroupID)
	require.False(t, v.RuleGroupLocked)
	require.Equal(t, "🇫🇮 Finland", v.ServerName)
	lan := res.Conns[byID["2"]]
	require.Equal(t, "safety", lan.RuleGroupID)
	require.True(t, lan.RuleGroupLocked)
	require.Equal(t, "10.0.0.5", lan.Host)
}

func TestStatsService_ClearWipesStoreAndPending(t *testing.T) {
	svc, _, col, _ := newStatsFixture(t)
	col.Tick(context.Background())
	require.NoError(t, svc.Clear())
	ov, err := svc.Overview("today")
	require.NoError(t, err)
	require.Zero(t, ov.Down)
}
