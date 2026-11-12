package stats

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "stats.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

var msk = time.FixedZone("MSK", 3*3600)

func TestStore_AddAccumulates(t *testing.T) {
	s := openTestStore(t)
	h := time.Date(2026, 11, 12, 10, 0, 0, 0, msk)
	d := Dims{App: "firefox", RuleID: "r1", Route: "proxy", ServerID: "s1"}
	require.NoError(t, s.Add(h.Add(5*time.Minute), d, Delta{Up: 10, Down: 100, Conns: 1}))
	require.NoError(t, s.Add(h.Add(40*time.Minute), d, Delta{Up: 5, Down: 50}))
	ov, err := s.Overview(PeriodToday, h.Add(50*time.Minute), msk)
	require.NoError(t, err)
	require.Equal(t, uint64(150), ov.Down)
	require.Equal(t, uint64(15), ov.Up)
	require.Len(t, ov.Rules, 1)
	require.Equal(t, 1, ov.Rules[0].Conns)
}

func TestStore_OverviewToday(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, 11, 12, 15, 30, 0, 0, msk)
	require.NoError(t, s.Add(time.Date(2026, 11, 12, 9, 10, 0, 0, msk), Dims{App: "firefox", RuleID: "r1", Route: "proxy"}, Delta{Up: 1, Down: 10, Conns: 1}))
	require.NoError(t, s.Add(time.Date(2026, 11, 12, 9, 20, 0, 0, msk), Dims{App: "steam", RuleID: "final", Route: "direct"}, Delta{Up: 2, Down: 20, Conns: 2}))
	require.NoError(t, s.Add(time.Date(2026, 11, 11, 23, 0, 0, 0, msk), Dims{App: "old", RuleID: "r1", Route: "proxy"}, Delta{Up: 99, Down: 99, Conns: 1}))
	require.NoError(t, s.Add(time.Date(2026, 11, 12, 11, 0, 0, 0, msk), Dims{RuleID: "rb", Route: "block"}, Delta{Conns: 3}))

	ov, err := s.Overview(PeriodToday, now, msk)
	require.NoError(t, err)
	require.Len(t, ov.Series, 24)
	require.True(t, ov.Series[0].Start.Equal(time.Date(2026, 11, 12, 0, 0, 0, 0, msk)))
	require.Equal(t, uint64(10), ov.Series[9].DownProxy)
	require.Equal(t, uint64(20), ov.Series[9].DownDirect)
	require.Equal(t, uint64(30), ov.Down)
	require.Equal(t, uint64(10), ov.DownProxy)
	require.Equal(t, uint64(20), ov.DownDirect)
	require.Equal(t, uint64(1), ov.UpProxy)
	require.Equal(t, 3, ov.Blocked)
	require.Equal(t, "steam", ov.Apps[0].App)
	require.Equal(t, uint64(20), ov.Apps[0].Down)
	require.Equal(t, uint64(0), ov.Apps[0].DownProxy)
	require.Equal(t, "rb", ov.Rules[0].RuleID)
	require.Equal(t, "block", ov.Rules[0].Route)
}

func TestStore_Overview7dDailyBuckets(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, 11, 12, 1, 0, 0, 0, msk)
	require.NoError(t, s.Add(time.Date(2026, 11, 11, 23, 30, 0, 0, msk), Dims{App: "a", RuleID: "final", Route: "proxy"}, Delta{Down: 7}))
	require.NoError(t, s.Add(time.Date(2026, 11, 12, 0, 30, 0, 0, msk), Dims{App: "a", RuleID: "final", Route: "proxy"}, Delta{Down: 3}))
	require.NoError(t, s.Add(time.Date(2026, 11, 1, 12, 0, 0, 0, msk), Dims{App: "a", RuleID: "final", Route: "proxy"}, Delta{Down: 1000}))
	ov, err := s.Overview(Period7d, now, msk)
	require.NoError(t, err)
	require.Len(t, ov.Series, 7)
	last := ov.Series[6]
	require.True(t, last.Start.Equal(time.Date(2026, 11, 12, 0, 0, 0, 0, msk)))
	require.Equal(t, uint64(3), last.Down)
	require.Equal(t, uint64(7), ov.Series[5].Down)
	require.Equal(t, uint64(10), ov.Down)
}

func TestStore_SessionsOnlineSeconds(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, msk)
	require.NoError(t, s.AddSession(time.Date(2026, 11, 11, 23, 0, 0, 0, msk), time.Date(2026, 11, 12, 1, 0, 0, 0, msk)))
	require.NoError(t, s.AddSession(time.Date(2026, 11, 12, 10, 0, 0, 0, msk), time.Date(2026, 11, 12, 10, 30, 0, 0, msk)))
	ov, err := s.Overview(PeriodToday, now, msk)
	require.NoError(t, err)
	require.Equal(t, int64(3600+1800), ov.OnlineSeconds)
	require.Equal(t, 2, ov.Sessions)
}

func TestStore_PruneKeeps30Days(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, msk)
	d := Dims{App: "a", RuleID: "final", Route: "proxy"}
	require.NoError(t, s.Add(now.Add(-31*24*time.Hour), d, Delta{Down: 1}))
	require.NoError(t, s.Add(now.Add(-29*24*time.Hour), d, Delta{Down: 2}))
	require.NoError(t, s.AddSession(now.Add(-40*24*time.Hour), now.Add(-40*24*time.Hour+time.Hour)))
	require.NoError(t, s.Prune(now, 30*24*time.Hour))
	ov, err := s.Overview(Period30d, now, msk)
	require.NoError(t, err)
	require.Equal(t, uint64(2), ov.Down)
	var n int
	require.NoError(t, s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n))
	require.Equal(t, 0, n)
}

func TestStore_Clear(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, msk)
	require.NoError(t, s.Add(now, Dims{App: "a", RuleID: "final", Route: "proxy"}, Delta{Down: 1}))
	require.NoError(t, s.AddSession(now.Add(-time.Hour), now))
	require.NoError(t, s.Clear())
	ov, err := s.Overview(PeriodToday, now, msk)
	require.NoError(t, err)
	require.Zero(t, ov.Down)
	require.Zero(t, ov.Sessions)
}

func TestStore_SchemaHasNoHostColumns(t *testing.T) {
	s := openTestStore(t)
	for _, table := range []string{"rollup", "sessions"} {
		rows, err := s.db.Query("SELECT name FROM pragma_table_info(?)", table)
		require.NoError(t, err)
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			for _, bad := range []string{"host", "domain", "dst", "ip", "addr"} {
				require.False(t, strings.Contains(strings.ToLower(name), bad), "%s.%s", table, name)
			}
		}
		require.NoError(t, rows.Close())
	}
}

func TestStore_OpenMigratesV1(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	db, err := Open(p)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS connections (id INTEGER, dst_domain TEXT)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	s, err := OpenStore(p)
	require.NoError(t, err)
	defer s.Close()
	var n int
	require.NoError(t, s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name IN ('connections','hourly_rollup')").Scan(&n))
	require.Equal(t, 0, n)
}
