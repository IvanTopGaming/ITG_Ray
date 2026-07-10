//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestRenderServiceUnit_EmbedsUID(t *testing.T) {
	unit := renderServiceUnit(1000)
	if !strings.Contains(unit, "ITGRAY_ALLOW_UID=1000") {
		t.Fatalf("service unit missing allow-uid env:\n%s", unit)
	}
	if !strings.Contains(unit, "/usr/local/lib/itgray/itgray-helper") {
		t.Fatalf("service unit missing ExecStart path:\n%s", unit)
	}
	// Socket-activation was dropped: the daemon self-binds its socket, so
	// the service must not depend on a .socket unit.
	if strings.Contains(unit, "itgray-helper.socket") {
		t.Fatalf("service unit must not reference the dropped socket unit:\n%s", unit)
	}
}
