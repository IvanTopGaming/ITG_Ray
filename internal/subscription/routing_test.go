package subscription

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testRoutingLink = "itgray://rules/import/eyJ2IjoxfQ"

func fetchRouting(t *testing.T, header, body string) (string, SyncMeta) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if header != "" {
			w.Header().Set("Routing", header)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	servers, meta, err := FetchParse(context.Background(), Subscription{ID: "s", URL: ts.URL}, 5*time.Second)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	return meta.RoutingLink, meta
}

func TestFetchParse_RoutingLinkFromHeader(t *testing.T) {
	link, _ := fetchRouting(t, "  "+testRoutingLink+" ", "vless://u@h:443#one\n")
	require.Equal(t, testRoutingLink, link)
}

func TestFetchParse_RoutingLinkFromPlainBody(t *testing.T) {
	link, meta := fetchRouting(t, "", "vless://u@h:443#one\n"+testRoutingLink+"\n")
	require.Equal(t, testRoutingLink, link)
	require.Equal(t, "imported=1 invalid=0 skipped=0", meta.Message)
}

func TestFetchParse_RoutingLinkFromBase64Body(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte(testRoutingLink + "\nvless://u@h:443#one\n"))
	link, _ := fetchRouting(t, "", body)
	require.Equal(t, testRoutingLink, link)
}

func TestFetchParse_HeaderWinsOverBody(t *testing.T) {
	other := "itgray://rules/import/b3RoZXI"
	link, _ := fetchRouting(t, testRoutingLink, "vless://u@h:443#one\n"+other+"\n")
	require.Equal(t, testRoutingLink, link)
}

func TestFetchParse_IgnoresForeignRoutingHeader(t *testing.T) {
	link, _ := fetchRouting(t, "happ://routing/onadd/e30", "vless://u@h:443#one\n")
	require.Empty(t, link)
}

func TestFetchParse_NoRoutingLink(t *testing.T) {
	link, _ := fetchRouting(t, "", "vless://u@h:443#one\n")
	require.Empty(t, link)
}
