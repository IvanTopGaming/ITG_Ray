package bindings

import (
	"strings"
	"unicode/utf8"
)

const (
	regionalIndicatorMin = 0x1F1E6 // 🇦
	regionalIndicatorMax = 0x1F1FF // 🇿
)

// extractLeadingFlagEmoji checks if name starts with a regional indicator
// pair (U+1F1E6..U+1F1FF). On match, returns the ISO-3166-1 alpha-2 code
// (e.g. "RU") and the trimmed name (leading pair + optional single space).
// Otherwise returns "" and the original name.
func extractLeadingFlagEmoji(name string) (country, clean string) {
	r1, sz1 := utf8.DecodeRuneInString(name)
	if r1 < regionalIndicatorMin || r1 > regionalIndicatorMax {
		return "", name
	}
	r2, sz2 := utf8.DecodeRuneInString(name[sz1:])
	if r2 < regionalIndicatorMin || r2 > regionalIndicatorMax {
		return "", name
	}
	code := string('A'+(r1-regionalIndicatorMin)) +
		string('A'+(r2-regionalIndicatorMin))
	rest := name[sz1+sz2:]
	rest = strings.TrimPrefix(rest, " ")
	return code, rest
}
