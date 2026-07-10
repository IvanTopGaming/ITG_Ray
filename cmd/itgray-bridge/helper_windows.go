//go:build windows

package main

import (
	"context"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/helper/client"
)

// helperPipe is the canonical Plan-B helper pipe path. Mirrors the
// constants exported by cmd/itgray-helper and cmd/itgray-cli — kept
// inline here to avoid pulling the helper main package into the GUI
// build graph.
const helperPipe = `\\.\pipe\ITGRay.Helper.v1`

// newHelperClient returns a lazily-connecting helper client. The helper
// named pipe is dialed ON DEMAND (first helper call) rather than once at
// startup, so the first-run flow works: the app launches before the user
// installs the helper service, and the next Connect after install dials a
// fresh, live pipe — no app restart needed. It also survives helper
// Restart/Reinstall, redialing after the old connection dies.
func newHelperClient(_ context.Context) chainctl.HelperClient {
	return newLazyHelperClient(func(ctx context.Context) (chainctl.HelperClient, error) {
		c, err := client.Dial(ctx, helperPipe)
		if err != nil {
			return nil, err
		}
		return chainctl.NewHelperAdapter(c, defaultTunName), nil
	})
}
