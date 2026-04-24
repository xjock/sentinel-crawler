package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/semaphore"

	"github.com/xjock/sentinel-crawler/internal/domain"
)

// Crawler 元数据抓取编排接口
type Crawler interface {
	Crawl(ctx context.Context, spec domain.TaskSpec) error
}

// crawler 实现
type crawler struct {
	provider       domain.Provider
	productRepo    domain.ProductRepository
	taskRepo       domain.TaskRepository
	taskQueue      domain.TaskQueue
	maxConcurrency int64
	pageSize       int
	skipExisting   bool
	logger         *slog.Logger
}

// NewCrawler 创建 Crawler
func NewCrawler(
	provider domain.Provider,
	productRepo domain.ProductRepository,
	taskRepo domain.TaskRepository,
	taskQueue domain.TaskQueue,
	maxConcurrency int,
	pageSize int,
	skipExisting bool,
	logger *slog.Logger,
) Crawler {
	if logger == nil {
		logger = slog.Default()
	}
	return &crawler{
		provider:       provider,
		productRepo:    productRepo,
		taskRepo:       taskRepo,
		taskQueue:      taskQueue,
		maxConcurrency: int64(maxConcurrency),
		pageSize:       pageSize,
		skipExisting:   skipExisting,
		logger:         logger,
	}
}

// Crawl 执行一次完整的元数据抓取任务
func (c *crawler) Crawl(ctx context.Context, spec domain.TaskSpec) error {
	query := spec.Query
	query.PageSize = c.pageSize
	if query.PageSize <= 0 {
		query.PageSize = 100
	}

	sem := semaphore.NewWeighted(c.maxConcurrency)
	page := 1
	totalSaved := 0

	for {
		query.Page = page

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire semaphore: %w", err)
		}

		result, err := c.provider.Search(ctx, query)
		sem.Release(1)

		if err != nil {
			return fmt.Errorf("search page %d: %w", page, err)
		}

		if len(result.Products) == 0 {
			break
		}

		var toSave []*domain.Product
		for _, p := range result.Products {
			if c.skipExisting {
				exists, err := c.productRepo.Exists(ctx, p.ID)
				if err != nil {
					c.logger.Warn("check exists failed", "product_id", p.ID, "error", err)
					continue
				}
				if exists {
					continue
				}
			}
			toSave = append(toSave, p)
		}

		if len(toSave) > 0 {
			if err := c.productRepo.SaveBatch(ctx, toSave); err != nil {
				c.logger.Warn("save batch failed", "page", page, "error", err)
				for _, p := range toSave {
					if err := c.productRepo.Save(ctx, p); err != nil {
						c.logger.Warn("save single failed", "product_id", p.ID, "error", err)
					}
				}
			}
			totalSaved += len(toSave)
		}

		c.logger.Info("crawled page",
			"page", page,
			"fetched", len(result.Products),
			"saved", len(toSave),
			"total_saved", totalSaved,
		)

		if !result.HasMore {
			break
		}
		page++
	}

	c.logger.Info("crawl completed", "total_saved", totalSaved)
	return nil
}
