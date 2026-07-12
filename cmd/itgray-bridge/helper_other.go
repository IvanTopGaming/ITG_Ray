//go:build !windows

package main

import (
	"context"

	"github.com/itg-team/itg-ray/internal/chainctl"
)

// logHelperAddr is the transport the log poller dials for OpReadLogs. On
// Linux this is the privileged daemon's unix socket; on macOS the dial is
// unsupported and the poller degrades to a no-op.
const logHelperAddr = "/run/itgray-helper.sock"

// newHelperClient returns the Linux/macOS helper client: a mode-routing
// adapter that sends SysProxy mode to the in-process core (sing-box + xray
// run directly in the bridge, no root) and TUN mode to the privileged
// helper daemon over its unix socket (root, sing-box auto_route). See
// internal/chainctl/helper_adapter_other.go and helper_daemon_other.go.
func newHelperClient(_ context.Context) chainctl.HelperClient {
	return chainctl.NewRoutingHelperClient()
}
