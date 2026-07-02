package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"
)

const (
	AppName            = "erzhuang-project"
	Version            = "v2"
	defaultAppBasePath = "/erzhuang-project"
	legacyAppBasePath  = "/erzhuang"
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
	AuthUserStore
}

type Handler struct {
	store                Store
	auth                 AuthConfig
	ossSmokeRunner       ossSmokeRunner
	assetMigrationRunner assetMigrationRunner
	stageASampleRunner   stageASourceSampleRunner
}

func NewHandler() http.Handler {
	return NewHandlerWithStores(NewMemoryStore(), designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStore(store Store) http.Handler {
	return NewHandlerWithStores(store, designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStores(store Store, designPlanRepo designplan.Repository, storeSpaceRepo storespace.Repository) http.Handler {
	return newHandlerWithServices(store, designplan.NewService(designPlanRepo), storespace.NewService(storeSpaceRepo), nil)
}

func NewHandlerWithServices(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, nil)
}

func NewHandlerWithServicesAndH5Monitor(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, h5MonitorService)
}

func newHandlerWithServices(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service) http.Handler {
	handler := &Handler{store: store, auth: AuthConfigFromEnv(), ossSmokeRunner: currentOSSSmokeRunner, assetMigrationRunner: currentAssetMigrationRunner, stageASampleRunner: currentStageASourceSampleRunner}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.healthHandler)
	mux.HandleFunc("GET /api/tasks", handler.tasksHandler)
	mux.HandleFunc("GET /api/auth/me", handler.authMeHandler)
	mux.HandleFunc("POST /api/auth/logout", handler.authLogoutHandler)
	mux.HandleFunc("GET /_/auth/callback", handler.authCallbackHandler)
	mux.HandleFunc("GET /logout", handler.authLogoutHandler)
	mux.HandleFunc("GET /api/users", handler.listUsersHandler)
	mux.HandleFunc("POST /api/users", handler.createUserHandler)
	mux.HandleFunc("PUT /api/users/{id}", handler.updateUserHandler)
	mux.HandleFunc("GET /api/ai-settings", handler.aiSettingsHandler)
	mux.HandleFunc("POST /api/ai-settings/toggle", handler.requirePermissionHandler(PermissionUserManage, handler.toggleAISettingsHandler))
	mux.HandleFunc("GET /api/admin/ops/env-check", handler.ossEnvCheckHandler)
	mux.HandleFunc("POST /api/admin/ops/oss-smoke", handler.ossSmokeHandler)
	mux.HandleFunc("POST /api/admin/ops/asset-migrate", handler.assetMigrationHandler)
	mux.HandleFunc("POST /api/admin/ops/stage-a-source-sample", handler.stageASourceSampleHandler)
	designplan.RegisterRoutesWithWriteGuard(mux, designPlanService, handler.storeWriteGuard)
	storespace.RegisterRoutesWithWriteGuard(mux, storeSpaceService, handler.storeWriteGuard)
	if h5MonitorService != nil {
		h5monitor.RegisterRoutes(mux, h5MonitorService)
	}
	registerFrontendRoutes(mux)
	return withBasePathAPIPrefixes(mux)
}

type ossSmokeResult = osssmoke.Result
type ossSmokeRunner func(ctx context.Context) (*ossSmokeResult, error)
type assetMigrationRunner func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error)
type stageASourceSampleRunner func(ctx context.Context, action string) (*stageASourceSampleResult, error)

func (h *Handler) storeWriteGuard(next http.HandlerFunc) http.HandlerFunc {
	return h.requirePermissionHandler(PermissionStoreWrite, next)
}

func withBasePathAPIPrefixes(next http.Handler) http.Handler {
	basePaths := configuredBasePaths()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, basePath := range basePaths {
			if r.URL.Path == basePath+"/health" {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = "/health"
				next.ServeHTTP(w, cloned)
				return
			}

			apiPrefix := basePath + "/api/"
			if strings.HasPrefix(r.URL.Path, apiPrefix) {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
				next.ServeHTTP(w, cloned)
				return
			}

			authPrefix := basePath + "/_/auth/"
			if strings.HasPrefix(r.URL.Path, authPrefix) {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
				next.ServeHTTP(w, cloned)
				return
			}

			if r.URL.Path == basePath+"/logout" {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = "/logout"
				next.ServeHTTP(w, cloned)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func configuredBasePaths() []string {
	basePath := normalizeBasePath(os.Getenv("APP_BASE_PATH"))
	paths := []string{basePath}
	if basePath != legacyAppBasePath {
		paths = append(paths, legacyAppBasePath)
	}
	return paths
}

func normalizeBasePath(value string) string {
	if value == "" || value == "/" {
		return defaultAppBasePath
	}
	value = "/" + strings.Trim(value, "/")
	if value == "/" {
		return defaultAppBasePath
	}
	return value
}

func registerFrontendRoutes(mux *http.ServeMux) {
	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		return
	}

	for _, basePath := range configuredBasePaths() {
		mux.Handle("GET "+basePath, frontendHandler(frontendDir, basePath))
		mux.Handle("GET "+basePath+"/", frontendHandler(frontendDir, basePath))
	}
}

func frontendHandler(frontendDir string, basePath string) http.Handler {
	fileServer := http.FileServer(http.Dir(frontendDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, basePath)
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
			return
		}

		if _, err := os.Stat(filepath.Join(frontendDir, path)); err != nil && filepath.Ext(path) == "" {
			http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
			return
		}

		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	})
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
