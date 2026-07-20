package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/itg-team/itg-ray/internal/hub"
)

type fakeSubs struct {
	listOut    []hub.SubView
	listErr    error
	addOut     hub.SubView
	addErr     error
	editOut    hub.SubView
	editErr    error
	removeErr  error
	syncOneErr error
	syncAllErr error

	gotAddURL, gotAddName, gotAddUA               string
	gotEditID, gotEditURL, gotEditName, gotEditUA string
	gotRemoveID, gotSyncOneID                     string
	syncAllCalled                                 bool
}

func (f *fakeSubs) List() ([]hub.SubView, error) {
	return f.listOut, f.listErr
}
func (f *fakeSubs) Add(url, name, ua string) (hub.SubView, error) {
	f.gotAddURL, f.gotAddName, f.gotAddUA = url, name, ua
	return f.addOut, f.addErr
}
func (f *fakeSubs) Edit(id, url, name, ua string) (hub.SubView, error) {
	f.gotEditID, f.gotEditURL, f.gotEditName, f.gotEditUA = id, url, name, ua
	return f.editOut, f.editErr
}
func (f *fakeSubs) Remove(id string) error {
	f.gotRemoveID = id
	return f.removeErr
}
func (f *fakeSubs) SyncOne(id string) error {
	f.gotSyncOneID = id
	return f.syncOneErr
}
func (f *fakeSubs) SyncAll() error {
	f.syncAllCalled = true
	return f.syncAllErr
}

func TestSubsListReturnsViews(t *testing.T) {
	want := []hub.SubView{{ID: "u1", Name: "Pool A"}}
	h := SubsHandlers{Svc: &fakeSubs{listOut: want}}
	got, err := h.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	views, ok := got.([]hub.SubView)
	if !ok {
		t.Fatalf("expected []hub.SubView, got %T", got)
	}
	if !reflect.DeepEqual(views, want) {
		t.Fatalf("views: got %v, want %v", views, want)
	}
}

func TestSubsListPropagatesError(t *testing.T) {
	h := SubsHandlers{Svc: &fakeSubs{listErr: errors.New("disk")}}
	if _, err := h.List(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestSubsAddPassesEmptyUserAgent(t *testing.T) {
	fake := &fakeSubs{addOut: hub.SubView{ID: "u1"}}
	h := SubsHandlers{Svc: fake}
	params := json.RawMessage(`{"url":"https://x/y","name":"P"}`)
	if _, err := h.Add(context.Background(), params); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if fake.gotAddURL != "https://x/y" || fake.gotAddName != "P" {
		t.Fatalf("forwarded: url=%q name=%q", fake.gotAddURL, fake.gotAddName)
	}
	if fake.gotAddUA != "" {
		t.Fatalf("expected empty userAgent (binding falls back to settings default), got %q", fake.gotAddUA)
	}
}

func TestSubsAddInvalidParams(t *testing.T) {
	h := SubsHandlers{Svc: &fakeSubs{}}
	if _, err := h.Add(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatalf("expected error on malformed params")
	}
}

func TestSubsEditPassesEmptyUserAgent(t *testing.T) {
	fake := &fakeSubs{editOut: hub.SubView{ID: "u9"}}
	h := SubsHandlers{Svc: fake}
	params := json.RawMessage(`{"id":"u9","url":"https://x/y","name":"Renamed"}`)
	got, err := h.Edit(context.Background(), params)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if v, _ := got.(hub.SubView); v.ID != "u9" {
		t.Fatalf("result: %+v", got)
	}
	if fake.gotEditID != "u9" || fake.gotEditURL != "https://x/y" || fake.gotEditName != "Renamed" {
		t.Fatalf("forwarded: id=%q url=%q name=%q", fake.gotEditID, fake.gotEditURL, fake.gotEditName)
	}
	if fake.gotEditUA != "" {
		t.Fatalf("expected empty userAgent, got %q", fake.gotEditUA)
	}
}

func TestSubsRemovePassesID(t *testing.T) {
	fake := &fakeSubs{}
	h := SubsHandlers{Svc: fake}
	if _, err := h.Remove(context.Background(), json.RawMessage(`{"id":"u3"}`)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fake.gotRemoveID != "u3" {
		t.Fatalf("id: got %q", fake.gotRemoveID)
	}
}

func TestSubsSyncOnePassesIDAndReturnsView(t *testing.T) {
	// SyncOne handler must call Svc.SyncOne(id) then re-fetch via Svc.List()
	// and return the matching SubView (protocol declares hub.SubView result,
	// but bindings.SyncOne returns only error).
	fake := &fakeSubs{
		listOut: []hub.SubView{{ID: "u4", Name: "After Sync"}},
	}
	h := SubsHandlers{Svc: fake}
	params := json.RawMessage(`{"id":"u4"}`)
	got, err := h.SyncOne(context.Background(), params)
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if fake.gotSyncOneID != "u4" {
		t.Fatalf("id: got %q", fake.gotSyncOneID)
	}
	v, ok := got.(hub.SubView)
	if !ok {
		t.Fatalf("expected hub.SubView, got %T", got)
	}
	if v.ID != "u4" || v.Name != "After Sync" {
		t.Fatalf("result view: %+v", v)
	}
}

func TestSubsSyncOneNotFoundAfterSync(t *testing.T) {
	// If SyncOne succeeds but the row vanished from List (race or backend bug),
	// the handler must error rather than return a zero SubView.
	fake := &fakeSubs{listOut: []hub.SubView{{ID: "other"}}}
	h := SubsHandlers{Svc: fake}
	if _, err := h.SyncOne(context.Background(), json.RawMessage(`{"id":"missing"}`)); err == nil {
		t.Fatalf("expected error on id-not-found-after-sync")
	}
}

func TestSubsSyncAllNoArgs(t *testing.T) {
	fake := &fakeSubs{}
	h := SubsHandlers{Svc: fake}
	if _, err := h.SyncAll(context.Background(), nil); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	if !fake.syncAllCalled {
		t.Fatalf("SyncAll not invoked")
	}
}

func TestSubsHandlersNilSvc(t *testing.T) {
	h := SubsHandlers{Svc: nil}
	cases := []struct {
		name string
		fn   func() (any, error)
	}{
		{"List", func() (any, error) { return h.List(context.Background(), nil) }},
		{"Add", func() (any, error) { return h.Add(context.Background(), json.RawMessage(`{"url":"x","name":"y"}`)) }},
		{"Edit", func() (any, error) {
			return h.Edit(context.Background(), json.RawMessage(`{"id":"a","url":"x","name":"y"}`))
		}},
		{"Remove", func() (any, error) { return h.Remove(context.Background(), json.RawMessage(`{"id":"a"}`)) }},
		{"SyncOne", func() (any, error) { return h.SyncOne(context.Background(), json.RawMessage(`{"id":"a"}`)) }},
		{"SyncAll", func() (any, error) { return h.SyncAll(context.Background(), nil) }},
	}
	for _, tc := range cases {
		if _, err := tc.fn(); err != nil {
			t.Errorf("%s with nil Svc should be no-op, got: %v", tc.name, err)
		}
	}
}
