package logstream

import "testing"

func TestStripANSI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\x1b[36mINFO\x1b[0m inbound", "INFO inbound"},
		{"\x1b[38;5;204m1542989500\x1b[0m 0ms] dns", "1542989500 0ms] dns"},
		{"no escapes here", "no escapes here"},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripANSI(c.in); got != c.want {
			t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
