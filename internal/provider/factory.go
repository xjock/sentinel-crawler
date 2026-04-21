package provider

import (
	"github.com/lucavallin/sentinel-crawler/internal/config"
	"github.com/lucavallin/sentinel-crawler/internal/domain"
	"github.com/lucavallin/sentinel-crawler/internal/provider/copernicus"
)

func init() {
	Register("copernicus", func(cfg config.CopernicusConfig) (domain.Provider, error) {
		return copernicus.NewProvider(cfg), nil
	})
}
