package configgen

import (
	"encoding/json"
	"testing"

	"github.com/itg-team/itg-ray/internal/rules"
)

func tunInbound(t *testing.T, in *SingboxInput) map[string]any {
	t.Helper()
	raw, err := BuildSingbox(in)
	if err != nil {
		t.Fatalf("BuildSingbox: %v", err)
	}
	var doc struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, ib := range doc.Inbounds {
		if ib["type"] == "tun" {
			return ib
		}
	}
	t.Fatalf("no tun inbound found")
	return nil
}

func TestBuildSingbox_TUN_ExcludeAddressSet(t *testing.T) {
	ib := tunInbound(t, &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		Rules:               rules.Model{DefaultAction: rules.ActionProxy},
		RouteExcludeAddress: []string{"203.0.113.7/32"},
	})
	excl, ok := ib["route_exclude_address"]
	if !ok {
		t.Fatal("expected route_exclude_address to be present")
	}
	got, _ := json.Marshal(excl)
	if string(got) != `["203.0.113.7/32"]` {
		t.Fatalf("route_exclude_address = %s, want [\"203.0.113.7/32\"]", got)
	}
}

func TestBuildSingbox_TUN_NoExcludeWhenUnset(t *testing.T) {
	ib := tunInbound(t, &SingboxInput{
		Mode: ModeTun, TunName: "ITGRay-TUN", TunIPv4: "198.18.0.1/15",
		Rules: rules.Model{DefaultAction: rules.ActionProxy},
	})
	if _, ok := ib["route_exclude_address"]; ok {
		t.Fatal("route_exclude_address must be absent when RouteExcludeAddress is empty")
	}
}
