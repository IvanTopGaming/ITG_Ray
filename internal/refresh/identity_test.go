package refresh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/stretchr/testify/require"
)

func TestScheduledSync_UsesCurrentIdentityOnEveryAttempt(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Subscriptions.UserAgent = "Global/1"
	require.NoError(t, config.Save(filepath.Join(dir, "config.json"), cfg))
	hwid := strings.Repeat("a", 64)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hwid.dat"), []byte(hwid), 0600))
	headers := make(chan http.Header, 8)
	var failOnce atomic.Bool
	failOnce.Store(true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		if failOnce.Swap(false) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("vless://u@node.example:443#demo"))
	}))
	t.Cleanup(ts.Close)
	sub := subscription.Stored{ID: "s1", URL: ts.URL, UserAgent: "Override/1"}
	store := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	require.NoError(t, store.Save([]subscription.Stored{sub}))
	d := NewDriver(Config{Subs: store, ServersPath: filepath.Join(dir, "servers.json"), SubFetchRetryBackoff: []time.Duration{time.Millisecond}})
	require.False(t, d.syncOne(context.Background(), sub))
	for range 2 {
		h := <-headers
		require.Equal(t, "Override/1", h.Get("User-Agent"))
		require.Equal(t, hwid, h.Get("x-hwid"))
		require.NotEmpty(t, h.Get("x-device-os"))
	}
	cfg.Subscriptions.UserAgent = "Global/2"
	cfg.Subscriptions.HWIDEnabled = false
	require.NoError(t, config.Save(filepath.Join(dir, "config.json"), cfg))
	sub.UserAgent = ""
	require.False(t, d.syncOne(context.Background(), sub))
	h := <-headers
	require.Equal(t, "Global/2", h.Get("User-Agent"))
	for _, key := range []string{"x-hwid", "x-device-os", "x-ver-os", "x-device-model"} {
		require.Empty(t, h.Get(key))
	}
	cfg.Subscriptions.HWIDEnabled = true
	require.NoError(t, config.Save(filepath.Join(dir, "config.json"), cfg))
	sub.UserAgent = "Override/2"
	require.False(t, d.syncOne(context.Background(), sub))
	h = <-headers
	require.Equal(t, "Override/2", h.Get("User-Agent"))
	require.Equal(t, hwid, h.Get("x-hwid"))
}

func TestScheduledSync_SettingsReadFailureDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0600))
	store := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	sub := subscription.Stored{ID: "s1", URL: "https://provider.example/sub"}
	require.NoError(t, store.Save([]subscription.Stored{sub}))
	called := false
	d := NewDriver(Config{Subs: store, ServersPath: filepath.Join(dir, "servers.json"), SyncFunc: func(context.Context, subscription.Subscription, []server.Server, time.Duration) ([]server.Server, subscription.SyncMeta, error) {
		called = true
		return nil, subscription.SyncMeta{}, nil
	}})
	require.False(t, d.syncOne(context.Background(), sub))
	require.False(t, called)
	subs, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "error", subs[0].LastStatus)
}
