package handlers

import (
	"context"
	"encoding/json"
	"errors"
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
func (s stubSnapshotter) GetPublicIP() (string, error)       { return "", nil }

// fakeSnapshotter extends stubSnapshotter with public-IP doubles for Task 3 tests.
type fakeSnapshotter struct {
	snap hub.Snapshot
	err  error
	// public IP doubles
	pubIP    string
	pubIPErr error
}

func (f *fakeSnapshotter) GetSnapshot() (hub.Snapshot, error) { return f.snap, f.err }
func (f *fakeSnapshotter) GetPublicIP() (string, error)       { return f.pubIP, f.pubIPErr }

func TestAppGetPublicIPReturnsValue(t *testing.T) {
	h := AppHandlers{Snap: &fakeSnapshotter{pubIP: "203.0.113.42"}}
	got, err := h.GetPublicIP(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPublicIP: %v", err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", got)
	}
	if s != "203.0.113.42" {
		t.Fatalf("ip mismatch: got %q", s)
	}
}

func TestAppGetPublicIPPropagatesError(t *testing.T) {
	h := AppHandlers{Snap: &fakeSnapshotter{pubIPErr: errors.New("not connected")}}
	if _, err := h.GetPublicIP(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestAppGetPublicIPNilSnap(t *testing.T) {
	h := AppHandlers{Snap: nil}
	got, err := h.GetPublicIP(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPublicIP with nil Snap should be no-op, got: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string for nil Snap, got %v", got)
	}
}
