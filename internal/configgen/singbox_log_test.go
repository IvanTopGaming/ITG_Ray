package configgen

import (
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestBuildSingbox_LogsMissingRuleSetPath(t *testing.T) {
	buf := logtest.Capture(t)
	in := inputWithRuleSetButNoPath(t)
	if _, err := BuildSingbox(in); err == nil {
		t.Fatal("expected BuildSingbox to fail on missing rule-set path")
	}
	out := buf.String()
	if !strings.Contains(out, "[configgen]") || !strings.Contains(out, "singbox build failed") {
		t.Fatalf("missing configgen failure log: %q", out)
	}
	if !strings.Contains(out, "tag=") {
		t.Fatalf("missing offending tag attr: %q", out)
	}
}
