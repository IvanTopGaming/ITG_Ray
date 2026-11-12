package ruleshare

import (
	"errors"
	"strings"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
)

func sampleGroups() []rules.Group {
	return []rules.Group{{
		ID:      "should-be-stripped",
		Name:    "Streaming",
		Enabled: true,
		Rules: []rules.Rule{{
			ID:      "also-stripped",
			Name:    "Netflix",
			Enabled: true,
			Action:  rules.ActionProxy,
			Conditions: rules.Conditions{
				Domains: []rules.DomainMatcher{{Kind: "suffix", Value: "netflix.com"}},
			},
		}},
	}}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	link, err := Encode("My set", sampleGroups())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.HasPrefix(link, ImportPrefix) {
		t.Fatalf("link missing prefix: %q", link)
	}
	p, err := Decode(link)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.Version != CurrentSchema {
		t.Errorf("version = %d, want %d", p.Version, CurrentSchema)
	}
	if p.Name != "My set" {
		t.Errorf("name = %q", p.Name)
	}
	if len(p.Groups) != 1 || len(p.Groups[0].Rules) != 1 {
		t.Fatalf("shape = %+v", p.Groups)
	}
	if p.Groups[0].Rules[0].Name != "Netflix" {
		t.Errorf("rule name = %q", p.Groups[0].Rules[0].Name)
	}
}

func TestEncodeStripsIDs(t *testing.T) {
	a := sampleGroups()
	b := sampleGroups()
	b[0].ID = "different"
	b[0].Rules[0].ID = "different-too"
	la, _ := Encode("n", a)
	lb, _ := Encode("n", b)
	if la != lb {
		t.Errorf("IDs not stripped: links differ")
	}
	p, _ := Decode(la)
	if p.Groups[0].ID != "" || p.Groups[0].Rules[0].ID != "" {
		t.Errorf("decoded IDs not empty: %q / %q", p.Groups[0].ID, p.Groups[0].Rules[0].ID)
	}
}

func TestEncodePreservesRuleEnabled(t *testing.T) {
	g := sampleGroups()
	g[0].Rules[0].Enabled = false
	link, _ := Encode("n", g)
	p, _ := Decode(link)
	if p.Groups[0].Rules[0].Enabled {
		t.Errorf("disabled rule came back enabled")
	}
}

func TestDecodeMalformed(t *testing.T) {
	cases := []string{
		"",
		"https://example.com",
		ImportPrefix + "!!!not-base64!!!",
		ImportPrefix + "aGVsbG8",
	}
	for _, c := range cases {
		if _, err := Decode(c); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decode(%q) err = %v, want ErrMalformed", c, err)
		}
	}
}

func TestDecodeUnsupportedVersion(t *testing.T) {
	link := encodeEnvelope(t, wireEnvelope{V: 999, Kind: "rules", Name: "n", Groups: sampleGroups()})
	if _, err := Decode(link); !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("err = %v, want ErrUnsupportedVersion", err)
	}
}

func TestDecodeWrongKind(t *testing.T) {
	link := encodeEnvelope(t, wireEnvelope{V: 1, Kind: "servers", Name: "n", Groups: sampleGroups()})
	if _, err := Decode(link); !errors.Is(err, ErrMalformed) {
		t.Errorf("err = %v, want ErrMalformed", err)
	}
}

func TestDecodeEmpty(t *testing.T) {
	link := encodeEnvelope(t, wireEnvelope{V: 1, Kind: "rules", Name: "n", Groups: nil})
	if _, err := Decode(link); !errors.Is(err, ErrEmpty) {
		t.Errorf("err = %v, want ErrEmpty", err)
	}
	link2 := encodeEnvelope(t, wireEnvelope{V: 1, Kind: "rules", Name: "n", Groups: []rules.Group{{Name: "g"}}})
	if _, err := Decode(link2); !errors.Is(err, ErrEmpty) {
		t.Errorf("no-rules err = %v, want ErrEmpty", err)
	}
}

func TestDecodeTooManyRules(t *testing.T) {
	var rs []rules.Rule
	for i := 0; i < maxRules+1; i++ {
		rs = append(rs, rules.Rule{
			Name:       "r",
			Action:     rules.ActionProxy,
			Conditions: rules.Conditions{IPCIDRs: []string{"1.2.3.4/32"}},
		})
	}
	link := encodeEnvelope(t, wireEnvelope{V: 1, Kind: "rules", Name: "n", Groups: []rules.Group{{Name: "g", Rules: rs}}})
	if _, err := Decode(link); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
}

func TestDecodeInvalidRuleRejected(t *testing.T) {
	bad := []rules.Group{{Name: "g", Rules: []rules.Rule{{
		Name:       "bad",
		Action:     rules.Action("nonsense"),
		Conditions: rules.Conditions{IPCIDRs: []string{"1.2.3.4/32"}},
	}}}}
	link := encodeEnvelope(t, wireEnvelope{V: 1, Kind: "rules", Name: "n", Groups: bad})
	if _, err := Decode(link); err == nil {
		t.Errorf("expected validation error, got nil")
	}
}

func TestEncodeRejectsNoRules(t *testing.T) {
	if _, err := Encode("n", []rules.Group{{Name: "empty"}}); !errors.Is(err, ErrEmpty) {
		t.Errorf("err = %v, want ErrEmpty", err)
	}
}

func boundGroups() []rules.Group {
	g := sampleGroups()
	g[0].Rules[0].ServerID = "srv-local"
	g[0].Rules[0].ServerSourceID = "sub-local"
	g[0].Rules[0].ServerName = "NL 2"
	return g
}

func TestExport_KeepsServerNameOnly(t *testing.T) {
	link, err := Encode("n", boundGroups())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	p, err := Decode(link)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	r := p.Groups[0].Rules[0]
	if r.ServerID != "" || r.ServerSourceID != "" {
		t.Fatalf("local ids leaked into export: %+v", r)
	}
	if r.ServerName != "NL 2" {
		t.Fatalf("server name lost: %+v", r)
	}
	if len(r.Conditions.Domains) != 1 || r.Conditions.Domains[0].Value != "netflix.com" {
		t.Fatalf("conditions lost: %+v", r.Conditions)
	}
}

func TestImport_DropsForeignServerIDsKeepsName(t *testing.T) {
	link := encodeEnvelope(t, wireEnvelope{V: CurrentSchema, Kind: "rules", Name: "n", Groups: boundGroups()})
	p, err := Decode(link)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	r := p.Groups[0].Rules[0]
	if r.ServerID != "" || r.ServerSourceID != "" {
		t.Fatalf("foreign ids kept on import: %+v", r)
	}
	if r.ServerName != "NL 2" {
		t.Fatalf("server name dropped on import: %+v", r)
	}
}

func TestImport_DropsServerNameOnNonProxyRule(t *testing.T) {
	g := boundGroups()
	g[0].Rules[0].Action = rules.ActionDirect
	link := encodeEnvelope(t, wireEnvelope{V: CurrentSchema, Kind: "rules", Name: "n", Groups: g})
	p, err := Decode(link)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if r := p.Groups[0].Rules[0]; r.ServerName != "" {
		t.Fatalf("server name kept on a direct rule: %+v", r)
	}
}
