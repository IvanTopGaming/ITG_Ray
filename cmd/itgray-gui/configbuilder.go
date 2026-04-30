package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/chainctl"
	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

// Default port assignments mirror cmd/itgray-cli/run.go's defaults so the
// GUI and CLI run with identical inbound/outbound topology.
const (
	defaultSocksPort = 1080 // sysproxy mode SOCKS5 inbound on sing-box
	defaultXrayPort  = 1081 // internal SOCKS5 between sing-box and xray
	defaultTunName   = "ITGRay-TUN"
	defaultTunCIDR   = "198.18.0.1/15"
)

// resolveServerIPv4 resolves host to an IPv4 literal. Mirrors the CLI
// version: lookup runs BEFORE any TUN/DNS hijack is installed so the
// query goes through the host's normal LAN DNS and we get a real IP, not
// a FakeIP synthetic. If host is already an IP literal LookupIP returns
// it as-is.
func resolveServerIPv4(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address for server host %q", host)
}

// loadRulesFromDataDir mirrors cmd/itgray-cli/rules.go's loadRules: a
// missing file means "use the default safety+user model", an unparsable
// file degrades silently to "proxy by default" so a corrupt rules.json
// can't block Connect. The GUI rule editor will land in a later task; for
// now this is the source of routing model truth.
func loadRulesFromDataDir(dataDir string) rules.Model {
	path := filepath.Join(dataDir, "rules.json")
	b, err := os.ReadFile(path) //nolint:gosec // dataDir is controlled by the user-config dir resolver
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rules.Model{
				DefaultAction: rules.ActionProxy,
				Groups: []rules.Group{
					{ID: "safety", Name: "Safety", Locked: true, Enabled: true, Rules: []rules.Rule{
						{ID: "private", Name: "Private IPs", Enabled: true, Action: rules.ActionDirect,
							Conditions: rules.Conditions{IPCIDRs: []string{
								"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8",
								"fc00::/7", "fe80::/10", "224.0.0.0/4",
							}}},
					}},
					{ID: "user", Name: "My Rules", Enabled: true},
				},
			}
		}
		return rules.Model{DefaultAction: rules.ActionProxy}
	}
	var m rules.Model
	_ = json.Unmarshal(b, &m)
	return m
}

// buildConfigs is the chainctl.ConfigBuilder closure for the running GUI
// process. It produces the (singboxJSON, xrayJSON) pair the Helper needs
// to bring the chain up. Mode mapping:
//   - chainctl.ModeTUN   → configgen.ModeTun (FakeIP, TunName/CIDR set)
//   - chainctl.ModeSysProxy → configgen.ModeSysProxy (mixed inbound)
func buildConfigs(dataDir string) chainctl.ConfigBuilder {
	return func(srv *server.Server, mode chainctl.Mode) (singboxJSON, xrayJSON []byte, err error) {
		var sbInput configgen.SingboxInput
		switch mode {
		case chainctl.ModeTUN:
			sbInput = configgen.SingboxInput{
				Mode:          configgen.ModeTun,
				FakeIP:        true,
				TunName:       defaultTunName,
				TunIPv4:       defaultTunCIDR,
				XraySOCKSHost: "127.0.0.1",
				XraySOCKSPort: defaultXrayPort,
				Rules:         loadRulesFromDataDir(dataDir),
			}
		case chainctl.ModeSysProxy:
			sbInput = configgen.SingboxInput{
				Mode:             configgen.ModeSysProxy,
				SocksInboundPort: defaultSocksPort,
				XraySOCKSHost:    "127.0.0.1",
				XraySOCKSPort:    defaultXrayPort,
				Rules:            loadRulesFromDataDir(dataDir),
			}
		default:
			return nil, nil, fmt.Errorf("buildConfigs: unknown mode %q", mode)
		}

		sb, err := configgen.BuildSingbox(&sbInput)
		if err != nil {
			return nil, nil, fmt.Errorf("BuildSingbox: %w", err)
		}
		serverIP, err := resolveServerIPv4(srv.Vless.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve server host %q: %w", srv.Vless.Address, err)
		}
		xr, err := configgen.BuildXray(&configgen.XrayInput{
			Server:    srv.Vless,
			ServerIP:  serverIP,
			SocksPort: defaultXrayPort,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("BuildXray: %w", err)
		}
		return sb, xr, nil
	}
}
