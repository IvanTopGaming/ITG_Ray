// Package subscription parses subscription responses and syncs servers.
package subscription

import (
	"bufio"
	"strings"

	"github.com/itg-team/itg-ray/internal/vless"
)

// ParseResult is returned by every format-specific parser.
// Configs holds the vless entries extracted; Skipped counts other URI schemes
// by name (e.g. "vmess", "trojan"); Invalid counts vless URIs that failed to parse.
type ParseResult struct {
	Configs []vless.Config
	Skipped map[string]int
	Invalid int
}

// ParsePlaintext parses a newline-separated list of URIs and keeps only valid vless:// entries.
func ParsePlaintext(s string) (ParseResult, error) {
	r := ParseResult{Skipped: map[string]int{}}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		scheme := schemeOf(line)
		if scheme != "vless" {
			if scheme != "" {
				r.Skipped[scheme]++
			}
			continue
		}
		c, err := vless.ParseURL(line)
		if err != nil {
			r.Invalid++
			continue
		}
		if _, nerr := c.Normalize(); nerr != nil {
			r.Invalid++
			continue
		}
		r.Configs = append(r.Configs, c)
	}
	if err := sc.Err(); err != nil {
		return ParseResult{}, err
	}
	return r, nil
}

func schemeOf(line string) string {
	i := strings.Index(line, "://")
	if i < 0 {
		return ""
	}
	return strings.ToLower(line[:i])
}
