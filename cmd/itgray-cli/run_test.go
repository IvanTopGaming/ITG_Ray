package main

import (
	"net"
	"strings"
	"testing"
)

// TestResolveServerIPv4_IPLiteral verifies that an IPv4 literal input
// is returned as-is, since net.LookupIP short-circuits IP literals.
// This is the most reliable assertion (no system resolver dependency).
func TestResolveServerIPv4_IPLiteral(t *testing.T) {
	got, err := resolveServerIPv4("8.8.8.8")
	if err != nil {
		t.Fatalf("resolveServerIPv4(\"8.8.8.8\") error: %v", err)
	}
	if got != "8.8.8.8" {
		t.Fatalf("got %q, want %q", got, "8.8.8.8")
	}
}

// TestResolveServerIPv4_Localhost asserts the helper resolves "localhost"
// successfully and returns something that parses as IPv4. We don't pin a
// specific octet value because /etc/hosts on some systems maps localhost
// to ::1 first, but To4() filtering should still find an IPv4 entry.
// If "localhost" has no IPv4 mapping at all (very unusual CI env), skip.
func TestResolveServerIPv4_Localhost(t *testing.T) {
	got, err := resolveServerIPv4("localhost")
	if err != nil {
		// Not all minimal CI environments resolve "localhost" to IPv4.
		t.Skipf("system resolver unable to resolve localhost to IPv4: %v", err)
	}
	if !strings.Contains(got, ".") {
		t.Fatalf("expected IPv4-looking string, got %q", got)
	}
	if ip := net.ParseIP(got); ip == nil || ip.To4() == nil {
		t.Fatalf("expected parseable IPv4, got %q", got)
	}
}

// TestResolveServerIPv4_NoSuchHost verifies a clean error path when the
// host does not resolve. We use a TLD reserved for documentation/invalid
// names per RFC 6761 ("invalid").
func TestResolveServerIPv4_NoSuchHost(t *testing.T) {
	_, err := resolveServerIPv4("itgray-nonexistent-host.invalid")
	if err == nil {
		t.Fatal("expected error for nonexistent host, got nil")
	}
}
