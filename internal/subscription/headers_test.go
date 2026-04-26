package subscription

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseHeaders_UserinfoFull(t *testing.T) {
	h := http.Header{}
	h.Set("Subscription-Userinfo", "upload=100; download=200; total=1000; expire=1767225600")
	h.Set("profile-update-interval", "24")
	h.Set("profile-title", "ZG9s") // base64 "dol" — panels often base64-wrap

	m := ParseHeaders(h)
	require.NotNil(t, m.Userinfo)
	require.Equal(t, int64(100), m.Userinfo.Upload)
	require.Equal(t, int64(200), m.Userinfo.Download)
	require.Equal(t, int64(1000), m.Userinfo.Total)
	require.Equal(t, time.Unix(1767225600, 0), *m.Userinfo.Expire)
	require.Equal(t, 24, m.UpdateInterval)
	require.Equal(t, "dol", m.ProfileTitle)
}

func TestParseHeaders_UserinfoPartial(t *testing.T) {
	h := http.Header{}
	h.Set("Subscription-Userinfo", "total=500")
	m := ParseHeaders(h)
	require.NotNil(t, m.Userinfo)
	require.Equal(t, int64(0), m.Userinfo.Upload)
	require.Equal(t, int64(500), m.Userinfo.Total)
	require.Nil(t, m.Userinfo.Expire)
}

func TestParseHeaders_None(t *testing.T) {
	m := ParseHeaders(http.Header{})
	require.Nil(t, m.Userinfo)
	require.Equal(t, 0, m.UpdateInterval)
	require.Empty(t, m.ProfileTitle)
}

func TestParseHeaders_TitlePlaintext(t *testing.T) {
	h := http.Header{}
	h.Set("profile-title", "My Panel")
	require.Equal(t, "My Panel", ParseHeaders(h).ProfileTitle)
}

func TestParseHeaders_UserinfoMalformedEntriesSkipped(t *testing.T) {
	h := http.Header{}
	h.Set("Subscription-Userinfo", "upload=abc; total=500; weird-no-equals; download=42")
	m := ParseHeaders(h)
	require.NotNil(t, m.Userinfo)
	require.Equal(t, int64(0), m.Userinfo.Upload)    // malformed, skipped
	require.Equal(t, int64(500), m.Userinfo.Total)   // valid
	require.Equal(t, int64(42), m.Userinfo.Download) // valid
}

func TestParseHeaders_UpdateIntervalNonNumericSkipped(t *testing.T) {
	h := http.Header{}
	h.Set("profile-update-interval", "weekly")
	m := ParseHeaders(h)
	require.Equal(t, 0, m.UpdateInterval)
}
