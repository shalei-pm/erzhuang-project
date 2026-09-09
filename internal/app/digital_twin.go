package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
)

func (h *Handler) digitalTwinSettings(ctx context.Context) (DigitalTwinSettings, error) {
	store, ok := h.store.(DigitalTwinSettingsStore)
	if !ok {
		return DigitalTwinSettings{}, errors.New("digital twin settings unavailable")
	}
	return store.GetDigitalTwinSettings(ctx)
}

func containsDigitalTwinStore(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func (h *Handler) filterDigitalTwinStores(ctx context.Context, response nvrmonitor.MonitorStoresResponse) (nvrmonitor.MonitorStoresResponse, error) {
	settings, err := h.digitalTwinSettings(ctx)
	if err != nil {
		return nvrmonitor.MonitorStoresResponse{}, err
	}
	out := nvrmonitor.MonitorStoresResponse{Cities: []nvrmonitor.StoreCityGroup{}}
	for _, group := range response.Cities {
		next := nvrmonitor.StoreCityGroup{City: group.City, Stores: []nvrmonitor.StoreInfo{}}
		for _, store := range group.Stores {
			if containsDigitalTwinStore(settings.StoreIDs, store.ExternalOrgID) {
				next.Stores = append(next.Stores, store)
			}
		}
		if len(next.Stores) > 0 {
			out.Cities = append(out.Cities, next)
		}
	}
	return out, nil
}

// A separate authorizer narrows only the digital twin routes, never normal monitoring.
type digitalTwinAuthorizer struct{ handler *Handler }

func (a digitalTwinAuthorizer) CanViewStore(r *http.Request, id string) (bool, error) {
	settings, err := a.handler.digitalTwinSettings(r.Context())
	if err != nil {
		return false, err
	}
	if !containsDigitalTwinStore(settings.StoreIDs, id) {
		return false, nil
	}
	return (nvrMonitorAuthorizer{handler: a.handler}).CanViewStore(r, id)
}

func (a digitalTwinAuthorizer) FilterStores(r *http.Request, response nvrmonitor.MonitorStoresResponse) (nvrmonitor.MonitorStoresResponse, error) {
	filtered, err := a.handler.filterDigitalTwinStores(r.Context(), response)
	if err != nil {
		return nvrmonitor.MonitorStoresResponse{}, err
	}
	return (nvrMonitorAuthorizer{handler: a.handler}).FilterStores(r, filtered)
}

func (a digitalTwinAuthorizer) RecordAudit(r *http.Request, event auditlog.AuditEvent) error {
	return (nvrMonitorAuthorizer{handler: a.handler}).RecordAudit(r, event)
}

func (h *Handler) digitalTwinSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	settings, err := h.digitalTwinSettings(r.Context())
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "数字孪生白名单读取失败，请稍后重试"})
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) digitalTwinCandidatesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.nvrMonitorService == nil {
		writeJSON(w, 503, map[string]string{"error": "机构资源服务暂不可用"})
		return
	}
	result, err := h.nvrMonitorService.ListStores(r.Context())
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "读取机构目录失败"})
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) updateDigitalTwinSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request struct {
		StoreIDs *[]string `json:"store_ids"`
		Version  string    `json:"version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	var extra any
	if err := decoder.Decode(&request); err != nil || request.StoreIDs == nil || request.Version == "" {
		writeJSON(w, 400, map[string]string{"error": "白名单参数无效"})
		return
	}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeJSON(w, 400, map[string]string{"error": "白名单参数无效"})
		return
	}
	ids, err := NormalizeDigitalTwinStoreIDs(*request.StoreIDs)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "机构 ID 必须为有效正整数，且不能超过名单容量"})
		return
	}
	// Empty lists remain savable when the resource service is unavailable (emergency disable).
	if len(ids) > 0 {
		if h.nvrMonitorService == nil {
			writeJSON(w, 503, map[string]string{"error": "机构资源服务暂不可用"})
			return
		}
		stores, err := h.nvrMonitorService.ListStores(r.Context())
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "校验机构失败，请稍后重试"})
			return
		}
		available := map[string]bool{}
		for _, group := range stores.Cities {
			for _, store := range group.Stores {
				available[store.ExternalOrgID] = true
			}
		}
		for _, id := range ids {
			if !available[id] {
				writeJSON(w, 400, map[string]string{"error": "机构 " + id + " 不存在或尚未配置可用监控"})
				return
			}
		}
	}
	identity, err := h.currentAuthIdentity(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	store, ok := h.store.(DigitalTwinSettingsStore)
	if !ok {
		writeJSON(w, 503, map[string]string{"error": "数字孪生设置暂不可用"})
		return
	}
	actor := actorFromAuthUser(identity.record, identity.user)
	event := auditlog.AuditEvent{Action: "system.digital_twin_whitelist.update", EntityType: "system_setting", Result: "success", UserID: actor.userID, UserEmail: actor.email, ActorDisplayName: actor.displayName, IPAddress: requestIPAddress(r), UserAgent: r.UserAgent(), RequestID: r.Header.Get("X-Request-ID")}
	result, err := store.UpdateDigitalTwinSettings(r.Context(), request.Version, ids, event)
	if errors.Is(err, ErrDigitalTwinSettingsConflict) {
		writeJSON(w, 409, map[string]string{"error": "白名单已被其他管理员更新，请重新加载后修改"})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "白名单保存或审计失败，本次修改未生效"})
		return
	}
	writeJSON(w, 200, result)
}
