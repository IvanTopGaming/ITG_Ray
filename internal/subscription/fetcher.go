package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FetchError classifies a failed fetch so callers can decide whether a retry
// is worthwhile. StatusCode is 0 when the failure was not an HTTP status
// (transport error, body read). RetryAfter carries the parsed Retry-After
// hint on 429/503 responses (0 when absent). Transient is true for failures
// that a later attempt might recover from (network/timeout, 408/429, 5xx) and
// false for ones that won't (4xx auth/not-found, malformed request).
type FetchError struct {
	StatusCode int
	RetryAfter time.Duration
	Transient  bool
	Err        error
}

func (e *FetchError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("http status %d", e.StatusCode)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "fetch error"
}

func (e *FetchError) Unwrap() error { return e.Err }

// IsTransient reports whether err wraps a FetchError marked transient, i.e.
// whether re-attempting the fetch could plausibly succeed.
func IsTransient(err error) bool {
	fe, ok := errors.AsType[*FetchError](err)
	return ok && fe.Transient
}

// RetryAfterHint returns the server-advertised Retry-After delay carried by a
// transient FetchError, or 0 when none was provided.
func RetryAfterHint(err error) time.Duration {
	if fe, ok := errors.AsType[*FetchError](err); ok {
		return fe.RetryAfter
	}
	return 0
}

// transientStatus reports whether an HTTP status code warrants a retry.
func transientStatus(code int) bool {
	switch {
	case code == http.StatusRequestTimeout, code == http.StatusTooManyRequests:
		return true
	case code >= 500 && code <= 599:
		return true
	default:
		return false
	}
}

// retryAfterFromHeader parses a Retry-After header (delta-seconds or HTTP-date
// form). Returns 0 when absent, unparseable, or non-positive.
func retryAfterFromHeader(h http.Header) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

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
		return FetchResult{}, &FetchError{Err: err}
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
		// Transport failure (DNS, connection, TLS, our per-fetch timeout) —
		// nearly always transient.
		return FetchResult{}, &FetchError{Transient: true, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fe := &FetchError{StatusCode: resp.StatusCode, Transient: transientStatus(resp.StatusCode)}
		if fe.Transient {
			fe.RetryAfter = retryAfterFromHeader(resp.Header)
		}
		return FetchResult{}, fe
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		// Connection dropped mid-body — retryable.
		return FetchResult{}, &FetchError{Transient: true, Err: err}
	}
	return FetchResult{
		Body:    string(body),
		Headers: ParseHeaders(resp.Header),
		Raw:     resp.Header,
	}, nil
}
