//go:build !windows

package bindings

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/chainctl"
)

func TestDefaultIdleModeIsSysProxyOnNonWindows(t *testing.T) {
	if got := defaultIdleMode(); got != chainctl.ModeSysProxy {
		t.Fatalf("defaultIdleMode() = %q, want %q", got, chainctl.ModeSysProxy)
	}
}
