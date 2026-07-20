// Package logging provides structured logging and secret redaction.
package logging

import (
	"errors"
	"net/url"
	"regexp"
)

// Sensitive-keyword alternation used by multiple rules.
// Note: bare 'sid' deliberately excluded — too common as session-id abbreviation.
// 'shortid' / 'short_id' cover the Reality short-id case specifically.
const secretKeywords = `uuid|password|passwd|pwd|pbk|publickey|public_key|shortid|short_id|token|bearer|basic|auth_key|authkey|api_key|apikey|secret`

var redactors = []struct {
	re   *regexp.Regexp
	repl string
}{
	// JSON-style: "key":"value"
	{regexp.MustCompile(`(?i)"(` + secretKeywords + `)"\s*:\s*"[^"]*"`), `"$1":"***redacted***"`},
	// key=value or key:"value" where value is quoted (may contain spaces).
	{regexp.MustCompile(`(?i)\b(` + secretKeywords + `)\s*[:=]\s*"[^"]+"`), `$1=***redacted***`},
	// key=value where value is unquoted.
	{regexp.MustCompile(`(?i)\b(` + secretKeywords + `)\s*[:=]\s*[A-Za-z0-9+/=_\-\.]+`), `$1=***redacted***`},
	// Bearer/Basic token — handles both quoted and unquoted forms, any outer key.
	{regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9+/=_\-.]+`), `$1 ***redacted***`},
	// Bare UUID (runs last — case-insensitive; VLESS UUIDs are spec'd lowercase but some tools emit uppercase).
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), `***redacted***`},
}

// urlWithPathOrQuery matches a scheme://host prefix together with anything
// following the first '/' or '?' — the part of a URL that can carry a
// subscription/panel access token as a bare path segment or query value with
// no recognizable "key=" marker (e.g. https://host/48Lki5P5I5gv/api/sub/<uuid>).
// The keyword- and UUID-anchored rules below only catch such tokens when
// they're prefixed by a known key name or dash-grouped like a UUID; a bare
// opaque token sails through untouched. This is a coarser, defense-in-depth
// net: it deliberately keeps scheme+host (useful for triage — which panel
// failed) and blanks everything after it, rather than trying to guess which
// path segments "look like" secrets.
var urlWithPathOrQuery = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>/?]+[/?][^\s"'<>]*`)

// Redact replaces known secret patterns in s with "***redacted***".
func Redact(s string) string {
	s = urlWithPathOrQuery.ReplaceAllStringFunc(s, redactURLPath)
	for _, r := range redactors {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// redactURLPath collapses a matched scheme://host/path?query string down to
// scheme://host/***redacted-path***. It re-parses the match with net/url
// (rather than trying to split scheme/host out via regex capture groups) so
// that authority-section credentials (https://user:pass@host/...) and IPv6
// hosts are handled correctly too. On parse failure (regex over-match on
// something that isn't actually a valid URL) it returns match unchanged —
// erring toward not mangling non-URL text rather than guessing.
func redactURLPath(match string) string {
	u, err := url.Parse(match)
	if err != nil || u.Host == "" {
		return match
	}
	return u.Scheme + "://" + u.Host + "/***redacted-path***"
}

// RedactError renders err for logging with secrets removed. It strips the
// embedded URL from *url.Error (which carries the full request URL, possibly
// with credentials) and passes the remainder through Redact.
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		// Drop ue.URL entirely; keep the operation + underlying cause.
		return Redact(ue.Op + ": " + ue.Err.Error())
	}
	return Redact(err.Error())
}
