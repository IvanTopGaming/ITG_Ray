package buildinfo

import "testing"

func TestXrayVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.260327.0", "26.3.27"},
		{"1.251115.0", "25.11.15"},
		{"1.8.4", "1.8.4"},
		{"weird", "weird"},
	}
	for _, c := range cases {
		if got := xrayVersion(c.in); got != c.want {
			t.Errorf("xrayVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
