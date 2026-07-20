package sysproxy

import (
	"errors"
	"fmt"
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

// TestErrNotifyOnly_DistinguishableViaErrorsIs pins the error-classification
// contract winManager.Set relies on: a notifyWinInet-only failure (registry
// state fully correct) must remain identifiable via errors.Is after being
// wrapped alongside the underlying cause, so callers can treat it
// differently from a real registry-write failure. The registry-writing
// code itself is Windows-only and untestable here; this test exercises the
// platform-independent wrapping/matching contract those call sites depend
// on.
func TestErrNotifyOnly_DistinguishableViaErrorsIs(t *testing.T) {
	cause := errors.New("wininet: load failed")
	wrapped := fmt.Errorf("sysproxy.Set: %w: %w", ErrNotifyOnly, cause)

	require.True(t, errors.Is(wrapped, ErrNotifyOnly),
		"a notify-only Set failure must be distinguishable via errors.Is")
	require.True(t, errors.Is(wrapped, cause),
		"the underlying wininet cause must still be reachable via errors.Is")

	realWriteFailure := fmt.Errorf("sysproxy.Set: ProxyServer: %w", errors.New("access denied"))
	require.False(t, errors.Is(realWriteFailure, ErrNotifyOnly),
		"a real registry-write failure must NOT be classified as notify-only")
}
