//go:build windows

package bindings

import "github.com/itg-team/itg-ray/internal/chainctl"

// defaultIdleMode is the connect mode surfaced by GetSnapshot when no chain
// is active. Windows defaults to TUN.
func defaultIdleMode() chainctl.Mode { return chainctl.ModeTUN }
