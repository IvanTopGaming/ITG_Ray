package subscription

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthFunc applies authentication to an outgoing HTTP request.
type AuthFunc func(*http.Request)

// AuthNone returns an AuthFunc that adds no Authorization header.
func AuthNone() AuthFunc { return func(*http.Request) {} }

// AuthBasic returns an AuthFunc that adds an HTTP Basic Authorization header.
func AuthBasic(user, pass string) AuthFunc {
	cred := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return func(r *http.Request) { r.Header.Set("Authorization", "Basic "+cred) }
}

// AuthBearer returns an AuthFunc that adds a Bearer Authorization header.
func AuthBearer(token string) AuthFunc {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

// FetchOptions controls a single subscription fetch.
type FetchOptions struct {
	URL       string
	UserAgent string
	Auth      AuthFunc
	Timeout   time.Duration

	// Identity headers (Remnawave x-hwid contract). Empty values are not
	// sent; resolver in cmd/itgray-gui/bindings/identity.go decides which
	// to populate from settings + per-sub override + hwid package.
	HWID        string // → x-hwid
	DeviceOS    string // → x-device-os
	OSVersion   string // → x-ver-os
	DeviceModel string // → x-device-model
}

// FetchResult is the parsed outcome of a successful fetch.
type FetchResult struct {
	Body    string
	Headers Headers
	Raw     http.Header
}

// Fetch issues a GET request and returns the body and parsed standard headers.
// Response body is capped at 10 MiB to defend against malicious upstreams.
func Fetch(ctx context.Context, opt FetchOptions) (FetchResult, error) {
	if opt.Timeout == 0 {
		opt.Timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opt.URL, http.NoBody)
	if err != nil {
		return FetchResult{}, err
	}
	if opt.UserAgent != "" {
		req.Header.Set("User-Agent", opt.UserAgent)
	}
	if opt.HWID != "" {
		req.Header.Set("x-hwid", opt.HWID)
	}
	if opt.DeviceOS != "" {
		req.Header.Set("x-device-os", opt.DeviceOS)
	}
	if opt.OSVersion != "" {
		req.Header.Set("x-ver-os", opt.OSVersion)
	}
	if opt.DeviceModel != "" {
		req.Header.Set("x-device-model", opt.DeviceModel)
	}
	if opt.Auth != nil {
		opt.Auth(req)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{
		Body:    string(body),
		Headers: ParseHeaders(resp.Header),
		Raw:     resp.Header,
	}, nil
}
