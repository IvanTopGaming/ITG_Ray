package subscription

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePlaintext_OnlyVless(t *testing.T) {
	in := `vless://u@h:443?type=tcp#one
vmess://abc
vless://u@h:80#two
trojan://zzz
`
	r, err := ParsePlaintext(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 2)
	require.Equal(t, "one", r.Configs[0].Remark)
	require.Equal(t, "two", r.Configs[1].Remark)
	require.Equal(t, map[string]int{"vmess": 1, "trojan": 1}, r.Skipped)
}

func TestParsePlaintext_EmptyAndBlankLines(t *testing.T) {
	in := "\n\nvless://u@h:443#n\n\n"
	r, err := ParsePlaintext(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
}

func TestParsePlaintext_InvalidVlessSkippedWithCount(t *testing.T) {
	in := "vless://broken\nvless://u@h:443#ok\n"
	r, err := ParsePlaintext(in)
	require.NoError(t, err)
	require.Len(t, r.Configs, 1)
	require.Equal(t, 1, r.Invalid)
}
