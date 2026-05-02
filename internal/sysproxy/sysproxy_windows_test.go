//go:build windows

package sysproxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/registry"
)

func TestWindows_RoundTrip(t *testing.T) {
	m := New()
	pre, _ := m.IsSet()
	t.Cleanup(func() {
		if pre {
			return
		}
		_ = m.Clear()
	})
	require.NoError(t, m.Set(Settings{Socks: "127.0.0.1:65432"}))
	on, err := m.IsSet()
	require.NoError(t, err)
	require.True(t, on)
	require.NoError(t, m.Clear())
	on, err = m.IsSet()
	require.NoError(t, err)
	require.False(t, on)
}

func TestWindows_Set_TriProtocolString(t *testing.T) {
	m := New()
	t.Cleanup(func() { _ = m.Clear() })
	require.NoError(t, m.Set(Settings{Socks: "127.0.0.1:1090", HTTP: "127.0.0.1:8889"}))

	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	require.NoError(t, err)
	defer func() { _ = k.Close() }()
	v, _, err := k.GetStringValue("ProxyServer")
	require.NoError(t, err)
	require.True(t, strings.Contains(v, "socks=127.0.0.1:1090"), "missing socks segment: %s", v)
	require.True(t, strings.Contains(v, "http=127.0.0.1:8889"), "missing http segment: %s", v)
	require.True(t, strings.Contains(v, "https=127.0.0.1:8889"), "missing https segment: %s", v)
}

func TestWindows_Set_OnlySocks(t *testing.T) {
	m := New()
	t.Cleanup(func() { _ = m.Clear() })
	require.NoError(t, m.Set(Settings{Socks: "127.0.0.1:1090"}))

	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	require.NoError(t, err)
	defer func() { _ = k.Close() }()
	v, _, err := k.GetStringValue("ProxyServer")
	require.NoError(t, err)
	require.Equal(t, "socks=127.0.0.1:1090", v)
}

func TestWindows_Set_BothEmpty_ClearsRegistry(t *testing.T) {
	m := New()
	t.Cleanup(func() { _ = m.Clear() })
	require.NoError(t, m.Set(Settings{Socks: "127.0.0.1:1090"}))
	require.NoError(t, m.Set(Settings{}))
	on, err := m.IsSet()
	require.NoError(t, err)
	require.False(t, on)
}
