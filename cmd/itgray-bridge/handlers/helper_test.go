package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeHelper struct {
	state    string
	stateErr error
	calls    []string
	failOn   map[string]error
}

func (f *fakeHelper) Status() (string, error) {
	if f.stateErr != nil {
		return "", f.stateErr
	}
	return f.state, nil
}
func (f *fakeHelper) Install(exePath string) error {
	f.calls = append(f.calls, "install:"+exePath)
	return f.failOn["install"]
}
func (f *fakeHelper) Start() error {
	f.calls = append(f.calls, "start")
	return f.failOn["start"]
}
func (f *fakeHelper) Stop() error {
	f.calls = append(f.calls, "stop")
	return f.failOn["stop"]
}
func (f *fakeHelper) Restart() error {
	f.calls = append(f.calls, "restart")
	return f.failOn["restart"]
}
func (f *fakeHelper) Reinstall() error {
	f.calls = append(f.calls, "reinstall")
	return f.failOn["reinstall"]
}
func (f *fakeHelper) InstallLinux() error {
	f.calls = append(f.calls, "installLinux")
	return f.failOn["installLinux"]
}
func (f *fakeHelper) UninstallLinux() error {
	f.calls = append(f.calls, "uninstallLinux")
	return f.failOn["uninstallLinux"]
}

func TestHelperLinuxMethods(t *testing.T) {
	fake := &fakeHelper{}
	h := HelperHandlers{Svc: fake}
	if _, err := h.InstallLinux(context.Background(), nil); err != nil {
		t.Fatalf("InstallLinux: %v", err)
	}
	if _, err := h.UninstallLinux(context.Background(), nil); err != nil {
		t.Fatalf("UninstallLinux: %v", err)
	}
	want := []string{"installLinux", "uninstallLinux"}
	if len(fake.calls) != len(want) || fake.calls[0] != want[0] || fake.calls[1] != want[1] {
		t.Fatalf("calls=%v want=%v", fake.calls, want)
	}
}

func TestHelperLinuxErrorsPropagate(t *testing.T) {
	fake := &fakeHelper{failOn: map[string]error{"installLinux": errors.New("pkexec declined")}}
	h := HelperHandlers{Svc: fake}
	if _, err := h.InstallLinux(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestHelperStatusReturnsTypedShape(t *testing.T) {
	h := HelperHandlers{Svc: &fakeHelper{state: "running"}}
	result, err := h.Status(context.Background(), nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	raw, _ := json.Marshal(result)
	if string(raw) != `{"state":"running"}` {
		t.Fatalf("got=%s want={\"state\":\"running\"}", raw)
	}
}

func TestHelperStatusPropagatesError(t *testing.T) {
	h := HelperHandlers{Svc: &fakeHelper{stateErr: errors.New("scm denied")}}
	if _, err := h.Status(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestHelperInstallPassesEmptyExePath(t *testing.T) {
	fake := &fakeHelper{}
	h := HelperHandlers{Svc: fake}
	if _, err := h.Install(context.Background(), nil); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "install:" {
		t.Fatalf("expected exactly one install:'' call, got %v", fake.calls)
	}
}

func TestHelperLifecycleMethods(t *testing.T) {
	fake := &fakeHelper{}
	h := HelperHandlers{Svc: fake}
	for _, m := range []struct {
		name string
		fn   func() (any, error)
	}{
		{"Start", func() (any, error) { return h.Start(context.Background(), nil) }},
		{"Stop", func() (any, error) { return h.Stop(context.Background(), nil) }},
		{"Restart", func() (any, error) { return h.Restart(context.Background(), nil) }},
		{"Reinstall", func() (any, error) { return h.Reinstall(context.Background(), nil) }},
	} {
		if _, err := m.fn(); err != nil {
			t.Fatalf("%s: %v", m.name, err)
		}
	}
	want := []string{"start", "stop", "restart", "reinstall"}
	if len(fake.calls) != len(want) {
		t.Fatalf("calls len=%d want=%d (%v)", len(fake.calls), len(want), fake.calls)
	}
	for i, w := range want {
		if fake.calls[i] != w {
			t.Fatalf("calls[%d]=%q want %q", i, fake.calls[i], w)
		}
	}
}

func TestHelperLifecycleErrorsPropagate(t *testing.T) {
	fake := &fakeHelper{failOn: map[string]error{"start": errors.New("scm")}}
	h := HelperHandlers{Svc: fake}
	if _, err := h.Start(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestHelperHandlersNilSvc(t *testing.T) {
	h := HelperHandlers{Svc: nil}
	if _, err := h.Status(context.Background(), nil); err != nil {
		t.Fatalf("Status with nil Svc should be no-op, got: %v", err)
	}
	for _, fn := range []func() (any, error){
		func() (any, error) { return h.Install(context.Background(), nil) },
		func() (any, error) { return h.Start(context.Background(), nil) },
		func() (any, error) { return h.Stop(context.Background(), nil) },
		func() (any, error) { return h.Restart(context.Background(), nil) },
		func() (any, error) { return h.Reinstall(context.Background(), nil) },
		func() (any, error) { return h.InstallLinux(context.Background(), nil) },
		func() (any, error) { return h.UninstallLinux(context.Background(), nil) },
	} {
		if _, err := fn(); err != nil {
			t.Fatalf("nil-Svc lifecycle method should be no-op, got: %v", err)
		}
	}
}
