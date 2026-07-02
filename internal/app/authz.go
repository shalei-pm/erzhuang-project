package app

import (
	"errors"
	"net/http"
	"strings"
	"time"
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

func hasPermission(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
