//go:build !windows

package main

import (
	"context"
	"errors"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
)

// errUnsupported mirrors the Wails GUI sentinel: chainctl receives this
// from chain-mutating helper methods on non-Windows builds. The bridge
// binary is shipped only for Windows in v0.1, but we keep it cross-
// platform so unit tests, lints, and editor tooling work on Linux / macOS
// dev machines. This file mirrors cmd/itgray-gui/helper_other.go.
var errUnsupported = errors.New("helper service is only available on Windows")

// newHelperClient returns the unsupported-platform stub on non-Windows.
func newHelperClient(_ context.Context) chainctl.HelperClient {
	return stubHelperClient{}
}

// stubHelperClient mirrors cmd/itgray-gui/helper_other.go: every chain-
// mutating method returns errUnsupported; teardown ops are idempotent
// no-ops so chainctl.tearDown rollback doesn't surface duplicate errors.
type stubHelperClient struct{}

func (stubHelperClient) StartChain(_ context.Context, _, _ []byte, _ chainctl.Mode) error {
	return errUnsupported
}
func (stubHelperClient) StopChain(_ context.Context) error              { return nil }
func (stubHelperClient) TunCreate(_ context.Context, _, _ string) error { return errUnsupported }
func (stubHelperClient) TunDestroy(_ context.Context) error             { return nil }
func (stubHelperClient) RouteSnapshot(_ context.Context) error          { return errUnsupported }
func (stubHelperClient) RouteAdd(_ context.Context, _ string) error     { return errUnsupported }
func (stubHelperClient) RouteRestore(_ context.Context) error           { return nil }
func (stubHelperClient) DnsSet(_ context.Context, _ []string) error     { return errUnsupported }
func (stubHelperClient) DnsRestore(_ context.Context) error             { return nil }
func (stubHelperClient) ServiceStatus(_ context.Context) (chainctl.ChainState, error) {
	return chainctl.ChainState{}, errUnsupported
}
