package main

import (
	"context"

	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/configgen"
	"github.com/itg-team/itg-ray/internal/geo"
	"github.com/itg-team/itg-ray/internal/rules"
)

type geoService struct {
	mgr        *geo.Manager
	configPath string
	dataDir    string
	ruleStore  *rules.Store
}

func unionGeoTags(ruleTags []string) []string {
	seen := make(map[string]bool, len(geo.BaseTags)+len(ruleTags))
	out := make([]string, 0, len(geo.BaseTags)+len(ruleTags))
	for _, t := range geo.BaseTags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range ruleTags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func (g *geoService) Refresh(ctx context.Context) error {
	c, err := config.Load(g.configPath)
	if err != nil {
		return err
	}
	model := loadRulesFromDataDir(g.dataDir, g.ruleStore)
	tags := unionGeoTags(configgen.GeoTags(model))
	return g.mgr.Refresh(ctx, geo.Source{
		Preset:    c.Network.GeoSource.Preset,
		CustomURL: c.Network.GeoSource.CustomURL,
	}, tags)
}
