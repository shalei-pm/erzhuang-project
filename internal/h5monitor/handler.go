package h5monitor

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

type Handler struct {
	service    *Service
	authorizer Authorizer
}

type AuthContext struct {
	UserID int64
	Role   string
}

type Authorizer interface {
	CurrentUser(r *http.Request) (AuthContext, error)
	CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error)
	FilterMonitorStores(r *http.Request, stores MonitorStoresResponse) (MonitorStoresResponse, error)
}

type denyAllAuthorizer struct{}

func (denyAllAuthorizer) CurrentUser(*http.Request) (AuthContext, error) {
	return AuthContext{}, ErrUnauthorized
}

func (denyAllAuthorizer) CanViewMonitorStore(*http.Request, string) (bool, error) {
	return false, ErrUnauthorized
}

func (denyAllAuthorizer) FilterMonitorStores(*http.Request, MonitorStoresResponse) (MonitorStoresResponse, error) {
	return MonitorStoresResponse{}, ErrUnauthorized
}

// AuditAuthorizer is optional so existing authorizers remain source-compatible.
type AuditAuthorizer interface {
	RecordAudit(r *http.Request, event auditlog.AuditEvent) error
}

var ErrUnauthorized = errors.New("h5monitor: unauthorized")
var ErrForbidden = errors.New("h5monitor: forbidden")

const (
	auditUnavailableCode = "audit_unavailable"
	serviceFailedCode    = "h5_monitor_service_failed"
	serviceFailedMessage = "监控服务请求失败，请稍后重试"
)

func NewHandler(service *Service) *Handler {
	return NewHandlerWithAuthorizer(service, denyAllAuthorizer{})
}

// NewHandlerForTesting creates a handler without authorization. It is kept
// explicit so production callers cannot accidentally bypass SSO by using the
// historical compatibility constructor.
func NewHandlerForTesting(service *Service) *Handler {
	return &Handler{service: service}
}

func NewHandlerWithAuthorizer(service *Service, authorizer Authorizer) *Handler {
	if authorizer == nil {
		authorizer = denyAllAuthorizer{}
	}
	return &Handler{service: service, authorizer: authorizer}
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	RegisterRoutesWithAuthorizer(mux, service, nil)
}

func RegisterRoutesWithAuthorizer(mux *http.ServeMux, service *Service, authorizer Authorizer) {
	if authorizer == nil {
		authorizer = denyAllAuthorizer{}
	}
	handler := NewHandlerWithAuthorizer(service, authorizer)
	mux.HandleFunc("GET /api/h5/monitor/stores", handler.getMonitorStores)
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor", handler.getMonitorHome)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url", handler.getLiveURL)
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments", handler.getRecordSegments)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url", handler.getPlaybackURL)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/disable-url", handler.disableURL)
}

func (h *Handler) getMonitorStores(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListMonitorStores(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.authorizer != nil {
		result, err = h.authorizer.FilterMonitorStores(r, result)
		if err != nil {
			writeAuthzError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getMonitorHome(w http.ResponseWriter, r *http.Request) {
	if !h.ensureCanViewStore(w, r, r.PathValue("externalOrgId")) {
		return
	}
	result, err := h.service.GetMonitorHome(r.Context(), r.PathValue("externalOrgId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getLiveURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	if !h.ensureCanViewStoreForAction(w, r, r.PathValue("externalOrgId"), channelID, "monitor.live_view") {
		return
	}
	var request LiveURLRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &request) {
			return
		}
	}
	if !h.applyServerIdentity(w, r, &request.UserID, &request.IsAdmin) {
		return
	}
	result, err := h.service.GetLiveURL(r.Context(), r.PathValue("externalOrgId"), channelID, request.UserID, request.IsAdmin, request.Protocol, request.Quality)
	if err != nil {
		h.recordMonitorAudit(r, "monitor.live_view", r.PathValue("externalOrgId"), channelID, "failed")
		writeServiceError(w, err)
		return
	}
	if err := h.recordMonitorAudit(r, "monitor.live_view", r.PathValue("externalOrgId"), channelID, "success"); err != nil {
		h.service.releaseConcurrency(request.UserID)
		writeAuditUnavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getRecordSegments(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	if !h.ensureCanViewStore(w, r, r.PathValue("externalOrgId")) {
		return
	}
	result, err := h.service.GetRecordSegments(r.Context(), r.PathValue("externalOrgId"), channelID, r.URL.Query().Get("date"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getPlaybackURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	if !h.ensureCanViewStoreForAction(w, r, r.PathValue("externalOrgId"), channelID, "monitor.playback_view") {
		return
	}
	var request PlaybackURLRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if !h.applyServerIdentity(w, r, &request.UserID, &request.IsAdmin) {
		return
	}
	result, err := h.service.GetPlaybackURL(r.Context(), r.PathValue("externalOrgId"), channelID, request)
	if err != nil {
		h.recordMonitorAudit(r, "monitor.playback_view", r.PathValue("externalOrgId"), channelID, "failed")
		writeServiceError(w, err)
		return
	}
	if err := h.recordMonitorAudit(r, "monitor.playback_view", r.PathValue("externalOrgId"), channelID, "success"); err != nil {
		h.service.releaseConcurrency(request.UserID)
		writeAuditUnavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) disableURL(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseChannelID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id", nil)
		return
	}
	if !h.ensureCanViewStore(w, r, r.PathValue("externalOrgId")) {
		return
	}
	var request DisableURLRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	userID := request.UserID
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}
	isAdmin := false
	if !h.applyServerIdentity(w, r, &userID, &isAdmin) {
		return
	}
	err = h.service.DisableURL(r.Context(), r.PathValue("externalOrgId"), channelID, request.URLID, userID)
	if err != nil {
		body := map[string]any{"ok": false, "error": serviceFailedMessage, "code": serviceFailedCode}
		if code := ezviz.ErrorCode(err); code != "" {
			body["code"] = code
		}
		writeJSON(w, http.StatusOK, body)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ensureCanViewStore(w http.ResponseWriter, r *http.Request, externalOrgID string) bool {
	return h.ensureCanViewStoreForAction(w, r, externalOrgID, 0, "")
}

func (h *Handler) ensureCanViewStoreForAction(w http.ResponseWriter, r *http.Request, externalOrgID string, channelID int64, action string) bool {
	if h.authorizer == nil {
		return true
	}
	ok, err := h.authorizer.CanViewMonitorStore(r, externalOrgID)
	if err != nil {
		if action != "" && (errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)) {
			h.recordMonitorAudit(r, action, externalOrgID, channelID, "denied")
		}
		writeAuthzError(w, err)
		return false
	}
	if !ok {
		if action != "" {
			h.recordMonitorAudit(r, action, externalOrgID, channelID, "denied")
		}
		writeError(w, http.StatusForbidden, "暂无监控访问权限", nil)
		return false
	}
	return true
}

func (h *Handler) recordMonitorAudit(r *http.Request, action, externalOrgID string, channelID int64, result string) error {
	if h.authorizer == nil {
		return nil
	}
	auditor, ok := h.authorizer.(AuditAuthorizer)
	if !ok {
		return errors.New("h5monitor: audit authorizer unavailable")
	}

	event := auditlog.AuditEvent{
		Action:        action,
		EntityType:    "channel",
		ExternalOrgID: externalOrgID,
		Result:        result,
		IPAddress:     monitorRequestIPAddress(r),
		UserAgent:     strings.TrimSpace(r.UserAgent()),
		RequestID:     strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if channelID != 0 {
		event.EntityID = &channelID
		event.ChannelID = &channelID
	}
	if h.authorizer != nil {
		if user, err := h.authorizer.CurrentUser(r); err == nil && user.UserID != 0 {
			event.UserID = &user.UserID
		}
	}
	return auditor.RecordAudit(r, event)
}

func (h *Handler) applyServerIdentity(w http.ResponseWriter, r *http.Request, userID *string, isAdmin *bool) bool {
	if h.authorizer == nil {
		return true
	}
	user, err := h.authorizer.CurrentUser(r)
	if err != nil {
		writeAuthzError(w, err)
		return false
	}
	if user.UserID <= 0 {
		writeAuthzError(w, ErrUnauthorized)
		return false
	}
	*userID = strconv.FormatInt(user.UserID, 10)
	*isAdmin = strings.EqualFold(strings.TrimSpace(user.Role), "admin")
	return true
}

func monitorRequestIPAddress(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func writeAuditUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": "监控审计失败，请稍后重试",
		"code":  auditUnavailableCode,
	})
}

func parseChannelID(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.PathValue("channelId"))
	if value == "" {
		return 0, errors.New("channel id is required")
	}
	return strconv.ParseInt(value, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	response := map[string]any{"error": message}
	if len(fields) > 0 {
		response["fields"] = fields
	}
	writeJSON(w, status, response)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationError *ValidationError
	if errors.As(err, &validationError) {
		writeError(w, http.StatusBadRequest, validationError.Error(), validationError.Fields)
		return
	}
	if code := ezviz.ErrorCode(err); code != "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": serviceFailedMessage, "code": code})
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found", nil)
		return
	}
	if errors.Is(err, ErrConcurrencyLimit) {
		writeError(w, http.StatusTooManyRequests, "监控请求过于频繁，请稍后重试", nil)
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error": serviceFailedMessage,
		"code":  serviceFailedCode,
	})
}

func writeAuthzError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "暂无监控访问权限", nil)
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error": "监控权限校验失败，请稍后重试",
		"code":  "h5_monitor_authorization_failed",
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", nil)
		return false
	}
	return true
}
