package subscription

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_DetectsBase64(t *testing.T) {
	body := "vless://u@h:443#a\nvless://u@h:80#b\n"
	in := base64.StdEncoding.EncodeToString([]byte(body))
	r, err := Parse(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 2)
}

func TestParse_DetectsSingboxJSON(t *testing.T) {
	in := `{"outbounds":[{"type":"vless","server":"h","server_port":443,"uuid":"u","tag":"x"}]}`
	r, err := Parse(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
}

func TestParse_FallsBackToPlaintext(t *testing.T) {
	in := "vless://u@h:443#one"
	r, err := Parse(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
}

func TestParse_Empty(t *testing.T) {
	_, err := Parse("")
	require.Error(t, err)
}

func TestParse_EmptyReturnsSentinel(t *testing.T) {
	_, err := Parse("")
	require.ErrorIs(t, err, ErrEmptyBody)
}

func TestParse_WhitespaceOnlyReturnsSentinel(t *testing.T) {
	_, err := Parse("   \n\t  ")
	require.ErrorIs(t, err, ErrEmptyBody)
}

func TestParse_Base64DecodableGarbageFallsThroughToPlaintext(t *testing.T) {
	// "hello world" base64-decodes cleanly but produces no vless URIs.
	// The base64 guard (len(Configs)+Invalid > 0) must let it fall through to plaintext.
	in := base64.StdEncoding.EncodeToString([]byte("hello world"))
	r, err := Parse(in)
	require.NoError(t, err)
	require.Empty(t, r.Configs)
	require.Zero(t, r.Invalid)
}
