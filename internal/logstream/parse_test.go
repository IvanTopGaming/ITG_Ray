package logstream

import "testing"

func TestParseLevel(t *testing.T) {
	cases := []struct{ source, line, want string }{
		{"sing-box", "2026-07-12 14:22:01 INFO router: started", "INFO"},
		{"sing-box", "+0000 WARN dns: timeout", "WARN"},
		{"sing-box", "2026 ERROR proxy: failed", "ERROR"},
		{"sing-box", "2026 FATAL boom", "ERROR"},
		{"sing-box", "2026 DEBUG verbose", "DEBUG"},
		{"xray", "2026/07/12 [Warning] core: retry", "WARN"},
		{"xray", "2026/07/12 [Error] tcp reset", "ERROR"},
		{"xray", "2026/07/12 [Info] connected", "INFO"},
		{"xray", "2026/07/12 [Debug] handshake", "DEBUG"},
		{"sing-box", "no level here", "INFO"},
	}
	for _, c := range cases {
		if got := ParseLevel(c.source, c.line); got != c.want {
			t.Errorf("ParseLevel(%q, %q) = %q, want %q", c.source, c.line, got, c.want)
		}
	}
}
