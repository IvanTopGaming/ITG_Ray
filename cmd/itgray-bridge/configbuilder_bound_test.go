package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

func boundSrv(id, ip string) server.Server {
	s := server.New(vless.Config{Address: ip, Port: 443, UUID: "00000000-0000-0000-0000-00000000000" + id[len(id)-1:], Transport: vless.TransportTCP, Security: vless.SecurityTLS, SNI: "x"}, server.OriginSubscription, "sub")
	s.ID = id
	s.Name = "srv " + id
	return s
}

type boundFixture struct {
	dataDir string
	issues  *routebind.Issues
	main    server.Server
}

func newBoundFixture(t *testing.T, servers []server.Server, rs ...rules.Rule) boundFixture {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, server.Save(filepath.Join(dir, "servers.json"), servers))
	m := rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{{ID: "g", Name: "g", Enabled: true, Rules: rs}}}
	require.NoError(t, rules.NewStore(dir).Save(m))
	return boundFixture{dataDir: dir, issues: &routebind.Issues{}, main: servers[0]}
}

func boundRule(id, serverID string, enabled bool) rules.Rule {
	return rules.Rule{ID: id, Name: id, Enabled: enabled, Action: rules.ActionProxy, ServerID: serverID, ServerSourceID: "sub", ServerName: "srv " + serverID, Conditions: rules.Conditions{Processes: []string{"app-" + id}}}
}

func (f boundFixture) build(t *testing.T, mode chainctl.Mode) (sb, xr map[string]any) {
	t.Helper()
	net := config.Network{
		Mode:     string(mode),
		TUN:      config.TUN{IPv4CIDR: "198.18.0.1/15", MTU: 1500},
		SysProxy: config.SysProxy{HTTPPort: 8888, SOCKSPort: 1080},
		IPv6Mode: "prefer-v4",
	}
	main := f.main
	sbJSON, xrJSON, err := buildConfigs(f.dataDir, "", nil, nil, f.issues)(&main, mode, net)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(sbJSON, &sb))
	require.NoError(t, json.Unmarshal(xrJSON, &xr))
	return sb, xr
}

func outboundTags(doc map[string]any) []string {
	var tags []string
	for _, o := range doc["outbounds"].([]any) {
		tags = append(tags, o.(map[string]any)["tag"].(string))
	}
	return tags
}

func ruleOutbound(sb map[string]any, process string) string {
	matches := func(m map[string]any) bool {
		ps, ok := m["process_name"].([]any)
		return ok && len(ps) > 0 && ps[0] == process
	}
	for _, r := range sb["route"].(map[string]any)["rules"].([]any) {
		m := r.(map[string]any)
		if matches(m) {
			return m["outbound"].(string)
		}
		if nested, ok := m["rules"].([]any); ok {
			for _, n := range nested {
				if matches(n.(map[string]any)) {
					return m["outbound"].(string)
				}
			}
		}
	}
	return ""
}

func countPrefix(tags []string, p string) int {
	n := 0
	for _, t := range tags {
		if strings.HasPrefix(t, p) {
			n++
		}
	}
	return n
}

func TestBuildConfigs_TwoRulesSameServerOneOutbound(t *testing.T) {
	f := newBoundFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")},
		boundRule("r1", "x2", true), boundRule("r2", "x2", true))
	sb, xr := f.build(t, chainctl.ModeTUN)
	require.Equal(t, 1, countPrefix(outboundTags(sb), "proxy-"))
	require.Equal(t, 1, countPrefix(outboundTags(xr), "proxy-"))
	require.Equal(t, "proxy-x2", ruleOutbound(sb, "app-r1"))
	require.Equal(t, "proxy-x2", ruleOutbound(sb, "app-r2"))
	require.NotNil(t, sb["experimental"].(map[string]any)["clash_api"])
}

func TestBuildConfigs_RuleBoundToMainCompilesToProxy(t *testing.T) {
	f := newBoundFixture(t, []server.Server{boundSrv("m1", "203.0.113.1")}, boundRule("r1", "m1", true))
	sb, xr := f.build(t, chainctl.ModeTUN)
	require.Equal(t, "proxy", ruleOutbound(sb, "app-r1"))
	require.Equal(t, 0, countPrefix(outboundTags(sb), "proxy-"))
	require.Equal(t, 0, countPrefix(outboundTags(xr), "proxy-"))
}

func TestBuildConfigs_DisabledRuleCreatesNoOutbound(t *testing.T) {
	dir := t.TempDir()
	servers := []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")}
	require.NoError(t, server.Save(filepath.Join(dir, "servers.json"), servers))
	m := rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{
		{ID: "g1", Name: "g1", Enabled: true, Rules: []rules.Rule{boundRule("r1", "x2", false)}},
		{ID: "g2", Name: "g2", Enabled: false, Rules: []rules.Rule{boundRule("r2", "x2", true)}},
	}}
	require.NoError(t, rules.NewStore(dir).Save(m))
	f := boundFixture{dataDir: dir, issues: &routebind.Issues{}, main: servers[0]}
	sb, xr := f.build(t, chainctl.ModeSysProxy)
	require.Equal(t, 0, countPrefix(outboundTags(sb), "proxy-"))
	require.Equal(t, 0, countPrefix(outboundTags(xr), "proxy-"))
}

func TestBuildConfigs_UnresolvableExtraFallsBack(t *testing.T) {
	orig := lookupServerIPv4
	t.Cleanup(func() { lookupServerIPv4 = orig })
	lookupServerIPv4 = func(host string) (string, error) {
		if host == "203.0.113.2" {
			return "", errors.New("no such host")
		}
		return orig(host)
	}
	f := newBoundFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")}, boundRule("r1", "x2", true))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Equal(t, "proxy", ruleOutbound(sb, "app-r1"))
	b := f.issues.Get()["r1"]
	require.Equal(t, routebind.StateUnusable, b.State)
	require.Equal(t, "dns", b.Reason)
}

func TestBuildConfigs_ExcludesEveryServerIP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("route_exclude_address is not used on windows")
	}
	f := newBoundFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")}, boundRule("r1", "x2", true))
	sb, _ := f.build(t, chainctl.ModeTUN)
	var excl []any
	for _, ib := range sb["inbounds"].([]any) {
		if m := ib.(map[string]any); m["type"] == "tun" {
			excl = m["route_exclude_address"].([]any)
		}
	}
	require.Contains(t, excl, "203.0.113.1/32")
	require.Contains(t, excl, "203.0.113.2/32")
}
