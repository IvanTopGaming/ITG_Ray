package clashapi

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type recorded struct {
	method, path, auth, body string
}

func fakeAPI(t *testing.T, mode string) (*Client, *[]recorded) {
	t.Helper()
	var calls []recorded
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, recorded{r.Method, r.URL.Path, r.Header.Get("Authorization"), string(b)})
		if r.Header.Get("Authorization") != "Bearer sec" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/configs" {
			_ = json.NewEncoder(w).Encode(map[string]any{"mode": mode})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)
	_, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return New(port, "sec"), &calls
}

func TestClient_SetModeSendsBearerAndBody(t *testing.T) {
	c, calls := fakeAPI(t, "Rule")
	require.NoError(t, c.SetMode(context.Background(), "global"))
	require.Len(t, *calls, 1)
	got := (*calls)[0]
	require.Equal(t, http.MethodPatch, got.method)
	require.Equal(t, "/configs", got.path)
	require.Equal(t, "Bearer sec", got.auth)
	require.JSONEq(t, `{"mode":"Global"}`, got.body)
}

func TestClient_ModeLowercases(t *testing.T) {
	c, _ := fakeAPI(t, "Direct")
	m, err := c.Mode(context.Background())
	require.NoError(t, err)
	require.Equal(t, "direct", m)
}

func TestClient_CloseConnectionsUsesDelete(t *testing.T) {
	c, calls := fakeAPI(t, "Rule")
	require.NoError(t, c.CloseConnections(context.Background()))
	require.Equal(t, http.MethodDelete, (*calls)[0].method)
	require.Equal(t, "/connections", (*calls)[0].path)
}

func TestClient_WrongSecretErrors(t *testing.T) {
	c, _ := fakeAPI(t, "Rule")
	c.secret = "nope"
	require.Error(t, c.SetMode(context.Background(), "rule"))
}

func TestClient_RejectsUnknownMode(t *testing.T) {
	c, calls := fakeAPI(t, "Rule")
	require.Error(t, c.SetMode(context.Background(), "turbo"))
	require.Empty(t, *calls)
}
