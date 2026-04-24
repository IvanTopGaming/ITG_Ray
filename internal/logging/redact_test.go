package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedact_UUID(t *testing.T) {
	in := "connected uuid=550e8400-e29b-41d4-a716-446655440000 server=NL-1"
	got := Redact(in)
	require.Equal(t, "connected uuid=***redacted*** server=NL-1", got)
}

func TestRedact_BearerAndBasic(t *testing.T) {
	in := `auth="Bearer eyJhbGci.xxx.yyy" basic="Basic dXNlcjpwYXNz"`
	got := Redact(in)
	require.NotContains(t, got, "eyJhbGci")
	require.NotContains(t, got, "dXNlcjpwYXNz")
	require.Contains(t, got, "***redacted***")
}

func TestRedact_PublicKeyAndShortID(t *testing.T) {
	in := "reality pbk=Pm3T3xYxMwPX5z4E publicKey=abcd1234efgh shortId=0011"
	got := Redact(in)
	require.NotContains(t, got, "Pm3T3xYxMwPX5z4E")
	require.NotContains(t, got, "abcd1234efgh")
}

func TestRedact_NoSecret(t *testing.T) {
	in := "plain informational message"
	require.Equal(t, in, Redact(in))
}
