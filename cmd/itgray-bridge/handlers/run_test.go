package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeRun struct {
	connectErr    error
	disconnectErr error

	gotServerID, gotMode string
	disconnectCalled     bool
	gotRoutingMode       string
	routingMode          string
}

func (f *fakeRun) SetRoutingMode(mode string) error {
	f.gotRoutingMode = mode
	return nil
}
func (f *fakeRun) GetRoutingMode() (string, error) { return f.routingMode, nil }

func (f *fakeRun) Connect(serverID, mode string) error {
	f.gotServerID, f.gotMode = serverID, mode
	return f.connectErr
}
func (f *fakeRun) Disconnect() error {
	f.disconnectCalled = true
	return f.disconnectErr
}

func TestRunConnectPassesServerIDAndMode(t *testing.T) {
	fake := &fakeRun{}
	h := RunHandlers{Svc: fake}
	params := json.RawMessage(`{"serverId":"s7","mode":"tun"}`)
	if _, err := h.Connect(context.Background(), params); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if fake.gotServerID != "s7" {
		t.Fatalf("serverId: got %q", fake.gotServerID)
	}
	if fake.gotMode != "tun" {
		t.Fatalf("mode: got %q", fake.gotMode)
	}
}

func TestRunConnectInvalidParams(t *testing.T) {
	h := RunHandlers{Svc: &fakeRun{}}
	if _, err := h.Connect(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatalf("expected error on malformed params")
	}
}

func TestRunConnectPropagatesError(t *testing.T) {
	h := RunHandlers{Svc: &fakeRun{connectErr: errors.New("dial failed")}}
	params := json.RawMessage(`{"serverId":"s7","mode":"tun"}`)
	if _, err := h.Connect(context.Background(), params); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunDisconnectInvokesSvc(t *testing.T) {
	fake := &fakeRun{}
	h := RunHandlers{Svc: fake}
	if _, err := h.Disconnect(context.Background(), nil); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if !fake.disconnectCalled {
		t.Fatalf("Disconnect not invoked")
	}
}

func TestRunDisconnectPropagatesError(t *testing.T) {
	h := RunHandlers{Svc: &fakeRun{disconnectErr: errors.New("teardown")}}
	if _, err := h.Disconnect(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRunHandlersNilSvc(t *testing.T) {
	h := RunHandlers{Svc: nil}
	cases := []struct {
		name string
		fn   func() (any, error)
	}{
		{"Connect", func() (any, error) {
			return h.Connect(context.Background(), json.RawMessage(`{"serverId":"a","mode":"tun"}`))
		}},
		{"Disconnect", func() (any, error) { return h.Disconnect(context.Background(), nil) }},
	}
	for _, tc := range cases {
		if _, err := tc.fn(); err != nil {
			t.Errorf("%s with nil Svc should be no-op, got: %v", tc.name, err)
		}
	}
}

func TestRunSetRoutingModePassesMode(t *testing.T) {
	fake := &fakeRun{}
	h := RunHandlers{Svc: fake}
	if _, err := h.SetRoutingMode(context.Background(), json.RawMessage(`{"mode":"direct"}`)); err != nil {
		t.Fatalf("SetRoutingMode: %v", err)
	}
	if fake.gotRoutingMode != "direct" {
		t.Fatalf("mode: got %q", fake.gotRoutingMode)
	}
	if _, err := h.SetRoutingMode(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("expected error on malformed params")
	}
}

func TestRunGetRoutingModeWrapsResult(t *testing.T) {
	h := RunHandlers{Svc: &fakeRun{routingMode: "global"}}
	got, err := h.GetRoutingMode(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetRoutingMode: %v", err)
	}
	b, _ := json.Marshal(got)
	if string(b) != `{"mode":"global"}` {
		t.Fatalf("result: %s", b)
	}
	got, _ = RunHandlers{}.GetRoutingMode(context.Background(), nil)
	b, _ = json.Marshal(got)
	if string(b) != `{"mode":"rule"}` {
		t.Fatalf("nil svc result: %s", b)
	}
}
