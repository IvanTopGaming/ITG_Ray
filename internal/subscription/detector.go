package subscription

import (
	"errors"
	"strings"
)

// ErrEmptyBody is returned by Parse when the trimmed input is empty.
var ErrEmptyBody = errors.New("empty subscription body")

// Parse auto-detects the subscription body format and dispatches to the matching parser.
func Parse(body string) (ParseResult, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ParseResult{}, ErrEmptyBody
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return parseJSON(trimmed)
	}
	if r, err := ParseBase64(trimmed); err == nil {
		if len(r.Configs)+r.Invalid+sumSkipped(r.Skipped) > 0 {
			return r, nil
		}
	} else if !errors.Is(err, ErrNotBase64) {
		return ParseResult{}, err
	}
	return ParsePlaintext(trimmed)
}

func parseJSON(body string) (ParseResult, error) {
	if r, err := ParseXray(body); err == nil {
		return r, nil
	}
	if r, err := ParseSingbox(body); err == nil && len(r.Configs)+r.Invalid+sumSkipped(r.Skipped) > 0 {
		return r, nil
	}
	return ParseResult{}, ErrUnrecognizedFormat
}
