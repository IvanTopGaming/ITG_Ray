package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
)

func geoTestModel() rules.Model {
	return rules.Model{
		DefaultAction: rules.ActionProxy,
		Groups: []rules.Group{{
			ID:      "g1",
			Name:    "My Rules",
			Enabled: true,
			Rules: []rules.Rule{{
				ID:      "r1",
				Enabled: true,
				Action:  rules.ActionDirect,
				Conditions: rules.Conditions{
					Geo: []string{"geosite:category-ru", "geoip:ru"},
				},
			}},
		}},
	}
}

func TestGeoTags(t *testing.T) {
	got := GeoTags(geoTestModel())
	want := map[string]bool{"geosite-category-ru": true, "geoip-ru": true}
	if len(got) != 2 {
		t.Fatalf("GeoTags = %v", got)
	}
	for _, tag := range got {
		if !want[tag] {
			t.Fatalf("unexpected tag %q in %v", tag, got)
		}
	}
}

func TestGeoTags_NoGeo(t *testing.T) {
	m := rules.Model{DefaultAction: rules.ActionProxy}
	if got := GeoTags(m); len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestBuildSingbox_GeoDeclaresLocalRuleSets(t *testing.T) {
	in := &SingboxInput{
		Mode:  ModeSysProxy,
		Rules: geoTestModel(),
		GeoRuleSets: map[string]string{
			"geosite-category-ru": "/cache/geo/sagernet/geosite-category-ru.srs",
			"geoip-ru":            "/cache/geo/sagernet/geoip-ru.srs",
		},
	}
	raw, err := BuildSingbox(in)
	if err != nil {
		t.Fatalf("BuildSingbox: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	route := doc["route"].(map[string]any)
	decls, ok := route["rule_set"].([]any)
	if !ok || len(decls) != 2 {
		t.Fatalf("route.rule_set = %v", route["rule_set"])
	}
	byTag := map[string]map[string]any{}
	for _, d := range decls {
		m := d.(map[string]any)
		byTag[m["tag"].(string)] = m
	}
	site := byTag["geosite-category-ru"]
	if site["type"] != "local" || site["format"] != "binary" ||
		site["path"] != "/cache/geo/sagernet/geosite-category-ru.srs" {
		t.Fatalf("geosite decl wrong: %v", site)
	}
	if _, hasExp := doc["experimental"]; hasExp {
		t.Fatalf("experimental block must not be emitted for local rule-sets: %v", doc["experimental"])
	}
}

func TestBuildSingbox_MissingPathErrors(t *testing.T) {
	in := &SingboxInput{
		Mode:        ModeSysProxy,
		Rules:       geoTestModel(),
		GeoRuleSets: map[string]string{"geosite-category-ru": "/c/geosite-category-ru.srs"},
	}
	_, err := BuildSingbox(in)
	if err == nil {
		t.Fatal("expected error for geoip-ru missing from GeoRuleSets")
	}
}

func TestBuildSingbox_NoGeoNoRuleSet(t *testing.T) {
	in := &SingboxInput{Mode: ModeSysProxy, Rules: rules.Model{DefaultAction: rules.ActionProxy}}
	raw, err := BuildSingbox(in)
	if err != nil {
		t.Fatalf("BuildSingbox: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	route := doc["route"].(map[string]any)
	if _, ok := route["rule_set"]; ok {
		t.Fatalf("no-geo config must not emit route.rule_set")
	}
	if _, ok := doc["experimental"]; ok {
		t.Fatalf("no-geo config must not emit experimental")
	}
}
