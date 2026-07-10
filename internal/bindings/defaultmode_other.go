//go:build !windows

package bindings

import "github.com/itg-team/itg-ray/internal/chainctl"

// defaultIdleMode is the connect mode surfaced by GetSnapshot when no chain
// is active. Non-Windows has no privileged TUN path yet (Phase B), so the
// out-of-the-box mode is SysProxy.
func defaultIdleMode() chainctl.Mode { return chainctl.ModeSysProxy }
