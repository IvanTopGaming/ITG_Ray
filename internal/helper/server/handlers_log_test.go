package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestReadLogsHandler_LogsRejectedName(t *testing.T) {
	buf := logtest.Capture(t)
	h := NewReadLogsHandler()
	if _, err := h(context.Background(), json.RawMessage(`{"name":"secret.txt"}`)); err == nil {
		t.Fatal("expected error for a disallowed log name")
	}
	out := buf.String()
	if !strings.Contains(out, "[helper]") {
		t.Fatalf("missing helper scope in log: %q", out)
	}
	if !strings.Contains(out, "read logs rejected") {
		t.Fatalf("missing rejection warn: %q", out)
	}
	if !strings.Contains(out, "name=secret.txt") {
		t.Fatalf("missing rejected name attr: %q", out)
	}
}
