package subscription

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBase64_Standard(t *testing.T) {
	body := "vless://u@h:443#one\nvless://u@h:80#two\n"
	in := base64.StdEncoding.EncodeToString([]byte(body))
	r, err := ParseBase64(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 2)
}

func TestParseBase64_URLSafe(t *testing.T) {
	body := "vless://u@h:443#one"
	in := base64.RawURLEncoding.EncodeToString([]byte(body))
	r, err := ParseBase64(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
}

func TestParseBase64_WithPadding(t *testing.T) {
	body := "vless://u@h:443#pad"
	in := base64.StdEncoding.EncodeToString([]byte(body)) // already has padding
	r, err := ParseBase64(in + "\n")
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
}

func TestParseBase64_Garbage(t *testing.T) {
	_, err := ParseBase64("!!!not-base64!!!")
	require.Error(t, err)
}
