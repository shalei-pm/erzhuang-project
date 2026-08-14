package resourceview

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type RouteMiddleware func(http.HandlerFunc) http.HandlerFunc
type MonitorAccessResolver func(r *http.Request, tenantID int64) (MonitorAccess, error)

type Handler struct {
	service       *Service
	monitorAccess MonitorAccessResolver
}

func RegisterRoutes(mux *http.ServeMux, service *Service, resolver MonitorAccessResolver) {
	RegisterRoutesWithReadGuard(mux, service, resolver, nil)
}

func RegisterRoutesWithReadGuard(mux *http.ServeMux, service *Service, resolver MonitorAccessResolver, readGuard RouteMiddleware) {
	handler := &Handler{service: service, monitorAccess: resolver}
	read := func(next http.HandlerFunc) http.HandlerFunc {
		if readGuard == nil {
			return next
		}
		return readGuard(next)
	}
	mux.HandleFunc("GET /api/store-space-resource-view/stores", read(handler.listStores))
	mux.HandleFunc("GET /api/store-space-resource-view/stores/{tenantId}", read(handler.getStore))
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeResourceJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "resource_view_not_configured", "error": "store space resource view is not configured"})
		return
	}
	filters := StoreFilters{
		Query:    strings.TrimSpace(r.URL.Query().Get("q")),
		CityID:   parseOptionalInt64(r.URL.Query().Get("city_id")),
		Page:     parsePositiveInt(r.URL.Query().Get("page"), 1),
		PageSize: parsePositiveInt(r.URL.Query().Get("page_size"), 20),
	}
	result, err := h.service.ListStores(r.Context(), filters, func(tenantID int64) MonitorAccess {
		if h.monitorAccess == nil {
			return MonitorAccess{}
		}
		access, err := h.monitorAccess(r, tenantID)
		if err != nil {
			return MonitorAccess{}
		}
		return access
	})
	if err != nil {
		writeResourceJSON(w, http.StatusInternalServerError, map[string]string{"code": "list_resource_stores_failed", "error": err.Error()})
		return
	}
	writeResourceJSON(w, http.StatusOK, result)
}

func (h *Handler) getStore(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeResourceJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "resource_view_not_configured", "error": "store space resource view is not configured"})
		return
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("tenantId")), 10, 64)
	if err != nil || tenantID <= 0 {
		writeResourceJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_tenant_id", "error": "invalid tenant id"})
		return
	}
	access := MonitorAccess{}
	if h.monitorAccess != nil {
		access, err = h.monitorAccess(r, tenantID)
		if err != nil {
			writeResourceJSON(w, http.StatusForbidden, map[string]string{"code": "resource_view_forbidden", "error": "forbidden"})
			return
		}
	}
	detail, err := h.service.GetStore(r.Context(), tenantID, access)
	if errors.Is(err, ErrNotFound) {
		writeResourceJSON(w, http.StatusNotFound, map[string]string{"code": "resource_store_not_found", "error": "store not found"})
		return
	}
	if err != nil {
		writeResourceJSON(w, http.StatusInternalServerError, map[string]string{"code": "get_resource_store_failed", "error": err.Error()})
		return
	}
	writeResourceJSON(w, http.StatusOK, detail)
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseOptionalInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func writeResourceJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
