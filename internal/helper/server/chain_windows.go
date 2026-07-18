//go:build windows

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/helper/adapter"
	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/gateway"
	"github.com/itg-team/itg-ray/internal/helper/route"
	"github.com/itg-team/itg-ray/internal/helper/runtime"
	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/itg-team/itg-ray/internal/helper/undo"
	"github.com/itg-team/itg-ray/internal/helper/xrayapi"
	"github.com/itg-team/itg-ray/internal/logging"
)

// coreStopper is the narrow surface stopBoth needs from a supervised
// child process. *supervisor.Child satisfies it.
type coreStopper interface {
	Stop(grace time.Duration) error
}

// stopBoth runs Stop(grace) on both cores concurrently. nil arguments
// are skipped. Returns (xrayErr, sbErr) — either may be nil. Worst-case
// wall time is grace, not 2*grace.
func stopBoth(grace time.Duration, xray, singbox coreStopper) (xrayErr, sbErr error) {
	var wg sync.WaitGroup
	if xray != nil {
		wg.Go(func() { xrayErr = xray.Stop(grace) })
	}
	if singbox != nil {
		wg.Go(func() { sbErr = singbox.Stop(grace) })
	}
	wg.Wait()
	return
}

// asStopper returns a coreStopper for the given child, or nil if the
// child itself is nil. Direct conversion `coreStopper(child)` would wrap
// a nil pointer in a non-nil interface — stopBoth's nil-check would miss.
func asStopper(c *supervisor.Child) coreStopper {
	if c == nil {
		return nil
	}
	return c
}

// chainState tracks the active session inside the helper process.
type chainState struct {
	sessionID string
	singbox   *supervisor.Child
	xray      *supervisor.Child
	tunLUID   uint64
	peerRoute route.Entry // /32 host route we added for the VLESS server
	snapshot  []route.Entry
	dnsPrior  *dns.Settings // nil if no DNS override applied
	dnsAlias  string
	// nrptName is the DisplayName of the NRPT rule we installed to force
	// all DNS through the TUN's hijack-dns route engine. Empty means no
	// rule installed (sysproxy mode, or AddNrptRule failed).
	nrptName string

	// xrayAPI is the gRPC client to xray-core's StatsService. Created in
	// NewStartChainHandler after xray spawns successfully; closed in
	// stopActiveChainLocked. nil before first StartChain.
	xrayAPI *xrayapi.Client
	// cachedUp/cachedDown are the last successful counter readings; used
	// by OpServiceStatus when a transient gRPC error happens.
	cachedUp   uint64
	cachedDown uint64
}

var (
	chainMu    sync.Mutex
	activeSess *chainState
)

// IsChainActive reports whether the helper has a running chain session.
// Safe for concurrent callers — used by OpServiceStatus to surface chain
// liveness to the user-level bridge for Reconcile-time adoption.
func IsChainActive() bool {
	chainMu.Lock()
	defer chainMu.Unlock()
	return activeSess != nil
}

// binaryPath looks up an adjacent binary by name. Two layouts are
// supported because both Wails (per-machine) and Electron NSIS
// (per-user) builds ship today:
//
//   - Wails: helper.exe + sing-box.exe + xray.exe siblings in
//     C:\Program Files\ITG Ray\
//   - Electron NSIS: helper.exe at resources/helper/, cores at
//     resources/cores/ per BUNDLE_LAYOUT in src/main/paths.ts
//
// Try the sibling layout first (matches existing Wails deployments),
// then fall back to ../cores/ for the Electron bundle.
func binaryPath(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if sibling := filepath.Join(dir, name); fileExists(sibling) {
		return sibling, nil
	}
	if bundled := filepath.Join(dir, "..", "cores", name); fileExists(bundled) {
		return bundled, nil
	}
	// Fall back to the sibling path so the spawn error surfaces a
	// recognisable location instead of an arbitrary %PATH% lookup.
	return filepath.Join(dir, name), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// newSessionID returns a 16-char hex session id from 8 random bytes.
// crypto/rand.Read only fails on a broken entropy source, so the error
// is intentionally discarded.
func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// NewStartChainHandler is the OpStartChain handler. It atomically
// captures restore state, applies the peer-route, spawns the cores, and
// discovers the new TUN adapter. Sing-box's auto_route owns the catch-all
// routes and DNS hijack natively. Any mid-flow failure rolls back.
//
//nolint:gocyclo,gocognit // orchestration sequence requires linear control flow so the rollback-flag pattern stays obvious
func NewStartChainHandler() Handler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a StartChainArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if a.ServerHost == "" || a.ServerPort == 0 {
			return nil, errors.New("server_host, server_port required")
		}
		var sysproxyMode bool
		switch a.Mode {
		case "", "tun":
			if a.TunName == "" {
				return nil, errors.New("tun_name required for tun mode")
			}
		case "sysproxy":
			sysproxyMode = true
		default:
			return nil, errors.New("invalid mode")
		}

		chainMu.Lock()
		defer chainMu.Unlock()
		if activeSess != nil {
			return nil, fmt.Errorf("chain already running (session=%s)", activeSess.sessionID)
		}

		slog.Info("chain start", slog.String("scope", "helper"), slog.String("mode", a.Mode))

		state := &chainState{
			sessionID: newSessionID(),
			dnsAlias:  a.DnsAlias,
		}

		// rollback runs in reverse order of operations performed; only the
		// operations whose `done` flag is set actually reverse.
		var (
			doneRuntime, doneRouteSnap, donePeerRoute, doneDnsSnap, doneSingbox, doneXray, doneNrpt bool
		)
		rollback := func() {
			if doneXray && state.xray != nil {
				_ = state.xray.Stop(2 * time.Second)
			}
			if doneSingbox && state.singbox != nil {
				_ = state.singbox.Stop(2 * time.Second)
			}
			if doneNrpt && state.nrptName != "" {
				if err := dns.RemoveNrptRule(state.nrptName); err != nil {
					slog.Warn("chain teardown: remove nrpt rule failed", slog.String("scope", "helper"),
						slog.String("session", state.sessionID), slog.String("rule", state.nrptName),
						slog.String("err", logging.RedactError(err)))
				}
			}
			if doneDnsSnap && state.dnsPrior != nil {
				if err := dns.Restore(*state.dnsPrior); err != nil {
					slog.Warn("chain teardown: dns restore failed", slog.String("scope", "helper"),
						slog.String("session", state.sessionID), slog.String("interface", state.dnsPrior.InterfaceAlias),
						slog.String("err", logging.RedactError(err)))
				}
			}
			if donePeerRoute {
				_ = route.Remove(state.peerRoute)
			}
			_ = doneRouteSnap // snapshot is read-only; nothing to roll back
			if doneRuntime {
				_ = os.RemoveAll(runtime.BasePath())
			}
		}

		// Step 1: prepare runtime dir.
		if err := runtime.EnsureClean(); err != nil {
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "runtime-ensure-clean"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("runtime.EnsureClean: %w", err)
		}
		doneRuntime = true

		// Step 2: persist configs.
		sbPath := runtime.ConfigPath("sing-box.json")
		if err := os.WriteFile(sbPath, a.SingboxConfig, 0o640); err != nil { //nolint:gosec // %ProgramData%, admin-only
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "write-sing-box-config"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("write sing-box config: %w", err)
		}
		xrPath := runtime.ConfigPath("xray.json")
		if err := os.WriteFile(xrPath, a.XrayConfig, 0o640); err != nil { //nolint:gosec // %ProgramData%, admin-only
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "write-xray-config"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("write xray config: %w", err)
		}

		// Step 3: snapshot routes for restore.
		if !sysproxyMode {
			snap, err := route.Snapshot()
			if err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "route-snapshot"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("route.Snapshot: %w", err)
			}
			state.snapshot = snap
			doneRouteSnap = true
		}

		// Step 4: peer-route via current default gateway.
		if !sysproxyMode {
			gw, err := gateway.Default()
			if err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "gateway-default"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("gateway.Default: %w", err)
			}

			// Resolve ServerHost to an IPv4 literal — peer-route's CIDR must be parseable
			// by netip.ParsePrefix. Resolution uses the host's DNS (the same one sing-box
			// will inherit on spawn) and runs BEFORE sing-box spawns, so it goes through
			// the original network path rather than sing-box's auto_route + DNS hijack.
			// Retry with backoff: on a reconnect the previous chain's DNS hijack/NRPT was
			// just torn down, so the first lookup can transiently return "no such host"
			// until the system resolver settles back to the physical adapter.
			serverV4, err := resolveServerIPv4(net.LookupIP, a.ServerHost, 6, 500*time.Millisecond)
			if err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "resolve-server-ipv4"), slog.String("err", logging.RedactError(err)))
				return nil, err
			}

			state.peerRoute = route.Entry{
				DestCIDR:      serverV4.String() + "/32",
				NextHop:       gw.NextHop,
				InterfaceLUID: gw.InterfaceLUID,
				Metric:        0,
			}
			if err := route.Add(state.peerRoute); err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "route-add-peer"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("route.Add(peer): %w", err)
			}
			donePeerRoute = true
		}

		// Step 5: optional DNS override (TUN mode only — sysproxy doesn't touch DNS).
		if !sysproxyMode && a.DnsAlias != "" && len(a.DnsServers) > 0 {
			prior, err := dns.Snapshot(a.DnsAlias)
			if err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "dns-snapshot"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("dns.Snapshot: %w", err)
			}
			if err := dns.Set(dns.Settings{InterfaceAlias: a.DnsAlias, Addresses: a.DnsServers}); err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "dns-set"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("dns.Set: %w", err)
			}
			state.dnsPrior = &prior
			doneDnsSnap = true
		}

		// Step 6: snapshot adapters before spawning sing-box (TUN mode only).
		var before []adapter.Adapter
		if !sysproxyMode {
			var err error
			before, err = adapter.Snapshot()
			if err != nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "adapter-snapshot"), slog.String("err", logging.RedactError(err)))
				return nil, fmt.Errorf("adapter.Snapshot(before): %w", err)
			}
		}

		// Step 7: spawn sing-box.
		sbExe, err := binaryPath("sing-box.exe")
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "binary-path-sing-box"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("binary path: %w", err)
		}
		sbLog := runtime.LogPath("sing-box.log")
		if err := runtime.RotateLog(sbLog); err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "rotate-sing-box-log"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("rotate sing-box log: %w", err)
		}
		state.singbox, err = supervisor.Spawn("sing-box", sbExe,
			[]string{"run", "-c", sbPath},
			sbLog)
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "spawn-sing-box"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("spawn sing-box: %w", err)
		}
		doneSingbox = true

		// Step 8: discover the new adapter (poll up to 10s). TUN mode only.
		if !sysproxyMode {
			var tunAdapter *adapter.Adapter
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					slog.Error("chain start failed", slog.String("scope", "helper"),
						slog.String("stage", "adapter-discovery"), slog.String("err", logging.RedactError(ctx.Err())))
					rollback()
					return nil, ctx.Err()
				default:
				}
				cur, err := adapter.Snapshot()
				if err == nil {
					added := adapter.Diff(before, cur)
					if len(added) > 0 {
						ad := added[0]
						tunAdapter = &ad
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
			if tunAdapter == nil {
				rollback()
				slog.Error("chain start failed", slog.String("scope", "helper"),
					slog.String("stage", "adapter-poll-timeout"),
					slog.String("err", "sing-box did not create a TUN adapter within 10s"))
				return nil, errors.New("sing-box did not create a TUN adapter within 10s")
			}
			state.tunLUID = tunAdapter.LUID
		}

		// Step 9: spawn xray. (sing-box auto_route now owns catch-all routes
		// and DNS hijack natively; helper no longer installs them.)
		xrExe, err := binaryPath("xray.exe")
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "binary-path-xray"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("binary path xray: %w", err)
		}
		xrLog := runtime.LogPath("xray.log")
		if err := runtime.RotateLog(xrLog); err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "rotate-xray-log"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("rotate xray log: %w", err)
		}
		state.xray, err = supervisor.Spawn("xray", xrExe,
			[]string{"-c", xrPath},
			xrLog)
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "spawn-xray"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("spawn xray: %w", err)
		}
		doneXray = true

		// xray is up; create a lazy gRPC client to its StatsService. No dial
		// yet — the first OpServiceStatus call dials, and a failure there is
		// non-fatal (last-cached values are returned).
		state.xrayAPI = xrayapi.New(fmt.Sprintf("127.0.0.1:%d", configgen.XrayAPIPort))

		// Step 9.5: install an NRPT rule that forces every DNS query to
		// 1.1.1.1, the transit IP. Without this, Windows resolver queries
		// the per-adapter DNS servers (TUN gets sing-box's auto_route gateway
		// 198.18.0.x — silent at gateway IP — while Ethernet keeps the ISP
		// resolver, which wins the parallel race and leaks the domain to
		// the ISP). With NRPT pointing at a transit IP, the query traverses
		// TUN → sing-box's route engine → hijack-dns → FakeIP. Best-effort:
		// failure is logged but doesn't abort the chain; user gets a chain
		// up with degraded privacy rather than an opaque-failed Connect.
		if !sysproxyMode {
			nrptName := "ITGRay-" + state.sessionID
			if err := dns.AddNrptRule(nrptName, ".", []string{"1.1.1.1"}); err != nil {
				slog.Warn("AddNrptRule failed; DNS may leak to ISP",
					slog.String("scope", "helper"), slog.String("session", state.sessionID),
					slog.String("err", logging.RedactError(err)))
			} else {
				state.nrptName = nrptName
				doneNrpt = true
			}
		}

		// Step 10: persist undo journal.
		if err := undo.Save(undoPath(), undo.Journal{
			TunName:  a.TunName,
			Routes:   state.snapshot,
			DNSPrior: dnsPriorAsList(state.dnsPrior),
		}); err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "undo-save"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("undo.Save: %w", err)
		}

		activeSess = state

		// Launch the crash watcher for THIS session. It clears activeSess on
		// an unexpected core exit so IsChainActive() flips false and chainctl's
		// drop handler reacts. Bound to `state` by pointer identity; a later
		// StopChain/StartChain swaps activeSess and the watcher no-ops.
		go watchChainExit(state)

		slog.Debug("chain start ok", slog.String("scope", "helper"),
			slog.String("session", state.sessionID), slog.String("mode", a.Mode))

		return json.Marshal(StartChainResult{
			SessionID:  state.sessionID,
			TunLUID:    state.tunLUID,
			SingboxPid: state.singbox.Pid(),
			XrayPid:    state.xray.Pid(),
		})
	}
}

// watchChainExit watches the started cores for THIS session and, on the
// first UNEXPECTED core exit, runs the same teardown StopChain performs so
// activeSess flips to nil. It is launched as a goroutine at the end of the
// OpStartChain success path and returns after the first exit + teardown
// (or immediately if the session was already replaced/torn down), so it
// cannot leak.
//
// Concurrency contract: it does NOT hold chainMu while waiting on the
// Done() channels — only while inspecting/clearing activeSess. The identity
// guard (pointer equality against the session it was bound to) makes the
// teardown a strict no-op if an explicit StopChain or a newer StartChain
// already swapped activeSess, so there is no double teardown.
func watchChainExit(sess *chainState) {
	// Wait for the first core to exit. Child.Done() on a nil child returns
	// an already-closed channel, which would fire this select immediately —
	// so only wait on the cores that were actually spawned. Both are always
	// non-nil on the success path, but guard defensively.
	var singboxDone, xrayDone <-chan struct{}
	if sess.singbox != nil {
		singboxDone = sess.singbox.Done()
	}
	if sess.xray != nil {
		xrayDone = sess.xray.Done()
	}

	var which string
	select {
	case <-singboxDone:
		which = "sing-box"
	case <-xrayDone:
		which = "xray"
	}

	chainMu.Lock()
	defer chainMu.Unlock()

	// Identity guard: only tear down if THIS session is still the active one.
	// If activeSess is nil (already stopped) or a different *chainState (a
	// newer StartChain ran), do nothing — the teardown already happened or
	// belongs to someone else.
	if activeSess != sess {
		return
	}

	slog.Warn("core exited unexpectedly; clearing active chain session",
		slog.String("scope", "helper"), slog.String("session", sess.sessionID), slog.String("core", which))

	// Same teardown StopActiveChain/OpStopChain run. stopActiveChainLocked
	// requires chainMu held and activeSess != nil — both satisfied here.
	_ = stopActiveChainLocked()
}

// undoPath returns the canonical undo journal location:
// %ProgramData%\ITG Ray\Helper\undo.json. The runtime root is one
// level deeper, so we step up via "..".
func undoPath() string {
	return runtime.ConfigPath("..\\undo.json")
}

// dnsPriorAsList wraps a single *dns.Settings into the []dns.Settings
// shape expected by undo.Journal. Returns nil when no DNS override was
// applied.
func dnsPriorAsList(prior *dns.Settings) []dns.Settings {
	if prior == nil {
		return nil
	}
	return []dns.Settings{*prior}
}

// NewStopChainHandler is the OpStopChain handler. It tears the chain
// down in reverse order. Best-effort: errors are accumulated and
// returned in the response but no individual op aborts the chain.
func NewStopChainHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a StopChainArgs
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("decode args: %w", err)
			}
		}

		chainMu.Lock()
		defer chainMu.Unlock()
		if activeSess == nil {
			return json.RawMessage(`{"status":"no-active-chain"}`), nil
		}
		if a.SessionID != "" && a.SessionID != activeSess.sessionID {
			return nil, fmt.Errorf("session id mismatch: caller=%s active=%s", a.SessionID, activeSess.sessionID)
		}

		slog.Info("chain stop", slog.String("scope", "helper"), slog.String("session", activeSess.sessionID))

		errs := stopActiveChainLocked()

		resp := map[string]any{
			"status": "stopped",
		}
		if len(errs) > 0 {
			resp["partial_errors"] = errs
		}
		return json.Marshal(resp)
	}
}

// StopActiveChain tears down the active chain synchronously, with no JSON
// layer. It is intended for in-process callers — chiefly the SCM Stop /
// Shutdown branch in cmd/itgray-helper/service_windows.go — that need to
// kill the supervised cores and undo all host mutations before the helper
// process exits, so we don't leak orphan sing-box.exe / xray.exe children.
//
// Safe to call when no chain is active (returns nil). Holds chainMu for
// the duration so it serializes correctly against any in-flight
// OpStopChain handler arriving on the named pipe.
func StopActiveChain() error {
	chainMu.Lock()
	defer chainMu.Unlock()
	if activeSess == nil {
		return nil
	}
	errs := stopActiveChainLocked()
	if len(errs) > 0 {
		return fmt.Errorf("partial: %v", errs)
	}
	return nil
}

// stopActiveChainLocked runs the teardown sequence. The caller MUST hold
// chainMu and MUST have verified activeSess != nil. Returns the list of
// best-effort errors accumulated during teardown (empty slice on full
// success). Always sets activeSess = nil before returning.
//
//nolint:gocyclo,gocognit // best-effort sequence requires linear control flow
func stopActiveChainLocked() []string {
	s := activeSess
	var errs []string

	// 1. Stop cores in parallel (worst case 2s — kill if not graceful by then).
	xrayErr, sbErr := stopBoth(2*time.Second, asStopper(s.xray), asStopper(s.singbox))
	if xrayErr != nil {
		errs = append(errs, "xray.Stop: "+xrayErr.Error())
	}
	if sbErr != nil {
		errs = append(errs, "singbox.Stop: "+sbErr.Error())
	}

	// 2. Restore DNS if we changed it (legacy dns_alias single-adapter path
	// only; sing-box auto_route teardown handles its own DNS hijack restore).
	if s.dnsPrior != nil {
		if err := dns.Restore(*s.dnsPrior); err != nil {
			errs = append(errs, "dns.Restore: "+err.Error())
		}
	}

	// 2b. Remove the NRPT rule we installed at chain start, in the
	// background. The PowerShell Remove-DnsClientNrptRule pipeline costs
	// ~1s and the chain is already down by this point, so blocking the
	// teardown RPC on it just makes Disconnect feel slow. It's idempotent
	// and best-effort (a stray rule points at the transit IP and is
	// cleared on the next connect/disconnect), so fire-and-forget is safe.
	if name := s.nrptName; name != "" {
		go func() {
			if err := dns.RemoveNrptRule(name); err != nil {
				slog.Warn("background NRPT removal failed",
					slog.String("scope", "helper"), slog.String("rule", name),
					slog.String("err", logging.RedactError(err)))
			}
		}()
	}

	// 3. Remove the peer-route.
	_ = route.Remove(s.peerRoute)

	// 4. Apply RouteRestore from snapshot (diff-add anything we evicted).
	current, err := route.Snapshot()
	if err == nil {
		want := indexRouteEntries(s.snapshot)
		have := indexRouteEntries(current)
		for k, e := range want {
			if _, ok := have[k]; !ok {
				if err := route.Add(e); err != nil {
					slog.Warn("chain teardown: route restore failed", slog.String("scope", "helper"),
						slog.String("session", s.sessionID), slog.String("dest", e.DestCIDR),
						slog.String("err", logging.RedactError(err)))
				}
			}
		}
	} else {
		errs = append(errs, "route.Snapshot(post): "+err.Error())
	}

	// 5. Clear undo journal.
	if err := undo.Clear(undoPath()); err != nil {
		errs = append(errs, "undo.Clear: "+err.Error())
	}

	// Note: do NOT wipe runtime.BasePath() here. Next session's OpStartChain
	// calls runtime.EnsureClean() which preserves *.log* and clears the rest.
	// Wiping here would defeat log preservation across sessions.

	// Close xray API client (best-effort; the conn may already be unusable
	// because xray itself just exited).
	if s.xrayAPI != nil {
		if err := s.xrayAPI.Close(); err != nil {
			errs = append(errs, "xrayAPI.Close: "+err.Error())
		}
	}

	activeSess = nil

	if len(errs) > 0 {
		slog.Error("chain stop failed", slog.String("scope", "helper"),
			slog.String("session", s.sessionID), slog.Int("count", len(errs)),
			slog.String("err", strings.Join(errs, "; ")))
	} else {
		slog.Debug("chain stop ok", slog.String("scope", "helper"), slog.String("session", s.sessionID))
	}
	return errs
}

// indexRouteEntries reuses the same key shape as the existing
// RouteRestore handler so behavior is consistent. Metric is intentionally
// excluded from the key — same dest+nh+luid is "the same route" for
// restore purposes.
func indexRouteEntries(es []route.Entry) map[string]route.Entry {
	out := make(map[string]route.Entry, len(es))
	for _, e := range es {
		key := fmt.Sprintf("%s|%s|%d", e.DestCIDR, e.NextHop, e.InterfaceLUID)
		out[key] = e
	}
	return out
}

// readChainCounters returns the latest outbound proxy uplink/downlink
// counters from xray-core, falling back to the last cached values if a
// gRPC call fails. Returns (0, 0, false) when no chain is active.
func readChainCounters(ctx context.Context) (up, down uint64, ok bool) {
	chainMu.Lock()
	defer chainMu.Unlock()
	if activeSess == nil || activeSess.xrayAPI == nil {
		return 0, 0, false
	}
	u, d, err := activeSess.xrayAPI.Counters(ctx)
	if err != nil {
		// Transient failure (xray API not yet up, conn blip). Return
		// cached values without raising; they'll refresh next tick.
		return activeSess.cachedUp, activeSess.cachedDown, true
	}
	activeSess.cachedUp = u
	activeSess.cachedDown = d
	return u, d, true
}
