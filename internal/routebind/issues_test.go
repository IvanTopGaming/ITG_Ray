package routebind

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssues_SetGetCopies(t *testing.T) {
	var nilIssues *Issues
	require.Empty(t, nilIssues.Get())
	nilIssues.Set(map[string]Binding{"x": {}})

	i := &Issues{}
	in := map[string]Binding{"r1": {RuleID: "r1", State: StateUnusable, Reason: "dns"}}
	i.Set(in)
	in["r2"] = Binding{}
	got := i.Get()
	require.Len(t, got, 1)
	got["r3"] = Binding{}
	require.Len(t, i.Get(), 1)
}
