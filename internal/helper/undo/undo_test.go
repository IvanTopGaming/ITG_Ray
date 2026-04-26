package undo

import (
	"path/filepath"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/route"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "undo.json")
	in := Journal{
		TunName: "ITGRay-TUN",
		Routes: []route.Entry{
			{DestCIDR: "0.0.0.0/0", NextHop: "198.18.0.2", InterfaceLUID: 99, Metric: 0},
		},
		DNSPrior: []dns.Settings{
			{InterfaceAlias: "Ethernet", Addresses: []string{"192.168.1.1"}},
		},
	}
	require.NoError(t, Save(path, in))
	got, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, in, got)
}

func TestLoadMissingReturnsZero(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	require.NoError(t, err)
	require.Empty(t, got.TunName)
	require.Empty(t, got.Routes)
	require.Empty(t, got.DNSPrior)
}

func TestClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "undo.json")
	require.NoError(t, Save(path, Journal{TunName: "x"}))
	require.NoError(t, Clear(path))
	got, err := Load(path)
	require.NoError(t, err)
	require.Empty(t, got.TunName)
}
