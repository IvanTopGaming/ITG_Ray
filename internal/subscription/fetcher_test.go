package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetch_UserAgentAndAuth(t *testing.T) {
	var gotUA, gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3")
		_, _ = w.Write([]byte("vless://u@h:443#x"))
	}))
	defer ts.Close()

	fr, err := Fetch(context.Background(), FetchOptions{
		URL:       ts.URL,
		UserAgent: "ITG-Ray/test",
		Auth:      AuthBearer("abc123"),
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, "ITG-Ray/test", gotUA)
	require.Equal(t, "Bearer abc123", gotAuth)
	require.NotNil(t, fr.Headers.Userinfo)
	require.Equal(t, int64(3), fr.Headers.Userinfo.Total)
	require.Equal(t, "vless://u@h:443#x", fr.Body)
}

func TestFetch_BasicAuth(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(""))
	}))
	defer ts.Close()

	_, _ = Fetch(context.Background(), FetchOptions{
		URL:     ts.URL,
		Auth:    AuthBasic("user", "pass"),
		Timeout: 5 * time.Second,
	})
	require.Equal(t, "Basic dXNlcjpwYXNz", gotAuth)
}

func TestFetch_Non2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	_, err := Fetch(context.Background(), FetchOptions{URL: ts.URL, Timeout: time.Second})
	require.Error(t, err)
}

func TestFetch_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := Fetch(ctx, FetchOptions{URL: ts.URL, Timeout: time.Second})
	require.Error(t, err)
}
