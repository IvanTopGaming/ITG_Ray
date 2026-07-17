package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestSync_LogsStartAndFailureWithRedactedURL(t *testing.T) {
	buf := logtest.Capture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sub := Subscription{ID: "sub-1", URL: srv.URL + "?token=SECRET"}
	_, _, err := Sync(context.Background(), sub, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected Sync to fail on HTTP 500")
	}
	out := buf.String()
	if !strings.Contains(out, "[sub]") {
		t.Fatalf("missing sub scope: %q", out)
	}
	if !strings.Contains(out, "sub sync failed") {
		t.Fatalf("missing failure log: %q", out)
	}
	if strings.Contains(out, "SECRET") {
		t.Fatalf("token leaked into logs: %q", out)
	}
	if !strings.Contains(out, "id=sub-1") {
		t.Fatalf("missing sub id attr: %q", out)
	}
}

func TestSync_LogsNetworkFailureWithoutURL(t *testing.T) {
	buf := logtest.Capture(t)

	sub := Subscription{ID: "sub-net", URL: "http://127.0.0.1:1/sub?token=SECRETTOKEN"}
	_, _, err := Sync(context.Background(), sub, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected Sync to fail on connection refused")
	}
	out := buf.String()
	if !strings.Contains(out, "sub sync failed") {
		t.Fatalf("missing failure log: %q", out)
	}
	if strings.Contains(out, "SECRETTOKEN") {
		t.Fatalf("token leaked into logs: %q", out)
	}
	// The dialed host:port may still appear via the underlying net.OpError
	// string (RedactError only strips the *url.Error's embedded request
	// URL, which is where the credential-bearing query lived); what matters
	// is that the request path + token are gone.
	if strings.Contains(out, "/sub?token=") {
		t.Fatalf("full sub URL/path leaked into logs: %q", out)
	}
}

func TestSync_LogsSuccessCounts(t *testing.T) {
	buf := logtest.Capture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// vless:// is the only scheme ParsePlaintext accepts; this is the
		// same fixture line used by TestSync_SuccessSetsCleanStatusAndMessage
		// in sync_test.go and is known to parse to exactly one server.
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	defer srv.Close()

	sub := Subscription{ID: "sub-ok", URL: srv.URL}
	servers, _, err := Sync(context.Background(), sub, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(servers) < 1 {
		t.Fatalf("expected >=1 parsed server, got %d", len(servers))
	}
	out := buf.String()
	if !strings.Contains(out, "sub synced") || !strings.Contains(out, "servers=") {
		t.Fatalf("missing success summary with counts: %q (parsed %d)", out, len(servers))
	}
}
