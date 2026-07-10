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
}

func TestRenderSocketUnit_Mode(t *testing.T) {
	unit := renderSocketUnit()
	if !strings.Contains(unit, "/run/itgray-helper.sock") {
		t.Fatalf("socket unit missing ListenStream:\n%s", unit)
	}
	if !strings.Contains(unit, "SocketMode=0660") {
		t.Fatalf("socket unit missing SocketMode:\n%s", unit)
	}
}
