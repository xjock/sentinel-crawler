package domain

import "context"

// ProductRepository 元数据持久化接口
type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	SaveBatch(ctx context.Context, products []*Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
	FindByQuery(ctx context.Context, query ProductQuery) ([]*Product, error)
	Exists(ctx context.Context, id string) (bool, error)
	Count(ctx context.Context, query ProductQuery) (int, error)
}

// TaskRepository 任务状态持久化接口
type TaskRepository interface {
	Create(ctx context.Context, task *Task) error
	Update(ctx context.Context, task *Task) error
	FindByID(ctx context.Context, id string) (*Task, error)
	FindByStatus(ctx context.Context, status TaskStatus, limit int) ([]*Task, error)
	List(ctx context.Context, filter TaskFilter) ([]*Task, error)
	Count(ctx context.Context, filter TaskFilter) (int, error)
}

// DownloadStateRepository 断点续传状态持久化接口
type DownloadStateRepository interface {
	Save(ctx context.Context, state *DownloadState) error
	FindByTaskID(ctx context.Context, taskID string) (*DownloadState, error)
	Delete(ctx context.Context, id string) error
}
