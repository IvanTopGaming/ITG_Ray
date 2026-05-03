//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/internal/helper/client"
)

// helperPipe is the canonical Plan-B helper pipe path. Mirrors the
// constants exported by cmd/itgray-helper and cmd/itgray-cli — kept
// inline here to avoid pulling the helper main package into the GUI
// build graph.
const helperPipe = `\\.\pipe\ITGRay.Helper.v1`

// newHelperClient dials the helper named pipe and returns it wrapped in
// a chainctl.HelperClient adapter. If the helper isn't running yet
// (e.g. user hasn't installed the service), we return a "missing helper"
// stub so the GUI still constructs successfully — Connect attempts will
// fail with a clear error at bringup time, but the dashboard shell is
// usable so the user can see the helper:state badge and react.
func newHelperClient(ctx context.Context) chainctl.HelperClient {
	c, err := client.Dial(ctx, helperPipe)
	if err != nil {
		slog.Warn("helper pipe dial failed; Connect will error until helper is installed", "err", err)
		return &missingHelperClient{err: err}
	}
	return chainctl.NewHelperAdapter(c, defaultTunName)
}

// missingHelperClient is a stub HelperClient used when the helper
// service isn't running. Every method returns a clear error so the
// chainctl bringup sequence fails fast with an actionable message
// rather than panicking on a nil interface.
type missingHelperClient struct{ err error }

func (m *missingHelperClient) wrap() error {
	return fmt.Errorf("helper unavailable: %w", m.err)
}

// StartChain reports the helper is unavailable.
func (m *missingHelperClient) StartChain(_ context.Context, _, _ []byte, _ chainctl.Mode) error {
	return m.wrap()
}

// StopChain is a no-op when the helper was never reached.
func (m *missingHelperClient) StopChain(_ context.Context) error { return nil }

// TunCreate reports the helper is unavailable.
func (m *missingHelperClient) TunCreate(_ context.Context, _, _ string) error { return m.wrap() }

// TunDestroy is a no-op when the helper was never reached.
func (m *missingHelperClient) TunDestroy(_ context.Context) error { return nil }

// RouteSnapshot reports the helper is unavailable.
func (m *missingHelperClient) RouteSnapshot(_ context.Context) error { return m.wrap() }

// RouteAdd reports the helper is unavailable.
func (m *missingHelperClient) RouteAdd(_ context.Context, _ string) error { return m.wrap() }

// RouteRestore is a no-op when the helper was never reached.
func (m *missingHelperClient) RouteRestore(_ context.Context) error { return nil }

// DnsSet reports the helper is unavailable.
func (m *missingHelperClient) DnsSet(_ context.Context, _ []string) error { return m.wrap() }

// DnsRestore is a no-op when the helper was never reached.
func (m *missingHelperClient) DnsRestore(_ context.Context) error { return nil }

// ServiceStatus reports the helper is unavailable.
func (m *missingHelperClient) ServiceStatus(_ context.Context) (chainctl.ChainState, error) {
	return chainctl.ChainState{}, m.wrap()
}
