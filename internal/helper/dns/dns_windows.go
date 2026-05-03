//go:build windows

// Package dns configures Windows DNS settings on host adapters by shelling
// out to netsh. We deliberately avoid WMI: it adds COM/MIDL surface for
// little gain and netsh is the canonical, documented route.
package dns

import (
	"errors"
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

// AddNrptRule installs a Name Resolution Policy Table rule that forces
// every DNS query matching the namespace to the given nameServers,
// bypassing per-adapter resolver bindings.
//
// We need this because sing-box's auto_route sets the TUN adapter's DNS
// to its own gateway IP (e.g. 198.18.0.2), but DNS packets destined for
// the TUN gateway IP itself are terminated by gVisor's stack rather
// than passing through the route engine — so the route-rule
// `{protocol:dns, action:hijack-dns}` never fires and FakeIP is silent.
// Pointing NRPT at a transit IP (1.1.1.1) makes Windows send DNS via
// TUN to that destination; the packet now traverses the route engine,
// hijack-dns fires, FakeIP responds. Without NRPT the parallel-resolver
// race against ISP DNS is non-deterministic and leaks domain queries
// to the ISP whenever it answers first.
//
// displayName is the user-visible label written to the rule (we
// search by it for removal). namespace="." matches every FQDN.
func AddNrptRule(displayName, namespace string, nameServers []string) error {
	if displayName == "" || namespace == "" || len(nameServers) == 0 {
		return errors.New("dns.AddNrptRule: displayName, namespace, and at least one nameServer required")
	}
	servers := strings.Join(quoteAll(nameServers), ",")
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", //nolint:gosec // displayName/namespace/servers are caller-controlled (helper-internal); single-quoted in the script.
		fmt.Sprintf(`Add-DnsClientNrptRule -Namespace '%s' -NameServers @(%s) -DisplayName '%s' -ErrorAction Stop`,
			psQuote(namespace), servers, psQuote(displayName)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Add-DnsClientNrptRule: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveNrptRule deletes any NRPT rule with the given DisplayName.
// Idempotent: succeeds silently if no matching rule exists, so callers
// can use this on a rollback path even when AddNrptRule never landed.
func RemoveNrptRule(displayName string) error {
	if displayName == "" {
		return errors.New("dns.RemoveNrptRule: displayName required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", //nolint:gosec // displayName is caller-controlled (helper-internal); single-quoted in the script.
		fmt.Sprintf(`Get-DnsClientNrptRule | Where-Object { $_.DisplayName -eq '%s' } | Remove-DnsClientNrptRule -Force -ErrorAction Stop`,
			psQuote(displayName)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Remove-DnsClientNrptRule(%s): %w (%s)", displayName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// psQuote escapes a string for embedding inside a PowerShell single-quoted
// literal: doubles any embedded single-quote.
func psQuote(s string) string { return strings.ReplaceAll(s, `'`, `''`) }

// quoteAll wraps each element of in[] in single-quotes after psQuote
// escaping, so the result can be joined with commas inside @(...).
func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = "'" + psQuote(s) + "'"
	}
	return out
}
