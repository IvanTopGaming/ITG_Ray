package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaults_AllRequiredFieldsPopulated guards every field consumed by the
// Tier 2a frontend SettingsView projection. If a new field is added to the
// frontend Settings type, extend this list.
func TestDefaults_AllRequiredFieldsPopulated(t *testing.T) {
	c := defaults()

	require.NotEmpty(t, c.General.Language)
	require.NotEmpty(t, c.Network.Mode)
	require.NotEmpty(t, c.Network.IPv6Mode)
	require.NotEmpty(t, c.Network.DNS.Mode)
	require.NotZero(t, c.Network.TUN.IPv4CIDR)
	require.NotZero(t, c.Network.TUN.MTU)
	require.NotZero(t, c.Network.SysProxy.HTTPPort)
	require.NotZero(t, c.Network.SysProxy.SOCKSPort)
	require.NotEmpty(t, c.Debug.LogLevel)
	// Booleans intentionally omitted from "non-zero" checks — false is a
	// valid default for AllowLAN, Autostart, etc.
}

// TestLoad_OldConfigGetsDefaultsForMissingFields asserts the existing
// defaults⊕overlay merge in Load() populates fields the on-disk config
// does not yet carry.
func TestLoad_OldConfigGetsDefaultsForMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Simulate a pre-Tier-2a config — only general+network.mode present.
	old := []byte(`{
	  "version": 1,
	  "general": {"language": "ru", "theme": "dark"},
	  "network": {"mode": "sysproxy"}
	}`)
	require.NoError(t, os.WriteFile(path, old, 0o600))

	c, err := Load(path)
	require.NoError(t, err)

	// Disk-present fields preserved.
	require.Equal(t, "ru", c.General.Language)
	require.Equal(t, "sysproxy", c.Network.Mode)
	// New fields populated from defaults.
	require.Equal(t, "prefer-v4", c.Network.IPv6Mode)
	require.Equal(t, "auto", c.Network.DNS.Mode)
	require.Equal(t, "info", c.Debug.LogLevel)
	require.True(t, c.Notifications.Sound)
}

// TestLoad_PartialOverlayDoesNotResetFilledFields makes sure the overlay
// direction is correct: disk values win over defaults for fields they
// supply, and unrelated default fields are not zeroed.
func TestLoad_PartialOverlayDoesNotResetFilledFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	disk := []byte(`{"network": {"allow_lan": true}}`)
	require.NoError(t, os.WriteFile(path, disk, 0o600))

	c, err := Load(path)
	require.NoError(t, err)

	require.True(t, c.Network.AllowLAN)
	require.Equal(t, "tun", c.Network.Mode) // default preserved
}
