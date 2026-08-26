package app

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
)

var (
	errUnauthorizedAuth = errors.New("auth unauthorized")
	errForbiddenAuth    = errors.New("auth forbidden")
)

func (h *Handler) currentAuthUser(r *http.Request) (AuthUserRecord, error) {
	cookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		if h.auth.Enabled {
			return AuthUserRecord{}, errUnauthorizedAuth
		}
		return AuthUserRecord{Role: RoleAdmin, Enabled: true}, nil
	}
	claims, err := h.auth.validateAPISIXSSOToken(cookie.Value, time.Now())
	if err != nil {
		if h.auth.Enabled {
			return AuthUserRecord{}, errUnauthorizedAuth
		}
		return AuthUserRecord{Role: RoleAdmin, Enabled: true}, nil
	}
	user := claims.authUser()
	record, err := h.store.GetAuthUserByEmail(r.Context(), user.Email)
	if errors.Is(err, errAuthUserNotFound) || (err == nil && !record.Enabled) {
		return AuthUserRecord{}, errForbiddenAuth
	}
	if err != nil {
		return AuthUserRecord{}, err
	}
	return record, nil
}

func (h *Handler) requirePermission(w http.ResponseWriter, r *http.Request, permission string) (AuthUserRecord, bool) {
	record, err := h.currentAuthUser(r)
	if errors.Is(err, errUnauthorizedAuth) {
		h.writeUnauthorizedAuth(w)
		return AuthUserRecord{}, false
	}
	if errors.Is(err, errForbiddenAuth) {
		h.writeForbiddenAuth(w)
		return AuthUserRecord{}, false
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load auth user failed"})
		return AuthUserRecord{}, false
	}
	if !hasPermission(record.permissions(), permission) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "暂无操作权限"})
		return AuthUserRecord{}, false
	}
	return record, true
}

func (h *Handler) requirePermissionHandler(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.requirePermission(w, r, permission); !ok {
			return
		}
		next(w, r)
	}
}

func (h *Handler) nvrLabAdminGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := h.currentAuthUser(r)
		if errors.Is(err, errUnauthorizedAuth) {
			h.writeUnauthorizedAuth(w)
			return
		}
		if errors.Is(err, errForbiddenAuth) {
			h.writeForbiddenAuth(w)
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load auth user failed"})
			return
		}
		if normalizeRole(user.Role) != RoleAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_lab_forbidden", "error": "暂无实验页面访问权限"})
			return
		}
		next(w, r)
	}
}

func hasPermission(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type h5MonitorAuthorizer struct {
	handler *Handler
}

type nvrMonitorAuthorizer struct {
	handler *Handler
}

func (a nvrMonitorAuthorizer) CanViewStore(r *http.Request, externalOrgID string) (bool, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return false, nvrMonitorAuthError(err)
	}
	return a.handler.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
}

func (a nvrMonitorAuthorizer) FilterStores(r *http.Request, response nvrmonitor.MonitorStoresResponse) (nvrmonitor.MonitorStoresResponse, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return nvrmonitor.MonitorStoresResponse{}, nvrMonitorAuthError(err)
	}
	if normalizeRole(user.Role) != RoleViewer {
		return response, nil
	}
	filtered := nvrmonitor.MonitorStoresResponse{}
	for _, group := range response.Cities {
		nextGroup := nvrmonitor.StoreCityGroup{City: group.City}
		for _, store := range group.Stores {
			ok, err := a.handler.store.CanUserViewMonitorStore(r.Context(), user, store.ExternalOrgID)
			if err != nil {
				return nvrmonitor.MonitorStoresResponse{}, err
			}
			if ok {
				nextGroup.Stores = append(nextGroup.Stores, store)
			}
		}
		if len(nextGroup.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, nextGroup)
		}
	}
	return filtered, nil
}

func nvrMonitorAuthError(err error) error {
	if errors.Is(err, errUnauthorizedAuth) {
		return nvrmonitor.ErrUnauthorized
	}
	if errors.Is(err, errForbiddenAuth) {
		return nvrmonitor.ErrForbidden
	}
	return err
}

func (a h5MonitorAuthorizer) CurrentUser(r *http.Request) (h5monitor.AuthContext, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return h5monitor.AuthContext{}, h5MonitorAuthError(err)
	}
	return h5monitor.AuthContext{UserID: user.ID, Role: normalizeRole(user.Role)}, nil
}

func (a h5MonitorAuthorizer) CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return false, h5MonitorAuthError(err)
	}
	return a.handler.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
}

func (a h5MonitorAuthorizer) FilterMonitorStores(r *http.Request, response h5monitor.MonitorStoresResponse) (h5monitor.MonitorStoresResponse, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return h5monitor.MonitorStoresResponse{}, h5MonitorAuthError(err)
	}
	if normalizeRole(user.Role) != RoleViewer {
		return response, nil
	}
	filtered := h5monitor.MonitorStoresResponse{}
	for _, group := range response.Cities {
		nextGroup := h5monitor.MonitorStoreCityGroup{City: group.City}
		for _, store := range group.Stores {
			ok, err := a.handler.store.CanUserViewMonitorStore(r.Context(), user, store.ExternalOrgID)
			if err != nil {
				return h5monitor.MonitorStoresResponse{}, err
			}
			if ok {
				nextGroup.Stores = append(nextGroup.Stores, store)
			}
		}
		if len(nextGroup.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, nextGroup)
		}
	}
	return filtered, nil
}

func h5MonitorAuthError(err error) error {
	if errors.Is(err, errUnauthorizedAuth) {
		return h5monitor.ErrUnauthorized
	}
	if errors.Is(err, errForbiddenAuth) {
		return h5monitor.ErrForbidden
	}
	return err
}
