package bindings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/logtest"
	"github.com/itg-team/itg-ray/internal/subscription"

	"github.com/stretchr/testify/require"
)

// TestSubsService_SyncAll_LogsPerSubOutcome seeds two subscriptions — one
// whose upstream 500s (sync fails) and one that returns a single valid
// vless:// node (sync succeeds) — and asserts SyncAll does not abort on the
// first failure: the per-sub failure must be swallowed (SyncAll's own
// return stays nil) and therefore logged at Warn, plus an Info completion
// summary with the ok/failed counts.
func TestSubsService_SyncAll_LogsPerSubOutcome(t *testing.T) {
	buf := logtest.Capture(t)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(working.Close)

	dir := t.TempDir()
	svc, subStore := newSubsServiceForTest(t, dir)
	require.NoError(t, subStore.Save([]subscription.Stored{
		{ID: "s1", Name: "fails", URL: failing.URL},
		{ID: "s2", Name: "works", URL: working.URL},
	}))

	require.NoError(t, svc.SyncAll())

	out := buf.String()
	if !strings.Contains(out, "[subs]") || !strings.Contains(out, "sub sync skipped") {
		t.Fatalf("missing per-sub swallowed-failure warn: %q", out)
	}
	if !strings.Contains(out, "subs sync complete") {
		t.Fatalf("missing completion summary: %q", out)
	}
	if !strings.Contains(out, "ok=1") || !strings.Contains(out, "failed=1") {
		t.Fatalf("missing ok/failed counts in completion summary: %q", out)
	}
}
