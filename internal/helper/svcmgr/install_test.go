package svcmgr

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStub_OnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub test runs only on non-windows")
	}
	require.ErrorContains(t, Install("ITGRayHelper", "C:\\bin\\itgray-helper.exe", "ITG Ray helper"), "Windows-only")
	require.ErrorContains(t, Uninstall("ITGRayHelper"), "Windows-only")
	require.ErrorContains(t, Start("ITGRayHelper"), "Windows-only")
	require.ErrorContains(t, Stop("ITGRayHelper"), "Windows-only")
	_, err := Status("ITGRayHelper")
	require.ErrorContains(t, err, "Windows-only")
}
