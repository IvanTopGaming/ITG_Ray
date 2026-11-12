package stats

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	snap Snapshot
	err  error
}

func (f *fakeSource) Connections(context.Context) (Snapshot, error) { return f.snap, f.err }

type collectorFixture struct {
	c     *Collector
	src   *fakeSource
	store *Store
	now   time.Time
}

func newCollectorFixture(t *testing.T) *collectorFixture {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	f := &collectorFixture{src: &fakeSource{}, store: store, now: time.Date(2026, 11, 12, 10, 0, 0, 0, msk)}
	f.c = NewCollector(f.src, store, func() string { return "main" }, func() time.Time { return f.now })
	return f
}

func (f *collectorFixture) tick(t *testing.T, conns ...Conn) {
	t.Helper()
	f.src.snap = Snapshot{Conns: conns}
	f.c.Tick(context.Background())
	f.now = f.now.Add(time.Second)
}

func (f *collectorFixture) overview(t *testing.T) Overview {
	t.Helper()
	require.NoError(t, f.c.Flush())
	ov, err := f.store.Overview(PeriodToday, f.now, msk)
	require.NoError(t, err)
	return ov
}

const markedRule = "domain_suffix=example.com || inbound=itg-rule-r123 => route(proxy-x)"

func TestAppName(t *testing.T) {
	require.Equal(t, "curl", AppName("/usr/bin/curl (itg)"))
	require.Equal(t, "Discord", AppName(`C:\Users\a\AppData\Local\Discord\Discord.exe`))
	require.Equal(t, "firefox", AppName("/usr/lib/firefox/firefox"))
	require.Equal(t, "", AppName(""))
}

func TestRuleIDFromClash(t *testing.T) {
	require.Equal(t, "r123", RuleIDFromClash(markedRule))
	require.Equal(t, "r-9_x", RuleIDFromClash("(process_name=a || inbound=itg-rule-r-9_x) => route(direct)"))
	require.Equal(t, "final", RuleIDFromClash("final"))
	require.Equal(t, "final", RuleIDFromClash(" => route(proxy)"))
	require.Equal(t, "", RuleIDFromClash("ip_cidr=10.0.0.0/8 => route(direct)"))
}

func TestRegistrableDomain(t *testing.T) {
	require.Equal(t, "example.co.uk", RegistrableDomain("a.b.example.co.uk"))
	require.Equal(t, "selectel.run", RegistrableDomain("p77kwjkml.selectel.run"))
	require.Equal(t, "", RegistrableDomain("178.248.239.46"))
	require.Equal(t, "", RegistrableDomain(""))
}

func TestCollector_DeltasAndNewConns(t *testing.T) {
	f := newCollectorFixture(t)
	c := Conn{ID: "1", Host: "example.com", Process: "/usr/bin/firefox (itg)", Rule: markedRule, Chains: []string{"proxy-x"}, Up: 100, Down: 1000}
	f.tick(t, c)
	c.Up, c.Down = 150, 1600
	f.tick(t, c)
	f.tick(t, Conn{ID: "2", Process: "/usr/bin/steam", Rule: "final", Chains: []string{"direct"}, Up: 1, Down: 10})
	ov := f.overview(t)
	require.Equal(t, uint64(1610), ov.Down)
	require.Equal(t, uint64(1600), ov.DownProxy)
	require.Equal(t, uint64(10), ov.DownDirect)
	rules := map[string]RuleRow{}
	for _, r := range ov.Rules {
		rules[r.RuleID] = r
	}
	require.Equal(t, 1, rules["r123"].Conns)
	require.Equal(t, "x", rules["r123"].ServerID)
	require.Equal(t, 1, rules["final"].Conns)
}

func TestCollector_MainProxyUsesMainServer(t *testing.T) {
	f := newCollectorFixture(t)
	f.tick(t, Conn{ID: "1", Process: "/usr/bin/a", Rule: "final", Chains: []string{"proxy"}, Down: 5})
	ov := f.overview(t)
	require.Equal(t, "main", ov.Rules[0].ServerID)
	require.Equal(t, "proxy", ov.Rules[0].Route)
}

func TestCollector_CounterResetIsNewConnection(t *testing.T) {
	f := newCollectorFixture(t)
	f.tick(t, Conn{ID: "1", Process: "/usr/bin/a", Rule: "final", Chains: []string{"direct"}, Down: 500})
	f.tick(t, Conn{ID: "1", Process: "/usr/bin/a", Rule: "final", Chains: []string{"direct"}, Down: 20})
	ov := f.overview(t)
	require.Equal(t, uint64(520), ov.Down)
	require.Equal(t, 2, ov.Rules[0].Conns)
}

func TestCollector_SourceErrorIdlesQuietly(t *testing.T) {
	f := newCollectorFixture(t)
	f.src.err = errors.New("dial tcp 127.0.0.1:13356: connect: connection refused")
	f.c.Tick(context.Background())
	f.c.Tick(context.Background())
	require.Empty(t, f.c.Live())
	ov := f.overview(t)
	require.Zero(t, ov.Down)
	require.Empty(t, ov.Rules)
}

func blockLine(ts time.Time, id string) string {
	return ts.Format("-0700 2006-01-02 15:04:05") + " INFO [1234 3ms] outbound/block[block-" + id + "]: blocked connection to ads.example:443"
}

func TestCollector_BlockLinesCountedPerRule(t *testing.T) {
	f := newCollectorFixture(t)
	f.c.OnLogLine("sing-box", blockLine(f.now.Add(time.Second), "rads"))
	f.c.OnLogLine("sing-box", blockLine(f.now.Add(2*time.Second), "rads"))
	f.c.OnLogLine("xray", blockLine(f.now.Add(2*time.Second), "rads"))
	f.c.OnLogLine("sing-box", f.now.Add(time.Second).Format("-0700 2006-01-02 15:04:05")+" INFO outbound/block[block]: blocked connection to [::1]:443")
	ov := f.overview(t)
	require.Equal(t, 2, ov.Blocked)
	require.Equal(t, "rads", ov.Rules[0].RuleID)
	require.Equal(t, "block", ov.Rules[0].Route)
}

func TestCollector_IgnoresBlockLinesOlderThanStart(t *testing.T) {
	f := newCollectorFixture(t)
	f.c.OnLogLine("sing-box", blockLine(f.now.Add(-time.Hour), "rads"))
	f.c.OnLogLine("sing-box", "garbage without timestamp outbound/block[block-rads]: blocked connection to x")
	ov := f.overview(t)
	require.Zero(t, ov.Blocked)
}

func TestCollector_LiveHasHostButStoreDoesNot(t *testing.T) {
	f := newCollectorFixture(t)
	c := Conn{ID: "1", Host: "p77kwjkml.selectel.run", DestIP: "178.248.239.46", Process: "/usr/bin/firefox", Rule: markedRule, Chains: []string{"proxy-x"}, Down: 100}
	f.tick(t, c)
	c.Down = 300
	f.tick(t, c)
	live := f.c.Live()
	require.Len(t, live, 1)
	require.Equal(t, "p77kwjkml.selectel.run", live[0].Host)
	require.Equal(t, "selectel.run", live[0].Suffix)
	require.Equal(t, "firefox", live[0].App)
	require.Equal(t, "r123", live[0].RuleID)
	require.Equal(t, uint64(200), live[0].DownBps)
	require.NoError(t, f.c.Flush())
	var n int
	require.NoError(t, f.store.db.QueryRow("SELECT COUNT(*) FROM rollup WHERE app LIKE '%selectel%' OR rule_id LIKE '%selectel%' OR server_id LIKE '%178.%'").Scan(&n))
	require.Zero(t, n)
	f.tick(t)
	require.Empty(t, f.c.Live())
}

func TestCollector_ActiveSession(t *testing.T) {
	f := newCollectorFixture(t)
	start := f.now
	f.c.SessionStart(start)
	since, ok := f.c.ActiveSince()
	require.True(t, ok)
	require.True(t, since.Equal(start))
	f.c.SessionEnd(start.Add(10 * time.Minute))
	_, ok = f.c.ActiveSince()
	require.False(t, ok)
	ov := f.overview(t)
	require.Equal(t, int64(600), ov.OnlineSeconds)
}

func TestCollector_TransientErrorDoesNotDoubleCount(t *testing.T) {
	f := newCollectorFixture(t)
	c := Conn{ID: "1", Process: "/usr/bin/a", Rule: "final", Chains: []string{"direct"}, Down: 1000}
	f.tick(t, c)
	f.src.err = errors.New("context deadline exceeded")
	f.c.Tick(context.Background())
	f.src.err = nil
	c.Down = 1500
	f.tick(t, c)
	ov := f.overview(t)
	require.Equal(t, uint64(1500), ov.Down)
	require.Equal(t, 1, ov.Rules[0].Conns)
}

func (f *collectorFixture) tickTotals(t *testing.T, down, up uint64, conns ...Conn) {
	t.Helper()
	f.src.snap = Snapshot{Conns: conns, DownTotal: down, UpTotal: up, HasTotals: true}
	f.c.Tick(context.Background())
	f.now = f.now.Add(time.Second)
}

func TestCollector_ClosedAndShortConnectionsReachTotals(t *testing.T) {
	f := newCollectorFixture(t)
	a := Conn{ID: "a", Process: "/usr/bin/a", Rule: "final", Chains: []string{"proxy"}, Down: 1000, Up: 10}
	f.tickTotals(t, 1000, 10, a)
	f.tickTotals(t, 1800, 30)
	ov := f.overview(t)
	require.Equal(t, uint64(1800), ov.Down)
	require.Equal(t, uint64(30), ov.Up)
	require.Equal(t, uint64(1000), ov.DownProxy)
	require.Equal(t, "a", ov.Apps[0].App)
	require.Equal(t, uint64(1000), ov.Apps[0].Down)
}

func TestCollector_TotalsResetIsNewBaseline(t *testing.T) {
	f := newCollectorFixture(t)
	f.tickTotals(t, 5000, 500)
	f.tickTotals(t, 100, 10)
	f.tickTotals(t, 300, 20)
	ov := f.overview(t)
	require.Equal(t, uint64(200), ov.Down)
	require.Equal(t, uint64(10), ov.Up)
}

func TestCollector_ResidualNeverNegative(t *testing.T) {
	f := newCollectorFixture(t)
	f.tickTotals(t, 0, 0)
	f.tickTotals(t, 100, 0, Conn{ID: "x", Process: "/usr/bin/a", Rule: "final", Chains: []string{"direct"}, Down: 400})
	ov := f.overview(t)
	require.Equal(t, uint64(400), ov.Down)
}
