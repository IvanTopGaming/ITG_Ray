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

const singbox1144Connections = `{"connections":[{"chains":["proxy-x"],"download":5183,"id":"8d0d8a8e-1","metadata":{"destinationIP":"","destinationPort":"443","dnsMode":"normal","host":"example.com","network":"tcp","processPath":"/usr/bin/curl (itg)","sourceIP":"127.0.0.1","sourcePort":"41000","type":"mixed/in"},"rule":"domain_suffix=example.com || inbound=itg-rule-r123 => route(proxy-x)","rulePayload":"","start":"2026-11-12T08:00:00.123+03:00","upload":1909},{"chains":["direct"],"download":10917,"id":"8d0d8a8e-2","metadata":{"destinationIP":"178.248.239.46","destinationPort":"443","host":"","network":"udp","processPath":""},"rule":"final","start":"2026-11-12T08:00:01+03:00","upload":1874}],"downloadTotal":16100,"memory":0,"uploadTotal":3783}`

func TestClient_ConnectionsParsesSingboxShape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/connections", r.URL.Path)
		require.Equal(t, "Bearer sec", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(singbox1144Connections))
	}))
	t.Cleanup(ts.Close)
	_, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	snap, err := New(port, "sec").Connections(context.Background())
	require.NoError(t, err)
	require.Len(t, snap.Conns, 2)
	c := snap.Conns[0]
	require.Equal(t, "8d0d8a8e-1", c.ID)
	require.Equal(t, "example.com", c.Host)
	require.Equal(t, "/usr/bin/curl (itg)", c.Process)
	require.Equal(t, []string{"proxy-x"}, c.Chains)
	require.Equal(t, uint64(1909), c.Up)
	require.Equal(t, uint64(5183), c.Down)
	require.Equal(t, "tcp", c.Network)
	require.False(t, c.Start.IsZero())
	require.Equal(t, "178.248.239.46", snap.Conns[1].DestIP)
	require.Equal(t, "final", snap.Conns[1].Rule)
}
