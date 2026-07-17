package core

import (
	"context"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestDryValidate_LogsSingboxRejection(t *testing.T) {
	buf := logtest.Capture(t)
	m := NewManager()
	// invalid sing-box JSON, minimal-but-valid or empty xray — the singbox
	// branch must fail first. Use clearly-broken JSON so UnmarshalJSONContext errors.
	err := m.DryValidate(context.Background(), []byte("{ not valid json"), []byte("{}"))
	if err == nil {
		t.Fatal("expected DryValidate to fail on broken singbox JSON")
	}
	out := buf.String()
	if !strings.Contains(out, "[core]") || !strings.Contains(out, "config validation failed") {
		t.Fatalf("missing scoped validation-failure log: %q", out)
	}
	if !strings.Contains(out, "engine=singbox") {
		t.Fatalf("missing engine attr: %q", out)
	}
}
