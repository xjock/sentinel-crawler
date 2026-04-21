package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lucavallin/sentinel-crawler/internal/domain"
)

// ProductRepository SQLite 实现
type ProductRepository struct {
	db *sql.DB
}

// NewProductRepository 创建 SQLite ProductRepository
func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// Save 保存单个产品
func (r *ProductRepository) Save(ctx context.Context, product *domain.Product) error {
	coordsJSON, err := json.Marshal(product.Footprint.Coordinates)
	if err != nil {
		return fmt.Errorf("marshal footprint: %w", err)
	}
	metaJSON, err := json.Marshal(product.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO products (id, name, platform, product_type, sensing_date, ingestion_date,
			footprint_type, footprint_coords, size, download_url, checksum, checksum_algo,
			metadata, raw_xml, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			platform=excluded.platform,
			product_type=excluded.product_type,
			sensing_date=excluded.sensing_date,
			ingestion_date=excluded.ingestion_date,
			footprint_type=excluded.footprint_type,
			footprint_coords=excluded.footprint_coords,
			size=excluded.size,
			download_url=excluded.download_url,
			checksum=excluded.checksum,
			checksum_algo=excluded.checksum_algo,
			metadata=excluded.metadata,
			raw_xml=excluded.raw_xml,
			source=excluded.source,
			updated_at=excluded.updated_at
	`, product.ID, product.Name, product.Platform, product.ProductType,
		product.SensingDate, product.IngestionDate,
		product.Footprint.Type, string(coordsJSON), product.Size,
		product.DownloadURL, product.Checksum, product.ChecksumAlgo,
		string(metaJSON), product.RawXML, product.Source,
		product.CreatedAt, product.UpdatedAt)

	if err != nil {
		return fmt.Errorf("save product: %w", err)
	}
	return nil
}

// SaveBatch 批量保存产品
func (r *ProductRepository) SaveBatch(ctx context.Context, products []*domain.Product) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO products (id, name, platform, product_type, sensing_date, ingestion_date,
			footprint_type, footprint_coords, size, download_url, checksum, checksum_algo,
			metadata, raw_xml, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			platform=excluded.platform,
			product_type=excluded.product_type,
			sensing_date=excluded.sensing_date,
			ingestion_date=excluded.ingestion_date,
			footprint_type=excluded.footprint_type,
			footprint_coords=excluded.footprint_coords,
			size=excluded.size,
			download_url=excluded.download_url,
			checksum=excluded.checksum,
			checksum_algo=excluded.checksum_algo,
			metadata=excluded.metadata,
			raw_xml=excluded.raw_xml,
			source=excluded.source,
			updated_at=excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, p := range products {
		coordsJSON, _ := json.Marshal(p.Footprint.Coordinates)
		metaJSON, _ := json.Marshal(p.Metadata)
		_, err := stmt.ExecContext(ctx,
			p.ID, p.Name, p.Platform, p.ProductType,
			p.SensingDate, p.IngestionDate,
			p.Footprint.Type, string(coordsJSON), p.Size,
			p.DownloadURL, p.Checksum, p.ChecksumAlgo,
			string(metaJSON), p.RawXML, p.Source,
			p.CreatedAt, p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("exec batch: %w", err)
		}
	}

	return tx.Commit()
}

// FindByID 按 ID 查询产品
func (r *ProductRepository) FindByID(ctx context.Context, id string) (*domain.Product, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, platform, product_type, sensing_date, ingestion_date,
			footprint_type, footprint_coords, size, download_url, checksum, checksum_algo,
			metadata, raw_xml, source, created_at, updated_at
		FROM products WHERE id = ?
	`, id)
	return r.scanProduct(row)
}

// FindByQuery 按条件查询产品
func (r *ProductRepository) FindByQuery(ctx context.Context, query domain.ProductQuery) ([]*domain.Product, error) {
	var conds []string
	var args []any

	if query.Platform != "" {
		conds = append(conds, "platform = ?")
		args = append(args, query.Platform)
	}
	if query.ProductType != "" {
		conds = append(conds, "product_type = ?")
		args = append(args, query.ProductType)
	}
	if !query.SensingFrom.IsZero() {
		conds = append(conds, "sensing_date >= ?")
		args = append(args, query.SensingFrom)
	}
	if !query.SensingTo.IsZero() {
		conds = append(conds, "sensing_date <= ?")
		args = append(args, query.SensingTo)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := query.PageSize
	if limit <= 0 {
		limit = 100
	}
	offset := (query.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	q := fmt.Sprintf(`
		SELECT id, name, platform, product_type, sensing_date, ingestion_date,
			footprint_type, footprint_coords, size, download_url, checksum, checksum_algo,
			metadata, raw_xml, source, created_at, updated_at
		FROM products %s ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, where)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// Exists 检查产品是否存在
func (r *ProductRepository) Exists(ctx context.Context, id string) (bool, error) {
	var cnt int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM products WHERE id = ?`, id).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("exists: %w", err)
	}
	return cnt > 0, nil
}

// Count 统计产品数量
func (r *ProductRepository) Count(ctx context.Context, query domain.ProductQuery) (int, error) {
	var cnt int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM products`).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return cnt, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *ProductRepository) scanProduct(s scanner) (*domain.Product, error) {
	var p domain.Product
	var coordsJSON, metaJSON string
	var sensing, ingestion sql.NullTime

	err := s.Scan(
		&p.ID, &p.Name, &p.Platform, &p.ProductType,
		&sensing, &ingestion,
		&p.Footprint.Type, &coordsJSON, &p.Size,
		&p.DownloadURL, &p.Checksum, &p.ChecksumAlgo,
		&metaJSON, &p.RawXML, &p.Source,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan product: %w", err)
	}

	if sensing.Valid {
		p.SensingDate = sensing.Time
	}
	if ingestion.Valid {
		p.IngestionDate = ingestion.Time
	}

	if coordsJSON != "" {
		_ = json.Unmarshal([]byte(coordsJSON), &p.Footprint.Coordinates)
	}
	if metaJSON != "" {
		_ = json.Unmarshal([]byte(metaJSON), &p.Metadata)
	}
	if p.Metadata == nil {
		p.Metadata = make(map[string]string)
	}

	return &p, nil
}
