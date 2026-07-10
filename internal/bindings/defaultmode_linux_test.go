//go:build linux

package bindings

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/chainctl"
)

func TestDefaultIdleModeIsTUNOnLinux(t *testing.T) {
	if got := defaultIdleMode(); got != chainctl.ModeTUN {
		t.Fatalf("defaultIdleMode() = %q, want %q", got, chainctl.ModeTUN)
	}
}
