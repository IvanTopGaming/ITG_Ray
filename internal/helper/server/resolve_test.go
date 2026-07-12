package server

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestResolveServerIPv4_RetriesTransientFailureThenSucceeds(t *testing.T) {
	calls := 0
	lookup := func(host string) ([]net.IP, error) {
		calls++
		if calls < 3 {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		return []net.IP{net.ParseIP("203.0.113.7")}, nil
	}
	ip, err := resolveServerIPv4(lookup, "okins.example.ru", 5, 0)
	if err != nil {
		t.Fatalf("err=%v, want success after retry", err)
	}
	if ip.String() != "203.0.113.7" {
		t.Fatalf("ip=%v, want 203.0.113.7", ip)
	}
	if calls != 3 {
		t.Fatalf("calls=%d, want 3 (2 transient failures + 1 success)", calls)
	}
}

func TestResolveServerIPv4_AllAttemptsFailReturnsWrappedError(t *testing.T) {
	sentinel := &net.DNSError{Err: "no such host", Name: "okins.example.ru", IsNotFound: true}
	calls := 0
	lookup := func(string) ([]net.IP, error) {
		calls++
		return nil, sentinel
	}
	_, err := resolveServerIPv4(lookup, "okins.example.ru", 4, 0)
	if err == nil {
		t.Fatal("err=nil, want failure after exhausting attempts")
	}
	if !strings.Contains(err.Error(), `resolve server host "okins.example.ru"`) {
		t.Fatalf("err=%q, want 'resolve server host' prefix", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v, want wrapped lookup error", err)
	}
	if calls != 4 {
		t.Fatalf("calls=%d, want 4 (all attempts used)", calls)
	}
}

func TestResolveServerIPv4_NoIPv4DoesNotRetry(t *testing.T) {
	calls := 0
	lookup := func(string) ([]net.IP, error) {
		calls++
		return []net.IP{net.ParseIP("2001:db8::1")}, nil // IPv6 only
	}
	_, err := resolveServerIPv4(lookup, "v6only.example.ru", 5, 0)
	if err == nil || !strings.Contains(err.Error(), `no IPv4 for server host "v6only.example.ru"`) {
		t.Fatalf("err=%v, want 'no IPv4 for server host'", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want 1 (a successful lookup with no v4 is terminal, not transient)", calls)
	}
}
