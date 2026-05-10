package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAppPingReturnsTimestamp(t *testing.T) {
	h := AppHandlers{}
	result, err := h.Ping(context.Background(), nil)
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	raw, _ := json.Marshal(result)
	// Result has shape {"pong":<unix-millis>,"version":"<string>"}
	if !strings.Contains(string(raw), `"pong":`) {
		t.Fatalf("missing pong field: %s", raw)
	}
}
