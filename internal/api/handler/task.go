package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xjock/sentinel-crawler/internal/domain"
	"github.com/xjock/sentinel-crawler/internal/usecase"
)

// TaskHandler 任务管理 Handler
type TaskHandler struct {
	tm usecase.TaskManager
}

// NewTaskHandler 创建 TaskHandler
func NewTaskHandler(tm usecase.TaskManager) *TaskHandler {
	return &TaskHandler{tm: tm}
}

// RegisterRoutes 注册路由
func (h *TaskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", h.CreateTask)
	mux.HandleFunc("GET /api/v1/tasks", h.ListTasks)
	mux.HandleFunc("GET /api/v1/tasks/{id}", h.GetTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/cancel", h.CancelTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/retry", h.RetryTask)
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Provider    string `json:"provider"`
	Platform    string `json:"platform"`
	ProductType string `json:"product_type"`
	SensingFrom string `json:"sensing_from"`
	SensingTo   string `json:"sensing_to"`
	DestDir     string `json:"dest_dir"`
	MaxRetries  int    `json:"max_retries"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Progress  float64 `json:"progress"`
	Error     string  `json:"error,omitempty"`
	Retries   int     `json:"retries"`
	CreatedAt string  `json:"created_at"`
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	spec := domain.TaskSpec{
		Provider:   req.Provider,
		DestDir:    req.DestDir,
		MaxRetries: req.MaxRetries,
	}
	if spec.Provider == "" {
		spec.Provider = "copernicus"
	}

	task, err := h.tm.CreateTask(r.Context(), spec)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, toTaskResponse(task))
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	filter := domain.TaskFilter{}

	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = domain.TaskType(t)
	}
	if s := r.URL.Query().Get("status"); s != "" {
		filter.Status = domain.TaskStatus(s)
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			filter.Page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil {
			filter.PageSize = n
		}
	}

	tasks, err := h.tm.ListTasks(r.Context(), filter)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var resp []TaskResponse
	for _, t := range tasks {
		resp = append(resp, toTaskResponse(t))
	}
	respondJSON(w, http.StatusOK, map[string]any{"tasks": resp})
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.tm.GetTask(r.Context(), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) CancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.tm.CancelTask(r.Context(), id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *TaskHandler) RetryTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.tm.RetryTask(r.Context(), id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "retried"})
}

func toTaskResponse(t *domain.Task) TaskResponse {
	return TaskResponse{
		ID:        t.ID,
		Type:      string(t.Type),
		Status:    string(t.Status),
		Progress:  t.Progress,
		Error:     t.Error,
		Retries:   t.Retries,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
