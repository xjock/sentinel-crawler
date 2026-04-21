package domain

import "context"

// TaskQueue 任务队列抽象
type TaskQueue interface {
	// Enqueue 将任务放入队列
	Enqueue(ctx context.Context, task *Task) error

	// Dequeue 从队列中取出一个待执行任务
	Dequeue(ctx context.Context) (*Task, error)

	// Ack 确认任务成功消费
	Ack(ctx context.Context, taskID string) error

	// Nack 任务消费失败，可选择是否重新入队
	Nack(ctx context.Context, taskID string, requeue bool) error

	// Close 关闭队列连接
	Close() error
}
