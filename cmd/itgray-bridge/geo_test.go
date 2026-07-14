package main

import (
	"sort"
	"testing"

	"github.com/itg-team/itg-ray/internal/geo"
)

func TestUnionGeoTags_IncludesBaseSet(t *testing.T) {
	got := unionGeoTags(nil)
	sort.Strings(got)
	want := append([]string{}, geo.BaseTags...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("empty rules → %v, want base set %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUnionGeoTags_DedupsRuleTags(t *testing.T) {
	got := unionGeoTags([]string{"geoip-ru", "geosite-netflix"})
	seen := map[string]int{}
	for _, tag := range got {
		seen[tag]++
	}
	if seen["geoip-ru"] != 1 {
		t.Fatalf("geoip-ru duplicated: %v", got)
	}
	if seen["geosite-netflix"] != 1 {
		t.Fatalf("rule tag geosite-netflix missing: %v", got)
	}
}
