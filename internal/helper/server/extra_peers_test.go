package server

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestExtraPeerCIDRs(t *testing.T) {
	lookup := func(h string) ([]net.IP, error) {
		if h == "b.example" {
			return []net.IP{net.ParseIP("3.3.3.3")}, nil
		}
		return nil, errors.New("no such host")
	}
	got, err := extraPeerCIDRs(lookup, net.ParseIP("1.1.1.1"), []string{"2.2.2.2", "b.example", "1.1.1.1", "2.2.2.2"}, 1, time.Millisecond)
	if err != nil {
		t.Fatalf("extraPeerCIDRs: %v", err)
	}
	want := []string{"2.2.2.2/32", "3.3.3.3/32"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExtraPeerCIDRs_LookupFailure(t *testing.T) {
	lookup := func(string) ([]net.IP, error) { return nil, errors.New("no such host") }
	if _, err := extraPeerCIDRs(lookup, nil, []string{"x.example"}, 1, time.Millisecond); err == nil {
		t.Fatal("expected error for unresolvable extra host")
	}
}
