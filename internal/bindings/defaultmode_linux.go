//go:build linux

package bindings

import "github.com/itg-team/itg-ray/internal/chainctl"

// defaultIdleMode is the connect mode surfaced by GetSnapshot when no chain
// is active. Linux has the privileged TUN helper (Phase B), so the
// out-of-the-box mode is TUN.
func defaultIdleMode() chainctl.Mode { return chainctl.ModeTUN }
