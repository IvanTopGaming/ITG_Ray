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

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
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

// nrptKeyPath is the local (non-GPO) NRPT store the DNS Client service
// reads; each direct child key is one rule and its name is arbitrary.
const nrptKeyPath = `SYSTEM\CurrentControlSet\Services\Dnscache\Parameters\DnsPolicyConfig`

// nrptOverrideDNS is the ConfigOptions bit that tells the resolver to send
// matching queries to this rule's GenericDNSServers (matches what
// Add-DnsClientNrptRule and Tailscale write).
const nrptOverrideDNS = 0x8

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
// displayName doubles as the rule's registry key name (the key name is
// arbitrary), so removal is a direct key delete. namespace="." matches
// every FQDN. Written directly to the registry instead of via
// Add-DnsClientNrptRule: the PowerShell cmdlet costs ~1s to spawn and it
// sits on the synchronous Connect path.
func AddNrptRule(displayName, namespace string, nameServers []string) error {
	if displayName == "" || namespace == "" || len(nameServers) == 0 {
		return errors.New("dns.AddNrptRule: displayName, namespace, and at least one nameServer required")
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, nrptKeyPath+`\`+displayName, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create NRPT rule %q: %w", displayName, err)
	}
	defer k.Close()
	if err := k.SetDWordValue("Version", 1); err != nil {
		return fmt.Errorf("set Version: %w", err)
	}
	if err := k.SetStringsValue("Name", []string{namespace}); err != nil {
		return fmt.Errorf("set Name: %w", err)
	}
	if err := k.SetStringValue("GenericDNSServers", strings.Join(nameServers, ";")); err != nil {
		return fmt.Errorf("set GenericDNSServers: %w", err)
	}
	if err := k.SetDWordValue("ConfigOptions", nrptOverrideDNS); err != nil {
		return fmt.Errorf("set ConfigOptions: %w", err)
	}
	return notifyDNSClient()
}

// RemoveNrptRule deletes the NRPT rule key of the given name. Idempotent:
// a missing key (rule never landed or already cleared) is not an error,
// so callers can use this on a rollback path.
func RemoveNrptRule(displayName string) error {
	if displayName == "" {
		return errors.New("dns.RemoveNrptRule: displayName required")
	}
	err := registry.DeleteKey(registry.LOCAL_MACHINE, nrptKeyPath+`\`+displayName)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete NRPT rule %q: %w", displayName, err)
	}
	return notifyDNSClient()
}

// notifyDNSClient forces the DNS Client (Dnscache) service to reload the
// NRPT from the registry. Raw registry writes are otherwise ignored — the
// policy is cached in the service's memory — so a SERVICE_CONTROL_PARAMCHANGE
// (the API equivalent of `sc control DnsCache paramchange`) is required.
func notifyDNSClient() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("SCM connect: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService("Dnscache")
	if err != nil {
		return fmt.Errorf("open Dnscache: %w", err)
	}
	defer s.Close()
	if _, err := s.Control(svc.ParamChange); err != nil {
		return fmt.Errorf("Dnscache paramchange: %w", err)
	}
	return nil
}
