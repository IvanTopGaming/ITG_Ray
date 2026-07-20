//go:build linux

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/itg-team/itg-ray/internal/helper/xrayapi"
	"github.com/itg-team/itg-ray/internal/logging"
)

// runtimeDir is the root-writable scratch directory where the privileged
// daemon persists each session's sing-box/xray configs and log files.
// Unlike Windows (%ProgramData%\ITG Ray\Helper\runtime), Linux has no
// per-session cleanup dance — sing-box's auto_route owns routes + DNS
// natively, so there is nothing host-level to preserve across sessions.
// runtimeDir is a var (not const) so tests can point it at a writable temp
// directory; in production it is always the root-owned /run path.
var runtimeDir = "/run/itgray-helper"

// spawnCore is the process spawner, overridable in tests.
var spawnCore = supervisor.Spawn

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

// stopChildBestEffort runs c.Stop(grace), overridable in tests. It backs
// the StartChain rollback path's core teardown so a Stop failure can be
// observed without a real spawned process.
var stopChildBestEffort = func(c *supervisor.Child, grace time.Duration) error {
	if c == nil {
		return nil
	}
	return c.Stop(grace)
}

// chainState tracks the active session inside the daemon process. On Linux
// it is deliberately thinner than the Windows twin: no route snapshot, no
// peer-route, no DNS/NRPT restore state — sing-box's auto_route creates the
// TUN and owns routes + DNS hijack as root, and the config's
// route_exclude_address (set bridge-side) handles the server-loop.
type chainState struct {
	sessionID string
	singbox   *supervisor.Child
	xray      *supervisor.Child

	// xrayAPI is the gRPC client to xray-core's StatsService. Created in
	// NewStartChainHandler after xray spawns successfully; closed in
	// stopActiveChainLocked. nil before first StartChain. Typed as the
	// narrow statsClient interface (rather than *xrayapi.Client directly)
	// so tests can substitute a fake that blocks, simulating a wedged
	// xray-core without a real gRPC server.
	xrayAPI statsClient
	// cachedUp/cachedDown are the last successful counter readings; used
	// by OpServiceStatus when a transient gRPC error happens.
	cachedUp   uint64
	cachedDown uint64
}

// statsClient is the narrow surface readChainCounters/stopActiveChainLocked
// need from xray-core's StatsService client. *xrayapi.Client satisfies it.
type statsClient interface {
	Counters(ctx context.Context) (up, down uint64, err error)
	Close() error
}

var (
	chainMu    sync.Mutex
	activeSess *chainState
)

// IsChainActive reports whether the daemon has a running chain session.
// Safe for concurrent callers — used by OpServiceStatus to surface chain
// liveness to the user-level bridge for Reconcile-time adoption.
func IsChainActive() bool {
	chainMu.Lock()
	defer chainMu.Unlock()
	return activeSess != nil
}

// binaryPath looks up an adjacent binary by name. The install copies
// sing-box + xray alongside the daemon into /usr/local/lib/itgray/, so the
// sibling-of-executable layout is enough on Linux (no Wails/Electron split
// or ../cores fallback like Windows). Falls back to the sibling path so a
// spawn error surfaces a recognisable location instead of a $PATH lookup.
func binaryPath(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), name), nil
}

// newSessionID returns a 16-char hex session id from 8 random bytes.
// crypto/rand.Read only fails on a broken entropy source, so the error
// is intentionally discarded.
func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// NewStartChainHandler is the OpStartChain handler. It persists the
// bridge-supplied configs, spawns sing-box then xray as external
// processes, and tracks the session. Sing-box's auto_route owns the TUN,
// catch-all routes, and DNS hijack natively — so unlike Windows this
// handler performs no route/gateway/adapter/DNS/NRPT host mutation. Any
// mid-flow failure rolls back whatever spawned.
func NewStartChainHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a StartChainArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if a.ServerHost == "" || a.ServerPort == 0 {
			return nil, errors.New("server_host, server_port required")
		}
		switch a.Mode {
		case "", "tun":
			if a.TunName == "" {
				return nil, errors.New("tun_name required for tun mode")
			}
		case "sysproxy":
			// no TUN required
		default:
			return nil, errors.New("invalid mode")
		}

		chainMu.Lock()
		defer chainMu.Unlock()
		if activeSess != nil {
			return nil, fmt.Errorf("chain already running (session=%s)", activeSess.sessionID)
		}

		slog.Info("chain start", slog.String("scope", "helper"), slog.String("mode", a.Mode))

		state := &chainState{sessionID: newSessionID()}

		// rollback runs in reverse order of operations performed; only the
		// operations whose `done` flag is set actually reverse.
		var doneSingbox, doneXray bool
		rollback := func() {
			if doneXray && state.xray != nil {
				if err := stopChildBestEffort(state.xray, 2*time.Second); err != nil {
					slog.Warn("chain teardown: xray stop failed", slog.String("scope", "helper"),
						slog.String("session", state.sessionID), slog.String("err", logging.RedactError(err)))
				}
			}
			if doneSingbox && state.singbox != nil {
				if err := stopChildBestEffort(state.singbox, 2*time.Second); err != nil {
					slog.Warn("chain teardown: singbox stop failed", slog.String("scope", "helper"),
						slog.String("session", state.sessionID), slog.String("err", logging.RedactError(err)))
				}
			}
		}

		// Step 1: prepare runtime dir.
		if err := os.MkdirAll(runtimeDir, 0o750); err != nil {
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "mkdir-runtime-dir"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("mkdir runtime dir: %w", err)
		}

		// Step 2: persist configs VERBATIM. The bridge already generated
		// them (including route_exclude_address for the server-loop), so this
		// handler never touches configgen. Only the operation name is logged
		// on failure — config bytes carry server credentials and must never
		// reach the log stream.
		sbPath := filepath.Join(runtimeDir, "sing-box.json")
		sbCfg := sanitizeCoreConfig("sing-box", a.SingboxConfig, runtimeDir)
		if err := os.WriteFile(sbPath, sbCfg, 0o640); err != nil { //nolint:gosec // /run/itgray-helper, root-only
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "write-sing-box-config"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("write sing-box config: %w", err)
		}
		xrPath := filepath.Join(runtimeDir, "xray.json")
		xrCfg := sanitizeCoreConfig("xray", a.XrayConfig, runtimeDir)
		if err := os.WriteFile(xrPath, xrCfg, 0o640); err != nil { //nolint:gosec // /run/itgray-helper, root-only
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "write-xray-config"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("write xray config: %w", err)
		}

		// Step 3: spawn sing-box (creates its own TUN via auto_route).
		sbExe, err := binaryPath("sing-box")
		if err != nil {
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "binary-path-sing-box"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("binary path: %w", err)
		}
		sbLog := filepath.Join(runtimeDir, "sing-box.log")
		state.singbox, err = spawnCore("sing-box", sbExe,
			[]string{"run", "-c", sbPath},
			sbLog)
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "spawn-sing-box"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("spawn sing-box: %w", err)
		}
		doneSingbox = true

		// Step 4: spawn xray.
		xrExe, err := binaryPath("xray")
		if err != nil {
			rollback()
			slog.Error("chain start failed", slog.String("scope", "helper"),
				slog.String("stage", "binary-path-xray"), slog.String("err", logging.RedactError(err)))
			return nil, fmt.Errorf("binary path xray: %w", err)
		}
		xrLog := filepath.Join(runtimeDir, "xray.log")
		state.xray, err = spawnCore("xray", xrExe,
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

		activeSess = state

		// Launch the crash watcher for THIS session. It clears activeSess on
		// an unexpected core exit so IsChainActive() flips false and chainctl's
		// drop handler reacts. Bound to `state` by pointer identity; a later
		// StopChain/StartChain swaps activeSess and the watcher no-ops.
		go watchChainExit(state)

		slog.Debug("chain start ok", slog.String("scope", "helper"),
			slog.String("session", state.sessionID), slog.String("mode", a.Mode))

		// TunLUID is a Windows-only concept; sing-box owns the TUN on Linux.
		return json.Marshal(StartChainResult{
			SessionID:  state.sessionID,
			TunLUID:    0,
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

// NewStopChainHandler is the OpStopChain handler. It tears the chain
// down (kills both cores) and clears the session. Best-effort: errors are
// accumulated and returned in the response but no individual op aborts.
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
// layer. It is intended for in-process callers — chiefly the daemon's
// shutdown branch — that need to kill the supervised cores before the
// daemon process exits, so we don't leak orphan sing-box / xray children.
//
// Safe to call when no chain is active (returns nil). Holds chainMu for
// the duration so it serializes correctly against any in-flight
// OpStopChain handler arriving on the unix socket.
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
func stopActiveChainLocked() []string {
	s := activeSess
	var errs []string

	// Stop cores in parallel (worst case 2s — kill if not graceful by then).
	// sing-box tears down its own auto_route TUN + routes + DNS hijack as it
	// exits, so there is no host-level restore to run afterwards on Linux.
	xrayErr, sbErr := stopBoth(2*time.Second, asStopper(s.xray), asStopper(s.singbox))
	if xrayErr != nil {
		errs = append(errs, "xray.Stop: "+xrayErr.Error())
	}
	if sbErr != nil {
		errs = append(errs, "singbox.Stop: "+sbErr.Error())
	}

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

// readChainCounters returns the latest outbound proxy uplink/downlink
// counters from xray-core, falling back to the last cached values if a
// gRPC call fails. Returns (0, 0, false) when no chain is active.
//
// chainMu is held only long enough to snapshot the active session's stats
// client and cached counters, and again afterwards to update the cache —
// the gRPC round-trip itself runs UNLOCKED, under its own bounded deadline
// independent of ctx's lifetime (status.go's caller chain threads the
// daemon's top-level context here, which is otherwise never cancelled
// until process shutdown). This is backend-review finding H4: without it,
// a wedged/slow xray-core StatsService blocks this function forever while
// holding chainMu, which in turn blocks OpStopChain (which also needs
// chainMu) — the user loses the ability to Disconnect.
func readChainCounters(ctx context.Context) (up, down uint64, ok bool) {
	chainMu.Lock()
	sess := activeSess
	if sess == nil || sess.xrayAPI == nil {
		chainMu.Unlock()
		return 0, 0, false
	}
	client := sess.xrayAPI
	cachedUp, cachedDown := sess.cachedUp, sess.cachedDown
	chainMu.Unlock()

	rctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	u, d, err := client.Counters(rctx)
	if err != nil {
		// Transient failure (xray API not yet up, conn blip, or the
		// bounded deadline above expired against a wedged core). Return
		// cached values without raising; they'll refresh next tick.
		return cachedUp, cachedDown, true
	}

	chainMu.Lock()
	// Identity guard: only cache into the session this reading belongs to.
	// If StopChain or a newer StartChain already swapped activeSess while
	// this call was in flight unlocked, don't resurrect stale counters
	// into whatever is active now.
	if activeSess == sess {
		sess.cachedUp = u
		sess.cachedDown = d
	}
	chainMu.Unlock()
	return u, d, true
}
