// Package refresh runs the periodic subscription-sync and latency-probe
// background loops while itgray-cli run holds the chain up.
package refresh

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/latency"
	"github.com/itg-team/itg-ray/internal/logging"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// Driver-internal default constants (from spec §5).
const (
	defaultSubInterval = 12 * time.Hour
	defaultProbeIntv   = 5 * time.Minute
	defaultProbeTO     = 5 * time.Second
	defaultProbeConc   = 16

	tickJitterPct     = 0.10             // ±10% on every tick
	firstSubJitterMax = 30 * time.Second // first-sub-tick stagger window
	firstProbeDelay   = 5 * time.Second  // probe waits this long before first run
	syncFetchTimeout  = 30 * time.Second
	lastStatusMaxLen  = 120
)

// SyncFn matches subscription.Sync. The driver uses a function-typed field
// (rather than calling subscription.Sync directly) so tests can inject a fake.
type SyncFn func(ctx context.Context, sub subscription.Subscription, existing []server.Server, timeout time.Duration) ([]server.Server, subscription.SyncMeta, error)

// ProbeFn matches latency.TCPConnect.
type ProbeFn func(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error)

// Config wires a Driver. All fields are optional except Subs and ServersPath.
type Config struct {
	Subs               subscription.Store
	ServersPath        string
	SyncFunc           SyncFn
	ProbeFunc          ProbeFn
	DefaultSubInterval time.Duration
	FirstSubJitterMax  time.Duration // override the default 30s startup-stagger window (mainly for tests)
	ProbeInterval      time.Duration
	ProbeTimeout       time.Duration
	ProbeConcurrency   int
	Now                func() time.Time
	Rand               *rand.Rand
	Log                *slog.Logger
	// OnSync, when set, is called with the subscription ID after each
	// sync attempt completes (success or recorded failure). The Electron
	// bridge uses it to publish hub events so the UI refreshes live.
	OnSync func(subID string)
}

// Driver owns the background goroutines.
type Driver struct {
	subs               subscription.Store
	serversPath        string
	syncFunc           SyncFn
	probeFunc          ProbeFn
	defaultSubInterval time.Duration
	firstSubJitterMax  time.Duration
	probeInterval      time.Duration
	probeTimeout       time.Duration
	probeConcurrency   int
	now                func() time.Time
	rand               *rand.Rand
	log                *slog.Logger
	onSync             func(subID string)

	serversMu sync.Mutex
	randMu    sync.Mutex
	wg        sync.WaitGroup
}

// NewDriver returns a Driver with defaults applied for any Config field
// left at its zero value.
func NewDriver(c Config) *Driver {
	d := &Driver{
		subs:               c.Subs,
		serversPath:        c.ServersPath,
		syncFunc:           c.SyncFunc,
		probeFunc:          c.ProbeFunc,
		defaultSubInterval: c.DefaultSubInterval,
		firstSubJitterMax:  c.FirstSubJitterMax,
		probeInterval:      c.ProbeInterval,
		probeTimeout:       c.ProbeTimeout,
		probeConcurrency:   c.ProbeConcurrency,
		now:                c.Now,
		rand:               c.Rand,
		log:                c.Log,
		onSync:             c.OnSync,
	}
	if d.syncFunc == nil {
		d.syncFunc = subscription.Sync
	}
	if d.probeFunc == nil {
		d.probeFunc = latency.TCPConnect
	}
	if d.defaultSubInterval == 0 {
		d.defaultSubInterval = defaultSubInterval
	}
	if d.firstSubJitterMax <= 0 {
		d.firstSubJitterMax = firstSubJitterMax
	}
	if d.probeInterval == 0 {
		d.probeInterval = defaultProbeIntv
	}
	if d.probeTimeout == 0 {
		d.probeTimeout = defaultProbeTO
	}
	if d.probeConcurrency == 0 {
		d.probeConcurrency = defaultProbeConc
	}
	if d.now == nil {
		d.now = time.Now
	}
	if d.rand == nil {
		d.rand = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // jitter, not crypto
	}
	if d.log == nil {
		d.log = slog.Default()
	}
	return d
}

// Run starts the per-sub goroutines and the probe ticker. It blocks until ctx
// is cancelled, then waits for in-flight ticks to finish before returning.
// Returns ctx.Err() (typically context.Canceled or DeadlineExceeded) on shutdown.
func (d *Driver) Run(ctx context.Context) error {
	subs, err := d.subs.Load()
	if err != nil {
		d.log.Error("refresh load subs failed",
			slog.String("scope", "refresh"),
			slog.String("err", logging.RedactError(err)),
		)
		// Still run probe loop — operator may add subs without restarting.
		subs = nil
	}
	d.log.Info("refresh started",
		slog.String("scope", "refresh"),
		slog.Int("subs", len(subs)),
	)
	for _, s := range subs {
		d.wg.Go(func() { d.runSub(ctx, s) })
	}

	d.log.Debug("refresh probe loop starting",
		slog.String("scope", "refresh"),
		slog.Duration("interval", d.probeInterval),
	)
	d.wg.Go(func() { d.runProbe(ctx) })

	<-ctx.Done()
	d.wg.Wait()
	return ctx.Err()
}

// jittered returns base * (1 ± pct * rand-uniform-in-[0,1)) using d.rand.
// Safe for concurrent use: per-sub goroutines and the probe goroutine all
// share d.rand, so access is serialized via d.randMu.
func (d *Driver) jittered(base time.Duration, pct float64) time.Duration {
	if base <= 0 {
		return 0
	}
	d.randMu.Lock()
	delta := (d.rand.Float64()*2 - 1) * pct
	d.randMu.Unlock()
	return time.Duration(float64(base) * (1 + delta))
}
