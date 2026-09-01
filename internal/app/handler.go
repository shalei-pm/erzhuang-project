package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/nvrlab"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
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
	store                    Store
	auth                     AuthConfig
	authSessionStore         authSessionStore
	now                      func() time.Time
	idleTimeout              time.Duration
	auditRecorder            auditlog.AuditRecorder
	ossSmokeRunner           ossSmokeRunner
	assetMigrationRunner     assetMigrationRunner
	assetStateBackfillRunner assetStateBackfillRunner
	stageASampleRunner       stageASourceSampleRunner
	stageATargetRunner       stageATargetSampleRunner
	mysqlCanaryRunner        mysqlCanaryImportRunner
	mysqlValidateRunner      mysqlCanaryValidateRunner
	mysqlInventoryRunner     mysqlAssetInventoryRunner
	storeSpaceService        *storespace.Service
	resourceViewService      *resourceview.Service
	nvrMonitorService        *nvrmonitor.Service
	monitorPlaybackMode      MonitorPlaybackMode
}

func NewHandler() http.Handler {
	return NewHandlerWithStores(NewMemoryStore(), designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStore(store Store) http.Handler {
	return NewHandlerWithStores(store, designplan.NewMemoryStore(), storespace.NewMemoryStore())
}

func NewHandlerWithStores(store Store, designPlanRepo designplan.Repository, storeSpaceRepo storespace.Repository) http.Handler {
	return newHandlerWithServices(store, designplan.NewService(designPlanRepo), storespace.NewService(storeSpaceRepo), nil, nil, nil, nil, MonitorPlaybackModeLegacy)
}

func NewHandlerWithServices(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, nil, nil, nil, nil, MonitorPlaybackModeLegacy)
}

func NewHandlerWithServicesAndH5Monitor(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, h5MonitorService, nil, nil, nil, MonitorPlaybackModeLegacy)
}

func NewHandlerWithServicesAndH5MonitorAndResourceView(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service, resourceViewService *resourceview.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, h5MonitorService, resourceViewService, nil, nil, MonitorPlaybackModeLegacy)
}

func NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVRLab(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service, resourceViewService *resourceview.Service, nvrLabService *nvrlab.Service) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, h5MonitorService, resourceViewService, nvrLabService, nil, MonitorPlaybackModeLegacy)
}

func NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVR(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service, resourceViewService *resourceview.Service, nvrLabService *nvrlab.Service, nvrMonitorService *nvrmonitor.Service, monitorPlaybackMode MonitorPlaybackMode) http.Handler {
	return newHandlerWithServices(store, designPlanService, storeSpaceService, h5MonitorService, resourceViewService, nvrLabService, nvrMonitorService, monitorPlaybackMode)
}

func newHandlerWithServices(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service, resourceViewService *resourceview.Service, nvrLabService *nvrlab.Service, nvrMonitorService *nvrmonitor.Service, monitorPlaybackMode MonitorPlaybackMode) http.Handler {
	var sessionStore authSessionStore
	if candidate, ok := store.(authSessionStore); ok {
		sessionStore = candidate
	} else if _, ok := store.(*MemoryStore); ok {
		// The memory adapter is only a local/test persistence substitute.
		sessionStore = newMemoryAuthSessionStore()
	}
	return newHandlerWithAuthSessionStore(store, designPlanService, storeSpaceService, h5MonitorService, resourceViewService, nvrLabService, nvrMonitorService, monitorPlaybackMode, sessionStore)
}

func newHandlerWithAuthSessionStore(store Store, designPlanService *designplan.Service, storeSpaceService *storespace.Service, h5MonitorService *h5monitor.Service, resourceViewService *resourceview.Service, nvrLabService *nvrlab.Service, nvrMonitorService *nvrmonitor.Service, monitorPlaybackMode MonitorPlaybackMode, sessionStore authSessionStore) http.Handler {
	var auditRecorder auditlog.AuditRecorder
	if recorder, ok := store.(auditlog.AuditRecorder); ok {
		auditRecorder = recorder
	}
	handler := &Handler{store: store, auth: AuthConfigFromEnv(), authSessionStore: sessionStore, now: time.Now, idleTimeout: defaultAuthIdleTimeout, auditRecorder: auditRecorder, ossSmokeRunner: currentOSSSmokeRunner, assetMigrationRunner: currentAssetMigrationRunner, assetStateBackfillRunner: currentAssetStateBackfillRunner, stageASampleRunner: currentStageASourceSampleRunner, stageATargetRunner: currentStageATargetSampleRunner, mysqlCanaryRunner: currentMySQLCanaryImportRunner, mysqlValidateRunner: currentMySQLCanaryValidateRunner, mysqlInventoryRunner: currentMySQLAssetInventoryRunner, storeSpaceService: storeSpaceService, resourceViewService: resourceViewService, nvrMonitorService: nvrMonitorService, monitorPlaybackMode: monitorPlaybackMode}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.healthHandler)
	mux.HandleFunc("GET /api/tasks", handler.tasksHandler)
	mux.HandleFunc("GET /api/auth/me", handler.authMeHandler)
	mux.HandleFunc("POST /api/auth/logout", handler.authLogoutHandler)
	mux.HandleFunc("GET /api/h5/monitor-mode", handler.requirePermissionHandler(PermissionStoreRead, handler.monitorModeHandler))
	mux.HandleFunc("GET /_/auth/callback", handler.authCallbackHandler)
	mux.HandleFunc("GET /logout", handler.authLogoutHandler)
	mux.HandleFunc("GET /api/users/monitor-store-scope-candidates", handler.listMonitorStoreScopeCandidatesHandler)
	mux.HandleFunc("GET /api/users", handler.listUsersHandler)
	mux.HandleFunc("POST /api/users", handler.createUserHandler)
	mux.HandleFunc("PUT /api/users/{id}", handler.updateUserHandler)
	mux.HandleFunc("GET /api/ai-settings", handler.aiSettingsHandler)
	mux.HandleFunc("POST /api/ai-settings/toggle", handler.requirePermissionHandler(PermissionUserManage, handler.toggleAISettingsHandler))
	mux.HandleFunc("GET /api/admin/audit-logs", handler.requirePermissionHandler(PermissionAuditView, handler.auditLogsHandler))
	mux.HandleFunc("GET /api/admin/ops/env-check", handler.ossEnvCheckHandler)
	mux.HandleFunc("POST /api/admin/ops/oss-smoke", handler.ossSmokeHandler)
	mux.HandleFunc("POST /api/admin/ops/asset-migrate", handler.assetMigrationHandler)
	mux.HandleFunc("POST /api/admin/ops/asset-state-backfill", handler.assetStateBackfillHandler)
	mux.HandleFunc("POST /api/admin/ops/stage-a-source-sample", handler.stageASourceSampleHandler)
	mux.HandleFunc("POST /api/admin/ops/stage-a-target-sample", handler.stageATargetSampleHandler)
	mux.HandleFunc("POST /api/admin/ops/mysql-canary-import", handler.mysqlCanaryImportHandler)
	mux.HandleFunc("GET /api/admin/ops/mysql-canary-validate", handler.mysqlCanaryValidateHandler)
	mux.HandleFunc("GET /api/admin/ops/mysql-asset-inventory", handler.mysqlAssetInventoryHandler)
	designplan.RegisterRoutesWithWriteGuard(mux, designPlanService, handler.storeWriteGuard)
	storespace.RegisterRoutesWithGuards(mux, storeSpaceService, handler.monitorVisibilityMiddleware, handler.storeWriteGuard)
	resourceview.RegisterRoutesWithReadGuard(mux, resourceViewService, handler.resourceViewMonitorAccess, handler.storeReadGuard)
	mux.HandleFunc("GET /api/store-space-resource-view/stores/{tenantId}/cameras/{cameraId}/snapshot", handler.storeReadGuard(handler.resourceViewLegacySnapshotHandler))
	mux.HandleFunc("POST /api/store-space/stores/{storeId}/channels/{channelId}/snapshot/view", handler.storeReadGuard(handler.recordChannelSnapshotView))
	nvrlab.RegisterRoutesWithAudit(mux, nvrLabService, handler.nvrLabAdminGuard, nvrMonitorAuthorizer{handler: handler})
	if monitorPlaybackMode == MonitorPlaybackModeNVR && nvrMonitorService != nil {
		nvrmonitor.RegisterRoutesWithAuthorizer(mux, nvrMonitorService, nvrMonitorAuthorizer{handler: handler})
	} else if h5MonitorService != nil {
		h5monitor.RegisterRoutesWithAuthorizer(mux, h5MonitorService, h5MonitorAuthorizer{handler: handler})
	}
	registerFrontendRoutes(mux)
	return withBasePathAPIPrefixes(handler.authGate(mux))
}

func (h *Handler) monitorModeHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"mode": string(h.monitorPlaybackMode)})
}

func (h *Handler) resourceViewLegacySnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if h.resourceViewService == nil || h.storeSpaceService == nil {
		http.NotFound(w, r)
		return
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("tenantId")), 10, 64)
	if err != nil || tenantID <= 0 {
		http.NotFound(w, r)
		return
	}
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if err != nil || cameraID <= 0 {
		http.NotFound(w, r)
		return
	}
	access, err := h.resourceViewMonitorAccess(r, tenantID)
	if err != nil || !access.CanViewMonitor {
		http.NotFound(w, r)
		return
	}
	name, err := h.resourceViewService.LegacySnapshotName(r.Context(), tenantID, cameraID, access)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reader, contentType, err := h.storeSpaceService.OpenChannelSnapshot(r.Context(), name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer reader.Close()
	w.Header().Set("Cache-Control", "private, no-store")
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("resource view: serve legacy snapshot failed: %v", err)
	}
}

func (h *Handler) recordChannelSnapshotView(w http.ResponseWriter, r *http.Request) {
	storeID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("storeId")), 10, 64)
	if err != nil || storeID <= 0 || h.storeSpaceService == nil {
		http.NotFound(w, r)
		return
	}
	channelID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("channelId")), 10, 64)
	if err != nil || channelID <= 0 {
		http.NotFound(w, r)
		return
	}
	store, err := h.storeSpaceService.GetStore(r.Context(), storeID)
	if err != nil || store == nil {
		http.NotFound(w, r)
		return
	}
	belongsToStore := false
	for _, recorder := range store.Recorders {
		for _, channel := range recorder.Channels {
			if channel.ID == channelID {
				belongsToStore = true
				break
			}
		}
		if belongsToStore {
			break
		}
	}
	if !belongsToStore {
		http.NotFound(w, r)
		return
	}
	if err := h.recordMonitorAudit(r.Context(), r, auditlog.AuditEvent{
		Action:        "snapshot.view",
		EntityType:    "channel",
		EntityID:      &channelID,
		ExternalOrgID: strings.TrimSpace(store.ExternalOrgID),
		Result:        "success",
	}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "audit_unavailable", "error": "截图审计失败，请稍后重试"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ossSmokeResult = osssmoke.Result
type ossSmokeRunner func(ctx context.Context) (*ossSmokeResult, error)
type assetMigrationRunner func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error)
type assetStateBackfillRunner func(ctx context.Context, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error)
type stageASourceSampleRunner func(ctx context.Context, action string) (*stageASourceSampleResult, error)
type stageATargetSampleRunner func(ctx context.Context) (*stageATargetSampleResult, error)
type mysqlCanaryImportRunner func(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error)
type mysqlCanaryValidateRunner func(ctx context.Context, externalOrgID string) (*mysqlCanaryValidateResult, error)
type mysqlAssetInventoryRunner func(ctx context.Context, request mysqlAssetInventoryRunRequest) (*mysqlAssetInventoryRunResult, error)

func (h *Handler) storeWriteGuard(next http.HandlerFunc) http.HandlerFunc {
	return h.requirePermissionHandler(PermissionStoreWrite, next)
}

func (h *Handler) storeReadGuard(next http.HandlerFunc) http.HandlerFunc {
	return h.requirePermissionHandler(PermissionStoreRead, next)
}

func (h *Handler) monitorVisibilityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resolver := storespace.MonitorVisibilityResolver(func(ctx context.Context, externalOrgID string) (bool, error) {
			user, err := h.currentAuthUser(r)
			if errors.Is(err, errUnauthorizedAuth) || errors.Is(err, errForbiddenAuth) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return h.store.CanUserViewMonitorStore(ctx, user, externalOrgID)
		})
		next(w, r.WithContext(storespace.WithMonitorVisibilityResolver(r.Context(), resolver)))
	}
}

func (h *Handler) resourceViewMonitorAccess(r *http.Request, tenantID int64) (resourceview.MonitorAccess, error) {
	externalOrgID := strconv.FormatInt(tenantID, 10)
	user, err := h.currentAuthUser(r)
	if err != nil {
		return resourceview.MonitorAccess{}, err
	}
	ok, err := h.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
	if err != nil {
		return resourceview.MonitorAccess{}, err
	}
	if !ok {
		return resourceview.MonitorAccess{}, nil
	}
	if h.monitorPlaybackMode == MonitorPlaybackModeNVR {
		if h.nvrMonitorService == nil {
			return resourceview.MonitorAccess{}, nil
		}
		cameras, err := h.nvrMonitorService.GetCameras(r.Context(), externalOrgID)
		if errors.Is(err, nvrmonitor.ErrStoreNotFound) || errors.Is(err, nvrmonitor.ErrNotConfigured) {
			return resourceview.MonitorAccess{}, nil
		}
		if err != nil {
			return resourceview.MonitorAccess{}, err
		}
		if len(cameras.Cameras) == 0 {
			return resourceview.MonitorAccess{}, nil
		}
	}
	return resourceview.MonitorAccess{
		CanViewMonitor: true,
		MonitorURL:     normalizeBasePath(os.Getenv("APP_BASE_PATH")) + "/h5/orgs/" + url.PathEscape(externalOrgID) + "/monitor",
	}, nil
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
			http.ServeFile(w, r, frontendPath(frontendDir, "index.html"))
			return
		}

		if _, err := os.Stat(frontendPath(frontendDir, path)); err != nil && filepath.Ext(path) == "" {
			http.ServeFile(w, r, frontendPath(frontendDir, "index.html"))
			return
		}

		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	})
}

func frontendPath(frontendDir string, name string) string {
	return filepath.Clean(frontendDir + string(os.PathSeparator) + name)
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

const auditLogDateLayout = "2006-01-02"

var auditLogActionPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._:-][a-z0-9]+)*$`)

type auditLogItemResponse struct {
	ActorDisplayName string    `json:"actor_display_name"`
	UserEmail        string    `json:"user_email"`
	Action           string    `json:"action"`
	EntityType       string    `json:"entity_type"`
	EntityID         *int64    `json:"entity_id"`
	StoreID          *int64    `json:"store_id"`
	ExternalOrgID    string    `json:"external_org_id"`
	ChannelID        *int64    `json:"channel_id"`
	Result           string    `json:"result"`
	CreatedAt        time.Time `json:"created_at"`
	Summary          string    `json:"summary"`
}

type auditLogListResponse struct {
	Items    []auditLogItemResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
}

func (h *Handler) auditLogsHandler(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAuditLogFilter(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	auditLogStore, ok := h.store.(AuditLogStore)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit log store unavailable"})
		return
	}
	page, err := auditLogStore.ListAuditLogs(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list audit logs failed"})
		return
	}

	items := make([]auditLogItemResponse, 0, len(page.Items))
	for _, log := range page.Items {
		items = append(items, auditLogItemResponse{
			ActorDisplayName: strings.TrimSpace(log.ActorDisplayName),
			UserEmail:        strings.TrimSpace(log.UserEmail),
			Action:           strings.TrimSpace(log.Action),
			EntityType:       strings.TrimSpace(log.EntityType),
			EntityID:         log.EntityID,
			StoreID:          log.StoreID,
			ExternalOrgID:    strings.TrimSpace(log.ExternalOrgID),
			ChannelID:        log.ChannelID,
			Result:           strings.TrimSpace(log.Result),
			CreatedAt:        log.CreatedAt,
			Summary:          auditLogSummary(log),
		})
	}
	writeJSON(w, http.StatusOK, auditLogListResponse{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    page.Total,
	})
}

func parseAuditLogFilter(values url.Values) (AuditLogFilter, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return AuditLogFilter{}, errors.New("load audit log timezone failed")
	}
	startAt, err := parseAuditLogDate(values.Get("start_time"), location)
	if err != nil {
		return AuditLogFilter{}, errors.New("invalid start_time")
	}
	endDate, err := parseAuditLogDate(values.Get("end_time"), location)
	if err != nil {
		return AuditLogFilter{}, errors.New("invalid end_time")
	}
	if startAt.After(endDate) {
		return AuditLogFilter{}, errors.New("start_time must not be after end_time")
	}
	if endDate.After(startAt.AddDate(0, 3, 0)) {
		return AuditLogFilter{}, errors.New("audit log date range must not exceed three months")
	}

	filter := AuditLogFilter{
		StartAt: startAt,
		EndAt:   endDate.AddDate(0, 0, 1),
	}
	if value := strings.TrimSpace(values.Get("user_id")); value != "" {
		userID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || userID <= 0 {
			return AuditLogFilter{}, errors.New("invalid user_id")
		}
		filter.UserID = &userID
	}
	filter.Action = strings.TrimSpace(values.Get("action"))
	if filter.Action != "" && (len(filter.Action) > 64 || !auditLogActionPattern.MatchString(filter.Action)) {
		return AuditLogFilter{}, errors.New("invalid action")
	}

	page, err := parseOptionalPositiveAuditLogInt(values.Get("page"))
	if err != nil {
		return AuditLogFilter{}, errors.New("invalid page")
	}
	filter.Page = page
	pageSize, err := parseOptionalPositiveAuditLogInt(values.Get("page_size"))
	if err != nil {
		return AuditLogFilter{}, errors.New("invalid page_size")
	}
	filter.PageSize = pageSize
	return filter, nil
}

func parseAuditLogDate(value string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("missing date")
	}
	parsed, err := time.ParseInLocation(auditLogDateLayout, value, location)
	if err != nil || parsed.Format(auditLogDateLayout) != value {
		return time.Time{}, errors.New("invalid date")
	}
	return parsed, nil
}

func parseOptionalPositiveAuditLogInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsed, nil
}

func auditLogSummary(log AuditLog) string {
	if detail := sanitizeAuditDetail(log.DetailJSON); len(detail) > 0 {
		var values map[string]json.RawMessage
		if err := json.Unmarshal(detail, &values); err == nil {
			var summary string
			if raw, ok := values["summary"]; ok && json.Unmarshal(raw, &summary) == nil {
				summary = strings.TrimSpace(summary)
				if summary != "" && summary != strings.TrimSpace(log.Action) {
					return summary
				}
			}
		}
	}
	action := strings.TrimSpace(log.Action)
	if auditLogActionPattern.MatchString(action) {
		return "Audit event: " + action
	}
	return "Audit event"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
