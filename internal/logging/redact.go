// Package logging provides structured logging and secret redaction.
package logging

import "regexp"

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
	{regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9+/=_\-\.]+`), `$1 ***redacted***`},
	// Bare UUID (runs last — case-insensitive; VLESS UUIDs are spec'd lowercase but some tools emit uppercase).
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), `***redacted***`},
}

func Redact(s string) string {
	for _, r := range redactors {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
