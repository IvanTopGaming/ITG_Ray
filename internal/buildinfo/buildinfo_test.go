package buildinfo

import (
	"strings"
	"testing"

	xraycore "github.com/xtls/xray-core/core"
)

func TestEnginesReportsXrayReleaseNotModulePseudoVersion(t *testing.T) {
	got := Engines()
	if !strings.Contains(got, "xray "+xraycore.Version()) {
		t.Fatalf("Engines() = %q, want it to report xray %s", got, xraycore.Version())
	}
	if strings.Contains(got, "1.260327") {
		t.Fatalf("Engines() = %q leaks the Go module pseudo-version", got)
	}
}

func TestEnginesReportsSingBoxModuleVersion(t *testing.T) {
	v := depVersion("github.com/sagernet/sing-box")
	if v == "" {
		t.Skip("sing-box is not linked into this test binary")
	}
	if !strings.Contains(Engines(), "sing-box "+v) {
		t.Fatalf("Engines() = %q, want sing-box %s", Engines(), v)
	}
}
