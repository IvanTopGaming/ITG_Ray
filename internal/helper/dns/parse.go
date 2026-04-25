// Package dns parsing helpers shared across platforms. The Windows-specific
// netsh invocations live in dns_windows.go; this file holds the locale-tolerant
// output parser so it can be exercised by cross-platform tests.
package dns

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// ipv4Pattern matches dotted-quad IPv4 addresses anywhere in a line.
// Used by parseDNSAddresses to be locale-tolerant against netsh's
// localized labels (e.g. Russian "DNS-серверы с настройкой через DHCP:  192.168.2.1").
var ipv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// parseDNSAddresses scans netsh output for IPv4 addresses, one per match.
// Tolerates localized prose surrounding the IP — extracts all dotted-quad
// tokens regardless of preceding text.
func parseDNSAddresses(text string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		matches := ipv4Pattern.FindAllString(line, -1)
		for _, m := range matches {
			if !looksLikeValidIP(m) {
				continue
			}
			out = append(out, m)
		}
	}
	return out
}

// looksLikeValidIP rejects matches like "999.999.999.999" by parsing octets.
func looksLikeValidIP(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}
