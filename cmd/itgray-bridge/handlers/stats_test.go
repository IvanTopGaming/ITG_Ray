package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/hub"
)

type fakeStats struct {
	gotPeriod string
	cleared   bool
}

func (f *fakeStats) Overview(p string) (hub.StatsOverview, error) {
	f.gotPeriod = p
	return hub.StatsOverview{Period: p}, nil
}
func (f *fakeStats) Connections() (hub.StatsConnections, error) {
	return hub.StatsConnections{Conns: []hub.StatsConn{{ID: "1"}}}, nil
}
func (f *fakeStats) Clear() error { f.cleared = true; return nil }

func TestStatsOverviewPassesPeriod(t *testing.T) {
	f := &fakeStats{}
	h := StatsHandlers{Svc: f}
	if _, err := h.Overview(context.Background(), json.RawMessage(`{"period":"7d"}`)); err != nil {
		t.Fatal(err)
	}
	if f.gotPeriod != "7d" {
		t.Fatalf("period %q", f.gotPeriod)
	}
	if _, err := h.Overview(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("expected error on malformed params")
	}
}

func TestStatsConnectionsAndClear(t *testing.T) {
	f := &fakeStats{}
	h := StatsHandlers{Svc: f}
	got, err := h.Connections(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(hub.StatsConnections).Conns[0].ID != "1" {
		t.Fatalf("got %+v", got)
	}
	if _, err := h.Clear(context.Background(), nil); err != nil || !f.cleared {
		t.Fatalf("clear: %v %v", err, f.cleared)
	}
}

func TestStatsNilSvc(t *testing.T) {
	h := StatsHandlers{}
	if _, err := h.Overview(context.Background(), json.RawMessage(`{"period":"today"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Connections(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}
