package chainctl

import (
	"testing"

	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func tcp(addr string) vless.Config {
	return vless.Config{Address: addr, Port: 443, UUID: "u", Transport: vless.TransportTCP, Security: vless.SecurityTLS, SNI: addr}
}

func TestXrayServerEndpoints_Extras(t *testing.T) {
	raw, err := configgen.BuildXray(&configgen.XrayInput{
		Server: tcp("main.example"), ServerIP: "1.1.1.1", SocksPort: 1081,
		Extra: []configgen.XrayServer{
			{Tag: "proxy-a", Server: tcp("a.example"), ServerIP: "2.2.2.2", SocksPort: 1082},
			{Tag: "proxy-b", Server: tcp("b.example"), ServerIP: "3.3.3.3", SocksPort: 1083},
		},
	})
	require.NoError(t, err)
	host, port, extra, err := xrayServerEndpoints(raw)
	require.NoError(t, err)
	require.Equal(t, "1.1.1.1", host)
	require.Equal(t, 443, port)
	require.Equal(t, []string{"2.2.2.2", "3.3.3.3"}, extra)
}

func TestXrayServerEndpoints_SingleServer(t *testing.T) {
	raw, err := configgen.BuildXray(&configgen.XrayInput{Server: tcp("main.example"), ServerIP: "1.1.1.1", SocksPort: 1081})
	require.NoError(t, err)
	host, _, extra, err := xrayServerEndpoints(raw)
	require.NoError(t, err)
	require.Equal(t, "1.1.1.1", host)
	require.Empty(t, extra)
}

func TestXrayServerEndpoints_MissingVnext(t *testing.T) {
	_, _, _, err := xrayServerEndpoints([]byte(`{"outbounds":[]}`))
	require.Error(t, err)
}
