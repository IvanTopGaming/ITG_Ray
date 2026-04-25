package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequest_RoundTrip(t *testing.T) {
	r := Request{
		ID:   42,
		Op:   OpTunCreate,
		Args: json.RawMessage(`{"name":"itg","ipv4_cidr":"198.18.0.1/15","mtu":1500}`),
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	var got Request
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, r, got)
}

func TestResponse_OK(t *testing.T) {
	r := NewOK(7, json.RawMessage(`{"tun_iface_id":12}`))
	require.Equal(t, uint64(7), r.ID)
	require.True(t, r.OK)
	require.Empty(t, r.Error)
	require.Equal(t, json.RawMessage(`{"tun_iface_id":12}`), r.Result)
}

func TestResponse_Error(t *testing.T) {
	r := NewError(7, "tun create failed: access denied")
	require.Equal(t, uint64(7), r.ID)
	require.False(t, r.OK)
	require.Equal(t, "tun create failed: access denied", r.Error)
	require.Nil(t, r.Result)
}

func TestOpStrings(t *testing.T) {
	require.Equal(t, "TunCreate", OpTunCreate.String())
	require.Equal(t, "RouteSnapshot", OpRouteSnapshot.String())
	require.Equal(t, "DnsRestore", OpDnsRestore.String())
}
