package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/lucavallin/sentinel-crawler/internal/domain"
)

// TaskManager 任务生命周期编排接口
type TaskManager interface {
	CreateTask(ctx context.Context, spec domain.TaskSpec) (*domain.Task, error)
	GetTask(ctx context.Context, id string) (*domain.Task, error)
	ListTasks(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error)
	CancelTask(ctx context.Context, id string) error
	RetryTask(ctx context.Context, id string) error
}

// taskManager 实现
type taskManager struct {
	taskRepo  domain.TaskRepository
	taskQueue domain.TaskQueue
	logger    *slog.Logger
}

// NewTaskManager 创建 TaskManager
func NewTaskManager(taskRepo domain.TaskRepository, taskQueue domain.TaskQueue, logger *slog.Logger) TaskManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &taskManager{
		taskRepo:  taskRepo,
		taskQueue: taskQueue,
		logger:    logger,
	}
}

// CreateTask 创建任务并入队
func (tm *taskManager) CreateTask(ctx context.Context, spec domain.TaskSpec) (*domain.Task, error) {
	task := &domain.Task{
		ID:        uuid.NewString(),
		Type:      domain.TaskTypeMetadataCrawl,
		Status:    domain.TaskStatusPending,
		Spec:      spec,
		Progress:  0,
		Retries:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if spec.DestDir != "" {
		task.Type = domain.TaskTypeDownload
	}

	if err := tm.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	if err := tm.taskQueue.Enqueue(ctx, task); err != nil {
		tm.logger.Warn("enqueue failed", "task_id", task.ID, "error", err)
		// 不入队也能通过 API 手动触发
	}

	tm.logger.Info("task created", "task_id", task.ID, "type", task.Type)
	return task, nil
}

// GetTask 获取任务详情
func (tm *taskManager) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	task, err := tm.taskRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

// ListTasks 列出任务
func (tm *taskManager) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error) {
	tasks, err := tm.taskRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

// CancelTask 取消任务
func (tm *taskManager) CancelTask(ctx context.Context, id string) error {
	task, err := tm.taskRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	if task.Status == domain.TaskStatusCompleted || task.Status == domain.TaskStatusFailed {
		return fmt.Errorf("cannot cancel task in status %s", task.Status)
	}

	task.Status = domain.TaskStatusCancelled
	task.UpdatedAt = time.Now()

	if err := tm.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	tm.logger.Info("task cancelled", "task_id", id)
	return nil
}

// RetryTask 重试失败任务
func (tm *taskManager) RetryTask(ctx context.Context, id string) error {
	task, err := tm.taskRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	if task.Status != domain.TaskStatusFailed && task.Status != domain.TaskStatusCancelled {
		return fmt.Errorf("can only retry failed or cancelled tasks")
	}

	task.Status = domain.TaskStatusPending
	task.Error = ""
	task.Progress = 0
	task.Retries++
	task.UpdatedAt = time.Now()

	if err := tm.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	if err := tm.taskQueue.Enqueue(ctx, task); err != nil {
		tm.logger.Warn("retry enqueue failed", "task_id", id, "error", err)
	}

	tm.logger.Info("task retried", "task_id", id, "retries", task.Retries)
	return nil
}
