package bindings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/itg-team/itg-ray/internal/config"
	"github.com/stretchr/testify/require"
)

// TestApplyNetwork_HttpPortKeyRenamed ensures the new patch key writes
// SysProxy.HTTPPort and the old "xrayPort" key is silently ignored
// (forward-compat).
func TestApplyNetwork_HttpPortKeyRenamed(t *testing.T) {
	c := config.Config{}
	applyNetwork(&c.Network, map[string]any{"httpPort": float64(9090)})
	require.Equal(t, 9090, c.Network.SysProxy.HTTPPort)

	c2 := config.Config{}
	applyNetwork(&c2.Network, map[string]any{"xrayPort": float64(9091)})
	require.Equal(t, 0, c2.Network.SysProxy.HTTPPort, "old xrayPort key must be ignored")
}

// TestApplyNotifications_QuotaLowKeyRenamed mirrors the network case.
func TestApplyNotifications_QuotaLowKeyRenamed(t *testing.T) {
	c := config.Config{}
	applyNotifications(&c.Notifications, map[string]any{"quotaLow": true})
	require.True(t, c.Notifications.QuotaLow)

	c2 := config.Config{}
	applyNotifications(&c2.Notifications, map[string]any{"onError": true})
	require.False(t, c2.Notifications.QuotaLow, "old onError key must be ignored")
}

// TestApplyNetwork_NewFields covers the slice-N additions in a single
// shot — tunCidr, tunMtu, allowLan, ipv6Mode, dnsMode, dnsServers — and
// verifies the dnsServers []any → []string filter drops empty entries.
func TestApplyNetwork_NewFields(t *testing.T) {
	c := config.Config{}
	applyNetwork(&c.Network, map[string]any{
		"defaultMode": "sysproxy",
		"tunCidr":     "10.0.0.1/24",
		"tunMtu":      float64(1400),
		"allowLan":    true,
		"ipv6Mode":    "disabled",
		"dnsMode":     "custom",
		"dnsServers":  []any{"1.1.1.1", "", nil, 42, "  9.9.9.9  ", "8.8.8.8"},
	})
	require.Equal(t, "sysproxy", c.Network.Mode)
	require.Equal(t, "10.0.0.1/24", c.Network.TUN.IPv4CIDR)
	require.Equal(t, 1400, c.Network.TUN.MTU)
	require.True(t, c.Network.AllowLAN)
	require.Equal(t, "disabled", c.Network.IPv6Mode)
	require.Equal(t, "custom", c.Network.DNS.Mode)
	require.Equal(t, []string{"1.1.1.1", "9.9.9.9", "8.8.8.8"}, c.Network.DNS.Servers)
}

// TestConfigStore_NormalizesLegacyAutoMode ensures an on-disk
// "mode": "auto" from pre-Tier-2a configs gets normalized to "tun"
// when surfaced via View(), so the now-removed Auto runtime branch
// is not silently exercised on upgrade.
func TestConfigStore_NormalizesLegacyAutoMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"network":{"mode":"auto"}}`), 0o600))
	store := NewConfigStore(path, "test", "test")
	view, err := store.View()
	require.NoError(t, err)
	require.Equal(t, "tun", view.Network.DefaultMode)
}
