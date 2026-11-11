package bindings

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/itg-team/itg-ray/internal/vless"

	"github.com/stretchr/testify/require"
)

func holdingSubServer(t *testing.T, entered chan<- string, label string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- label
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)
	return ts
}

func waitEntered(t *testing.T, entered <-chan string, n int) {
	t.Helper()
	for i := range n {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of %d fetches started", i, n)
		}
	}
}

func seedServer(t *testing.T, dir, sourceID string) {
	t.Helper()
	require.NoError(t, server.Save(filepath.Join(dir, "servers.json"), []server.Server{{
		ID:       "keep",
		Origin:   server.OriginSubscription,
		SourceID: sourceID,
		Name:     "DE",
		Vless: vless.Config{
			Address:   "h",
			Port:      443,
			UUID:      "00000000-0000-0000-0000-000000000000",
			Transport: vless.TransportTCP,
		},
	}}))
}

func TestSubsService_CancelSync_AbortsFetchAndKeepsPreviousState(t *testing.T) {
	entered := make(chan string, 1)
	ts := holdingSubServer(t, entered, "s1")
	dir := t.TempDir()
	svc, store := newSubsServiceForTest(t, dir)
	prevSync := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, store.Save([]subscription.Stored{{ID: "s1", Name: "p", URL: ts.URL, LastSyncAt: prevSync, LastStatus: "ok"}}))
	seedServer(t, dir, "s1")

	done := make(chan error, 1)
	go func() { done <- svc.SyncOne("s1") }()
	waitEntered(t, entered, 1)
	svc.CancelSync("s1")

	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrSyncCanceled)
	case <-time.After(3 * time.Second):
		t.Fatal("SyncOne did not return after CancelSync")
	}

	subs, err := store.Load()
	require.NoError(t, err)
	require.True(t, subs[0].LastSyncAt.Equal(prevSync))
	require.Equal(t, "ok", subs[0].LastStatus)
	list, err := server.Load(filepath.Join(dir, "servers.json"))
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "keep", list[0].ID)
}

func TestSubsService_CancelSync_UnknownIDIsNoop(t *testing.T) {
	svc, _ := newSubsServiceForTest(t, t.TempDir())
	svc.CancelSync("nope")
	svc.CancelAll()
}

func TestSubsService_CancelSync_OnlyTargetsOneSubscription(t *testing.T) {
	entered := make(chan string, 1)
	held := holdingSubServer(t, entered, "s1")
	release := make(chan struct{})
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte("vless://22222222-2222-2222-2222-222222222222@1.2.3.4:443?type=tcp&security=tls&sni=x#B\n"))
	}))
	t.Cleanup(ok.Close)
	svc, store := newSubsServiceForTest(t, t.TempDir())
	require.NoError(t, store.Save([]subscription.Stored{
		{ID: "s1", Name: "a", URL: held.URL},
		{ID: "s2", Name: "b", URL: ok.URL},
	}))

	d1 := make(chan error, 1)
	d2 := make(chan error, 1)
	go func() { d1 <- svc.SyncOne("s1") }()
	go func() { d2 <- svc.SyncOne("s2") }()
	waitEntered(t, entered, 1)
	svc.CancelSync("s1")
	require.ErrorIs(t, <-d1, ErrSyncCanceled)
	close(release)
	require.NoError(t, <-d2)
}

func TestSubsService_CancelAll_StopsSyncAll(t *testing.T) {
	entered := make(chan string, 2)
	ts1 := holdingSubServer(t, entered, "s1")
	ts2 := holdingSubServer(t, entered, "s2")
	svc, store := newSubsServiceForTest(t, t.TempDir())
	require.NoError(t, store.Save([]subscription.Stored{
		{ID: "s1", Name: "a", URL: ts1.URL},
		{ID: "s2", Name: "b", URL: ts2.URL},
	}))

	done := make(chan error, 1)
	go func() { done <- svc.SyncAll() }()
	waitEntered(t, entered, 2)
	svc.CancelAll()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("SyncAll did not return after CancelAll")
	}
	subs, err := store.Load()
	require.NoError(t, err)
	for _, s := range subs {
		require.True(t, s.LastSyncAt.IsZero(), s.ID)
	}
}

func TestSubsService_CancelAll_SkipsQueuedSyncs(t *testing.T) {
	entered := make(chan string, maxConcurrentSyncs+4)
	subs := make([]subscription.Stored, 0, maxConcurrentSyncs+4)
	for i := range maxConcurrentSyncs + 4 {
		ts := holdingSubServer(t, entered, "x")
		subs = append(subs, subscription.Stored{ID: "s" + string(rune('a'+i)), Name: "n", URL: ts.URL})
	}
	svc, store := newSubsServiceForTest(t, t.TempDir())
	require.NoError(t, store.Save(subs))

	done := make(chan error, 1)
	go func() { done <- svc.SyncAll() }()
	waitEntered(t, entered, maxConcurrentSyncs)
	svc.CancelAll()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("SyncAll did not return after CancelAll")
	}
	require.Empty(t, entered, "queued subscriptions were fetched after CancelAll")
}
