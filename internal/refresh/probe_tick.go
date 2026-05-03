package refresh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
)

// probeOnce probes every server in servers.json with bounded concurrency and
// applies the results via a snapshot-then-apply pattern: the server list is
// loaded under serversMu, the lock is released for the duration of the probe
// (which can take seconds), then re-acquired to merge results into whatever
// state the file is in NOW (a sync may have run in between).
func (d *Driver) probeOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("refresh.probe panic", "panic", r)
		}
	}()
	start := d.now()

	d.serversMu.Lock()
	snapshot, err := server.Load(d.serversPath)
	d.serversMu.Unlock()
	if err != nil {
		d.log.Error("refresh.probe.load", "err", err)
		return
	}
	if len(snapshot) == 0 {
		return
	}

	type result struct {
		ID       string
		LatencyP *int
	}
	results := make([]result, len(snapshot))
	sem := make(chan struct{}, d.probeConcurrency)
	var wg sync.WaitGroup
	for i := range snapshot {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					d.log.Error("refresh.probe.worker panic", "id", snapshot[i].ID, "panic", r)
				}
			}()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			addr := net.JoinHostPort(snapshot[i].Vless.Address, fmt.Sprintf("%d", snapshot[i].Vless.Port))
			dur, perr := d.probeFunc(ctx, addr, d.probeTimeout)
			if perr != nil {
				results[i] = result{ID: snapshot[i].ID, LatencyP: nil}
				return
			}
			ms := int(dur / time.Millisecond)
			results[i] = result{ID: snapshot[i].ID, LatencyP: &ms}
		})
	}
	wg.Wait()

	if ctx.Err() != nil {
		return
	}

	// Apply: re-load (the file may have changed during the probe), update only
	// IDs we still know about, save back.
	d.serversMu.Lock()
	defer d.serversMu.Unlock()
	current, err := server.Load(d.serversPath)
	if err != nil {
		d.log.Error("refresh.probe.reload", "err", err)
		return
	}
	byID := make(map[string]int, len(current))
	for i := range current {
		byID[current[i].ID] = i
	}
	var ok, failed int
	for _, r := range results {
		if r.ID == "" {
			continue
		}
		idx, found := byID[r.ID]
		if !found {
			continue // server was deleted between snapshot and apply
		}
		current[idx].LatencyMS = r.LatencyP
		if r.LatencyP == nil {
			failed++
		} else {
			ok++
		}
	}
	if err := server.Save(d.serversPath, current); err != nil {
		d.log.Error("refresh.probe.save", "err", err)
		return
	}
	d.log.Info("latency.probe",
		slog.Int("total", len(snapshot)),
		slog.Int("ok", ok),
		slog.Int("failed", failed),
		slog.Duration("duration", d.now().Sub(start)),
	)
}

// runProbe is the single probe scheduler. The first probe fires after
// firstProbeDelay (5s) so the first wave of syncOne calls has a chance to
// add new servers. Subsequent probes fire ProbeInterval ±10% after the
// previous probe completed.
func (d *Driver) runProbe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("refresh.runProbe panic", "panic", r)
		}
	}()

	timer := time.NewTimer(firstProbeDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			d.probeOnce(ctx)
			next := d.jittered(d.probeInterval, tickJitterPct)
			if next <= 0 {
				next = d.probeInterval
			}
			timer.Reset(next)
		}
	}
}
