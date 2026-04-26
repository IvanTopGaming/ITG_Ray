package sysproxy

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_PlatformBehaviour(t *testing.T) {
	m := New()
	if runtime.GOOS != "windows" {
		require.ErrorIs(t, m.Set("127.0.0.1:1080"), ErrUnsupported)
		require.NoError(t, m.Clear()) // safe no-op
	}
}
