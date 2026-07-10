//go:build !windows

package main

import (
	"context"

	"github.com/itg-team/itg-ray/internal/chainctl"
)

// newHelperClient returns the Linux/macOS helper client: an in-process,
// core-backed adapter that runs sing-box + xray directly in the bridge
// process for SysProxy mode. TUN mode is rejected until Phase B. See
// internal/chainctl/helper_adapter_other.go.
func newHelperClient(_ context.Context) chainctl.HelperClient {
	return chainctl.NewCoreHelperClient()
}
