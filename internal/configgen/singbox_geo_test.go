package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
)

func geoModel() rules.Model {
	return rules.Model{
		DefaultAction: rules.ActionProxy,
		Groups: []rules.Group{{
			Enabled: true,
			Rules: []rules.Rule{{
				ID:      "geo-ru",
				Enabled: true, Action: rules.ActionDirect,
				Conditions: rules.Conditions{Geo: []string{"geosite:ru", "geoip:ru"}},
			}},
		}},
	}
}

func fullDoc(t *testing.T, in *SingboxInput) map[string]any {
	t.Helper()
	raw, err := BuildSingbox(in)
	if err != nil {
		t.Fatalf("BuildSingbox: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}

func TestGeoRuleSetURL(t *testing.T) {
	base := "https://raw.githubusercontent.com/SagerNet"
	if got := geoRuleSetURL("geosite-ru", base); got != base+"/sing-geosite/rule-set/geosite-ru.srs" {
		t.Fatalf("geosite url = %s", got)
	}
	if got := geoRuleSetURL("geoip-ru", base); got != base+"/sing-geoip/rule-set/geoip-ru.srs" {
		t.Fatalf("geoip url = %s", got)
	}
	// trailing slash on base is trimmed
	if got := geoRuleSetURL("geosite-ru", base+"/"); got != base+"/sing-geosite/rule-set/geosite-ru.srs" {
		t.Fatalf("trailing-slash url = %s", got)
	}
}

func TestBuildSingbox_GeoDeclaresRuleSets(t *testing.T) {
	doc := fullDoc(t, &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		Rules: geoModel(),
	})
	route := doc["route"].(map[string]any)
	rsList, ok := route["rule_set"].([]any)
	if !ok || len(rsList) != 2 {
		t.Fatalf("want 2 rule_set decls, got %v", route["rule_set"])
	}
	byTag := map[string]map[string]any{}
	for _, e := range rsList {
		m := e.(map[string]any)
		byTag[m["tag"].(string)] = m
	}
	gs := byTag["geosite-ru"]
	if gs == nil || gs["type"] != "remote" || gs["format"] != "binary" ||
		gs["download_detour"] != "proxy" ||
		gs["url"] != "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-ru.srs" {
		t.Fatalf("geosite-ru decl wrong: %v", gs)
	}
	if byTag["geoip-ru"]["url"] != "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-ru.srs" {
		t.Fatalf("geoip-ru url wrong: %v", byTag["geoip-ru"])
	}
	exp, ok := doc["experimental"].(map[string]any)
	if !ok || exp["cache_file"].(map[string]any)["enabled"] != true {
		t.Fatalf("cache_file not enabled: %v", doc["experimental"])
	}
}

func TestBuildSingbox_GeoCustomBaseURL(t *testing.T) {
	doc := fullDoc(t, &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		Rules: geoModel(), GeoBaseURL: "https://mirror.example/geo",
	})
	route := doc["route"].(map[string]any)
	for _, e := range route["rule_set"].([]any) {
		m := e.(map[string]any)
		if m["tag"] == "geosite-ru" &&
			m["url"] != "https://mirror.example/geo/sing-geosite/rule-set/geosite-ru.srs" {
			t.Fatalf("custom base not applied: %v", m["url"])
		}
	}
}

func TestBuildSingbox_NoGeoNoRuleSet(t *testing.T) {
	doc := fullDoc(t, &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		Rules: rules.Model{DefaultAction: rules.ActionProxy},
	})
	route := doc["route"].(map[string]any)
	if _, ok := route["rule_set"]; ok {
		t.Fatal("no geo → no route.rule_set block")
	}
	if _, ok := doc["experimental"]; ok {
		t.Fatal("no geo → no experimental.cache_file block")
	}
}
