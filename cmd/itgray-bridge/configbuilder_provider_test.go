package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/chainctl"
	"github.com/itg-team/itg-ray/internal/routebind"
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func otherSrv(id, ip, source, name string) server.Server {
	s := boundSrv(id, ip)
	s.SourceID = source
	s.Name = name
	return s
}

func providerRule(id, serverName string) rules.Rule {
	return rules.Rule{ID: id, Name: id, Enabled: true, Action: rules.ActionProxy, ServerSourceID: "sub", ServerName: serverName,
		Conditions: rules.Conditions{Processes: []string{"app-" + id}}}
}

func newProviderFixture(t *testing.T, servers []server.Server, rs ...rules.Rule) boundFixture {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, server.Save(filepath.Join(dir, "servers.json"), servers))
	m := rules.Model{DefaultAction: rules.ActionProxy, Groups: []rules.Group{
		{ID: "user", Name: "Mine", Enabled: true},
		{ID: "src-sub", Name: "Provider", Locked: true, Enabled: true, SourceID: "sub", Rules: rs},
	}}
	require.NoError(t, rules.NewStore(dir).Save(m))
	return boundFixture{dataDir: dir, issues: &routebind.Issues{}, main: servers[0]}
}

func TestProviderRule_RoutesThroughNamedServerOfItsSubscription(t *testing.T) {
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")}, providerRule("p1", "srv x2"))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Equal(t, "proxy-x2", ruleOutbound(sb, "app-p1"))
}

func TestProviderRule_ServerOnlyInAnotherSubscriptionIsSkipped(t *testing.T) {
	f := newProviderFixture(t, []server.Server{
		otherSrv("m1", "203.0.113.1", "other", "Main"),
		otherSrv("x2", "203.0.113.2", "other", "NL"),
		otherSrv("m3", "203.0.113.3", "", "NL"),
	}, providerRule("p1", "NL"))
	sb, xr := f.build(t, chainctl.ModeTUN)
	require.Empty(t, ruleOutbound(sb, "app-p1"))
	require.Equal(t, 0, countPrefix(outboundTags(xr), "proxy-"))
}

func TestProviderRule_ForgedServerIDFromAnotherSubscriptionIsSkipped(t *testing.T) {
	forged := providerRule("p1", "NL")
	forged.ServerID = "x2"
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), otherSrv("x2", "203.0.113.2", "other", "NL")}, forged)
	sb, xr := f.build(t, chainctl.ModeTUN)
	require.Empty(t, ruleOutbound(sb, "app-p1"))
	require.Equal(t, 0, countPrefix(outboundTags(xr), "proxy-"))
}

func TestProviderRule_MissingServerFallsToGeneralRules(t *testing.T) {
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1")}, providerRule("p1", "gone"))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Empty(t, ruleOutbound(sb, "app-p1"))
}

func TestProviderRule_UserDisabledServerFallsToGeneralRules(t *testing.T) {
	x2 := boundSrv("x2", "203.0.113.2")
	x2.Disabled = true
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), x2}, providerRule("p1", "srv x2"))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Empty(t, ruleOutbound(sb, "app-p1"))
}

func TestProviderRule_DownServerNeverFallsBackToAnotherServer(t *testing.T) {
	orig := lookupServerIPv4
	t.Cleanup(func() { lookupServerIPv4 = orig })
	lookupServerIPv4 = func(host string) (string, error) {
		if host == "203.0.113.2" {
			return "", errors.New("no such host")
		}
		return orig(host)
	}
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1"), boundSrv("x2", "203.0.113.2")}, providerRule("p1", "srv x2"))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.True(t, strings.HasPrefix(ruleOutbound(sb, "app-p1"), "block"), ruleOutbound(sb, "app-p1"))
	sbProxy, _ := f.build(t, chainctl.ModeSysProxy)
	require.True(t, strings.HasPrefix(ruleOutbound(sbProxy, "app-p1"), "block"), ruleOutbound(sbProxy, "app-p1"))
}

func TestProviderRule_BoundToActiveServerUsesMainProxy(t *testing.T) {
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1")}, providerRule("p1", "srv m1"))
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Equal(t, "proxy", ruleOutbound(sb, "app-p1"))
}

func TestProviderRule_PlainProxyUsesActiveServer(t *testing.T) {
	plain := providerRule("p1", "")
	plain.ServerSourceID = ""
	f := newProviderFixture(t, []server.Server{boundSrv("m1", "203.0.113.1")}, plain)
	sb, _ := f.build(t, chainctl.ModeTUN)
	require.Equal(t, "proxy", ruleOutbound(sb, "app-p1"))
}
