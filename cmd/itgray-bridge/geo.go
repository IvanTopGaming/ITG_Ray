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

func (g *geoService) Refresh(ctx context.Context) error {
	c, err := config.Load(g.configPath)
	if err != nil {
		return err
	}
	model := loadRulesFromDataDir(g.dataDir, g.ruleStore)
	tags := configgen.GeoTags(model)
	if len(tags) == 0 {
		return nil
	}
	return g.mgr.Refresh(ctx, geo.Source{
		Preset:    c.Network.GeoSource.Preset,
		CustomURL: c.Network.GeoSource.CustomURL,
	}, tags)
}
