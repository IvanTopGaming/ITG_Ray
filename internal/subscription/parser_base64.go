package subscription

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ErrNotBase64 is returned when the input cannot be decoded by any of the four
// tolerated base64 variants (std/url, padded/raw).
var ErrNotBase64 = errors.New("not base64")

// ParseBase64 decodes a base64-wrapped URI list produced by most subscription panels.
// It tolerates all four variants of base64 encoding: standard, URL-safe, with and
// without padding. Whitespace (incl. newlines) is stripped before decoding.
func ParseBase64(s string) (ParseResult, error) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', ' ', '\t':
			return -1
		}
		return r
	}, s)

	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}
	var decoded []byte
	var lastErr error
	for _, d := range decoders {
		b, err := d.DecodeString(clean)
		if err == nil {
			decoded = b
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		return ParseResult{}, ErrNotBase64
	}
	return ParsePlaintext(string(decoded))
}
