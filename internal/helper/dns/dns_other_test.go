//go:build !windows

package dns

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

	if got, err := Snapshot("Ethernet"); err == nil || got.InterfaceAlias != "" || got.Addresses != nil {
		t.Fatalf("Snapshot: want error+zero Settings, got %+v / err=%v", got, err)
	} else if !errors.Is(err, errPlatform) {
		t.Fatalf("Snapshot: want errPlatform, got %v", err)
	} else if !strings.Contains(err.Error(), "Windows-only") {
		t.Fatalf("Snapshot: want %q in error message, got %q", "Windows-only", err.Error())
	}

	s := Settings{InterfaceAlias: "Ethernet", Addresses: []string{"1.1.1.1", "8.8.8.8"}}
	if err := Set(s); !errors.Is(err, errPlatform) {
		t.Fatalf("Set: want errPlatform, got %v", err)
	}
	if err := Restore(s); !errors.Is(err, errPlatform) {
		t.Fatalf("Restore: want errPlatform, got %v", err)
	}
}
