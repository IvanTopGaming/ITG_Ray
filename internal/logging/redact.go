// Package logging provides structured logging and secret redaction.
package logging

import "regexp"

var redactors = []struct {
	re   *regexp.Regexp
	repl string
}{
	// key=value or key:"value" where key is a sensitive keyword and value is quoted (may contain spaces).
	{regexp.MustCompile(`(?i)\b(uuid|password|pbk|publickey|shortid|sid|token|bearer|basic)\s*[:=]\s*"[^"]+"`), `$1=***redacted***`},
	// key=value where key is a sensitive keyword and value is unquoted.
	{regexp.MustCompile(`(?i)\b(uuid|password|pbk|publickey|shortid|sid|token|bearer|basic)\s*[:=]\s*[A-Za-z0-9+/=_\-\.]+`), `$1=***redacted***`},
	// Bearer/Basic token appearing inside a quoted string value (any key).
	{regexp.MustCompile(`(?i)"(Bearer|Basic)\s+[A-Za-z0-9+/=_\-\.]+"`), "$1 ***redacted***"},
	// Bare UUID (runs last so uuid=<uuid> is caught by the key=value rules above first).
	{regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), `***redacted***`},
}

func Redact(s string) string {
	for _, r := range redactors {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
