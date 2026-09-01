package nvrmonitor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type Authorizer interface {
	CanViewStore(r *http.Request, externalOrgID string) (bool, error)
	FilterStores(r *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error)
}

// AuditAuthorizer is optional so existing Authorizer implementations keep
// working. Audit failures on successful media operations are fail-closed.
type AuditAuthorizer interface {
	RecordAudit(r *http.Request, event auditlog.AuditEvent) error
}

// SnapshotBackfillAuthorizer is intentionally optional so the normal monitor
// read APIs retain their existing authorization contract. Production wiring
// implements it and rejects browser uploads from view-only accounts.
type SnapshotBackfillAuthorizer interface {
	CanBackfillSnapshot(r *http.Request, externalOrgID string) (bool, error)
}

const maxSnapshotUploadBytes = 2 << 20
const maxStreamSessionBodyBytes = 4096
const snapshotRefreshPrepareAction = "snapshot.refresh.prepare"

var errRequestBodyTooLarge = errors.New("nvr monitor request body too large")

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
	if err := decodeStreamSessionRequest(w, r, &request); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "stream_session_too_large", "error": "取流参数过大"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stream_session", "error": "取流参数无效"})
		}
		return
	}
	response, err := h.service.CreateSession(r.Context(), externalOrgID, cameraID, request)
	action := "monitor.live_view"
	if request.Mode == ModePlayback {
		action = "monitor.playback_view"
	}
	if err != nil {
		_ = h.recordAudit(r, newCameraAuditEvent(r, action, "failed", externalOrgID, cameraID))
		writeServiceError(w, err)
		return
	}
	if err := h.recordAudit(r, newCameraAuditEvent(r, action, "success", externalOrgID, cameraID)); err != nil {
		writeAuditError(w)
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
		_ = h.recordAudit(r, newCameraAuditEvent(r, "snapshot.download", "failed", externalOrgID, cameraID))
		http.NotFound(w, r)
		return
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, maxSnapshotUploadBytes+1))
	if err != nil || len(body) > maxSnapshotUploadBytes {
		_ = h.recordAudit(r, newCameraAuditEvent(r, "snapshot.download", "failed", externalOrgID, cameraID))
		http.NotFound(w, r)
		return
	}
	mediaType, _, mediaTypeErr := mime.ParseMediaType(contentType)
	if mediaTypeErr != nil || !strings.EqualFold(mediaType, "image/jpeg") {
		_ = h.recordAudit(r, newCameraAuditEvent(r, "snapshot.download", "failed", externalOrgID, cameraID))
		http.NotFound(w, r)
		return
	}
	if err := h.recordAudit(r, newCameraAuditEvent(r, "snapshot.download", "success", externalOrgID, cameraID)); err != nil {
		writeAuditError(w)
		return
	}
	// Keep thumbnails private and prevent a mislabelled object from being
	// interpreted as executable content by a browser.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, bytes.NewReader(body)); err != nil {
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
	if err != nil || !strings.EqualFold(mediaType, "image/jpeg") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"code": "invalid_snapshot_content_type", "error": "截图格式无效"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSnapshotUploadBytes))
	if isRequestBodyTooLarge(err) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "snapshot_too_large", "error": "截图文件过大"})
		return
	}
	if err != nil || len(body) == 0 || len(body) > maxSnapshotUploadBytes || http.DetectContentType(body) != "image/jpeg" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_snapshot", "error": "截图内容无效"})
		return
	}
	auditEnabled := h.hasAuditAuthorizer()
	// The prepare event is the write gate; the final event is recorded only
	// after SaveSnapshot succeeds. If final auditing fails, a real asset store
	// rolls back the deterministic object before the request is rejected.
	if auditEnabled && h.recordAudit(r, newCameraAuditEvent(r, snapshotRefreshPrepareAction, "success", externalOrgID, cameraID)) != nil {
		writeAuditError(w)
		return
	}
	var rollback SnapshotRollback
	var saveErr error
	if auditEnabled {
		rollback, saveErr = h.service.SaveSnapshotWithRollback(r.Context(), externalOrgID, cameraID, bytes.NewReader(body))
	} else {
		saveErr = h.service.SaveSnapshot(r.Context(), externalOrgID, cameraID, bytes.NewReader(body))
	}
	if saveErr != nil {
		if auditEnabled {
			_ = h.recordAudit(r, newCameraAuditEvent(r, "snapshot.refresh", "failed", externalOrgID, cameraID))
		}
		writeServiceError(w, saveErr)
		return
	}
	if auditEnabled && h.recordAudit(r, newCameraAuditEvent(r, "snapshot.refresh", "success", externalOrgID, cameraID)) != nil {
		if rollbackErr := rollback.Rollback(r.Context()); rollbackErr != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_snapshot_rollback_failed", "error": "截图刷新审计失败且回滚失败，请立即检查截图状态"})
			return
		}
		writeAuditError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeStreamSessionRequest(w http.ResponseWriter, r *http.Request, request *StreamSessionRequest) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxStreamSessionBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		if isRequestBodyTooLarge(err) {
			return errRequestBodyTooLarge
		}
		return err
	}
	// A second JSON value or any non-whitespace suffix is invalid. Without this
	// check, a valid request followed by attacker-controlled data is accepted.
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if isRequestBodyTooLarge(err) {
			return errRequestBodyTooLarge
		}
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
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
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthorized) {
			_ = h.recordAudit(r, newDeniedAuditEvent(r, auditActionForRequest(r), externalOrgID))
		}
		writeAuthorizationError(w, err)
		return false
	}
	if !ok {
		_ = h.recordAudit(r, newDeniedAuditEvent(r, auditActionForRequest(r), externalOrgID))
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
	if h.authorizer == nil {
		return true
	}
	authorizer, ok := h.authorizer.(SnapshotBackfillAuthorizer)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_snapshot_backfill_unavailable", "error": "截图回填暂不可用"})
		return false
	}
	allowed, err := authorizer.CanBackfillSnapshot(r, externalOrgID)
	if err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthorized) {
			_ = h.recordAudit(r, newDeniedAuditEvent(r, "snapshot.refresh", externalOrgID))
		}
		writeAuthorizationError(w, err)
		return false
	}
	if !allowed {
		_ = h.recordAudit(r, newDeniedAuditEvent(r, "snapshot.refresh", externalOrgID))
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_snapshot_backfill_forbidden", "error": "暂无截图回填权限"})
		return false
	}
	return true
}

func (h *Handler) recordAudit(r *http.Request, event auditlog.AuditEvent) error {
	if h.authorizer == nil {
		return nil
	}
	authorizer, ok := h.authorizer.(AuditAuthorizer)
	if !ok {
		return errors.New("nvrmonitor: audit authorizer unavailable")
	}
	return authorizer.RecordAudit(r, event)
}

func (h *Handler) hasAuditAuthorizer() bool {
	return h.authorizer != nil
}

func newCameraAuditEvent(r *http.Request, action string, result string, externalOrgID string, cameraID int64) auditlog.AuditEvent {
	return auditlog.AuditEvent{
		Action:        action,
		EntityType:    "camera",
		EntityID:      int64Pointer(cameraID),
		ExternalOrgID: strings.TrimSpace(externalOrgID),
		IPAddress:     requestIPAddress(r),
		UserAgent:     strings.TrimSpace(r.UserAgent()),
		RequestID:     strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Result:        result,
	}
}

func newDeniedAuditEvent(r *http.Request, action string, externalOrgID string) auditlog.AuditEvent {
	event := auditlog.AuditEvent{
		Action:        action,
		ExternalOrgID: strings.TrimSpace(externalOrgID),
		IPAddress:     requestIPAddress(r),
		UserAgent:     strings.TrimSpace(r.UserAgent()),
		RequestID:     strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Result:        "denied",
	}
	if cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64); err == nil && cameraID > 0 {
		event.EntityType = "camera"
		event.EntityID = int64Pointer(cameraID)
	}
	return event
}

func auditActionForRequest(r *http.Request) string {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/stream-session"):
		return streamDeniedAuditAction(r)
	case strings.HasSuffix(path, "/snapshot") && r.Method == http.MethodPost:
		return "snapshot.refresh"
	case strings.HasSuffix(path, "/snapshot"):
		return "snapshot.download"
	default:
		return "monitor.camera_list"
	}
}

func streamDeniedAuditAction(r *http.Request) string {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err == nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request StreamSessionRequest
		if json.Unmarshal(body, &request) == nil && request.Mode == ModePlayback {
			return "monitor.playback_view"
		}
	}
	return "monitor.live_view"
}

func int64Pointer(value int64) *int64 {
	return &value
}

func requestIPAddress(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func writeAuditError(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_monitor_audit_failed", "error": "监控审计失败，请稍后重试"})
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
	case errors.Is(err, ErrSnapshotTransactionUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_snapshot_transaction_unavailable", "error": "截图存储暂不支持可靠刷新，请稍后重试"})
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
