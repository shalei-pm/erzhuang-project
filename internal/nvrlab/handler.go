package nvrlab

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type RouteMiddleware func(http.HandlerFunc) http.HandlerFunc

type Handler struct {
	service *Service
	auditor AuditAuthorizer
}

type AuditAuthorizer interface {
	RecordAudit(r *http.Request, event auditlog.AuditEvent) error
}

func RegisterRoutes(mux *http.ServeMux, service *Service, adminGuard RouteMiddleware) {
	registerRoutes(mux, service, adminGuard, nil)
}

func RegisterRoutesWithAudit(mux *http.ServeMux, service *Service, adminGuard RouteMiddleware, auditor AuditAuthorizer) {
	registerRoutes(mux, service, adminGuard, auditor)
}

func registerRoutes(mux *http.ServeMux, service *Service, adminGuard RouteMiddleware, auditor AuditAuthorizer) {
	handler := &Handler{service: service, auditor: auditor}
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		if adminGuard == nil {
			return next
		}
		return adminGuard(next)
	}
	mux.HandleFunc("GET /api/h5/nvr-lab/10001/cameras", guard(handler.listCameras))
	mux.HandleFunc("POST /api/h5/nvr-lab/10001/cameras/{cameraId}/stream-session", guard(handler.createSession))
}

func (h *Handler) listCameras(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_lab_not_configured", "error": "取流实验页暂未配置"})
		return
	}
	response, err := h.service.ListCameras(r.Context(), ExperimentTenantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_lab_not_configured", "error": "取流实验页暂未配置"})
		return
	}
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if err != nil || cameraID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_camera_id", "error": "摄像头参数无效"})
		return
	}
	var request StreamSessionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stream_session", "error": "取流参数无效"})
		return
	}
	response, err := h.service.CreateSession(r.Context(), ExperimentTenantID, cameraID, request)
	if err != nil {
		h.recordAudit(r, cameraID, "failed")
		writeServiceError(w, err)
		return
	}
	if err := h.recordAudit(r, cameraID, "success"); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_lab_audit_failed", "error": "监控审计失败，请稍后重试"})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) recordAudit(r *http.Request, cameraID int64, result string) error {
	if h.auditor == nil {
		return nil
	}
	return h.auditor.RecordAudit(r, auditlog.AuditEvent{
		Action:        "monitor.live_view",
		EntityType:    "camera",
		EntityID:      &cameraID,
		ExternalOrgID: strconv.FormatInt(ExperimentTenantID, 10),
		Result:        result,
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrExperimentNotFound), errors.Is(err, ErrCameraNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "nvr_lab_camera_not_found", "error": "未找到可用摄像头"})
	case errors.Is(err, ErrInvalidStreamMode), errors.Is(err, ErrInvalidPlaybackWindow):
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stream_session", "error": "回放时间范围无效"})
	case errors.Is(err, ErrNotConfigured):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "nvr_lab_not_configured", "error": "取流实验页暂未配置"})
	case errors.Is(err, ErrAuthorizationTimeout):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{"code": "nvr_stream_authorization_timeout", "error": "取流鉴权超时，请稍后重试"})
	case errors.Is(err, ErrAuthorizationFailed):
		code := "nvr_stream_authorization_failed"
		var diagnostic *authorizationFailureError
		if errors.As(err, &diagnostic) {
			log.Printf("nvr stream authorization failed: category=%s value=%d", diagnostic.category, diagnostic.value)
			code = "nvr_stream_authorization_" + diagnostic.category
			if diagnostic.value != 0 {
				code += "_" + strconv.Itoa(diagnostic.value)
			}
		} else {
			log.Print("nvr stream authorization failed: category=unknown value=0")
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": code, "error": "取流鉴权失败，请稍后重试"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "nvr_lab_failed", "error": "取流实验页请求失败"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
