package configgen

import (
	"crypto/x509"
	"encoding/json"
	"net"
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/app/proxyman"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"
	xtls "github.com/xtls/xray-core/transport/internet/tls"
)

func TestBuildXray_TLSPreservesServerIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, address, override, sni, want string
	}{
		{"resolved domain", "vpn.example.com", "192.0.2.10", "", "vpn.example.com"},
		{"explicit SNI", "vpn.example.com", "192.0.2.10", "certificate.example.com", "certificate.example.com"},
		{"unresolved domain", "vpn.example.com", "", "", "vpn.example.com"},
		{"IPv4 literal", "192.0.2.10", "192.0.2.10", "", "192.0.2.10"},
		{"IPv6 literal", "2001:db8::10", "", "", "2001:db8::10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := vless.Config{
				Address: tc.address, Port: 443, UUID: "00000000-0000-0000-0000-000000000001",
				Security: vless.SecurityTLS, Transport: vless.TransportTCP, SNI: tc.sni,
			}
			_, err := srv.Normalize()
			require.NoError(t, err)
			input := XrayInput{Server: srv, ServerIP: tc.override, SocksPort: 1081}
			raw, err := BuildXray(&input)
			require.NoError(t, err)
			require.Equal(t, srv, input.Server)
			var cfg conf.Config
			require.NoError(t, json.Unmarshal(raw, &cfg))
			native, err := cfg.Build()
			require.NoError(t, err)
			sender, err := native.Outbound[0].SenderSettings.GetInstance()
			require.NoError(t, err)
			tlsNative, err := sender.(*proxyman.SenderConfig).StreamSettings.SecuritySettings[0].GetInstance()
			require.NoError(t, err)
			destination := tc.address
			if tc.override != "" {
				destination = tc.override
			}
			actual := tlsNative.(*xtls.Config).GetTLSConfig(xtls.WithDestination(xnet.TCPDestination(xnet.ParseAddress(destination), 443)))
			require.Equal(t, tc.want, actual.ServerName)
			cert := &x509.Certificate{DNSNames: []string{tc.want}}
			if ip := net.ParseIP(tc.want); ip != nil {
				cert = &x509.Certificate{IPAddresses: []net.IP{ip}}
			}
			require.NoError(t, cert.VerifyHostname(actual.ServerName))
		})
	}
}
