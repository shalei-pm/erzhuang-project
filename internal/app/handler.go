package app

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"
)

const (
	AppName = "erzhuang-project"
	Version = "v2"
)

type HealthResponse struct {
	App        string `json:"app"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	Database   string `json:"database"`
	AssetStore string `json:"asset_store"`
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
	AISettingsStore
}

type Handler struct {
	store Store
}

func NewHandler() http.Handler {
	return NewHandlerWithStores(NewMemoryStore(), designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStore(store Store) http.Handler {
	return NewHandlerWithStores(store, designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStores(store Store, designPlanRepo designplan.Repository, storeSpaceRepo storespace.Repository) http.Handler {
	return NewHandlerWithServices(store, designplan.NewService(designPlanRepo), storespace.NewService(storeSpaceRepo))
}

func NewHandlerWithServices(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service) http.Handler {
	handler := &Handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.healthHandler)
	mux.HandleFunc("GET /api/tasks", handler.tasksHandler)
	mux.HandleFunc("GET /api/ai-settings", handler.aiSettingsHandler)
	mux.HandleFunc("POST /api/ai-settings/toggle", handler.toggleAISettingsHandler)
	designplan.RegisterRoutes(mux, designPlanService)
	storespace.RegisterRoutes(mux, storeSpaceService)
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
		App:        AppName,
		Status:     status,
		Version:    Version,
		Database:   database,
		AssetStore: assets.ModeFromEnv(),
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

func (h *Handler) aiSettingsHandler(w http.ResponseWriter, r *http.Request) {
	provider, err := h.store.GetAIProvider(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load ai settings failed"})
		return
	}
	writeJSON(w, http.StatusOK, AISettingsFromProvider(provider))
}

func (h *Handler) toggleAISettingsHandler(w http.ResponseWriter, r *http.Request) {
	provider, err := h.store.GetAIProvider(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load ai settings failed"})
		return
	}
	nextProvider := NextAIProvider(provider)
	if err := h.store.SetAIProvider(r.Context(), nextProvider); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save ai settings failed"})
		return
	}
	writeJSON(w, http.StatusOK, AISettingsFromProvider(nextProvider))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
