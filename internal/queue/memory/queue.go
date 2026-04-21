package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/lucavallin/sentinel-crawler/internal/domain"
)

// TaskQueue 内存队列实现
type TaskQueue struct {
	mu       sync.Mutex
	tasks    chan *domain.Task
	inflight map[string]*domain.Task
	closed   bool
}

// NewTaskQueue 创建内存队列
func NewTaskQueue(bufferSize int) *TaskQueue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &TaskQueue{
		tasks:    make(chan *domain.Task, bufferSize),
		inflight: make(map[string]*domain.Task),
	}
}

// Enqueue 将任务放入队列
func (q *TaskQueue) Enqueue(_ context.Context, task *domain.Task) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return errors.New("queue is closed")
	}
	q.mu.Unlock()

	select {
	case q.tasks <- task:
		return nil
	default:
		return errors.New("queue is full")
	}
}

// Dequeue 从队列中取出一个待执行任务
func (q *TaskQueue) Dequeue(ctx context.Context) (*domain.Task, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case task, ok := <-q.tasks:
		if !ok {
			return nil, errors.New("queue is closed")
		}
		q.mu.Lock()
		q.inflight[task.ID] = task
		q.mu.Unlock()
		return task, nil
	}
}

// Ack 确认任务成功消费
func (q *TaskQueue) Ack(_ context.Context, taskID string) error {
	q.mu.Lock()
	delete(q.inflight, taskID)
	q.mu.Unlock()
	return nil
}

// Nack 任务消费失败，可选择是否重新入队
func (q *TaskQueue) Nack(_ context.Context, taskID string, requeue bool) error {
	q.mu.Lock()
	task, ok := q.inflight[taskID]
	delete(q.inflight, taskID)
	q.mu.Unlock()

	if !ok {
		return errors.New("task not in flight")
	}

	if requeue {
		select {
		case q.tasks <- task:
			return nil
		default:
			return errors.New("queue is full, cannot requeue")
		}
	}
	return nil
}

// Close 关闭队列
func (q *TaskQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.tasks)
	return nil
}
