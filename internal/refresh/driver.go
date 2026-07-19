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

	// maxInAttemptRetryWait caps how long a single scheduled sync will honor a
	// server Retry-After hint before giving up the in-attempt retry and letting
	// the scheduler back off instead. Keeps one sync from blocking for minutes.
	maxInAttemptRetryWait = 30 * time.Second
)

// defaultSubFetchRetryBackoff is the level-1 (in-attempt) retry schedule: on a
// transient sync failure the scheduled loop re-fetches after these waits before
// giving up the attempt. Length = max retries.
var defaultSubFetchRetryBackoff = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// defaultSubRetryBackoff is the level-2 (scheduler) backoff: after a transient
// failure the next tick fires after these growing delays instead of the full
// update interval, capped at the interval. Reset to the interval on success or
// on a permanent (non-transient) failure.
var defaultSubRetryBackoff = []time.Duration{1 * time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute}

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
	// SubFetchRetryBackoff overrides the level-1 in-attempt retry schedule. A
	// nil slice applies the package default; a non-nil empty slice disables
	// in-attempt retries (used by tests to keep them fast).
	SubFetchRetryBackoff []time.Duration
	// SubRetryBackoff overrides the level-2 scheduler backoff schedule. A nil
	// slice applies the package default; a non-nil empty slice disables
	// backoff (next tick always at the full interval).
	SubRetryBackoff  []time.Duration
	ProbeInterval    time.Duration
	ProbeTimeout     time.Duration
	ProbeConcurrency int
	Now              func() time.Time
	Rand             *rand.Rand
	Log              *slog.Logger
	// OnSync, when set, is called with the subscription ID after each
	// sync attempt completes (success or recorded failure). The Electron
	// bridge uses it to publish hub events so the UI refreshes live.
	OnSync func(subID string)
}

// Driver owns the background goroutines.
type Driver struct {
	subs                 subscription.Store
	serversPath          string
	syncFunc             SyncFn
	probeFunc            ProbeFn
	defaultSubInterval   time.Duration
	firstSubJitterMax    time.Duration
	subFetchRetryBackoff []time.Duration
	subRetryBackoff      []time.Duration
	probeInterval        time.Duration
	probeTimeout         time.Duration
	probeConcurrency     int
	now                  func() time.Time
	rand                 *rand.Rand
	log                  *slog.Logger
	onSync               func(subID string)

	serversMu sync.Mutex
	randMu    sync.Mutex
	wg        sync.WaitGroup
}

// NewDriver returns a Driver with defaults applied for any Config field
// left at its zero value.
func NewDriver(c Config) *Driver {
	d := &Driver{
		subs:                 c.Subs,
		serversPath:          c.ServersPath,
		syncFunc:             c.SyncFunc,
		probeFunc:            c.ProbeFunc,
		defaultSubInterval:   c.DefaultSubInterval,
		firstSubJitterMax:    c.FirstSubJitterMax,
		subFetchRetryBackoff: c.SubFetchRetryBackoff,
		subRetryBackoff:      c.SubRetryBackoff,
		probeInterval:        c.ProbeInterval,
		probeTimeout:         c.ProbeTimeout,
		probeConcurrency:     c.ProbeConcurrency,
		now:                  c.Now,
		rand:                 c.Rand,
		log:                  c.Log,
		onSync:               c.OnSync,
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
	if d.subFetchRetryBackoff == nil {
		d.subFetchRetryBackoff = defaultSubFetchRetryBackoff
	}
	if d.subRetryBackoff == nil {
		d.subRetryBackoff = defaultSubRetryBackoff
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
