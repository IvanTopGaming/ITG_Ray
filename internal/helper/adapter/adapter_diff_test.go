package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiff_NewAdapterAfter(t *testing.T) {
	before := []Adapter{{LUID: 1, FriendlyName: "Ethernet"}}
	after := []Adapter{
		{LUID: 1, FriendlyName: "Ethernet"},
		{LUID: 99, FriendlyName: "ITGRay-TUN"},
	}
	added := Diff(before, after)
	require.Len(t, added, 1)
	require.Equal(t, uint64(99), added[0].LUID)
}

func TestDiff_NoChange(t *testing.T) {
	before := []Adapter{{LUID: 1}}
	after := []Adapter{{LUID: 1}}
	require.Empty(t, Diff(before, after))
}

func TestDiff_RemovedIsIgnored(t *testing.T) {
	before := []Adapter{{LUID: 1}, {LUID: 2}}
	after := []Adapter{{LUID: 1}}
	require.Empty(t, Diff(before, after))
}

func TestDiff_MultipleNew(t *testing.T) {
	before := []Adapter{{LUID: 1}}
	after := []Adapter{{LUID: 1}, {LUID: 2}, {LUID: 3}}
	added := Diff(before, after)
	require.Len(t, added, 2)
}
