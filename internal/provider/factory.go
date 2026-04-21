package provider

import (
	"github.com/xjock/sentinel-crawler/internal/config"
	"github.com/xjock/sentinel-crawler/internal/domain"
	"github.com/xjock/sentinel-crawler/internal/provider/copernicus"
)

func init() {
	Register("copernicus", func(cfg config.CopernicusConfig) (domain.Provider, error) {
		return copernicus.NewProvider(cfg), nil
	})
}
