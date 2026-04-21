package domain

import "time"

// SearchQuery 元数据搜索条件
type SearchQuery struct {
	Platform      string
	ProductType   string
	SensingFrom   time.Time
	SensingTo     time.Time
	IngestionFrom time.Time
	IngestionTo   time.Time
	Footprint     Geometry
	Page          int
	PageSize      int
	OrderBy       string
	OrderDir      string
}

// SearchResult 搜索结果
type SearchResult struct {
	Products   []*Product
	TotalCount int
	Page       int
	PageSize   int
	HasMore    bool
}
