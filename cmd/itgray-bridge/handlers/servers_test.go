package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

type fakeServers struct {
	listOut   []hub.ServerView
	listErr   error
	addOut    hub.ServerView
	addErr    error
	editOut   hub.ServerView
	editChg   bool
	editErr   error
	removeErr error
	togErr    error
	testErr   error

	gotAddURI, gotAddName              string
	gotEditID, gotEditURI, gotEditName string
	gotRemoveID, gotTogID, gotTestID   string
}

func (f *fakeServers) List() ([]hub.ServerView, error) {
	return f.listOut, f.listErr
}
func (f *fakeServers) Add(uri, name string) (hub.ServerView, error) {
	f.gotAddURI, f.gotAddName = uri, name
	return f.addOut, f.addErr
}
func (f *fakeServers) Edit(id, uri, name string) (hub.ServerView, bool, error) {
	f.gotEditID, f.gotEditURI, f.gotEditName = id, uri, name
	return f.editOut, f.editChg, f.editErr
}
func (f *fakeServers) Remove(id string) error {
	f.gotRemoveID = id
	return f.removeErr
}
func (f *fakeServers) ToggleFavorite(id string) error {
	f.gotTogID = id
	return f.togErr
}
func (f *fakeServers) TestLatency(id string) error {
	f.gotTestID = id
	return f.testErr
}

func TestServersListReturnsViews(t *testing.T) {
	want := []hub.ServerView{{ID: "s1", Name: "First"}, {ID: "s2", Name: "Second"}}
	h := ServersHandlers{Svc: &fakeServers{listOut: want}}
	got, err := h.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	views, ok := got.([]hub.ServerView)
	if !ok {
		t.Fatalf("expected []hub.ServerView, got %T", got)
	}
	if !reflect.DeepEqual(views, want) {
		t.Fatalf("views: got %v, want %v", views, want)
	}
}

func TestServersListPropagatesError(t *testing.T) {
	h := ServersHandlers{Svc: &fakeServers{listErr: errors.New("disk")}}
	if _, err := h.List(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestServersAddPassesParams(t *testing.T) {
	fake := &fakeServers{addOut: hub.ServerView{ID: "s1"}}
	h := ServersHandlers{Svc: fake}
	params := json.RawMessage(`{"uri":"vless://abc@host:443","name":"My Server"}`)
	got, err := h.Add(context.Background(), params)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if fake.gotAddURI != "vless://abc@host:443" {
		t.Fatalf("uri: got %q", fake.gotAddURI)
	}
	if fake.gotAddName != "My Server" {
		t.Fatalf("name: got %q", fake.gotAddName)
	}
	if v, _ := got.(hub.ServerView); v.ID != "s1" {
		t.Fatalf("result: %+v", got)
	}
}

func TestServersAddInvalidParams(t *testing.T) {
	h := ServersHandlers{Svc: &fakeServers{}}
	if _, err := h.Add(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatalf("expected error on malformed params")
	}
}

func TestServersEditReturnsViewAndChanged(t *testing.T) {
	fake := &fakeServers{editOut: hub.ServerView{ID: "s7"}, editChg: true}
	h := ServersHandlers{Svc: fake}
	params := json.RawMessage(`{"id":"s7","uri":"vless://x@h:1","name":"Renamed"}`)
	got, err := h.Edit(context.Background(), params)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	res, ok := got.(serversEditResult)
	if !ok {
		t.Fatalf("expected serversEditResult, got %T", got)
	}
	if res.View.ID != "s7" || !res.VlessChanged {
		t.Fatalf("result: %+v", res)
	}
	if fake.gotEditID != "s7" || fake.gotEditURI != "vless://x@h:1" || fake.gotEditName != "Renamed" {
		t.Fatalf("forwarded: id=%q uri=%q name=%q", fake.gotEditID, fake.gotEditURI, fake.gotEditName)
	}
}

func TestServersRemovePassesID(t *testing.T) {
	fake := &fakeServers{}
	h := ServersHandlers{Svc: fake}
	if _, err := h.Remove(context.Background(), json.RawMessage(`{"id":"s9"}`)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fake.gotRemoveID != "s9" {
		t.Fatalf("id: got %q", fake.gotRemoveID)
	}
}

func TestServersToggleFavoritePassesID(t *testing.T) {
	fake := &fakeServers{}
	h := ServersHandlers{Svc: fake}
	if _, err := h.ToggleFavorite(context.Background(), json.RawMessage(`{"id":"s2"}`)); err != nil {
		t.Fatalf("ToggleFavorite: %v", err)
	}
	if fake.gotTogID != "s2" {
		t.Fatalf("id: got %q", fake.gotTogID)
	}
}

func TestServersTestLatencyPassesID(t *testing.T) {
	fake := &fakeServers{}
	h := ServersHandlers{Svc: fake}
	if _, err := h.TestLatency(context.Background(), json.RawMessage(`{"id":"s3"}`)); err != nil {
		t.Fatalf("TestLatency: %v", err)
	}
	if fake.gotTestID != "s3" {
		t.Fatalf("id: got %q", fake.gotTestID)
	}
}

func TestServersHandlersNilSvc(t *testing.T) {
	h := ServersHandlers{Svc: nil}
	cases := []struct {
		name string
		fn   func() (any, error)
	}{
		{"List", func() (any, error) { return h.List(context.Background(), nil) }},
		{"Add", func() (any, error) { return h.Add(context.Background(), json.RawMessage(`{"uri":"x","name":"y"}`)) }},
		{"Edit", func() (any, error) {
			return h.Edit(context.Background(), json.RawMessage(`{"id":"a","uri":"x","name":"y"}`))
		}},
		{"Remove", func() (any, error) { return h.Remove(context.Background(), json.RawMessage(`{"id":"a"}`)) }},
		{"ToggleFavorite", func() (any, error) { return h.ToggleFavorite(context.Background(), json.RawMessage(`{"id":"a"}`)) }},
		{"TestLatency", func() (any, error) { return h.TestLatency(context.Background(), json.RawMessage(`{"id":"a"}`)) }},
	}
	for _, tc := range cases {
		if _, err := tc.fn(); err != nil {
			t.Errorf("%s with nil Svc should be no-op, got: %v", tc.name, err)
		}
	}
}
