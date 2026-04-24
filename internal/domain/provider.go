package domain

import (
	"context"
	"io"
)

// Provider 定义统一的数据源接口
type Provider interface {
	// Name 返回 Provider 唯一标识名
	Name() string

	// Search 按条件搜索元数据，返回分页结果
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)

	// GetProduct 获取单个产品的完整元数据
	GetProduct(ctx context.Context, id string) (*Product, error)

	// Fetch 获取产品数据流，由调用方决定写入目标
	Fetch(ctx context.Context, product *Product) (io.ReadCloser, error)

	// HealthCheck 检查数据源可用性
	HealthCheck(ctx context.Context) error
}
