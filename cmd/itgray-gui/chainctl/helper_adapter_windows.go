//go:build windows

package chainctl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	helperserver "github.com/itg-team/itg-ray/internal/helper/server"
)

// HelperAdapter wraps an *internal/helper/client.Client so it satisfies
// the chainctl.HelperClient surface.
//
// IMPORTANT: the Plan-B helper bundles route-snapshot, peer-route, TUN
// discovery, and sing-box/xray spawn inside the OpStartChain handler
// (see internal/helper/server/chain_windows.go). The granular ops listed
// in HelperClient (TunCreate, RouteSnapshot, RouteAdd, DnsSet, …) DO
// exist as separate handlers but are not needed when StartChain runs:
// calling them in addition would either duplicate work (RouteSnapshot)
// or fail because StartChain already owns the resource (TunCreate /
// RouteAdd on the same /32). The chainctl Controller calls the granular
// ops via this adapter; we make those no-ops so the Controller's bringUp
// sequence works in production while the unit tests still get full
// per-op coverage against the in-memory fake.
//
// C.T10 will revisit this once the GUI integrates with the real helper
// — at that point we may either delete the granular ops from
// HelperClient, or split OpStartChain into its components on the helper
// side. For now: StartChain + StopChain + ServiceStatus do real work;
// everything else is a no-op.
//
// Server endpoint: OpStartChain requires ServerHost/ServerPort in its
// args (the helper resolves the host and adds a /32 peer-route via the
// current default gateway BEFORE sing-box spawns). The adapter extracts
// those from the xray-core config we hand to StartChain — the VLESS
// vnext outbound carries the real server address/port. This keeps the
// adapter a stable, server-agnostic singleton: each Connect call
// supplies its own xray config, and the adapter never needs to be
// rebuilt when the user switches servers.
type HelperAdapter struct {
	c         *client.Client
	tunName   string
	sessionID string // populated on first successful StartChain
}

// NewHelperAdapter builds an adapter around an already-dialed helper
// client. Only the TUN adapter name is captured at construction (fixed
// per process); the per-Connect server endpoint travels through the
// xrayJSON passed to StartChain.
func NewHelperAdapter(c *client.Client, tunName string) *HelperAdapter {
	return &HelperAdapter{c: c, tunName: tunName}
}

// xrayServerEndpoint pulls the (address, port) tuple out of the VLESS
// vnext outbound emitted by configgen.BuildXray. The shape is
// outbounds[0].settings.vnext[0].{address, port}; we only need the
// minimum for json.Unmarshal so a missing optional field doesn't
// fail the decode.
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

// StartChain bundles the configs into OpStartChain. The session id
// returned by the helper is captured so StopChain can address the same
// session.
func (a *HelperAdapter) StartChain(ctx context.Context, singboxJSON, xrayJSON []byte, mode Mode) error {
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
	raw, err := a.c.Call(ctx, protocol.OpStartChain, args)
	if err != nil {
		return err
	}
	var res helperserver.StartChainResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("decode StartChain result: %w", err)
	}
	a.sessionID = res.SessionID
	return nil
}

// StopChain addresses the captured session id (if any) so a stale GUI
// can't tear down a chain it didn't start.
func (a *HelperAdapter) StopChain(ctx context.Context) error {
	args, err := json.Marshal(helperserver.StopChainArgs{SessionID: a.sessionID})
	if err != nil {
		return fmt.Errorf("marshal StopChain: %w", err)
	}
	_, err = a.c.Call(ctx, protocol.OpStopChain, args)
	a.sessionID = ""
	return err
}

// ServiceStatus calls OpServiceStatus and projects the helper's
// (version, uptime) tuple onto the chainctl ChainState shape. The real
// counters are not yet exposed by the helper — the poller's speed
// computation will read zero deltas until C.T14 wires real stats.
func (a *HelperAdapter) ServiceStatus(ctx context.Context) (ChainState, error) {
	raw, err := a.c.Call(ctx, protocol.OpServiceStatus, nil)
	if err != nil {
		return ChainState{}, err
	}
	var res helperserver.ServiceStatusResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return ChainState{}, fmt.Errorf("decode ServiceStatus: %w", err)
	}
	_ = res // version/uptime not yet surfaced; kept to validate decoding round-trip
	// "Helper is up" doesn't directly tell us "chain is running" — the
	// chainctl Controller is the source of truth for chain state in this
	// design. We report Running=true whenever the helper responds; the
	// poller's crash-detection branch only fires if the helper itself
	// goes away (Call returns an error → caller skips the tick).
	return ChainState{Running: true}, nil
}

// The remaining ops are no-ops because OpStartChain bundles them on the
// helper side (see comment on HelperAdapter). They satisfy the interface
// so chainctl.Controller's bringUp / tearDown sequence type-checks.

// TunCreate is a no-op (handled inside OpStartChain).
func (a *HelperAdapter) TunCreate(_ context.Context, _, _ string) error { return nil }

// TunDestroy is a no-op (handled inside OpStopChain).
func (a *HelperAdapter) TunDestroy(_ context.Context) error { return nil }

// RouteSnapshot is a no-op (handled inside OpStartChain).
func (a *HelperAdapter) RouteSnapshot(_ context.Context) error { return nil }

// RouteAdd is a no-op (the peer-route is added inside OpStartChain).
func (a *HelperAdapter) RouteAdd(_ context.Context, _ string) error { return nil }

// RouteRestore is a no-op (handled inside OpStopChain).
func (a *HelperAdapter) RouteRestore(_ context.Context) error { return nil }

// DnsSet is a no-op (sing-box's TUN inbound owns DNS hijack natively).
func (a *HelperAdapter) DnsSet(_ context.Context, _ []string) error { return nil }

// DnsRestore is a no-op (handled inside OpStopChain).
func (a *HelperAdapter) DnsRestore(_ context.Context) error { return nil }
