package chainctl

import (
	"context"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestReconcile_LogsRecoveryOutcome(t *testing.T) {
	buf := logtest.Capture(t)
	c := newReconcileTestController(t)
	c.Reconcile(context.Background())
	out := buf.String()
	if !strings.Contains(out, "[chainctl]") || !strings.Contains(out, "reconcile") {
		t.Fatalf("missing chainctl reconcile log: %q", out)
	}
}
