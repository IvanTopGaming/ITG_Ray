package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/stretchr/testify/require"
)

func TestSync_NewImportAndMerge(t *testing.T) {
	body1 := "vless://u@h:443#one\nvless://u@h:80#two\n"
	body2 := "vless://u@h:443#one\nvless://u@h:9000#three\n"
	step := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if step == 0 {
			_, _ = w.Write([]byte(body1))
		} else {
			_, _ = w.Write([]byte(body2))
		}
	}))
	defer ts.Close()

	sub := Subscription{ID: "sub1", URL: ts.URL, UserAgent: "ITG/1.0"}

	servers, meta, err := Sync(context.Background(), sub, nil, 5*time.Second)
	require.NoError(t, err)
	require.Len(t, servers, 2)
	require.Equal(t, "ok", meta.Status)

	step = 1
	servers2, _, err := Sync(context.Background(), sub, servers, 5*time.Second)
	require.NoError(t, err)
	require.Len(t, servers2, 2) // "one" kept, "two" removed, "three" added

	var remarks []string
	for _, s := range servers2 {
		remarks = append(remarks, s.Remark)
	}
	require.Contains(t, remarks, "one")
	require.Contains(t, remarks, "three")
	require.NotContains(t, remarks, "two")
}

func TestSync_ErrorBubblesUpAsStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(502)
	}))
	defer ts.Close()

	sub := Subscription{ID: "s", URL: ts.URL}
	_, meta, err := Sync(context.Background(), sub, nil, 2*time.Second)
	require.Error(t, err)
	require.Equal(t, "error", meta.Status)
	require.NotEmpty(t, meta.Message)
}

func TestSync_SuccessSetsCleanStatusAndMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3")
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@1.2.3.4:443?type=tcp&security=tls&sni=x#A\n"))
	}))
	t.Cleanup(srv.Close)

	_, meta, err := Sync(context.Background(),
		Subscription{ID: "s1", URL: srv.URL, UserAgent: "test"},
		nil, 5*time.Second)

	require.NoError(t, err)
	require.Equal(t, "ok", meta.Status)
	require.Contains(t, meta.Message, "imported=")
	require.Contains(t, meta.Message, "invalid=")
	require.Contains(t, meta.Message, "skipped=")
}

func TestSync_FailureSetsErrorStatusAndMessage(t *testing.T) {
	_, meta, err := Sync(context.Background(),
		Subscription{ID: "s1", URL: "http://127.0.0.1:1/nonexistent", UserAgent: "test"},
		nil, 100*time.Millisecond)
	require.Error(t, err)
	require.Equal(t, "error", meta.Status)
	require.NotEmpty(t, meta.Message, "Message should carry the error detail")
}

// keep server import reachable
var _ = server.Server{}
