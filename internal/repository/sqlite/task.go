package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xjock/sentinel-crawler/internal/domain"
)

// TaskRepository SQLite 实现
type TaskRepository struct {
	db *sql.DB
}

// NewTaskRepository 创建 SQLite TaskRepository
func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create 创建任务
func (r *TaskRepository) Create(ctx context.Context, task *domain.Task) error {
	specJSON, err := json.Marshal(task.Spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tasks (id, type, status, spec, progress, error, retries, worker_id, started_at, ended_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.Type, task.Status, string(specJSON), task.Progress, task.Error,
		task.Retries, task.WorkerID, task.StartedAt, task.EndedAt, task.CreatedAt, task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// Update 更新任务
func (r *TaskRepository) Update(ctx context.Context, task *domain.Task) error {
	specJSON, err := json.Marshal(task.Spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE tasks SET type=?, status=?, spec=?, progress=?, error=?,
			retries=?, worker_id=?, started_at=?, ended_at=?, updated_at=?
		WHERE id=?
	`, task.Type, task.Status, string(specJSON), task.Progress, task.Error,
		task.Retries, task.WorkerID, task.StartedAt, task.EndedAt, task.UpdatedAt, task.ID)

	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// FindByID 按 ID 查询任务
func (r *TaskRepository) FindByID(ctx context.Context, id string) (*domain.Task, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, status, spec, progress, error, retries, worker_id, started_at, ended_at, created_at, updated_at
		FROM tasks WHERE id = ?
	`, id)
	return r.scanTask(row)
}

// FindByStatus 按状态查询任务
func (r *TaskRepository) FindByStatus(ctx context.Context, status domain.TaskStatus, limit int) ([]*domain.Task, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type, status, spec, progress, error, retries, worker_id, started_at, ended_at, created_at, updated_at
		FROM tasks WHERE status = ? ORDER BY created_at ASC LIMIT ?
	`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("find by status: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		t, err := r.scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// List 任务列表
func (r *TaskRepository) List(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error) {
	var conds []string
	var args []any

	if filter.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, filter.Status)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := filter.PageSize
	if limit <= 0 {
		limit = 100
	}
	offset := (filter.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	q := fmt.Sprintf(`
		SELECT id, type, status, spec, progress, error, retries, worker_id, started_at, ended_at, created_at, updated_at
		FROM tasks %s ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, where)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		t, err := r.scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Count 统计任务数量
func (r *TaskRepository) Count(ctx context.Context, filter domain.TaskFilter) (int, error) {
	var conds []string
	var args []any

	if filter.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, filter.Status)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	q := fmt.Sprintf(`SELECT COUNT(1) FROM tasks %s`, where)
	var cnt int
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}
	return cnt, nil
}

func (r *TaskRepository) scanTask(s scanner) (*domain.Task, error) {
	var t domain.Task
	var specJSON string
	var started, ended sql.NullTime

	err := s.Scan(
		&t.ID, &t.Type, &t.Status, &specJSON, &t.Progress, &t.Error,
		&t.Retries, &t.WorkerID, &started, &ended, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}

	if started.Valid {
		t.StartedAt = &started.Time
	}
	if ended.Valid {
		t.EndedAt = &ended.Time
	}

	if specJSON != "" {
		_ = json.Unmarshal([]byte(specJSON), &t.Spec)
	}

	return &t, nil
}
