package bindings

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/stretchr/testify/require"
)

type routingCall struct{ id, name, link string }

type routingRecorder struct {
	mu    sync.Mutex
	calls []routingCall
}

func (r *routingRecorder) apply(id, name, link string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, routingCall{id, name, link})
	return nil
}

func (r *routingRecorder) last() routingCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[len(r.calls)-1]
}

func (r *routingRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func newRoutingSubs(t *testing.T, header string, status int) (*SubsService, *routingRecorder, string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if header != "" {
			w.Header().Set("Routing", header)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(ts.Close)
	dir := t.TempDir()
	rec := &routingRecorder{}
	subStore := subscription.FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	require.NoError(t, subStore.Save([]subscription.Stored{{ID: "s1", Name: "okins", URL: ts.URL}}))
	svc := NewSubsService(SubsDeps{
		SubStore:     subStore,
		ServerStore:  fileServerStore{path: filepath.Join(dir, "servers.json")},
		Hub:          hub.New(),
		ApplyRouting: rec.apply,
	})
	return svc, rec, ts.URL
}

func TestSubsSync_PassesRoutingLinkToRules(t *testing.T) {
	svc, rec, _ := newRoutingSubs(t, "itgray://rules/import/eyJ2IjoxfQ", http.StatusOK)
	require.NoError(t, svc.SyncOne("s1"))
	require.Equal(t, routingCall{"s1", "okins", "itgray://rules/import/eyJ2IjoxfQ"}, rec.last())
}

func TestSubsSync_MissingLinkClearsRouting(t *testing.T) {
	svc, rec, _ := newRoutingSubs(t, "", http.StatusOK)
	require.NoError(t, svc.SyncOne("s1"))
	require.Equal(t, routingCall{"s1", "okins", ""}, rec.last())
}

func TestSubsSync_FailedFetchLeavesRoutingAlone(t *testing.T) {
	svc, rec, _ := newRoutingSubs(t, "itgray://rules/import/eyJ2IjoxfQ", http.StatusForbidden)
	require.Error(t, svc.SyncOne("s1"))
	require.Zero(t, rec.count())
}

func TestSubsRemove_ClearsRouting(t *testing.T) {
	svc, rec, _ := newRoutingSubs(t, "", http.StatusOK)
	require.NoError(t, svc.Remove("s1"))
	require.Equal(t, routingCall{"s1", "", ""}, rec.last())
}
