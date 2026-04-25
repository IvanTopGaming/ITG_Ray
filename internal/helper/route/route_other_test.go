//go:build !windows

package route

import (
	"errors"
	"strings"
	"testing"
)

// TestStubReturnsPlatformError exercises the non-Windows stubs so the package
// has at least one CI-runnable test on the Linux build host. The Windows code
// path is exercised in B11.1 VM smoke.
func TestStubReturnsPlatformError(t *testing.T) {
	t.Parallel()

	if got, err := Snapshot(); err == nil || got != nil {
		t.Fatalf("Snapshot: want error+nil slice, got %v / err=%v", got, err)
	} else if !errors.Is(err, errPlatform) {
		t.Fatalf("Snapshot: want errPlatform, got %v", err)
	} else if !strings.Contains(err.Error(), "Windows-only") {
		t.Fatalf("Snapshot: want %q in error message, got %q", "Windows-only", err.Error())
	}

	e := Entry{DestCIDR: "10.0.0.0/8", NextHop: "10.0.0.1", InterfaceLUID: 1, Metric: 5}
	if err := Add(e); !errors.Is(err, errPlatform) {
		t.Fatalf("Add: want errPlatform, got %v", err)
	}
	if err := Remove(e); !errors.Is(err, errPlatform) {
		t.Fatalf("Remove: want errPlatform, got %v", err)
	}
}
