package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/supervisor"

	"github.com/stretchr/testify/require"
)

type failedCloseStats struct{ calls int }

func (f *failedCloseStats) Counters(context.Context) (uint64, uint64, error) { return 0, 0, nil }
func (f *failedCloseStats) Close() error                                     { f.calls++; return errors.New("already closed") }

func TestStopPartialFailureRetainsCleanupWithoutReportingRunning(t *testing.T) {
	stats := &failedCloseStats{}
	sess := &chainState{sessionID: "owned", xrayAPI: stats}
	withActiveSess(t, sess)
	handler := NewStopChainHandler()
	raw, err := handler(context.Background(), json.RawMessage(`{"session_id":"owned"}`))
	require.NoError(t, err)
	require.Contains(t, string(raw), "partial_errors")
	chainMu.Lock()
	retained := activeSess == sess
	chainMu.Unlock()
	require.True(t, retained, "failed cleanup must remain retryable")
	require.False(t, IsChainActive(), "retained cleanup must not be adopted as a running VPN")
	status, err := NewServiceStatusHandler("test", time.Now(), IsChainActive)(context.Background(), nil)
	require.NoError(t, err)
	require.Contains(t, string(status), `"cleanup_pending":true`)
	_, _, running := readChainCounters(context.Background())
	require.False(t, running)
	_, err = NewStartChainHandler()(context.Background(), json.RawMessage(`{"server_host":"192.0.2.1","server_port":443,"mode":"sysproxy"}`))
	require.Error(t, err, "new chain must not discard pending cleanup")
	raw, err = handler(context.Background(), json.RawMessage(`{"session_id":"owned"}`))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "partial_errors")
	require.Equal(t, 1, stats.calls, "closed API must not be closed repeatedly")
	chainMu.Lock()
	cleared := activeSess == nil
	chainMu.Unlock()
	require.True(t, cleared)
}

func TestStopRetriesOnlyUnfinishedCores(t *testing.T) {
	original := stopChainCores
	t.Cleanup(func() { stopChainCores = original })
	calls := 0
	stopChainCores = func(_ time.Duration, xray, singbox coreStopper) (error, error) {
		calls++
		if calls == 1 {
			require.NotNil(t, xray)
			require.NotNil(t, singbox)
			return nil, errors.New("process still alive")
		}
		require.Nil(t, xray)
		require.NotNil(t, singbox)
		return nil, nil
	}
	sess := &chainState{sessionID: "owned", xray: &supervisor.Child{}, singbox: &supervisor.Child{}}
	withActiveSess(t, sess)
	require.Error(t, StopActiveChain())
	require.False(t, IsChainActive())
	chainMu.Lock()
	require.Same(t, sess, activeSess)
	require.Nil(t, sess.xray)
	require.NotNil(t, sess.singbox)
	chainMu.Unlock()
	require.NoError(t, StopActiveChain())
	require.Equal(t, 2, calls)
	chainMu.Lock()
	require.Nil(t, activeSess)
	chainMu.Unlock()
}
