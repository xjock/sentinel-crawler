package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lucavallin/sentinel-crawler/internal/api/handler"
	"github.com/lucavallin/sentinel-crawler/internal/api/middleware"
	"github.com/lucavallin/sentinel-crawler/internal/usecase"
)

// Server HTTP API 服务
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer 创建 HTTP 服务
func NewServer(
	addr string,
	tm usecase.TaskManager,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()

	// Handler 注册
	hh := handler.NewHealthHandler()
	hh.RegisterRoutes(mux)

	th := handler.NewTaskHandler(tm)
	th.RegisterRoutes(mux)

	// 中间件链
	var h http.Handler = mux
	h = middleware.Recovery(h, logger)
	h = middleware.Logging(h, logger)

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      h,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		logger: logger,
	}
}

// Start 启动服务
func (s *Server) Start() error {
	s.logger.Info("starting http server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// Shutdown 优雅关闭
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server")
	return s.httpServer.Shutdown(ctx)
}
