package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
)

const monitorScreenshotTimeLayout = "2006-01-02 15:04:05"

type monitorScreenshotWatermarkSettingsResponse struct {
	Enabled bool `json:"enabled"`
}

type monitorScreenshotWatermarkSettingsRequest struct {
	Enabled *bool `json:"enabled"`
}

type monitorScreenshotMetadataResponse struct {
	WatermarkEnabled bool   `json:"watermark_enabled"`
	DisplayName      string `json:"display_name,omitempty"`
	CapturedAt       string `json:"captured_at,omitempty"`
}

func (h *Handler) monitorScreenshotWatermarkSettingsStore() (MonitorScreenshotSettingsStore, bool) {
	store, ok := h.store.(MonitorScreenshotSettingsStore)
	return store, ok && store != nil
}

func (h *Handler) monitorScreenshotWatermarkSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := h.monitorScreenshotWatermarkSettingsStore()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图水印设置暂不可用"})
		return
	}
	enabled, err := store.GetMonitorScreenshotWatermarkEnabled(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取截图水印设置失败"})
		return
	}
	writeJSON(w, http.StatusOK, monitorScreenshotWatermarkSettingsResponse{Enabled: enabled})
}

func (h *Handler) updateMonitorScreenshotWatermarkSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := h.monitorScreenshotWatermarkSettingsStore()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图水印设置暂不可用"})
		return
	}
	var request monitorScreenshotWatermarkSettingsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "截图水印设置参数无效"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "截图水印设置参数无效"})
		return
	}
	previous, err := store.GetMonitorScreenshotWatermarkEnabled(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取截图水印设置失败"})
		return
	}
	if err := store.SetMonitorScreenshotWatermarkEnabled(r.Context(), *request.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "保存截图水印设置失败"})
		return
	}
	detail, _ := json.Marshal(map[string]any{"summary": "更新监控截图水印设置", "previous_enabled": previous, "enabled": *request.Enabled})
	if err := h.recordMonitorAudit(r.Context(), r, auditlog.AuditEvent{Action: "system.monitor_screenshot_watermark.update", EntityType: "system_setting", Result: "success", DetailJSON: detail}); err != nil {
		if rollbackErr := store.SetMonitorScreenshotWatermarkEnabled(r.Context(), previous); rollbackErr != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图水印设置审计失败且无法回滚，请联系管理员"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图水印设置审计失败，已取消本次修改"})
		return
	}
	writeJSON(w, http.StatusOK, monitorScreenshotWatermarkSettingsResponse{Enabled: *request.Enabled})
}

func (h *Handler) nvrScreenshotMetadataHandler(w http.ResponseWriter, r *http.Request) {
	externalOrgID := strings.TrimSpace(r.PathValue("externalOrgId"))
	cameraID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("cameraId")), 10, 64)
	if externalOrgID == "" || err != nil || cameraID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "摄像头参数无效"})
		return
	}
	identity, err := h.currentAuthIdentity(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	allowed, err := h.store.CanUserViewMonitorStore(r.Context(), identity.record, externalOrgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "校验监控权限失败"})
		return
	}
	if !allowed {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "暂无该门店监控访问权限"})
		return
	}
	if h.nvrMonitorService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "工控机监控暂未配置"})
		return
	}
	cameras, err := h.nvrMonitorService.GetCameras(r.Context(), externalOrgID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未找到可用摄像头"})
		return
	}
	var cameraTarget nvrmonitor.CameraAuditTarget
	for _, camera := range cameras.Cameras {
		if camera.ID == cameraID {
			cameraTarget = nvrmonitor.CameraAuditTarget{
				ExternalOrgID: cameras.ExternalOrgID,
				StoreName:     cameras.StoreName,
				CameraID:      camera.ID,
				CameraName:    camera.Name,
				SpaceType:     camera.SpaceType,
				SpaceName:     camera.SpaceName,
			}
			break
		}
	}
	if cameraTarget.CameraID == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未找到可用摄像头"})
		return
	}
	store, ok := h.monitorScreenshotWatermarkSettingsStore()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图水印设置暂不可用"})
		return
	}
	enabled, err := store.GetMonitorScreenshotWatermarkEnabled(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取截图水印设置失败"})
		return
	}
	response := monitorScreenshotMetadataResponse{WatermarkEnabled: enabled}
	if enabled {
		response.DisplayName = screenshotWatermarkDisplayName(identity)
		response.CapturedAt = monitorScreenshotTime(h.authNow())
	}
	detail, _ := json.Marshal(map[string]string{"summary": nvrmonitor.CameraAuditSummary(cameraTarget), "captured_at": response.CapturedAt})
	if err := h.recordMonitorAudit(r.Context(), r, auditlog.AuditEvent{Action: "monitor.screenshot", EntityType: "camera", EntityID: int64Pointer(cameraID), ExternalOrgID: externalOrgID, Result: "success", DetailJSON: detail}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "截图审计记录失败，请重试"})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func monitorScreenshotTime(value time.Time) string {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return value.UTC().Format(monitorScreenshotTimeLayout)
	}
	return value.In(location).Format(monitorScreenshotTimeLayout)
}

func screenshotWatermarkDisplayName(identity authenticatedAuthContext) string {
	for _, value := range []string{identity.record.DisplayName, identity.user.DisplayName, identity.record.Username, identity.user.Username, identity.record.Email, identity.user.Email} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "当前用户"
}
