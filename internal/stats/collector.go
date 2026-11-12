package stats

import (
	"context"
	"net"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

type Conn struct {
	ID      string
	Host    string
	DestIP  string
	Process string
	Rule    string
	Chains  []string
	Up      uint64
	Down    uint64
	Start   time.Time
	Network string
}

type Snapshot struct {
	Conns []Conn
}

type ConnSource interface {
	Connections(ctx context.Context) (Snapshot, error)
}

type LiveConn struct {
	ID       string
	App      string
	Host     string
	Suffix   string
	DestIP   string
	RuleID   string
	Route    string
	ServerID string
	Network  string
	DownBps  uint64
	UpBps    uint64
	Start    time.Time
}

type connState struct {
	up, down uint64
}

type pendingKey struct {
	hour int64
	dims Dims
}

type Collector struct {
	src        ConnSource
	store      *Store
	mainServer func() string
	now        func() time.Time
	started    time.Time

	mu           sync.Mutex
	prev         map[string]connState
	lastTick     time.Time
	pending      map[pendingKey]Delta
	live         []LiveConn
	sessionStart time.Time
	sessionOpen  bool
}

func NewCollector(src ConnSource, store *Store, mainServer func() string, now func() time.Time) *Collector {
	return &Collector{
		src:        src,
		store:      store,
		mainServer: mainServer,
		now:        now,
		started:    now(),
		prev:       map[string]connState{},
		pending:    map[pendingKey]Delta{},
	}
}

var (
	markerRe   = regexp.MustCompile(`inbound=itg-rule-([^\s)\]]+)`)
	blockRe    = regexp.MustCompile(`outbound/block\[block-([^\]]+)\]: blocked connection`)
	logStampRe = regexp.MustCompile(`([+-]\d{4}) (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
	procUserRe = regexp.MustCompile(`\s+\([^)]*\)$`)
)

func AppName(processPath string) string {
	p := strings.TrimSpace(procUserRe.ReplaceAllString(processPath, ""))
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, `\`, "/")
	base := path.Base(p)
	if strings.HasSuffix(strings.ToLower(base), ".exe") {
		base = base[:len(base)-4]
	}
	return base
}

func RuleIDFromClash(rule string) string {
	if m := markerRe.FindStringSubmatch(rule); m != nil {
		return m[1]
	}
	if rule == "final" || strings.HasPrefix(rule, " => ") {
		return "final"
	}
	return ""
}

func RegistrableDomain(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}
	d, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}
	return d
}

func (c *Collector) route(chains []string) (route, server string) {
	if len(chains) == 0 {
		return "", ""
	}
	switch tag := chains[0]; {
	case tag == "direct":
		return "direct", ""
	case tag == "proxy":
		return "proxy", c.mainServer()
	case strings.HasPrefix(tag, "proxy-"):
		return "proxy", strings.TrimPrefix(tag, "proxy-")
	default:
		return "", ""
	}
}

func (c *Collector) addPending(at time.Time, d Dims, delta Delta) {
	k := pendingKey{hour: hourKey(at), dims: d}
	cur := c.pending[k]
	cur.Up += delta.Up
	cur.Down += delta.Down
	cur.Conns += delta.Conns
	c.pending[k] = cur
}

func (c *Collector) Tick(ctx context.Context) {
	snap, err := c.src.Connections(ctx)
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.prev = map[string]connState{}
		c.live = nil
		c.lastTick = time.Time{}
		return
	}
	elapsed := now.Sub(c.lastTick).Seconds()
	if c.lastTick.IsZero() || elapsed <= 0 {
		elapsed = 1
	}
	next := make(map[string]connState, len(snap.Conns))
	live := make([]LiveConn, 0, len(snap.Conns))
	for _, conn := range snap.Conns {
		route, server := c.route(conn.Chains)
		app := AppName(conn.Process)
		ruleID := RuleIDFromClash(conn.Rule)
		var d Delta
		if p, ok := c.prev[conn.ID]; ok && conn.Up >= p.up && conn.Down >= p.down {
			d = Delta{Up: conn.Up - p.up, Down: conn.Down - p.down}
		} else {
			d = Delta{Up: conn.Up, Down: conn.Down, Conns: 1}
		}
		next[conn.ID] = connState{up: conn.Up, down: conn.Down}
		if route != "" && (d.Up > 0 || d.Down > 0 || d.Conns > 0) {
			c.addPending(now, Dims{App: app, RuleID: ruleID, Route: route, ServerID: server}, d)
		}
		host := conn.Host
		if host == "" {
			host = conn.DestIP
		}
		live = append(live, LiveConn{
			ID:       conn.ID,
			App:      app,
			Host:     host,
			Suffix:   RegistrableDomain(conn.Host),
			DestIP:   conn.DestIP,
			RuleID:   ruleID,
			Route:    route,
			ServerID: server,
			Network:  conn.Network,
			DownBps:  uint64(float64(d.Down) / elapsed),
			UpBps:    uint64(float64(d.Up) / elapsed),
			Start:    conn.Start,
		})
	}
	sort.Slice(live, func(i, j int) bool {
		bi, bj := live[i].DownBps+live[i].UpBps, live[j].DownBps+live[j].UpBps
		if bi != bj {
			return bi > bj
		}
		return live[i].Start.After(live[j].Start)
	})
	c.prev = next
	c.live = live
	c.lastTick = now
}

func (c *Collector) OnLogLine(source, message string) {
	if source != "sing-box" {
		return
	}
	m := blockRe.FindStringSubmatch(message)
	if m == nil {
		return
	}
	ts := logStampRe.FindStringSubmatch(message)
	if ts == nil {
		return
	}
	at, err := time.Parse("-0700 2006-01-02 15:04:05", ts[1]+" "+ts[2])
	if err != nil || at.Before(c.started.Truncate(time.Second)) {
		return
	}
	c.mu.Lock()
	c.addPending(at, Dims{RuleID: m[1], Route: "block"}, Delta{Conns: 1})
	c.mu.Unlock()
}

func (c *Collector) Live() []LiveConn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]LiveConn(nil), c.live...)
}

func (c *Collector) Flush() error {
	c.mu.Lock()
	pending := c.pending
	c.pending = map[pendingKey]Delta{}
	c.mu.Unlock()
	for k, d := range pending {
		if err := c.store.Add(time.Unix(k.hour, 0), k.dims, d); err != nil {
			c.mu.Lock()
			for k2, d2 := range pending {
				cur := c.pending[k2]
				cur.Up += d2.Up
				cur.Down += d2.Down
				cur.Conns += d2.Conns
				c.pending[k2] = cur
			}
			c.mu.Unlock()
			return err
		}
		delete(pending, k)
	}
	return nil
}

func (c *Collector) SessionStart(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessionOpen {
		return
	}
	c.sessionStart = t
	c.sessionOpen = true
}

func (c *Collector) SessionEnd(t time.Time) {
	c.mu.Lock()
	start, open := c.sessionStart, c.sessionOpen
	c.sessionOpen = false
	c.mu.Unlock()
	if open {
		_ = c.store.AddSession(start, t)
	}
}

func (c *Collector) ActiveSince() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionStart, c.sessionOpen
}
