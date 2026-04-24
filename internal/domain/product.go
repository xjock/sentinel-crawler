package domain

import "time"

// Product 表示一个哨兵卫星产品元数据
type Product struct {
	ID            string
	Name          string
	Platform      string // S1A, S1B, S2A, S2B, S3A, S3B, S5P
	ProductType   string // GRD, SLC, OCN, L1C, L2A, etc.
	SensingDate   time.Time
	IngestionDate time.Time
	Footprint     Geometry
	Size          int64
	DownloadURL   string
	Checksum      string
	ChecksumAlgo  string            // MD5, SHA256, etc.
	Metadata      map[string]string // 扩展元数据
	RawXML        string            // 原始 OpenSearch/XML 响应
	Source        string            // 数据源标识，如 "copernicus", "aws"
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ProductQuery 产品查询条件
type ProductQuery struct {
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
