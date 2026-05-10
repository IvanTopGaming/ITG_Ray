package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
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

func TestAppGetSnapshotReturnsSnapshot(t *testing.T) {
	want := hub.Snapshot{
		Status:      hub.StatusIdle,
		Mode:        "tun",
		HelperState: "missing",
		Version:     "test-1.0",
	}
	h := AppHandlers{Snap: stubSnapshotter{snap: want}}
	result, err := h.GetSnapshot(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	got, ok := result.(hub.Snapshot)
	if !ok {
		t.Fatalf("expected hub.Snapshot, got %T", result)
	}
	if got.Status != want.Status || got.Mode != want.Mode || got.Version != want.Version {
		t.Fatalf("snapshot mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

type stubSnapshotter struct {
	snap hub.Snapshot
	err  error
}

func (s stubSnapshotter) GetSnapshot() (hub.Snapshot, error) { return s.snap, s.err }
