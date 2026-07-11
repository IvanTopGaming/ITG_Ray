package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_LoadDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing.json")
	c, err := Load(p)
	require.NoError(t, err)
	require.Equal(t, "en", c.General.Language)
	require.True(t, c.KillSwitch.Enabled)
	require.Equal(t, "tun", c.Network.Mode)
	require.Equal(t, 1500, c.Network.TUN.MTU)
}

func TestDefaults_GeoSource(t *testing.T) {
	c := defaults()
	if c.Network.GeoSource.Preset != "runetfreedom" {
		t.Fatalf("GeoSource.Preset default = %q", c.Network.GeoSource.Preset)
	}
	if c.Network.GeoSource.CustomURL != "" {
		t.Fatalf("GeoSource.CustomURL default = %q", c.Network.GeoSource.CustomURL)
	}
}

func TestConfig_SaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	c, _ := Load(p)
	c.General.Language = "ru"
	c.KillSwitch.AlwaysOn = true
	require.NoError(t, Save(p, c))

	c2, err := Load(p)
	require.NoError(t, err)
	require.Equal(t, "ru", c2.General.Language)
	require.True(t, c2.KillSwitch.AlwaysOn)
}
