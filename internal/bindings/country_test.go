package bindings

import "testing"

func TestExtractLeadingFlagEmoji(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		in          string
		wantCountry string
		wantClean   string
	}{
		{"flag + space + name", "🇷🇺 Okins-ITG", "RU", "Okins-ITG"},
		{"flag immediately followed by name", "🇺🇸Server", "US", "Server"},
		{"flag only", "🇩🇪", "DE", ""},
		{"plain ascii name", "Plain Name", "", "Plain Name"},
		{"single regional indicator", "🇷", "", "🇷"},
		{"flag not at start", "R 🇷🇺 Server", "", "R 🇷🇺 Server"},
		{"empty", "", "", ""},
		{"flag + double space", "🇷🇺  Two-spaces", "RU", " Two-spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			country, clean := extractLeadingFlagEmoji(tc.in)
			if country != tc.wantCountry {
				t.Errorf("country: got %q, want %q", country, tc.wantCountry)
			}
			if clean != tc.wantClean {
				t.Errorf("clean: got %q, want %q", clean, tc.wantClean)
			}
		})
	}
}
