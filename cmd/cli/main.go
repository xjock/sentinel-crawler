package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xjock/sentinel-crawler/internal/config"
	"github.com/xjock/sentinel-crawler/internal/domain"
	"github.com/xjock/sentinel-crawler/internal/provider/copernicus"
	"github.com/xjock/sentinel-crawler/internal/queue/memory"
	"github.com/xjock/sentinel-crawler/internal/repository/sqlite"
	"github.com/xjock/sentinel-crawler/internal/usecase"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch command {
	case "migrate":
		if err := runMigrate(ctx, cfg, logger); err != nil {
			logger.Error("migrate failed", "error", err)
			os.Exit(1)
		}
	case "crawl":
		if err := runCrawl(ctx, cfg, logger); err != nil {
			logger.Error("crawl failed", "error", err)
			os.Exit(1)
		}
	case "task":
		if err := runTask(ctx, cfg, logger); err != nil {
			logger.Error("task command failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: cli [options] <command>

Commands:
  migrate    Run database migrations
  crawl      Trigger a metadata crawl task
  task       Task management (list, get)

Options:
  -config string
        path to config file (default "configs/config.yaml")`)
}

func runMigrate(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	db, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		return err
	}

	logger.Info("migration completed", "dsn", cfg.Database.DSN)
	return nil
}

func runCrawl(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	db, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	productRepo := sqlite.NewProductRepository(db)
	taskRepo := sqlite.NewTaskRepository(db)
	queue := memory.NewTaskQueue(100)

	var provider domain.Provider
	switch cfg.Providers.Active {
	case "copernicus":
		provider = copernicus.NewProvider(cfg.Providers.Copernicus)
	default:
		log.Fatalf("unknown provider: %s", cfg.Providers.Active)
	}

	tm := usecase.NewTaskManager(taskRepo, queue, logger)
	crawler := usecase.NewCrawler(provider, productRepo, taskRepo, queue,
		cfg.Crawler.MaxConcurrency, cfg.Crawler.PageSize, cfg.Crawler.SkipExisting, logger)

	spec := domain.TaskSpec{
		Provider:   cfg.Providers.Active,
		Query:      domain.SearchQuery{PageSize: cfg.Crawler.PageSize},
		MaxRetries: cfg.Crawler.RetryAttempts,
	}

	task, err := tm.CreateTask(ctx, spec)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	logger.Info("task created", "task_id", task.ID)

	if err := crawler.Crawl(ctx, spec); err != nil {
		return fmt.Errorf("crawl: %w", err)
	}

	logger.Info("crawl completed", "task_id", task.ID)
	return nil
}

func runTask(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	db, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	taskRepo := sqlite.NewTaskRepository(db)
	queue := memory.NewTaskQueue(100)
	tm := usecase.NewTaskManager(taskRepo, queue, logger)

	if flag.NArg() < 2 {
		return fmt.Errorf("task subcommand required: list, get")
	}

	sub := flag.Arg(1)
	switch sub {
	case "list":
		tasks, err := tm.ListTasks(ctx, domain.TaskFilter{PageSize: 20})
		if err != nil {
			return err
		}
		for _, t := range tasks {
			fmt.Printf("%s | %s | %s | %.0f%% | %s\n", t.ID, t.Type, t.Status, t.Progress*100, t.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	case "get":
		if flag.NArg() < 3 {
			return fmt.Errorf("task ID required")
		}
		task, err := tm.GetTask(ctx, flag.Arg(2))
		if err != nil {
			return err
		}
		fmt.Printf("ID: %s\nType: %s\nStatus: %s\nProgress: %.0f%%\nError: %s\nCreated: %s\n",
			task.ID, task.Type, task.Status, task.Progress*100, task.Error, task.CreatedAt.Format("2006-01-02 15:04:05"))
	default:
		return fmt.Errorf("unknown task subcommand: %s", sub)
	}
	return nil
}
