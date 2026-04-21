package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lucavallin/sentinel-crawler/internal/domain"
	"github.com/lucavallin/sentinel-crawler/internal/usecase"
)

// Worker 任务消费 Worker
type Worker struct {
	queue      domain.TaskQueue
	taskRepo   domain.TaskRepository
	crawler    usecase.Crawler
	downloader usecase.Downloader
	logger     *slog.Logger
	workers    int
}

// NewWorker 创建 Worker
func NewWorker(
	queue domain.TaskQueue,
	taskRepo domain.TaskRepository,
	crawler usecase.Crawler,
	downloader usecase.Downloader,
	workers int,
	logger *slog.Logger,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if workers <= 0 {
		workers = 1
	}
	return &Worker{
		queue:      queue,
		taskRepo:   taskRepo,
		crawler:    crawler,
		downloader: downloader,
		logger:     logger,
		workers:    workers,
	}
}

// Start 启动 Worker 池，阻塞直到 context 取消
func (w *Worker) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go w.run(ctx, &wg, i)
	}
	wg.Wait()
	w.logger.Info("all workers stopped")
}

func (w *Worker) run(ctx context.Context, wg *sync.WaitGroup, id int) {
	defer wg.Done()
	workerID := fmt.Sprintf("worker-%d", id)
	w.logger.Info("worker started", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker shutting down", "worker_id", workerID)
			return
		default:
		}

		task, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Warn("dequeue failed", "worker_id", workerID, "error", err)
			continue
		}

		w.logger.Info("task dequeued", "worker_id", workerID, "task_id", task.ID, "type", task.Type)
		w.processTask(ctx, task, workerID)
	}
}

func (w *Worker) processTask(ctx context.Context, task *domain.Task, workerID string) {
	// 标记为运行中
	now := time.Now()
	task.Status = domain.TaskStatusRunning
	task.WorkerID = workerID
	task.StartedAt = &now
	task.UpdatedAt = now

	if err := w.taskRepo.Update(ctx, task); err != nil {
		w.logger.Error("mark running failed", "task_id", task.ID, "error", err)
		_ = w.queue.Nack(ctx, task.ID, true)
		return
	}

	// 执行
	var execErr error
	switch task.Type {
	case domain.TaskTypeMetadataCrawl:
		execErr = w.crawler.Crawl(ctx, task.Spec)
	case domain.TaskTypeDownload:
		execErr = w.processDownloadTask(ctx, task)
	case domain.TaskTypeFullPipeline:
		execErr = w.processFullPipeline(ctx, task)
	default:
		execErr = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if execErr != nil {
		w.handleFailure(ctx, task, execErr)
		return
	}

	w.handleSuccess(ctx, task)
}

func (w *Worker) processDownloadTask(ctx context.Context, task *domain.Task) error {
	// 下载任务需要查询产品列表
	// 简化实现：从 metadata 中查找待下载的产品
	// TODO: 实际实现需要关联 ProductRepository 查询
	return fmt.Errorf("download task not yet fully implemented")
}

func (w *Worker) processFullPipeline(ctx context.Context, task *domain.Task) error {
	if err := w.crawler.Crawl(ctx, task.Spec); err != nil {
		return fmt.Errorf("crawl phase: %w", err)
	}
	// TODO: 抓取完成后自动触发下载
	return nil
}

func (w *Worker) handleSuccess(ctx context.Context, task *domain.Task) {
	now := time.Now()
	task.Status = domain.TaskStatusCompleted
	task.Progress = 1.0
	task.Error = ""
	task.EndedAt = &now
	task.UpdatedAt = now

	if err := w.taskRepo.Update(ctx, task); err != nil {
		w.logger.Error("mark completed failed", "task_id", task.ID, "error", err)
	}

	if err := w.queue.Ack(ctx, task.ID); err != nil {
		w.logger.Warn("ack failed", "task_id", task.ID, "error", err)
	}

	w.logger.Info("task completed", "task_id", task.ID, "worker_id", task.WorkerID)
}

func (w *Worker) handleFailure(ctx context.Context, task *domain.Task, execErr error) {
	now := time.Now()
	task.Status = domain.TaskStatusFailed
	task.Error = execErr.Error()
	task.Progress = 0
	task.Retries++
	task.UpdatedAt = now

	if err := w.taskRepo.Update(ctx, task); err != nil {
		w.logger.Error("mark failed failed", "task_id", task.ID, "error", err)
	}

	shouldRequeue := task.Retries < task.Spec.MaxRetries
	if err := w.queue.Nack(ctx, task.ID, shouldRequeue); err != nil {
		w.logger.Warn("nack failed", "task_id", task.ID, "error", err)
	}

	w.logger.Error("task failed",
		"task_id", task.ID,
		"worker_id", task.WorkerID,
		"error", execErr,
		"retries", task.Retries,
		"will_requeue", shouldRequeue,
	)
}
