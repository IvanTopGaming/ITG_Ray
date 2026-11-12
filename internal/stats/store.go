package stats

import (
	"database/sql"
	"math"
	"sort"
	"time"
)

type Dims struct {
	App      string
	RuleID   string
	Route    string
	ServerID string
}

type Delta struct {
	Up    uint64
	Down  uint64
	Conns int
}

type Period string

const (
	PeriodToday Period = "today"
	Period7d    Period = "7d"
	Period30d   Period = "30d"
)

func ValidPeriod(p Period) bool {
	return p == PeriodToday || p == Period7d || p == Period30d
}

type Point struct {
	Start      time.Time
	Down, Up   uint64
	DownProxy  uint64
	DownDirect uint64
	UpProxy    uint64
	UpDirect   uint64
}

type AppRow struct {
	App       string
	Down, Up  uint64
	DownProxy uint64
	UpProxy   uint64
}

type RuleRow struct {
	RuleID   string
	Route    string
	ServerID string
	Conns    int
}

type Overview struct {
	Down, Up      uint64
	DownProxy     uint64
	DownDirect    uint64
	UpProxy       uint64
	UpDirect      uint64
	Blocked       int
	OnlineSeconds int64
	Sessions      int
	Series        []Point
	Apps          []AppRow
	Rules         []RuleRow
}

const topLimit = 10

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func toInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func toUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func hourKey(t time.Time) int64 { return t.UTC().Truncate(time.Hour).Unix() }

func (s *Store) Add(at time.Time, d Dims, delta Delta) error {
	_, err := s.db.Exec(`INSERT INTO rollup(hour_utc, app, rule_id, route, server_id, bytes_up, bytes_down, conns)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hour_utc, app, rule_id, route, server_id) DO UPDATE SET
			bytes_up = bytes_up + excluded.bytes_up,
			bytes_down = bytes_down + excluded.bytes_down,
			conns = conns + excluded.conns`,
		hourKey(at), d.App, d.RuleID, d.Route, d.ServerID, toInt64(delta.Up), toInt64(delta.Down), delta.Conns)
	return err
}

func (s *Store) AddSession(start, end time.Time) error {
	if !end.After(start) {
		return nil
	}
	_, err := s.db.Exec("INSERT INTO sessions(start_utc, end_utc) VALUES(?, ?)", start.Unix(), end.Unix())
	return err
}

func (s *Store) Prune(now time.Time, keep time.Duration) error {
	cut := now.Add(-keep)
	if _, err := s.db.Exec("DELETE FROM rollup WHERE hour_utc < ?", hourKey(cut)); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM sessions WHERE end_utc < ?", cut.Unix())
	return err
}

func (s *Store) Clear() error {
	if _, err := s.db.Exec("DELETE FROM rollup"); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM sessions")
	return err
}

func periodBuckets(p Period, now time.Time, loc *time.Location) (starts []time.Time, end time.Time) {
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch p {
	case Period7d, Period30d:
		days := 7
		if p == Period30d {
			days = 30
		}
		first := midnight.AddDate(0, 0, -(days - 1))
		for i := 0; i < days; i++ {
			starts = append(starts, first.AddDate(0, 0, i))
		}
		return starts, midnight.AddDate(0, 0, 1)
	default:
		for i := 0; i < 24; i++ {
			starts = append(starts, midnight.Add(time.Duration(i)*time.Hour))
		}
		return starts, midnight.AddDate(0, 0, 1)
	}
}

func bucketIndex(starts []time.Time, t time.Time) int {
	i := sort.Search(len(starts), func(i int) bool { return starts[i].After(t) }) - 1
	return i
}

func (s *Store) Overview(p Period, now time.Time, loc *time.Location) (Overview, error) {
	starts, end := periodBuckets(p, now, loc)
	begin := starts[0]
	ov := Overview{Series: make([]Point, len(starts))}
	for i, st := range starts {
		ov.Series[i].Start = st
	}
	rows, err := s.db.Query(`SELECT hour_utc, app, rule_id, route, server_id, bytes_up, bytes_down, conns
		FROM rollup WHERE hour_utc >= ? AND hour_utc < ?`, hourKey(begin), end.Unix())
	if err != nil {
		return ov, err
	}
	apps := map[string]*AppRow{}
	type ruleKey struct{ id, route, server string }
	ruleConns := map[ruleKey]int{}
	for rows.Next() {
		var hour, up, down int64
		var conns int
		var d Dims
		if err := rows.Scan(&hour, &d.App, &d.RuleID, &d.Route, &d.ServerID, &up, &down, &conns); err != nil {
			_ = rows.Close()
			return ov, err
		}
		t := time.Unix(hour, 0).In(loc)
		idx := bucketIndex(starts, t)
		if idx < 0 || t.Before(begin) {
			continue
		}
		u, dn := toUint64(up), toUint64(down)
		pt := &ov.Series[idx]
		pt.Up += u
		pt.Down += dn
		ov.Up += u
		ov.Down += dn
		switch d.Route {
		case "proxy":
			pt.UpProxy += u
			pt.DownProxy += dn
			ov.UpProxy += u
			ov.DownProxy += dn
		case "direct":
			pt.UpDirect += u
			pt.DownDirect += dn
			ov.UpDirect += u
			ov.DownDirect += dn
		case "block":
			ov.Blocked += conns
		}
		if d.App != "" && d.Route != "block" {
			a := apps[d.App]
			if a == nil {
				a = &AppRow{App: d.App}
				apps[d.App] = a
			}
			a.Up += u
			a.Down += dn
			if d.Route == "proxy" {
				a.UpProxy += u
				a.DownProxy += dn
			}
		}
		if d.RuleID != "" && conns > 0 {
			ruleConns[ruleKey{d.RuleID, d.Route, d.ServerID}] += conns
		}
	}
	if err := rows.Close(); err != nil {
		return ov, err
	}
	for _, a := range apps {
		ov.Apps = append(ov.Apps, *a)
	}
	sort.Slice(ov.Apps, func(i, j int) bool {
		ti, tj := ov.Apps[i].Down+ov.Apps[i].Up, ov.Apps[j].Down+ov.Apps[j].Up
		if ti != tj {
			return ti > tj
		}
		return ov.Apps[i].App < ov.Apps[j].App
	})
	if len(ov.Apps) > topLimit {
		ov.Apps = ov.Apps[:topLimit]
	}
	for k, n := range ruleConns {
		ov.Rules = append(ov.Rules, RuleRow{RuleID: k.id, Route: k.route, ServerID: k.server, Conns: n})
	}
	sort.Slice(ov.Rules, func(i, j int) bool {
		if ov.Rules[i].Conns != ov.Rules[j].Conns {
			return ov.Rules[i].Conns > ov.Rules[j].Conns
		}
		return ov.Rules[i].RuleID < ov.Rules[j].RuleID
	})
	if len(ov.Rules) > topLimit {
		ov.Rules = ov.Rules[:topLimit]
	}
	sessRows, err := s.db.Query("SELECT start_utc, end_utc FROM sessions WHERE end_utc > ? AND start_utc < ?", begin.Unix(), end.Unix())
	if err != nil {
		return ov, err
	}
	for sessRows.Next() {
		var a, b int64
		if err := sessRows.Scan(&a, &b); err != nil {
			_ = sessRows.Close()
			return ov, err
		}
		if a < begin.Unix() {
			a = begin.Unix()
		}
		if b > end.Unix() {
			b = end.Unix()
		}
		ov.OnlineSeconds += b - a
		ov.Sessions++
	}
	return ov, sessRows.Close()
}
