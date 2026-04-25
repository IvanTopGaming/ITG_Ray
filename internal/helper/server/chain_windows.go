//go:build windows

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/adapter"
	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/gateway"
	"github.com/itg-team/itg-ray/internal/helper/route"
	"github.com/itg-team/itg-ray/internal/helper/runtime"
	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/itg-team/itg-ray/internal/helper/undo"
)

// StartChainArgs is the JSON payload of OpStartChain.
type StartChainArgs struct {
	SingboxConfig json.RawMessage `json:"singbox_config"`
	XrayConfig    json.RawMessage `json:"xray_config"`
	ServerHost    string          `json:"server_host"`
	ServerPort    int             `json:"server_port"`
	TunName       string          `json:"tun_name"`
	DnsAlias      string          `json:"dns_alias,omitempty"`
	DnsServers    []string        `json:"dns_servers,omitempty"`
}

// StartChainResult is what OpStartChain returns on success.
type StartChainResult struct {
	SessionID  string `json:"session_id"`
	TunLUID    uint64 `json:"tun_luid"`
	SingboxPid int    `json:"singbox_pid"`
	XrayPid    int    `json:"xray_pid"`
}

// StopChainArgs is the JSON payload of OpStopChain.
type StopChainArgs struct {
	SessionID string `json:"session_id"` // optional; used to defend against stale callers
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
}

var (
	chainMu    sync.Mutex
	activeSess *chainState
)

// binaryPath looks up the Helper's adjacent binary by name. Helper.exe lives
// at C:\Program Files\ITG Ray\itgray-helper.exe and sing-box.exe / xray.exe
// are deployed beside it.
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

// NewStartChainHandler is the OpStartChain handler. It atomically
// captures restore state, applies routes, spawns the cores, discovers
// the new TUN adapter, and installs the catch-all route. Any mid-flow
// failure rolls back.
//
//nolint:gocyclo,gocognit // orchestration sequence requires linear control flow so the rollback-flag pattern stays obvious
func NewStartChainHandler() Handler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a StartChainArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if a.ServerHost == "" || a.ServerPort == 0 || a.TunName == "" {
			return nil, errors.New("server_host, server_port, tun_name required")
		}

		chainMu.Lock()
		defer chainMu.Unlock()
		if activeSess != nil {
			return nil, fmt.Errorf("chain already running (session=%s)", activeSess.sessionID)
		}

		state := &chainState{
			sessionID: newSessionID(),
			dnsAlias:  a.DnsAlias,
		}

		// rollback runs in reverse order of operations performed; only the
		// operations whose `done` flag is set actually reverse.
		var (
			doneRuntime, doneRouteSnap, donePeerRoute, doneDnsSnap, doneSingbox, doneXray, doneCatchAll bool
		)
		rollback := func() {
			if doneCatchAll {
				_ = route.Remove(route.Entry{
					DestCIDR: "0.0.0.0/0", NextHop: "0.0.0.0",
					InterfaceLUID: state.tunLUID, Metric: 0,
				})
			}
			if doneXray && state.xray != nil {
				_ = state.xray.Stop(2 * time.Second)
			}
			if doneSingbox && state.singbox != nil {
				_ = state.singbox.Stop(2 * time.Second)
			}
			if doneDnsSnap && state.dnsPrior != nil {
				_ = dns.Restore(*state.dnsPrior)
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
			return nil, fmt.Errorf("runtime.EnsureClean: %w", err)
		}
		doneRuntime = true

		// Step 2: persist configs.
		sbPath := runtime.ConfigPath("sing-box.json")
		if err := os.WriteFile(sbPath, a.SingboxConfig, 0o640); err != nil { //nolint:gosec // %ProgramData%, admin-only
			rollback()
			return nil, fmt.Errorf("write sing-box config: %w", err)
		}
		xrPath := runtime.ConfigPath("xray.json")
		if err := os.WriteFile(xrPath, a.XrayConfig, 0o640); err != nil { //nolint:gosec // %ProgramData%, admin-only
			rollback()
			return nil, fmt.Errorf("write xray config: %w", err)
		}

		// Step 3: snapshot routes for restore.
		snap, err := route.Snapshot()
		if err != nil {
			rollback()
			return nil, fmt.Errorf("route.Snapshot: %w", err)
		}
		state.snapshot = snap
		doneRouteSnap = true

		// Step 4: peer-route via current default gateway.
		gw, err := gateway.Default()
		if err != nil {
			rollback()
			return nil, fmt.Errorf("gateway.Default: %w", err)
		}
		state.peerRoute = route.Entry{
			DestCIDR:      a.ServerHost + "/32",
			NextHop:       gw.NextHop,
			InterfaceLUID: gw.InterfaceLUID,
			Metric:        0,
		}
		if err := route.Add(state.peerRoute); err != nil {
			rollback()
			return nil, fmt.Errorf("route.Add(peer): %w", err)
		}
		donePeerRoute = true

		// Step 5: optional DNS override.
		if a.DnsAlias != "" && len(a.DnsServers) > 0 {
			prior, err := dns.Snapshot(a.DnsAlias)
			if err != nil {
				rollback()
				return nil, fmt.Errorf("dns.Snapshot: %w", err)
			}
			if err := dns.Set(dns.Settings{InterfaceAlias: a.DnsAlias, Addresses: a.DnsServers}); err != nil {
				rollback()
				return nil, fmt.Errorf("dns.Set: %w", err)
			}
			state.dnsPrior = &prior
			doneDnsSnap = true
		}

		// Step 6: snapshot adapters before spawning sing-box.
		before, err := adapter.Snapshot()
		if err != nil {
			rollback()
			return nil, fmt.Errorf("adapter.Snapshot(before): %w", err)
		}

		// Step 7: spawn sing-box.
		sbExe, err := binaryPath("sing-box.exe")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("binary path: %w", err)
		}
		state.singbox, err = supervisor.Spawn("sing-box", sbExe,
			[]string{"run", "-c", sbPath},
			runtime.LogPath("sing-box.log"))
		if err != nil {
			rollback()
			return nil, fmt.Errorf("spawn sing-box: %w", err)
		}
		doneSingbox = true

		// Step 8: discover the new adapter (poll up to 10s).
		var tunAdapter *adapter.Adapter
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
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
			time.Sleep(250 * time.Millisecond)
		}
		if tunAdapter == nil {
			rollback()
			return nil, errors.New("sing-box did not create a TUN adapter within 10s")
		}
		state.tunLUID = tunAdapter.LUID

		// Step 9: install the catch-all via the new TUN.
		catchAll := route.Entry{
			DestCIDR:      "0.0.0.0/0",
			NextHop:       "0.0.0.0",
			InterfaceLUID: state.tunLUID,
			Metric:        0,
		}
		if err := route.Add(catchAll); err != nil {
			rollback()
			return nil, fmt.Errorf("route.Add(catch-all): %w", err)
		}
		doneCatchAll = true

		// Step 10: spawn xray.
		xrExe, err := binaryPath("xray.exe")
		if err != nil {
			rollback()
			return nil, fmt.Errorf("binary path xray: %w", err)
		}
		state.xray, err = supervisor.Spawn("xray", xrExe,
			[]string{"-c", xrPath},
			runtime.LogPath("xray.log"))
		if err != nil {
			rollback()
			return nil, fmt.Errorf("spawn xray: %w", err)
		}
		doneXray = true

		// Step 11: persist undo journal.
		if err := undo.Save(undoPath(), undo.Journal{
			TunName:  a.TunName,
			Routes:   state.snapshot,
			DNSPrior: dnsPriorAsList(state.dnsPrior),
		}); err != nil {
			rollback()
			return nil, fmt.Errorf("undo.Save: %w", err)
		}

		activeSess = state

		return json.Marshal(StartChainResult{
			SessionID:  state.sessionID,
			TunLUID:    state.tunLUID,
			SingboxPid: state.singbox.Pid(),
			XrayPid:    state.xray.Pid(),
		})
	}
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
//
//nolint:gocyclo,gocognit // best-effort sequence requires linear control flow
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

		s := activeSess
		var errs []string

		// 1. Stop cores (xray first, then sing-box).
		if s.xray != nil {
			if err := s.xray.Stop(5 * time.Second); err != nil {
				errs = append(errs, "xray.Stop: "+err.Error())
			}
		}
		if s.singbox != nil {
			if err := s.singbox.Stop(5 * time.Second); err != nil {
				errs = append(errs, "singbox.Stop: "+err.Error())
			}
		}

		// 2. Remove catch-all route (sing-box may have already cleaned up).
		_ = route.Remove(route.Entry{
			DestCIDR:      "0.0.0.0/0",
			NextHop:       "0.0.0.0",
			InterfaceLUID: s.tunLUID,
			Metric:        0,
		})

		// 3. Restore DNS if we changed it.
		if s.dnsPrior != nil {
			if err := dns.Restore(*s.dnsPrior); err != nil {
				errs = append(errs, "dns.Restore: "+err.Error())
			}
		}

		// 4. Remove the peer-route.
		_ = route.Remove(s.peerRoute)

		// 5. Apply RouteRestore from snapshot (diff-add anything we evicted).
		current, err := route.Snapshot()
		if err == nil {
			want := indexRouteEntries(s.snapshot)
			have := indexRouteEntries(current)
			for k, e := range want {
				if _, ok := have[k]; !ok {
					_ = route.Add(e)
				}
			}
		} else {
			errs = append(errs, "route.Snapshot(post): "+err.Error())
		}

		// 6. Clear undo journal.
		if err := undo.Clear(undoPath()); err != nil {
			errs = append(errs, "undo.Clear: "+err.Error())
		}

		// 7. Wipe runtime dir.
		_ = os.RemoveAll(runtime.BasePath())

		activeSess = nil

		resp := map[string]any{
			"status": "stopped",
		}
		if len(errs) > 0 {
			resp["partial_errors"] = errs
		}
		return json.Marshal(resp)
	}
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
