package main

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/clashapi"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/geo"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

var lookupServerIPv4 = resolveServerIPv4

type boundPlan struct {
	xray        []configgen.XrayServer
	proxies     []configgen.SingboxProxy
	tagByServer map[string]string
	ips         []string
}

func planBoundServers(model rules.Model, servers []server.Server, mainID string, bindings map[string]routebind.Binding) boundPlan {
	plan := boundPlan{tagByServer: map[string]string{}}
	byID := make(map[string]*server.Server, len(servers))
	for i := range servers {
		byID[servers[i].ID] = &servers[i]
	}
	failed := map[string]string{}
	for _, g := range model.Groups {
		if !g.Enabled {
			continue
		}
		for _, r := range g.Rules {
			if !r.Enabled || r.Action != rules.ActionProxy || r.ServerID == "" || r.ServerID == mainID {
				continue
			}
			b, ok := bindings[r.ID]
			if !ok || (b.State != routebind.StateOK && b.State != routebind.StateRebound) {
				continue
			}
			if _, done := plan.tagByServer[r.ServerID]; done {
				continue
			}
			if reason, bad := failed[r.ServerID]; bad {
				b.State, b.Reason = routebind.StateUnusable, reason
				bindings[r.ID] = b
				continue
			}
			reason := ""
			srv := byID[r.ServerID]
			var vc vless.Config
			var ip string
			if srv == nil {
				reason = "missing"
			} else {
				vc = srv.Vless
				if _, err := vc.Normalize(); err != nil {
					reason = "config"
				} else if resolved, err := lookupServerIPv4(vc.Address); err != nil {
					reason = "dns"
				} else {
					ip = resolved
				}
			}
			if reason != "" {
				failed[r.ServerID] = reason
				b.State, b.Reason = routebind.StateUnusable, reason
				bindings[r.ID] = b
				continue
			}
			tag := configgen.ExtraTag(r.ServerID)
			port := defaultXrayPort + 1 + len(plan.xray)
			plan.tagByServer[r.ServerID] = tag
			plan.xray = append(plan.xray, configgen.XrayServer{Tag: tag, Server: vc, ServerIP: ip, SocksPort: port})
			plan.proxies = append(plan.proxies, configgen.SingboxProxy{Tag: tag, Port: port})
			plan.ips = append(plan.ips, ip)
		}
	}
	for id, b := range bindings {
		if b.State == routebind.StateOK || b.State == routebind.StateRebound {
			if reason, bad := failed[b.ServerID]; bad {
				b.State, b.Reason = routebind.StateUnusable, reason
				bindings[id] = b
			}
		}
	}
	return plan
}

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
//   - chainctl.ModeTUN   → configgen.ModeTun (FakeIP, TunName/CIDR/MTU set,
//     plus the same loopback socks/http inbounds sysproxy mode exposes so
//     explicitly-proxy-configured apps have an endpoint to target)
//   - chainctl.ModeSysProxy → configgen.ModeSysProxy (socks + http inbounds, or mixed fallback)
//
// Network values come from chainctl's per-Connect read of config.Network
// (Tier 2b runtime wiring). chainctl.{ClampMTU,ResolveDNS,MapIPv6Strategy}
// translate user-facing knobs into sing-box-compatible shapes.
func buildConfigs(dataDir, configPath string, store *rules.Store, geoMgr *geo.Manager, issues *routebind.Issues) chainctl.ConfigBuilder {
	return func(srv *server.Server, mode chainctl.Mode, net config.Network) (singboxJSON, xrayJSON []byte, err error) {
		logLevel := ""
		if c, lerr := config.Load(configPath); lerr == nil {
			logLevel = c.Debug.LogLevel
		}
		serverIP, err := lookupServerIPv4(srv.Vless.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve server host %q: %w", srv.Vless.Address, err)
		}

		servers, _ := server.Load(filepath.Join(dataDir, "servers.json"))
		model, bindings := routebind.Resolve(loadRulesFromDataDir(dataDir, store), servers)
		plan := planBoundServers(model, servers, srv.ID, bindings)
		secret, err := clashapi.LoadOrCreateSecret(dataDir)
		if err != nil {
			return nil, nil, err
		}
		clash := &configgen.ClashAPI{Port: configgen.ClashAPIPort, Secret: secret}
		outboundFor := func(r *rules.Rule) string { return plan.tagByServer[r.ServerID] }
		exclude := serverExcludeForTUN(serverIP)
		if len(exclude) > 0 {
			for _, ip := range plan.ips {
				exclude = append(exclude, ip+"/32")
			}
		}
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
				SocksInboundPort:    net.SysProxy.SOCKSPort,
				HTTPInboundPort:     net.SysProxy.HTTPPort,
				TunName:             defaultTunName,
				TunIPv4:             net.TUN.IPv4CIDR,
				MTU:                 chainctl.ClampMTU(net.TUN.MTU),
				XraySOCKSHost:       "127.0.0.1",
				XraySOCKSPort:       defaultXrayPort,
				DNSUpstreams:        chainctl.ResolveDNS(net.DNS),
				AllowLAN:            net.AllowLAN,
				IPv6Strategy:        chainctl.MapIPv6Strategy(net.IPv6Mode),
				TunIPv6:             chainctl.TunIPv6OrDefault(net.TUN.IPv6CIDR),
				FakeIPv6Range:       chainctl.FakeIPv6Range(net.IPv6Mode),
				GeoRuleSets:         geoSets,
				Rules:               model,
				LogLevel:            logLevel,
				RouteExcludeAddress: exclude,
				ExtraProxies:        plan.proxies,
				RuleOutbound:        outboundFor,
				ClashAPI:            clash,
				MarkRules:           true,
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
				ExtraProxies:     plan.proxies,
				RuleOutbound:     outboundFor,
				ClashAPI:         clash,
				MarkRules:        true,
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
			Extra:     plan.xray,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("BuildXray: %w", err)
		}
		issues.Set(bindings)
		return sb, xr, nil
	}
}
