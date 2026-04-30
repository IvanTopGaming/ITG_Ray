package bindings

import (
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
