package logstream

import (
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func stripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	return ansiRE.ReplaceAllString(s, "")
}
