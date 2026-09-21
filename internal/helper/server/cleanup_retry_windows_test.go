package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/route"
	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

type failedCloseStats struct{ calls int }

func (f *failedCloseStats) Counters(context.Context) (uint64, uint64, error) { return 0, 0, nil }
func (f *failedCloseStats) Close() error                                     { f.calls++; return errors.New("already closed") }

func TestStopPartialFailurePreservesOnlyPendingWindowsResources(t *testing.T) {
	originalStop, originalDNS, originalNRPT := stopChainCores, restoreChainDNS, removeChainNRPT
	originalRemove, originalSnapshot, originalAdd, originalUndo := removeChainRoute, snapshotChainRoutes, addChainRoute, clearChainUndo
	t.Cleanup(func() {
		stopChainCores, restoreChainDNS, removeChainNRPT = originalStop, originalDNS, originalNRPT
		removeChainRoute, snapshotChainRoutes, addChainRoute, clearChainUndo = originalRemove, originalSnapshot, originalAdd, originalUndo
	})
	stopCalls, dnsCalls, nrptCalls, removeCalls, snapshotCalls, addCalls, undoCalls := 0, 0, 0, 0, 0, 0, 0
	stopChainCores = func(_ time.Duration, xray, singbox coreStopper) (error, error) {
		stopCalls++
		if stopCalls == 1 {
			return nil, errors.New("core still alive")
		}
		require.Nil(t, xray)
		require.NotNil(t, singbox)
		return nil, nil
	}
	restoreChainDNS = func(dns.Settings) error {
		dnsCalls++
		if dnsCalls == 1 {
			return errors.New("dns unavailable")
		}
		return nil
	}
	removeChainNRPT = func(string) error { nrptCalls++; return nil }
	removeChainRoute = func(route.Entry) error { removeCalls++; return windows.ERROR_NOT_FOUND }
	snapshotChainRoutes = func() ([]route.Entry, error) {
		snapshotCalls++
		if snapshotCalls == 1 {
			return nil, errors.New("route table unavailable")
		}
		return nil, nil
	}
	addChainRoute = func(route.Entry) error { addCalls++; return nil }
	clearChainUndo = func(string) error { undoCalls++; return nil }
	stats := &failedCloseStats{}
	sess := &chainState{sessionID: "owned", xray: &supervisor.Child{}, singbox: &supervisor.Child{}, xrayAPI: stats, dnsPrior: &dns.Settings{}, nrptName: "owned", peerRoute: route.Entry{DestCIDR: "192.0.2.1/32"}, snapshot: []route.Entry{{DestCIDR: "0.0.0.0/0"}}}
	withActiveSess(t, sess)
	handler := NewStopChainHandler()
	raw, err := handler(context.Background(), json.RawMessage(`{"session_id":"owned"}`))
	require.NoError(t, err)
	require.Contains(t, string(raw), "partial_errors")
	require.False(t, IsChainActive())
	require.Zero(t, undoCalls, "failed cleanup must preserve recovery journal")
	chainMu.Lock()
	retained := activeSess == sess
	chainMu.Unlock()
	require.True(t, retained)
	_, _, running := readChainCounters(context.Background())
	require.False(t, running)
	_, err = NewStartChainHandler()(context.Background(), json.RawMessage(`{"server_host":"192.0.2.1","server_port":443,"mode":"sysproxy"}`))
	require.Error(t, err)
	raw, err = handler(context.Background(), json.RawMessage(`{"session_id":"owned"}`))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "partial_errors")
	require.Equal(t, 2, stopCalls)
	require.Equal(t, 2, dnsCalls)
	require.Equal(t, 1, nrptCalls)
	require.Equal(t, 1, removeCalls)
	require.Equal(t, 2, snapshotCalls)
	require.Equal(t, 1, addCalls)
	require.Equal(t, 1, undoCalls)
	require.Equal(t, 1, stats.calls)
	chainMu.Lock()
	cleared := activeSess == nil
	chainMu.Unlock()
	require.True(t, cleared)
}
