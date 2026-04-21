package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lucavallin/sentinel-crawler/internal/domain"
)

// Downloader 影像下载编排接口
type Downloader interface {
	Download(ctx context.Context, taskID string, product *domain.Product, destDir string) error
	DownloadBatch(ctx context.Context, taskID string, products []*domain.Product, destDir string) error
}

// downloader 实现
type downloader struct {
	provider       domain.Provider
	stateRepo      domain.DownloadStateRepository
	taskRepo       domain.TaskRepository
	workers        int
	chunkSize      int64
	verifyChecksum bool
	resume         bool
	logger         *slog.Logger
}

// NewDownloader 创建 Downloader
func NewDownloader(
	provider domain.Provider,
	stateRepo domain.DownloadStateRepository,
	taskRepo domain.TaskRepository,
	workers int,
	chunkSize int64,
	verifyChecksum bool,
	resume bool,
	logger *slog.Logger,
) Downloader {
	if logger == nil {
		logger = slog.Default()
	}
	return &downloader{
		provider:       provider,
		stateRepo:      stateRepo,
		taskRepo:       taskRepo,
		workers:        workers,
		chunkSize:      chunkSize,
		verifyChecksum: verifyChecksum,
		resume:         resume,
		logger:         logger,
	}
}

// Download 下载指定产品
func (d *downloader) Download(ctx context.Context, taskID string, product *domain.Product, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	destPath := filepath.Join(destDir, product.Name+".zip")

	// 检查历史状态
	var state *domain.DownloadState
	if d.resume {
		s, err := d.stateRepo.FindByTaskID(ctx, taskID)
		if err != nil {
			d.logger.Warn("find state failed", "task_id", taskID, "error", err)
		}
		if s != nil && s.Status != "completed" {
			state = s
			destPath = state.DestPath
		}
	}

	// 获取数据流
	reader, err := d.provider.Fetch(ctx, product)
	if err != nil {
		return fmt.Errorf("fetch product: %w", err)
	}
	defer reader.Close()

	// 打开文件
	flag := os.O_CREATE | os.O_WRONLY
	if state != nil && state.ReceivedBytes > 0 {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(destPath, flag, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if state == nil {
		state = &domain.DownloadState{
			ID:               taskID + "_" + product.ID,
			TaskID:           taskID,
			ProductID:        product.ID,
			DestPath:         destPath,
			ChecksumExpected: product.Checksum,
			Status:           "downloading",
		}
	} else {
		state.Status = "downloading"
	}

	// 流式写入
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = d.saveState(ctx, state)
				return fmt.Errorf("write file: %w", werr)
			}
			written += int64(n)
			state.ReceivedBytes = written
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = d.saveState(ctx, state)
			return fmt.Errorf("read stream: %w", err)
		}
	}

	// 同步到磁盘
	if err := f.Sync(); err != nil {
		d.logger.Warn("fsync failed", "path", destPath, "error", err)
	}

	// 校验（简化版）
	if d.verifyChecksum && product.Checksum != "" {
		// TODO: 实际校验 MD5/SHA256
		d.logger.Info("checksum verification skipped (TODO)", "product_id", product.ID)
	}

	state.Status = "completed"
	if err := d.stateRepo.Delete(ctx, state.ID); err != nil {
		d.logger.Warn("delete state failed", "error", err)
	}

	d.logger.Info("download completed", "product_id", product.ID, "path", destPath, "bytes", written)
	return nil
}

// DownloadBatch 批量下载
func (d *downloader) DownloadBatch(ctx context.Context, taskID string, products []*domain.Product, destDir string) error {
	semaphore := make(chan struct{}, d.workers)
	var errs []error

	for _, p := range products {
		semaphore <- struct{}{}
		go func(product *domain.Product) {
			defer func() { <-semaphore }()
			if err := d.Download(ctx, taskID, product, destDir); err != nil {
				errs = append(errs, fmt.Errorf("download %s: %w", product.ID, err))
			}
		}(p)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- struct{}{}
	}

	if len(errs) > 0 {
		return fmt.Errorf("batch download: %d errors", len(errs))
	}
	return nil
}

func (d *downloader) saveState(ctx context.Context, state *domain.DownloadState) error {
	if err := d.stateRepo.Save(ctx, state); err != nil {
		d.logger.Warn("save state failed", "error", err)
		return err
	}
	return nil
}
