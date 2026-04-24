package provider

import (
	"fmt"
	"sync"

	"github.com/xjock/sentinel-crawler/internal/config"
	"github.com/xjock/sentinel-crawler/internal/domain"
)

// Factory Provider 工厂函数类型
type Factory func(cfg config.CopernicusConfig) (domain.Provider, error)

var (
	registry = make(map[string]Factory)
	mu       sync.RWMutex
)

// Register 注册 Provider 工厂
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Get 获取已注册的 Provider 工厂
func Get(name string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	return f, ok
}

// List 列出所有已注册的 Provider 名称
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Build 根据全局配置构建当前激活的 Provider
func Build(providersCfg config.ProvidersConfig) (domain.Provider, error) {
	factory, ok := Get(providersCfg.Active)
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", providersCfg.Active)
	}

	switch providersCfg.Active {
	case "copernicus":
		return factory(providersCfg.Copernicus)
	default:
		return nil, fmt.Errorf("unsupported active provider: %q", providersCfg.Active)
	}
}
