package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Version carries the release tag into `itgray-helper --version`, and the
// build scripts set it with -ldflags "-X main.Version=...". That only works on
// a var: the linker cannot rewrite a constant, so declaring it const silently
// pinned every release to the placeholder.
func TestVersionIsOverridableByLdflags(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	// Windows needs the .exe suffix: with an explicit -o the toolchain writes
	// exactly the name given, and exec then refuses to run an extension-less
	// file.
	name := "itgray-helper-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	const want = "v9.9.9-ldflags-probe"

	build := exec.Command("go", "build", "-ldflags", "-X main.Version="+want, "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run --version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), want) {
		t.Fatalf("--version did not report the linker-supplied version.\ngot:  %s\nwant it to contain: %s", out, want)
	}
}
