//go:build windows

// Package dns configures Windows DNS settings on host adapters by shelling
// out to netsh. We deliberately avoid WMI: it adds COM/MIDL surface for
// little gain and netsh is the canonical, documented route.
package dns

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Settings is the per-interface DNS state.
type Settings struct {
	InterfaceAlias string   `json:"interface_alias"`
	Addresses      []string `json:"addresses"`
}

// Snapshot reads the current DNS server addresses for an interface.
func Snapshot(alias string) (Settings, error) {
	out, err := runNetsh("interface", "ipv4", "show", "dnsservers", `name=`+strconv.Quote(alias))
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		InterfaceAlias: alias,
		Addresses:      parseDNSAddresses(out),
	}, nil
}

// Set replaces the DNS server list for an interface. An empty addresses
// list reverts to DHCP.
func Set(s Settings) error {
	if len(s.Addresses) == 0 {
		_, err := runNetsh("interface", "ipv4", "set", "dnsservers", `name=`+strconv.Quote(s.InterfaceAlias), "source=dhcp")
		return err
	}
	if _, err := runNetsh("interface", "ipv4", "set", "dnsservers", `name=`+strconv.Quote(s.InterfaceAlias), "static", s.Addresses[0], "primary"); err != nil {
		return err
	}
	for i, a := range s.Addresses[1:] {
		if _, err := runNetsh("interface", "ipv4", "add", "dnsservers", `name=`+strconv.Quote(s.InterfaceAlias), "address="+a, fmt.Sprintf("index=%d", i+2)); err != nil {
			return err
		}
	}
	return nil
}

// Restore replaces the current DNS server list with the snapshot's.
func Restore(s Settings) error { return Set(s) }

func runNetsh(args ...string) (string, error) {
	cmd := exec.Command("netsh", args...) // #nosec G204 -- exec.Command does not spawn a shell; netsh handles its own arg parsing
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("netsh %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseDNSAddresses(text string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !looksLikeIP(line) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func looksLikeIP(s string) bool {
	dots := strings.Count(s, ".")
	if dots != 3 {
		return false
	}
	for _, r := range s {
		if r != '.' && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
