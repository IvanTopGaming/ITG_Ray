package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/geo"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

// Default port assignments for non-user-configurable internal endpoints.
// SOCKS/HTTP inbound ports are now sourced from config.Network.SysProxy
// (Tier 2b); TUN CIDR is sourced from config.Network.TUN.IPv4CIDR.
const (
	defaultXrayPort = 1081 // internal SOCKS5 between sing-box and xray
	defaultTunName  = "ITGRay-TUN"
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

// loadRulesFromDataDir reads the user's rules.json via rules.Store,
// returning the canonical Safety + My Rules default when the file is
// missing or corrupt. The historical inline default now lives in
// rules.DefaultModel so the RulesService bindings and the chain
// builder agree on the same on-disk shape.
func loadRulesFromDataDir(dataDir string, store *rules.Store) rules.Model {
	if store != nil {
		m, _ := store.Load()
		return m
	}
	s := rules.NewStore(dataDir)
	m, _ := s.Load()
	return m
}

// buildConfigs is the chainctl.ConfigBuilder closure for the running GUI
// process. It produces the (singboxJSON, xrayJSON) pair the Helper needs
// to bring the chain up. Mode mapping:
//   - chainctl.ModeTUN   → configgen.ModeTun (FakeIP, TunName/CIDR/MTU set)
//   - chainctl.ModeSysProxy → configgen.ModeSysProxy (socks + http inbounds, or mixed fallback)
//
// Network values come from chainctl's per-Connect read of config.Network
// (Tier 2b runtime wiring). chainctl.{ClampMTU,ResolveDNS,MapIPv6Strategy}
// translate user-facing knobs into sing-box-compatible shapes.
func buildConfigs(dataDir, configPath string, store *rules.Store, geoMgr *geo.Manager) chainctl.ConfigBuilder {
	return func(srv *server.Server, mode chainctl.Mode, net config.Network) (singboxJSON, xrayJSON []byte, err error) {
		logLevel := ""
		if c, lerr := config.Load(configPath); lerr == nil {
			logLevel = c.Debug.LogLevel
		}
		serverIP, err := resolveServerIPv4(srv.Vless.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve server host %q: %w", srv.Vless.Address, err)
		}

		model := loadRulesFromDataDir(dataDir, store)
		geoSets := map[string]string{}
		if tags := configgen.GeoTags(model); len(tags) > 0 && geoMgr != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			geoSets, err = geoMgr.Resolve(ctx, geo.Source{
				Preset:    net.GeoSource.Preset,
				CustomURL: net.GeoSource.CustomURL,
			}, tags)
			cancel()
			if err != nil {
				return nil, nil, fmt.Errorf("geo resolve: %w", err)
			}
		}

		var sbInput configgen.SingboxInput
		switch mode {
		case chainctl.ModeTUN:
			sbInput = configgen.SingboxInput{
				Mode:                configgen.ModeTun,
				FakeIP:              true,
				TunName:             defaultTunName,
				TunIPv4:             net.TUN.IPv4CIDR,
				MTU:                 chainctl.ClampMTU(net.TUN.MTU),
				XraySOCKSHost:       "127.0.0.1",
				XraySOCKSPort:       defaultXrayPort,
				DNSUpstreams:        chainctl.ResolveDNS(net.DNS),
				AllowLAN:            net.AllowLAN,
				IPv6Strategy:        chainctl.MapIPv6Strategy(net.IPv6Mode),
				GeoRuleSets:         geoSets,
				Rules:               model,
				LogLevel:            logLevel,
				RouteExcludeAddress: serverExcludeForTUN(serverIP),
			}
		case chainctl.ModeSysProxy:
			sbInput = configgen.SingboxInput{
				Mode:             configgen.ModeSysProxy,
				SocksInboundPort: net.SysProxy.SOCKSPort,
				HTTPInboundPort:  net.SysProxy.HTTPPort,
				XraySOCKSHost:    "127.0.0.1",
				XraySOCKSPort:    defaultXrayPort,
				DNSUpstreams:     chainctl.ResolveDNS(net.DNS),
				AllowLAN:         net.AllowLAN,
				IPv6Strategy:     chainctl.MapIPv6Strategy(net.IPv6Mode),
				GeoRuleSets:      geoSets,
				Rules:            model,
				LogLevel:         logLevel,
			}
		default:
			return nil, nil, fmt.Errorf("buildConfigs: unknown mode %q", mode)
		}

		sb, err := configgen.BuildSingbox(&sbInput)
		if err != nil {
			return nil, nil, fmt.Errorf("BuildSingbox: %w", err)
		}
		vc := srv.Vless
		if _, nerr := vc.Normalize(); nerr != nil {
			return nil, nil, fmt.Errorf("server %q has an incompatible config: %w", srv.Name, nerr)
		}
		xr, err := configgen.BuildXray(&configgen.XrayInput{
			Server:    vc,
			ServerIP:  serverIP,
			SocksPort: defaultXrayPort,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("BuildXray: %w", err)
		}
		return sb, xr, nil
	}
}
