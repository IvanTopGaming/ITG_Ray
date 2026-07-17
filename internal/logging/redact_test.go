package logging

import (
	"errors"
	"net/url"
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

func TestRedact_JSONStyle(t *testing.T) {
	cases := []struct {
		in, banned string
	}{
		{`{"uuid":"550e8400-e29b-41d4-a716-446655440000"}`, "550e8400"},
		{`{"pbk":"Pm3T3xYxMwPX5z4E"}`, "Pm3T3xYxMwPX5z4E"},
		{`{"publicKey":"abcd1234efgh"}`, "abcd1234efgh"},
		{`{"password":"hunter2"}`, "hunter2"},
		{`{"token":"xyz-123"}`, "xyz-123"},
	}
	for _, c := range cases {
		out := Redact(c.in)
		require.NotContains(t, out, c.banned, "input=%q output=%q", c.in, out)
		require.Contains(t, out, "***redacted***")
	}
}

func TestRedact_UnquotedBearerAndBasic(t *testing.T) {
	// HTTP-style Authorization headers.
	in := "Authorization: Bearer eyJabc.def.ghi"
	got := Redact(in)
	require.NotContains(t, got, "eyJabc")
	require.Contains(t, got, "***redacted***")

	in2 := "Authorization: Basic dXNlcjpwYXNz"
	got2 := Redact(in2)
	require.NotContains(t, got2, "dXNlcjpwYXNz")
	require.Contains(t, got2, "***redacted***")
}

func TestRedact_UppercaseUUID(t *testing.T) {
	in := "id 550E8400-E29B-41D4-A716-446655440000 end"
	got := Redact(in)
	require.NotContains(t, got, "550E8400")
	require.Contains(t, got, "***redacted***")
}

func TestRedact_AuthKeyAndSecret(t *testing.T) {
	cases := []string{
		`auth_key=topsecret123`,
		`authKey=topsecret123`,
		`api_key=xyz`,
		`apiKey=xyz`,
		`secret=hush`,
		`passwd=hunter2`,
	}
	for _, c := range cases {
		out := Redact(c)
		require.Contains(t, out, "***redacted***", "input=%q output=%q", c, out)
	}
}

func TestRedact_NoFalsePositiveOnSID(t *testing.T) {
	// 'sid=' alone (not 'shortId=' / 'short_id=') should NOT be redacted — too common as session-id abbreviation.
	in := "user sid=session-abc-not-a-secret"
	got := Redact(in)
	require.Contains(t, got, "session-abc-not-a-secret")
}

func TestRedactError_DropsURLFromURLError(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://panel.example.com/sub/abc?token=SECRETTOKEN",
		Err: errors.New("dial tcp: i/o timeout"),
	}
	got := RedactError(err)
	require.NotContains(t, got, "panel.example.com")
	require.NotContains(t, got, "abc")
	require.NotContains(t, got, "SECRETTOKEN")
	require.Contains(t, got, "i/o timeout")
}
