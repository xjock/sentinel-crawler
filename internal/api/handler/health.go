package handler

import (
	"encoding/json"
	"net/http"
)

// HealthHandler 健康检查 Handler
type HealthHandler struct{}

// NewHealthHandler 创建 HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// RegisterRoutes 注册路由
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)
}

// Health 健康检查
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// Ready 就绪检查
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
