package sysproxy

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_PlatformBehaviour(t *testing.T) {
	m := New()
	if runtime.GOOS != "windows" {
		require.NoError(t, m.Set(Settings{Socks: "127.0.0.1:1080"}))
		require.NoError(t, m.Clear())
	}
}
