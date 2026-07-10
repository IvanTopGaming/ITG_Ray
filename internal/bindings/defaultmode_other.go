//go:build !windows && !linux

package bindings

import "github.com/itg-team/itg-ray/internal/chainctl"

// defaultIdleMode is the connect mode surfaced by GetSnapshot when no chain
// is active. Darwin has no privileged TUN path yet, so the out-of-the-box
// mode is SysProxy. (Linux defaults to TUN — see defaultmode_linux.go.)
func defaultIdleMode() chainctl.Mode { return chainctl.ModeSysProxy }
