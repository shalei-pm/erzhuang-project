package nvrmonitor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

type Authorizer interface {
	CanViewStore(r *http.Request, externalOrgID string) (bool, error)
	FilterStores(r *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error)
}

// SnapshotBackfillAuthorizer is intentionally optional so the normal monitor
// read APIs retain their existing authorization contract. Production wiring
// implements it and rejects browser uploads from view-only accounts.
type SnapshotBackfillAuthorizer interface {
	CanBackfillSnapshot(r *http.Request, externalOrgID string) (bool, error)
}

const maxSnapshotUploadBytes = 2 << 20

type Handler struct {
	service    *Service
	authorizer Authorizer
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	RegisterRoutesWithAuthorizer(mux, service, nil)
}

func RegisterRoutesWithAuthorizer(mux *http.ServeMux, service *Service, authorizer Authorizer) {
	handler := &Handler{service: service, authorizer: authorizer}
	mux.HandleFunc("GET /api/h5/nvr-monitor/stores", handler.listStores)
	mux.HandleFunc("GET /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras", handler.listCameras)
	mux.HandleFunc("GET /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras/{cameraId}/snapshot", handler.getSnapshot)
	mux.HandleFunc("POST /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras/{cameraId}/snapshot", handler.saveSnapshot)
	mux.HandleFunc("POST /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras/{cameraId}/stream-session", handler.createSession)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_not_configured", "error": "工控机监控暂未配置"})
		return
	}
	response, err := h.service.ListStores(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.authorizer != nil {
		response, err = h.authorizer.FilterStores(r, response)
		if err != nil {
			writeAuthorizationError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) listCameras(w http.ResponseWriter, r *http.Request) {
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	if !h.ensureCanViewStore(w, r, externalOrgID) {
		return
	}
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_not_configured", "error": "工控机监控暂未配置"})
		return
	}
	response, err := h.service.GetCameras(r.Context(), externalOrgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	if !h.ensureCanViewStore(w, r, externalOrgID) {
		return
	}
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_not_configured", "error": "工控机监控暂未配置"})
		return
	}
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if err != nil || cameraID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_camera_id", "error": "摄像头参数无效"})
		return
	}
	var request StreamSessionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stream_session", "error": "取流参数无效"})
		return
	}
	response, err := h.service.CreateSession(r.Context(), externalOrgID, cameraID, request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	if !h.ensureCanViewStore(w, r, externalOrgID) {
		return
	}
	if h.service == nil {
		http.NotFound(w, r)
		return
	}
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if err != nil || cameraID <= 0 {
		http.NotFound(w, r)
		return
	}
	reader, contentType, err := h.service.OpenSnapshot(r.Context(), externalOrgID, cameraID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer reader.Close()
	// Keep thumbnails private, but allow a later deliberate recapture to become
	// visible without holding an immutable browser cache for a week.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if _, err := io.Copy(w, reader); err != nil {
		return
	}
}

func (h *Handler) saveSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	if !h.ensureCanBackfillSnapshot(w, r, externalOrgID) {
		return
	}
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_not_configured", "error": "工控机监控暂未配置"})
		return
	}
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if err != nil || cameraID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_camera_id", "error": "摄像头参数无效"})
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "image/jpeg" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"code": "invalid_snapshot_content_type", "error": "截图格式无效"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSnapshotUploadBytes))
	if err != nil || len(body) == 0 || len(body) > maxSnapshotUploadBytes || http.DetectContentType(body) != "image/jpeg" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_snapshot", "error": "截图内容无效"})
		return
	}
	if err := h.service.SaveSnapshot(r.Context(), externalOrgID, cameraID, bytes.NewReader(body)); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ensureCanViewStore(w http.ResponseWriter, r *http.Request, externalOrgID string) bool {
	if externalOrgID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_monitor_store_not_found", "error": "未找到可用门店"})
		return false
	}
	if h.authorizer == nil {
		return true
	}
	ok, err := h.authorizer.CanViewStore(r, externalOrgID)
	if err != nil {
		writeAuthorizationError(w, err)
		return false
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_monitor_forbidden", "error": "暂无该门店监控访问权限"})
		return false
	}
	return true
}

func (h *Handler) ensureCanBackfillSnapshot(w http.ResponseWriter, r *http.Request, externalOrgID string) bool {
	if externalOrgID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_monitor_store_not_found", "error": "未找到可用门店"})
		return false
	}
	authorizer, ok := h.authorizer.(SnapshotBackfillAuthorizer)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_snapshot_backfill_unavailable", "error": "截图回填暂不可用"})
		return false
	}
	allowed, err := authorizer.CanBackfillSnapshot(r, externalOrgID)
	if err != nil {
		writeAuthorizationError(w, err)
		return false
	}
	if !allowed {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_snapshot_backfill_forbidden", "error": "暂无截图回填权限"})
		return false
	}
	return true
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "nvr_monitor_unauthorized", "error": "登录状态已失效，请重新登录"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_monitor_forbidden", "error": "暂无该门店监控访问权限"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "nvr_monitor_authorization_failed", "error": "监控权限校验失败"})
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrStoreNotFound), errors.Is(err, ErrCameraNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_monitor_camera_not_found", "error": "未找到可用摄像头"})
	case errors.Is(err, ErrInvalidStreamMode), errors.Is(err, ErrInvalidPlaybackWindow):
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stream_session", "error": "回放时间范围无效"})
	case errors.Is(err, ErrNotConfigured):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_not_configured", "error": "工控机监控暂未配置"})
	case errors.Is(err, ErrAuthorizationTimeout):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{"code": "nvr_stream_authorization_timeout", "error": "取流鉴权超时，请稍后重试"})
	case errors.Is(err, ErrAuthorizationFailed):
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "nvr_stream_authorization_failed", "error": "取流鉴权失败，请稍后重试"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "nvr_monitor_failed", "error": "工控机监控请求失败"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
