package domain

import "time"

// TaskType 任务类型
type TaskType string

const (
	TaskTypeMetadataCrawl TaskType = "metadata_crawl"
	TaskTypeDownload      TaskType = "download"
	TaskTypeFullPipeline  TaskType = "full_pipeline"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 表示一个抓取任务
type Task struct {
	ID        string
	Type      TaskType
	Status    TaskStatus
	Spec      TaskSpec
	Progress  float64
	Error     string
	Retries   int
	WorkerID  string
	StartedAt *time.Time
	EndedAt   *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaskSpec 任务规格
type TaskSpec struct {
	Provider   string
	Query      SearchQuery
	DestDir    string
	MaxRetries int
}

// TaskFilter 任务列表过滤条件
type TaskFilter struct {
	Type     TaskType
	Status   TaskStatus
	Page     int
	PageSize int
	OrderBy  string
	OrderDir string
}
