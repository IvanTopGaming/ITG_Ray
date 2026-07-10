//go:build !windows

package chainctl

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	helperserver "github.com/itg-team/itg-ray/internal/helper/server"
)

// daemonSocketPath is the fixed unix-socket the privileged helper daemon
// listens on (see internal/helper/server, Linux transport). Not
// configurable — the daemon's systemd unit and SO_PEERCRED auth both key
// off this path.
const daemonSocketPath = "/run/itgray-helper.sock"

// daemonHelperClient is the Linux TUN-mode backend: it satisfies the
// chainctl.HelperClient surface by dialing the privileged helper daemon
// over its unix socket and issuing OpStartChain / OpStopChain /
// OpServiceStatus. It is the Linux analog of helper_adapter_windows.go's
// HelperAdapter (which dials a winio named pipe instead).
//
// IMPORTANT: like the Windows helper, the Linux daemon bundles
// route-snapshot, peer-route, TUN creation, sing-box auto_route and
// sing-box/xray spawn inside its OpStartChain handler. The granular ops
// (TunCreate, RouteSnapshot, RouteAdd, DnsSet, …) therefore do NOT run
// separately from this client — chainctl's bringUp calls them, but here
// they are no-ops so the sequence type-checks while StartChain owns all
// the privileged work.
//
// Server endpoint: OpStartChain requires ServerHost/ServerPort in its args
// (the daemon resolves the host and adds a /32 peer-route via the current
// default gateway BEFORE sing-box spawns). We extract those from the
// xray-core config handed to StartChain — its VLESS vnext outbound carries
// the real server address/port — which keeps this a stable, server-agnostic
// singleton: each Connect supplies its own xray config.
//
// Connection: the socket is dialed lazily on the first StartChain and
// redialled if a prior connection went stale, so a daemon restart between
// Connect cycles heals without rebuilding the client.
type daemonHelperClient struct {
	mu        sync.Mutex
	c         *client.Client
	tunName   string
	sessionID string // populated on first successful StartChain
}

// newDaemonHelperClient builds the TUN-mode backend. Only the TUN adapter
// name is captured up front (fixed per process); the per-Connect server
// endpoint travels through the xrayJSON passed to StartChain, and the socket
// connection is established lazily.
func newDaemonHelperClient() HelperClient {
	return &daemonHelperClient{tunName: defaultTunName}
}

// dial returns a live helper-daemon client, connecting on first use. Callers
// hold no lock; dial takes m.mu internally.
func (a *daemonHelperClient) dial(ctx context.Context) (*client.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c != nil {
		return a.c, nil
	}
	c, err := client.Dial(ctx, daemonSocketPath)
	if err != nil {
		return nil, fmt.Errorf("dial helper daemon: %w", err)
	}
	a.c = c
	return a.c, nil
}

// resetConn drops a stale connection so the next dial reconnects. Called
// when a Call fails at the transport level.
func (a *daemonHelperClient) resetConn() {
	a.mu.Lock()
	if a.c != nil {
		_ = a.c.Close()
		a.c = nil
	}
	a.mu.Unlock()
}

// xrayServerEndpoint pulls the (address, port) tuple out of the VLESS vnext
// outbound emitted by configgen.BuildXray. The shape is
// outbounds[0].settings.vnext[0].{address, port}; we only need the minimum
// for json.Unmarshal so a missing optional field doesn't fail the decode.
//
// (Copied from helper_adapter_windows.go — that file is build-tagged
// `windows`, so this `!windows` file cannot share the symbol.)
func xrayServerEndpoint(xrayJSON []byte) (string, int, error) {
	var doc struct {
		Outbounds []struct {
			Settings struct {
				Vnext []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
				} `json:"vnext"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(xrayJSON, &doc); err != nil {
		return "", 0, fmt.Errorf("decode xray config: %w", err)
	}
	if len(doc.Outbounds) == 0 || len(doc.Outbounds[0].Settings.Vnext) == 0 {
		return "", 0, fmt.Errorf("xray config: vnext outbound missing")
	}
	v := doc.Outbounds[0].Settings.Vnext[0]
	if v.Address == "" || v.Port == 0 {
		return "", 0, fmt.Errorf("xray config: vnext address/port empty")
	}
	return v.Address, v.Port, nil
}

// StartChain bundles the configs into OpStartChain over the daemon socket.
// The session id returned by the daemon is captured so StopChain can address
// the same session. A transport-level failure resets the connection and
// retries once, so a daemon restart between Connect cycles self-heals.
func (a *daemonHelperClient) StartChain(ctx context.Context, singboxJSON, xrayJSON []byte, mode Mode) error {
	host, port, err := xrayServerEndpoint(xrayJSON)
	if err != nil {
		return err
	}
	args, err := json.Marshal(helperserver.StartChainArgs{
		SingboxConfig: singboxJSON,
		XrayConfig:    xrayJSON,
		ServerHost:    host,
		ServerPort:    port,
		TunName:       a.tunName,
		Mode:          string(mode),
	})
	if err != nil {
		return fmt.Errorf("marshal StartChain: %w", err)
	}

	raw, err := a.call(ctx, protocol.OpStartChain, args)
	if err != nil {
		return err
	}
	var res helperserver.StartChainResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("decode StartChain result: %w", err)
	}
	a.mu.Lock()
	a.sessionID = res.SessionID
	a.mu.Unlock()
	return nil
}

// StopChain addresses the captured session id (if any) so a stale caller
// can't tear down a chain it didn't start.
func (a *daemonHelperClient) StopChain(ctx context.Context) error {
	a.mu.Lock()
	sid := a.sessionID
	a.mu.Unlock()
	args, err := json.Marshal(helperserver.StopChainArgs{SessionID: sid})
	if err != nil {
		return fmt.Errorf("marshal StopChain: %w", err)
	}
	_, err = a.call(ctx, protocol.OpStopChain, args)
	a.mu.Lock()
	a.sessionID = ""
	a.mu.Unlock()
	return err
}

// ServiceStatus calls OpServiceStatus and projects the daemon's response
// onto the chainctl ChainState shape. Byte counters come from xray-core's
// StatsService via the daemon; when no chain is active they are zero.
// Running mirrors the daemon's ChainActive flag so the poller's
// crash-detection branch fires on teardown and Reconcile can adopt an
// already-running chain on bridge startup.
func (a *daemonHelperClient) ServiceStatus(ctx context.Context) (ChainState, error) {
	raw, err := a.call(ctx, protocol.OpServiceStatus, nil)
	if err != nil {
		return ChainState{}, err
	}
	var res helperserver.ServiceStatusResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return ChainState{}, fmt.Errorf("decode ServiceStatus: %w", err)
	}
	return ChainState{
		Running:   res.ChainActive,
		UpBytes:   res.UpBytes,
		DownBytes: res.DownBytes,
	}, nil
}

// call dials lazily and issues one RPC. On a transport-level failure it
// drops the connection and retries once against a fresh dial, healing a
// daemon that restarted between Connect cycles.
func (a *daemonHelperClient) call(ctx context.Context, op protocol.Op, args json.RawMessage) (json.RawMessage, error) {
	c, err := a.dial(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := c.Call(ctx, op, args)
	if err != nil {
		a.resetConn()
		c, derr := a.dial(ctx)
		if derr != nil {
			return nil, err
		}
		return c.Call(ctx, op, args)
	}
	return raw, nil
}

// The remaining ops are no-ops because OpStartChain bundles them on the
// daemon side (see comment on daemonHelperClient). They satisfy the
// interface so chainctl.Controller's bringUp / tearDown sequence type-checks.

// TunCreate is a no-op (handled inside OpStartChain).
func (a *daemonHelperClient) TunCreate(_ context.Context, _, _ string) error { return nil }

// TunDestroy is a no-op (handled inside OpStopChain).
func (a *daemonHelperClient) TunDestroy(_ context.Context) error { return nil }

// RouteSnapshot is a no-op (handled inside OpStartChain).
func (a *daemonHelperClient) RouteSnapshot(_ context.Context) error { return nil }

// RouteAdd is a no-op (the peer-route is added inside OpStartChain).
func (a *daemonHelperClient) RouteAdd(_ context.Context, _ string) error { return nil }

// RouteRestore is a no-op (handled inside OpStopChain).
func (a *daemonHelperClient) RouteRestore(_ context.Context) error { return nil }

// DnsSet is a no-op (sing-box's TUN inbound owns DNS hijack natively).
func (a *daemonHelperClient) DnsSet(_ context.Context, _ []string) error { return nil }

// DnsRestore is a no-op (handled inside OpStopChain).
func (a *daemonHelperClient) DnsRestore(_ context.Context) error { return nil }
