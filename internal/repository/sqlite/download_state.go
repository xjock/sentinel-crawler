package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/xjock/sentinel-crawler/internal/domain"
)

// DownloadStateRepository SQLite 实现
type DownloadStateRepository struct {
	db *sql.DB
}

// NewDownloadStateRepository 创建 SQLite DownloadStateRepository
func NewDownloadStateRepository(db *sql.DB) *DownloadStateRepository {
	return &DownloadStateRepository{db: db}
}

// Save 保存下载状态
func (r *DownloadStateRepository) Save(ctx context.Context, state *domain.DownloadState) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO download_states (id, task_id, product_id, dest_path, total_bytes, received_bytes, checksum_expected, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_id=excluded.task_id,
			product_id=excluded.product_id,
			dest_path=excluded.dest_path,
			total_bytes=excluded.total_bytes,
			received_bytes=excluded.received_bytes,
			checksum_expected=excluded.checksum_expected,
			status=excluded.status,
			updated_at=excluded.updated_at
	`, state.ID, state.TaskID, state.ProductID, state.DestPath,
		state.TotalBytes, state.ReceivedBytes, state.ChecksumExpected, state.Status,
		state.CreatedAt, state.UpdatedAt)

	if err != nil {
		return fmt.Errorf("save download state: %w", err)
	}
	return nil
}

// FindByTaskID 按任务 ID 查询下载状态
func (r *DownloadStateRepository) FindByTaskID(ctx context.Context, taskID string) (*domain.DownloadState, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, task_id, product_id, dest_path, total_bytes, received_bytes, checksum_expected, status, created_at, updated_at
		FROM download_states WHERE task_id = ?
	`, taskID)

	var s domain.DownloadState
	err := row.Scan(
		&s.ID, &s.TaskID, &s.ProductID, &s.DestPath,
		&s.TotalBytes, &s.ReceivedBytes, &s.ChecksumExpected, &s.Status,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find download state: %w", err)
	}
	return &s, nil
}

// Delete 删除下载状态
func (r *DownloadStateRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM download_states WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete download state: %w", err)
	}
	return nil
}
