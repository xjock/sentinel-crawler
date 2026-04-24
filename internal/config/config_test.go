package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected driver sqlite, got %s", cfg.Database.Driver)
	}
	if cfg.Providers.Active != "copernicus" {
		t.Errorf("expected active copernicus, got %s", cfg.Providers.Active)
	}
	if cfg.Providers.Copernicus.BaseURL == "" {
		t.Error("expected copernicus base_url to be set")
	}
	if cfg.Crawler.PageSize != 100 {
		t.Errorf("expected page_size 100, got %d", cfg.Crawler.PageSize)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	content := `
server:
  port: 9090
providers:
  active: aws
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Providers.Active != "aws" {
		t.Errorf("expected active aws, got %s", cfg.Providers.Active)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("SENTINEL_CRAWLER_SERVER_PORT", "7777")
	defer os.Unsetenv("SENTINEL_CRAWLER_SERVER_PORT")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", cfg.Server.Port)
	}
}
