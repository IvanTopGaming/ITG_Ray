package main

import (
	"fmt"
	"os"
	"strings"
)

// readCombinedLogs concatenates the rotated log history oldest-first:
// path.<keep> … path.1 then the current path. Missing files are skipped.
func readCombinedLogs(path string, keep int) string {
	var b strings.Builder
	for i := keep; i >= 1; i-- {
		if data, err := os.ReadFile(fmt.Sprintf("%s.%d", path, i)); err == nil {
			b.Write(data)
		}
	}
	if data, err := os.ReadFile(path); err == nil {
		b.Write(data)
	}
	return b.String()
}
