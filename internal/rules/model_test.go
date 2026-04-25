package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate_OK(t *testing.T) {
	r := Rule{
		ID:      "r1",
		Name:    "Steam",
		Enabled: true,
		Action:  ActionDirect,
		Conditions: Conditions{
			Processes: []string{"steam.exe"},
			Ports:     []PortSpec{{Single: 443}, {From: 27015, To: 27030}},
		},
	}
	require.NoError(t, r.Validate())
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		r    Rule
	}{
		{"no id", Rule{Action: ActionProxy}},
		{"bad action", Rule{ID: "x", Action: Action("zzz")}},
		{"empty conditions", Rule{ID: "x", Action: ActionProxy}},
		{"bad port range", Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{Ports: []PortSpec{{From: 500, To: 100}}}}},
		{"bad cidr", Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{IPCIDRs: []string{"not-a-cidr"}}}},
	}
	for _, c := range cases {
		require.Error(t, c.r.Validate(), c.name)
	}
}

func TestPortSpec_Covers(t *testing.T) {
	require.True(t, PortSpec{Single: 443}.Covers(443))
	require.False(t, PortSpec{Single: 443}.Covers(444))
	require.True(t, PortSpec{From: 10, To: 20}.Covers(15))
	require.False(t, PortSpec{From: 10, To: 20}.Covers(21))
}

func TestValidate_DomainKindMustBeKnown(t *testing.T) {
	r := Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{Domains: []DomainMatcher{{Kind: "sufix", Value: "example.com"}}}}
	err := r.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "kind")
}

func TestValidate_DomainEmptyValue(t *testing.T) {
	r := Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{Domains: []DomainMatcher{{Kind: "exact", Value: ""}}}}
	require.Error(t, r.Validate())
}

func TestValidate_DomainBadRegex(t *testing.T) {
	r := Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{Domains: []DomainMatcher{{Kind: "regex", Value: "[unclosed"}}}}
	require.Error(t, r.Validate())
}

func TestValidate_PortBothSingleAndRange(t *testing.T) {
	r := Rule{ID: "x", Action: ActionProxy, Conditions: Conditions{Ports: []PortSpec{{Single: 443, From: 100, To: 200}}}}
	err := r.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Single and From/To")
}
