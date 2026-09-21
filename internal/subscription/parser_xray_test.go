package subscription

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

const xrayRealityOutbound = `{"protocol":"vless","tag":"proxy-a","settings":{"vnext":[{"address":"a.example","port":443,"users":[{"id":"user-a","encryption":"none","flow":"xtls-rprx-vision"}]}]},"streamSettings":{"network":"raw","security":"reality","realitySettings":{"serverName":"cover.example","fingerprint":"chrome","password":"public-key","shortId":"abcd","spiderX":"/hello"}}}`

func TestParse_XrayReality(t *testing.T) {
	r, err := Parse(`{"remarks":"Alpha","outbounds":[` + xrayRealityOutbound + `,{"protocol":"freedom"},{"protocol":"blackhole"},{"protocol":"dns"}]}`)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Empty(t, r.Skipped)
	require.Zero(t, r.Invalid)
	require.Equal(t, vless.Config{Address: "a.example", Port: 443, UUID: "user-a", Encryption: "none", Flow: "xtls-rprx-vision", Remark: "Alpha", Transport: vless.TransportTCP, Security: vless.SecurityReality, SNI: "cover.example", Fingerprint: "chrome", RealityPublicKey: "public-key", RealityShortID: "abcd", RealitySpiderX: "/hello"}, r.Configs[0])
}

func TestParse_XrayArrayAndBalancerDeduplicateNodes(t *testing.T) {
	second := `{"protocol":"vless","tag":"proxy-b","settings":{"address":"b.example","port":8443,"id":"user-b","encryption":"none"},"streamSettings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"b.example","alpn":["http/1.1"],"fingerprint":"firefox"},"wsSettings":{"path":"/ws","host":"front.example"}}}`
	body := `[{"remarks":"Automatic","routing":{"balancers":[{"tag":"auto","selector":["proxy-"]}]},"outbounds":[` + xrayRealityOutbound + `,` + second + `]},{"remarks":"Alpha","outbounds":[` + xrayRealityOutbound + `]},{"remarks":"Beta","outbounds":[` + second + `]},{"remarks":"Unsupported","outbounds":[{"protocol":"hysteria","settings":{"address":"hy.example","port":443,"password":"secret"}}]}]`
	for _, input := range []string{body, base64.StdEncoding.EncodeToString([]byte(body))} {
		r, err := Parse(input)
		require.NoError(t, err)
		require.Len(t, r.Configs, 2)
		require.Equal(t, "Alpha", r.Configs[0].Remark)
		require.Equal(t, "Beta", r.Configs[1].Remark)
		require.Equal(t, "front.example", r.Configs[1].WSHost)
		require.Equal(t, "/ws", r.Configs[1].Path)
		require.Equal(t, []string{"http/1.1"}, r.Configs[1].ALPN)
		require.Equal(t, 1, r.Skipped["hysteria"])
	}
}

func TestParse_XrayTransports(t *testing.T) {
	tests := []struct {
		name, stream string
		want         vless.Config
	}{
		{"grpc", `{"network":"grpc","grpcSettings":{"serviceName":"service","multiMode":true}}`, vless.Config{Transport: vless.TransportGRPC, GRPCServiceName: "service", GRPCMode: "multi"}},
		{"httpupgrade", `{"network":"httpupgrade","httpupgradeSettings":{"path":"/upgrade","host":"front.example"}}`, vless.Config{Transport: vless.TransportHTTPUpgrade, Path: "/upgrade", WSHost: "front.example"}},
		{"xhttp", `{"network":"xhttp","xhttpSettings":{"path":"/x","mode":"auto","host":"front.example","extra":{"path":"/effective","mode":"packet-up","xmux":{"maxConcurrency":8}}}}`, vless.Config{Transport: vless.TransportXHTTP, Path: "/x", XHTTPMode: "auto", WSHost: "front.example"}},
		{"kcp", `{"network":"kcp","kcpSettings":{"seed":"seed","header":{"type":"srtp"}}}`, vless.Config{Transport: vless.TransportMKCP, Seed: "seed", HeaderType: "srtp"}},
		{"ws headers", `{"network":"ws","wsSettings":{"path":"/socket","headers":{"Host":"front.example"}}}`, vless.Config{Transport: vless.TransportWS, Path: "/socket", WSHost: "front.example"}},
		{"tcp header", `{"network":"tcp","tcpSettings":{"header":{"type":"http"}}}`, vless.Config{Transport: vless.TransportTCP, HeaderType: "http"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"user"},"streamSettings":` + tc.stream + `}]}`)
			require.NoError(t, err)
			require.Len(t, r.Configs, 1)
			want := tc.want
			want.Address = "node.example"
			want.Port = 443
			want.UUID = "user"
			want.Encryption = "none"
			want.Security = vless.SecurityNone
			require.Equal(t, want, r.Configs[0])
		})
	}
}

func TestParse_XrayRejectsUnusableOutboundsWithoutLosingValidOnes(t *testing.T) {
	for name, bad := range map[string]string{
		"invalid port":        `{"protocol":"vless","settings":{"address":"bad.example","port":70000,"id":"u"}}`,
		"missing users":       `{"protocol":"vless","settings":{"vnext":[{"address":"bad.example","port":443}]}}`,
		"wrong type":          `{"protocol":"vless","settings":{"address":"bad.example","port":"invalid","id":"u"}}`,
		"unknown transport":   `{"protocol":"vless","settings":{"address":"bad.example","port":443,"id":"u"},"streamSettings":{"network":"unknown"}}`,
		"missing reality key": `{"protocol":"vless","settings":{"address":"bad.example","port":443,"id":"u"},"streamSettings":{"security":"reality","realitySettings":{"serverName":"cover.example"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Parse(`{"outbounds":[` + bad + `,` + xrayRealityOutbound + `]}`)
			require.NoError(t, err)
			require.Len(t, r.Configs, 1)
			require.Equal(t, 1, r.Invalid)
		})
	}
}

func TestParse_XrayDoesNotFlattenChainedConnections(t *testing.T) {
	for name, extra := range map[string]string{
		"proxy settings": `"proxySettings":{"tag":"upstream"}`,
		"dialer proxy":   `"streamSettings":{"sockopt":{"dialerProxy":"upstream"}}`,
		"split download": `"streamSettings":{"network":"xhttp","xhttpSettings":{"extra":{"downloadSettings":{"address":"other.example"}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Parse(`{"outbounds":[{"protocol":"vless","settings":{"address":"private.example","port":443,"id":"user"},` + extra + `}]}`)
			require.NoError(t, err)
			require.Empty(t, r.Configs)
			require.Equal(t, 1, r.Skipped["xray-complex"])
		})
	}
}

func TestParse_JSONIsNeverReinterpretedAsURIs(t *testing.T) {
	for _, body := range []string{`[{"support":"https://provider.example/help"}]`, `{"outbounds":[`, `{"other":"https://provider.example/help"}`} {
		r, err := Parse(body)
		require.Error(t, err)
		require.Empty(t, r.Configs)
		require.Empty(t, r.Skipped)
		require.NotContains(t, err.Error(), "provider.example")
	}
}

func TestParse_UnsupportedOnlyRetainsProtocolCounts(t *testing.T) {
	for _, body := range []string{
		`{"outbounds":[{"type":"hysteria2","server":"hy.example","server_port":443}]}`,
		base64.StdEncoding.EncodeToString([]byte("hysteria2://secret@hy.example:443")),
	} {
		r, err := Parse(body)
		require.NoError(t, err)
		require.Empty(t, r.Configs)
		require.Equal(t, 1, r.Skipped["hysteria2"])
	}
}

func TestParse_XrayStreamAliases(t *testing.T) {
	for _, tc := range []struct{ network, settings, path, header string }{
		{"tcp", `"rawSettings":{"header":{"type":"http"}}`, "", "http"},
		{"raw", `"tcpSettings":{"header":{"type":"http"}},"rawSettings":{}`, "", ""},
		{"xhttp", `"splithttpSettings":{"path":"/legacy"}`, "/legacy", ""},
		{"splithttp", `"splithttpSettings":{"path":"/legacy"},"xhttpSettings":{"path":"/current"}`, "/current", ""},
	} {
		t.Run(tc.network+tc.settings, func(t *testing.T) {
			r, err := Parse(fmt.Sprintf(`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"u"},"streamSettings":{"network":%q,%s}}]}`, tc.network, tc.settings))
			require.NoError(t, err)
			require.Len(t, r.Configs, 1)
			require.Equal(t, tc.path, r.Configs[0].Path)
			require.Equal(t, tc.header, r.Configs[0].HeaderType)
		})
	}
}

func TestParse_XrayUnsupportedStreamOptions(t *testing.T) {
	for _, stream := range []string{
		`{"network":"grpc","grpcSettings":{"authority":"front.example"}}`,
		`{"network":"tcp","rawSettings":{"header":{"request":{"path":["/custom"]}}}}`,
		`{"network":"xhttp","xhttpSettings":{"sessionPlacement":"header","sessionKey":"X-Session"}}`,
		`{"network":"xhttp","xhttpSettings":{"extra":{"seqPlacement":"header","uplinkDataPlacement":"cookie"}}}`,
	} {
		t.Run(stream, func(t *testing.T) {
			r, err := Parse(`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"u"},"streamSettings":` + stream + `}]}`)
			require.NoError(t, err)
			require.Empty(t, r.Configs)
			require.Equal(t, 1, r.Skipped["xray-complex"])
		})
	}
}

func TestParse_XrayPreservesVisionUDP443(t *testing.T) {
	body := strings.ReplaceAll(xrayRealityOutbound, "xtls-rprx-vision", "xtls-rprx-vision-udp443")
	r, err := Parse(`{"outbounds":[` + body + `]}`)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Equal(t, "xtls-rprx-vision-udp443", r.Configs[0].Flow)
	require.Zero(t, r.Invalid)
}

func TestParse_XrayRejectsFlowChanges(t *testing.T) {
	for _, flow := range []string{"invalid-flow", "xtls-rprx-vision"} {
		r, err := Parse(`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"u","flow":"` + flow + `"},"streamSettings":{"network":"ws","security":"tls"}}]}`)
		require.NoError(t, err)
		require.Empty(t, r.Configs)
		require.Equal(t, 1, r.Invalid)
	}
}

func TestParse_XrayMalformedDocumentsDoNotDiscardGoodProfiles(t *testing.T) {
	for _, bad := range []string{`{"outbounds":"invalid"}`, `42`, `null`, `{"remarks":23,"outbounds":[]}`, `{"unexpected":true}`} {
		t.Run(bad, func(t *testing.T) {
			body := `[{"remarks":"Alpha","outbounds":[` + xrayRealityOutbound + `]},` + bad + `]`
			r, err := Parse(body)
			require.NoError(t, err)
			require.Len(t, r.Configs, 1)
			require.Equal(t, "Alpha", r.Configs[0].Remark)
			require.Equal(t, 1, r.Invalid)
		})
	}
}

func TestParse_XrayMissingOutboundProtocolIsInvalid(t *testing.T) {
	r, err := Parse(`{"outbounds":[` + xrayRealityOutbound + `,{"settings":{"address":"node.example","port":443,"id":"u"}}]}`)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Equal(t, 1, r.Invalid)
}

func TestParse_XraySkipsFinalMask(t *testing.T) {
	r, err := Parse(`{"outbounds":[{"protocol":"vless","settings":{"address":"node.example","port":443,"id":"u"},"streamSettings":{"network":"tcp","finalmask":{"tcp":[{"type":"header-custom","settings":{"request":{"value":["custom"]}}}]}}}]}`)
	require.NoError(t, err)
	require.Empty(t, r.Configs)
	require.Equal(t, 1, r.Skipped["xray-complex"])
}

func TestParse_XrayKeepsConnectionVariants(t *testing.T) {
	second := strings.ReplaceAll(xrayRealityOutbound, "cover.example", "other.example")
	r, err := Parse(`[{"remarks":"A","outbounds":[` + xrayRealityOutbound + `]},{"remarks":"B","outbounds":[` + second + `]},{"outbounds":[` + xrayRealityOutbound + `,` + second + `]}]`)
	require.NoError(t, err)
	require.Len(t, r.Configs, 2)
	require.Equal(t, "cover.example", r.Configs[0].SNI)
	require.Equal(t, "other.example", r.Configs[1].SNI)
	require.Equal(t, "A", r.Configs[0].Remark)
	require.Equal(t, "B", r.Configs[1].Remark)
}
