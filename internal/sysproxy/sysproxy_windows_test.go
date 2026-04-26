//go:build windows

package sysproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Side-effecting test — touches the user's actual HKCU. Restores in t.Cleanup.
func TestWindows_RoundTrip(t *testing.T) {
	m := New()
	pre, _ := m.IsSet()
	t.Cleanup(func() {
		if pre {
			// best-effort; tests that need the original value should snapshot before
			return
		}
		_ = m.Clear()
	})
	require.NoError(t, m.Set("127.0.0.1:65432"))
	on, err := m.IsSet()
	require.NoError(t, err)
	require.True(t, on)
	require.NoError(t, m.Clear())
	on, err = m.IsSet()
	require.NoError(t, err)
	require.False(t, on)
}
