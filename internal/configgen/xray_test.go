package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/vless"
	"github.com/stretchr/testify/require"
)

func TestBuildXray_Reality_XHTTP(t *testing.T) {
	srv := vless.Config{
		Address: "example.com", Port: 443,
		UUID: "u", Flow: "xtls-rprx-vision", Encryption: "none",
		Security: vless.SecurityReality, SNI: "www.cloudflare.com",
		Fingerprint:      "chrome",
		RealityPublicKey: "pk", RealityShortID: "01", RealitySpiderX: "/",
		Transport: vless.TransportXHTTP, Path: "/abc", XHTTPMode: "packet-up",
	}
	b, err := BuildXray(&XrayInput{Server: srv, SocksPort: 1081})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	inbounds := doc["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	in0 := inbounds[0].(map[string]any)
	require.Equal(t, "socks", in0["protocol"])
	require.Equal(t, float64(1081), in0["port"])

	outbounds := doc["outbounds"].([]any)
	out0 := outbounds[0].(map[string]any)
	require.Equal(t, "vless", out0["protocol"])
	stream := out0["streamSettings"].(map[string]any)
	require.Equal(t, "reality", stream["security"])
	reality := stream["realitySettings"].(map[string]any)
	require.Equal(t, "pk", reality["publicKey"])
	require.Equal(t, "xhttp", stream["network"])
	xh := stream["xhttpSettings"].(map[string]any)
	require.Equal(t, "/abc", xh["path"])
	require.Equal(t, "packet-up", xh["mode"])
}

func TestBuildXray_TLS_WebSocket(t *testing.T) {
	srv := vless.Config{
		Address: "h", Port: 443, UUID: "u",
		Security: vless.SecurityTLS, SNI: "h.example.com",
		Transport: vless.TransportWS, Path: "/ws", WSHost: "cdn.example.com",
		ALPN: []string{"h2", "http/1.1"},
	}
	b, err := BuildXray(&XrayInput{Server: srv, SocksPort: 1081})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	stream := doc["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
	require.Equal(t, "tls", stream["security"])
	require.Equal(t, "ws", stream["network"])
	ws := stream["wsSettings"].(map[string]any)
	require.Equal(t, "/ws", ws["path"])
	require.Equal(t, "cdn.example.com", ws["headers"].(map[string]any)["Host"])
}
