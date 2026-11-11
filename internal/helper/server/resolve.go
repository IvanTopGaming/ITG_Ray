package server

import (
	"fmt"
	"net"
	"time"
)

// resolveServerIPv4 resolves host to an IPv4 literal for the Windows
// peer-route, retrying transient lookup failures with a fixed backoff.
//
// On a reconnect the previous chain's sing-box auto_route + DNS hijack
// (NRPT) is torn down immediately before StartChain runs, so the very
// first net.LookupIP can hit a system resolver that has not yet settled
// back to the physical adapter and returns "no such host". A manual
// retry from the orb worked for exactly that reason — the network had
// recovered by then. Retrying here closes that window without forcing a
// full chain rebuild.
//
// A successful lookup that yields no IPv4 record is terminal (not a
// transient DNS fault), so it is returned immediately without spinning.
func resolveServerIPv4(lookup func(string) ([]net.IP, error), host string, attempts int, delay time.Duration) (net.IP, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lookupErr error
	for i := 0; i < attempts; i++ {
		ips, err := lookup(host)
		if err == nil {
			for _, ip := range ips {
				if v4 := ip.To4(); v4 != nil {
					return v4, nil
				}
			}
			return nil, fmt.Errorf("no IPv4 for server host %q", host)
		}
		lookupErr = err
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	return nil, fmt.Errorf("resolve server host %q: %w", host, lookupErr)
}

func extraPeerCIDRs(lookup func(string) ([]net.IP, error), main net.IP, hosts []string, attempts int, delay time.Duration) ([]string, error) {
	seen := map[string]bool{}
	if main != nil {
		seen[main.String()] = true
	}
	var out []string
	for _, h := range hosts {
		ip := net.ParseIP(h).To4()
		if ip == nil {
			resolved, err := resolveServerIPv4(lookup, h, attempts, delay)
			if err != nil {
				return nil, err
			}
			ip = resolved
		}
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key+"/32")
	}
	return out, nil
}
