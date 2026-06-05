package app

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shalei-pm/erzhuang-project/internal/designplan"
)

const (
	AppName = "erzhuang-project"
	Version = "v2"
)

type HealthResponse struct {
	App      string `json:"app"`
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type Store interface {
	Name() string
	Ping(ctx context.Context) error
	ListTasks(ctx context.Context) ([]Task, error)
}

type Handler struct {
	store Store
}

func NewHandler() http.Handler {
	return NewHandlerWithStores(NewMemoryStore(), designplan.NewMemoryStore())
}

func NewHandlerWithStore(store Store) http.Handler {
	return NewHandlerWithStores(store, designplan.NewMemoryStore())
}

func NewHandlerWithStores(store Store, designPlanRepo designplan.Repository) http.Handler {
	handler := &Handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.healthHandler)
	mux.HandleFunc("GET /api/tasks", handler.tasksHandler)
	designplan.RegisterRoutes(mux, designplan.NewService(designPlanRepo))
	return mux
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	database := h.store.Name()
	if err := h.store.Ping(r.Context()); err != nil {
		status = "degraded"
		database = "error"
	}

	writeJSON(w, http.StatusOK, HealthResponse{
		App:      AppName,
		Status:   status,
		Version:  Version,
		Database: database,
	})
}

func (h *Handler) tasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.ListTasks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "list tasks failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
