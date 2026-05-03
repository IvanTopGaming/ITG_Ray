//go:build !windows

package main

import (
	"context"
	"errors"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
)

// errUnsupported is the sentinel HelperClient surface non-Windows builds
// expose. The GUI binary is shipped only for Windows in v0.1, but we
// keep the build cross-platform so unit tests, lints, and editor
// tooling work on Linux/macOS dev machines.
var errUnsupported = errors.New("helper service is only available on Windows")

// newHelperClient returns an unsupported-platform stub on non-Windows.
// Connect attempts will fail with errUnsupported but every other GUI
// surface (servers, subscriptions, settings) keeps working.
func newHelperClient(_ context.Context) chainctl.HelperClient {
	return stubHelperClient{}
}

// stubHelperClient is the non-Windows HelperClient. Every method that
// would touch real OS state returns errUnsupported; teardown ops are
// idempotent no-ops so chainctl's tearDown rollback path doesn't keep
// surfacing the same error.
type stubHelperClient struct{}

// StartChain returns errUnsupported on non-Windows.
func (stubHelperClient) StartChain(_ context.Context, _, _ []byte, _ chainctl.Mode) error {
	return errUnsupported
}

// StopChain is a no-op on non-Windows so tearDown is idempotent.
func (stubHelperClient) StopChain(_ context.Context) error { return nil }

// TunCreate returns errUnsupported on non-Windows.
func (stubHelperClient) TunCreate(_ context.Context, _, _ string) error { return errUnsupported }

// TunDestroy is a no-op on non-Windows.
func (stubHelperClient) TunDestroy(_ context.Context) error { return nil }

// RouteSnapshot returns errUnsupported on non-Windows.
func (stubHelperClient) RouteSnapshot(_ context.Context) error { return errUnsupported }

// RouteAdd returns errUnsupported on non-Windows.
func (stubHelperClient) RouteAdd(_ context.Context, _ string) error { return errUnsupported }

// RouteRestore is a no-op on non-Windows.
func (stubHelperClient) RouteRestore(_ context.Context) error { return nil }

// DnsSet returns errUnsupported on non-Windows.
func (stubHelperClient) DnsSet(_ context.Context, _ []string) error { return errUnsupported }

// DnsRestore is a no-op on non-Windows.
func (stubHelperClient) DnsRestore(_ context.Context) error { return nil }

// ServiceStatus returns errUnsupported on non-Windows.
func (stubHelperClient) ServiceStatus(_ context.Context) (chainctl.ChainState, error) {
	return chainctl.ChainState{}, errUnsupported
}
