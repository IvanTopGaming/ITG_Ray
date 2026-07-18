package ruleshare

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/itg-team/itg-ray/internal/rules"
)

const (
	CurrentSchema   = 1
	ImportPrefix    = "itgray://rules/import/"
	maxPayloadBytes = 64 * 1024
	maxRules        = 500
)

var (
	ErrMalformed          = errors.New("ruleshare: malformed link")
	ErrUnsupportedVersion = errors.New("ruleshare: unsupported schema version")
	ErrTooLarge           = errors.New("ruleshare: payload too large")
	ErrEmpty              = errors.New("ruleshare: no rules in payload")
)

type Payload struct {
	Version int
	Name    string
	Groups  []rules.Group
}

type wireEnvelope struct {
	V      int           `json:"v"`
	Kind   string        `json:"kind"`
	Name   string        `json:"name"`
	Groups []rules.Group `json:"groups"`
}

func countRules(groups []rules.Group) int {
	n := 0
	for _, g := range groups {
		n += len(g.Rules)
	}
	return n
}

func Encode(name string, groups []rules.Group) (string, error) {
	if countRules(groups) == 0 {
		return "", ErrEmpty
	}
	norm := make([]rules.Group, len(groups))
	for i, g := range groups {
		rs := make([]rules.Rule, len(g.Rules))
		for j := range g.Rules {
			rs[j] = g.Rules[j]
			rs[j].ID = ""
		}
		norm[i] = rules.Group{Name: g.Name, Enabled: g.Enabled, Rules: rs}
	}
	b, err := json.Marshal(wireEnvelope{V: CurrentSchema, Kind: "rules", Name: name, Groups: norm})
	if err != nil {
		return "", fmt.Errorf("ruleshare: marshal: %w", err)
	}
	return ImportPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func Decode(link string) (Payload, error) {
	if !strings.HasPrefix(link, ImportPrefix) {
		return Payload{}, ErrMalformed
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, ImportPrefix))
	if err != nil {
		return Payload{}, fmt.Errorf("%w: base64: %w", ErrMalformed, err)
	}
	if len(raw) > maxPayloadBytes {
		return Payload{}, ErrTooLarge
	}
	var env wireEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Payload{}, fmt.Errorf("%w: json: %w", ErrMalformed, err)
	}
	if env.V > CurrentSchema {
		return Payload{}, ErrUnsupportedVersion
	}
	if env.Kind != "rules" {
		return Payload{}, ErrMalformed
	}
	if countRules(env.Groups) == 0 {
		return Payload{}, ErrEmpty
	}
	if countRules(env.Groups) > maxRules {
		return Payload{}, ErrTooLarge
	}
	for gi := range env.Groups {
		for ri := range env.Groups[gi].Rules {
			chk := env.Groups[gi].Rules[ri]
			if chk.ID == "" {
				chk.ID = "import"
			}
			if err := chk.Validate(); err != nil {
				return Payload{}, fmt.Errorf("ruleshare: rule %q: %w", chk.Name, err)
			}
			env.Groups[gi].Rules[ri].ID = ""
		}
		env.Groups[gi].ID = ""
	}
	return Payload{Version: env.V, Name: env.Name, Groups: env.Groups}, nil
}

func encodeEnvelope(t interface{ Fatalf(string, ...any) }, env wireEnvelope) string {
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("encodeEnvelope: %v", err)
	}
	return ImportPrefix + base64.RawURLEncoding.EncodeToString(b)
}
