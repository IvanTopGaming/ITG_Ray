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

func TestFetch_SendsAllIdentityHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("vmess://test"))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), FetchOptions{
		URL:         srv.URL,
		UserAgent:   "ITGRay/1.0",
		HWID:        "abcd1234",
		DeviceOS:    "Linux",
		OSVersion:   "Ubuntu 24.04",
		DeviceModel: "MacBookPro18,2",
	})
	require.NoError(t, err)
	require.Equal(t, "ITGRay/1.0", got.Get("User-Agent"))
	require.Equal(t, "abcd1234", got.Get("X-Hwid"))
	require.Equal(t, "Linux", got.Get("X-Device-Os"))
	require.Equal(t, "Ubuntu 24.04", got.Get("X-Ver-Os"))
	require.Equal(t, "MacBookPro18,2", got.Get("X-Device-Model"))
}

func TestFetch_OmitsEmptyIdentityHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("vmess://test"))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), FetchOptions{URL: srv.URL, UserAgent: "ITGRay/1.0"})
	require.NoError(t, err)
	require.Equal(t, "ITGRay/1.0", got.Get("User-Agent"))
	require.Equal(t, "", got.Get("X-Hwid"))
	require.Equal(t, "", got.Get("X-Device-Os"))
	require.Equal(t, "", got.Get("X-Ver-Os"))
	require.Equal(t, "", got.Get("X-Device-Model"))
}

func TestFetch_ClassifiesErrors(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		retryAfter    string
		wantTransient bool
		wantRetryHint time.Duration
	}{
		{"500 server error", 500, "", true, 0},
		{"502 bad gateway", 502, "", true, 0},
		{"503 with Retry-After", 503, "7", true, 7 * time.Second},
		{"429 too many", 429, "12", true, 12 * time.Second},
		{"408 request timeout", 408, "", true, 0},
		{"404 not found", 404, "", false, 0},
		{"403 forbidden (bad token)", 403, "", false, 0},
		{"401 unauthorized", 401, "", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			_, err := Fetch(context.Background(), FetchOptions{URL: ts.URL, Timeout: time.Second})
			require.Error(t, err)
			require.Equal(t, tc.wantTransient, IsTransient(err), "transient classification for %d", tc.status)
			require.Equal(t, tc.wantRetryHint, RetryAfterHint(err), "Retry-After hint for %d", tc.status)
		})
	}
}

func TestFetch_TransportErrorIsTransient(t *testing.T) {
	// Nothing is listening → connection refused, a transient transport error.
	_, err := Fetch(context.Background(), FetchOptions{URL: "http://127.0.0.1:1", Timeout: time.Second})
	require.Error(t, err)
	require.True(t, IsTransient(err), "connection refused must be transient")
}

func TestIsTransient_NonFetchErrorIsFalse(t *testing.T) {
	require.False(t, IsTransient(context.Canceled))
	require.False(t, IsTransient(nil))
}
