package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lucavallin/sentinel-crawler/internal/config"
	"github.com/lucavallin/sentinel-crawler/internal/domain"
	"github.com/lucavallin/sentinel-crawler/internal/provider/copernicus"
	"github.com/lucavallin/sentinel-crawler/internal/queue/memory"
	"github.com/lucavallin/sentinel-crawler/internal/repository/sqlite"
	"github.com/lucavallin/sentinel-crawler/internal/usecase"
	"github.com/lucavallin/sentinel-crawler/internal/worker"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Log.Level),
	}))

	// 数据库
	db, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}

	// Provider
	var provider domain.Provider
	switch cfg.Providers.Active {
	case "copernicus":
		provider = copernicus.NewProvider(cfg.Providers.Copernicus)
	default:
		logger.Error("unknown provider", "active", cfg.Providers.Active)
		os.Exit(1)
	}

	// Repository
	productRepo := sqlite.NewProductRepository(db)
	taskRepo := sqlite.NewTaskRepository(db)
	stateRepo := sqlite.NewDownloadStateRepository(db)
	queue := memory.NewTaskQueue(1000)

	// UseCase
	crawler := usecase.NewCrawler(
		provider, productRepo, taskRepo, queue,
		cfg.Crawler.MaxConcurrency, cfg.Crawler.PageSize,
		cfg.Crawler.SkipExisting, logger,
	)
	downloader := usecase.NewDownloader(
		provider, stateRepo, taskRepo,
		cfg.Download.Workers, cfg.Download.ChunkSize,
		cfg.Download.VerifyChecksum, cfg.Download.Resume,
		logger,
	)

	// Worker
	w := worker.NewWorker(queue, taskRepo, crawler, downloader, cfg.Crawler.MaxConcurrency, logger)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("crawler worker started", "workers", cfg.Crawler.MaxConcurrency)

	// 启动 Worker（阻塞）
	w.Start(ctx)

	logger.Info("crawler worker stopped")
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
