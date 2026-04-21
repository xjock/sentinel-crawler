package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/xjock/sentinel-crawler/internal/api"
	"github.com/xjock/sentinel-crawler/internal/config"
	"github.com/xjock/sentinel-crawler/internal/queue/memory"
	"github.com/xjock/sentinel-crawler/internal/repository/sqlite"
	"github.com/xjock/sentinel-crawler/internal/usecase"
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

	// Repository
	taskRepo := sqlite.NewTaskRepository(db)
	queue := memory.NewTaskQueue(100)

	// UseCase
	tm := usecase.NewTaskManager(taskRepo, queue, logger)

	// HTTP Server
	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	server := api.NewServer(addr, tm, logger)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
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
