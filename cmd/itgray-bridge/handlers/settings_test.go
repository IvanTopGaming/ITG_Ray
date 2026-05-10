package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

type fakeSettings struct {
	view       hub.SettingsView
	viewErr    error
	gotSection string
	gotPatch   map[string]any
	updateErr  error
}

func (f *fakeSettings) Get() (hub.SettingsView, error) {
	if f.viewErr != nil {
		return hub.SettingsView{}, f.viewErr
	}
	return f.view, nil
}

func (f *fakeSettings) Update(section string, patch map[string]any) (hub.SettingsView, error) {
	if f.updateErr != nil {
		return hub.SettingsView{}, f.updateErr
	}
	f.gotSection = section
	f.gotPatch = patch
	return f.view, nil
}

func TestSettingsGetReturnsView(t *testing.T) {
	want := hub.SettingsView{
		General: hub.GeneralSettings{Language: "en"},
	}
	h := SettingsHandlers{Svc: &fakeSettings{view: want}}
	result, err := h.Get(context.Background(), nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, ok := result.(hub.SettingsView)
	if !ok {
		t.Fatalf("expected hub.SettingsView, got %T", result)
	}
	if got.General.Language != "en" {
		t.Fatalf("language mismatch: %+v", got)
	}
}

func TestSettingsGetPropagatesError(t *testing.T) {
	h := SettingsHandlers{Svc: &fakeSettings{viewErr: errors.New("disk")}}
	if _, err := h.Get(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestSettingsUpdatePassesSectionAndPatch(t *testing.T) {
	fake := &fakeSettings{view: hub.SettingsView{}}
	h := SettingsHandlers{Svc: fake}
	params := json.RawMessage(`{"section":"network","patch":{"socksPort":1080}}`)
	if _, err := h.Update(context.Background(), params); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if fake.gotSection != "network" {
		t.Fatalf("section: got %q, want %q", fake.gotSection, "network")
	}
	wantPatch := map[string]any{"socksPort": float64(1080)}
	if !reflect.DeepEqual(fake.gotPatch, wantPatch) {
		t.Fatalf("patch: got %v, want %v", fake.gotPatch, wantPatch)
	}
}

func TestSettingsUpdateInvalidParams(t *testing.T) {
	h := SettingsHandlers{Svc: &fakeSettings{}}
	if _, err := h.Update(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatalf("expected error on malformed params")
	}
}

func TestSettingsUpdatePropagatesError(t *testing.T) {
	h := SettingsHandlers{Svc: &fakeSettings{updateErr: errors.New("validation")}}
	params := json.RawMessage(`{"section":"network","patch":{}}`)
	if _, err := h.Update(context.Background(), params); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestSettingsHandlersNilSvc(t *testing.T) {
	h := SettingsHandlers{Svc: nil}
	if _, err := h.Get(context.Background(), nil); err != nil {
		t.Fatalf("Get with nil Svc should be no-op, got: %v", err)
	}
	params := json.RawMessage(`{"section":"general","patch":{}}`)
	if _, err := h.Update(context.Background(), params); err != nil {
		t.Fatalf("Update with nil Svc should be no-op, got: %v", err)
	}
}
