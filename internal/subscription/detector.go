package subscription

import (
	"errors"
	"strings"
)

// ErrEmptyBody is returned by Parse when the trimmed input is empty.
var ErrEmptyBody = errors.New("empty subscription body")

// Parse auto-detects the subscription body format and dispatches to the matching parser.
// Detection order: sing-box JSON (begins with '{') → base64 → plaintext URI list.
func Parse(body string) (ParseResult, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ParseResult{}, ErrEmptyBody
	}
	if strings.HasPrefix(trimmed, "{") {
		if r, err := ParseSingbox(trimmed); err == nil && len(r.Configs)+r.Invalid > 0 {
			return r, nil
		}
	}
	if r, err := ParseBase64(trimmed); err == nil && len(r.Configs)+r.Invalid > 0 {
		return r, nil
	}
	return ParsePlaintext(trimmed)
}
