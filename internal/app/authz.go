package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
)

var (
	errUnauthorizedAuth         = errors.New("auth unauthorized")
	errForbiddenAuth            = errors.New("auth forbidden")
	errSessionIdleTimeout       = errors.New("auth session idle timeout")
	errAuthSessionUnavailable   = errors.New("auth session unavailable")
	errAuditUnavailable         = errors.New("audit recorder unavailable")
	errAuditMutationUnavailable = errors.New("transactional audit mutation unavailable")
)

type authContextKey struct{}

type authenticatedAuthContext struct {
	record AuthUserRecord
	user   AuthUserResponse
}

type authSessionAuthentication struct {
	identity    authenticatedAuthContext
	newToken    string
	createdNew  bool
}

func (h *Handler) currentAuthUser(r *http.Request) (AuthUserRecord, error) {
	identity, err := h.currentAuthIdentity(r)
	if err != nil {
		return AuthUserRecord{}, err
	}
	return identity.record, nil
}

func (h *Handler) currentAuthIdentity(r *http.Request) (authenticatedAuthContext, error) {
	if identity, ok := r.Context().Value(authContextKey{}).(authenticatedAuthContext); ok {
		return identity, nil
	}
	authentication, err := h.authenticateRequest(r)
	if err != nil {
		return authenticatedAuthContext{}, err
	}
	return authentication.identity, nil
}

func (h *Handler) authenticateRequest(r *http.Request) (authSessionAuthentication, error) {
	if !h.auth.Enabled {
		return authSessionAuthentication{identity: authenticatedAuthContext{
			record: AuthUserRecord{Role: RoleAdmin, Enabled: true},
		}}, nil
	}
	cookie, err := r.Cookie(h.auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return authSessionAuthentication{}, errUnauthorizedAuth
	}
	now := h.authNow()
	claims, err := h.auth.validateAPISIXSSOToken(cookie.Value, now)
	if err != nil {
		return authSessionAuthentication{}, errUnauthorizedAuth
	}
	claimsUser := claims.authUser()
	record, err := h.store.GetAuthUserByEmail(r.Context(), claimsUser.Email)
	if errors.Is(err, errAuthUserNotFound) || (err == nil && !record.Enabled) {
		return authSessionAuthentication{}, errForbiddenAuth
	}
	if err != nil {
		return authSessionAuthentication{}, err
	}
	if h.authSessionStore == nil {
		return authSessionAuthentication{}, errAuthSessionUnavailable
	}
	identity := authenticatedAuthContext{record: record, user: claimsUser}
	localCookie, localErr := r.Cookie(authSessionCookieName)
	if localErr != nil || strings.TrimSpace(localCookie.Value) == "" {
		token, err := h.authSessionStore.CreateAuthSession(r.Context(), AuthSessionCreate{
			UserID:    record.ID,
			SSOSubject: claims.Sub,
			IPAddress: requestIPAddress(r),
			UserAgent: strings.TrimSpace(r.UserAgent()),
			Now:       now,
		})
		if err != nil || strings.TrimSpace(token) == "" {
			return authSessionAuthentication{}, errAuthSessionUnavailable
		}
		return authSessionAuthentication{identity: identity, newToken: token, createdNew: true}, nil
	}
	active, err := h.authSessionStore.TouchAuthSession(r.Context(), localCookie.Value, record.ID, now, h.authIdleTimeout())
	if err != nil {
		return authSessionAuthentication{}, errAuthSessionUnavailable
	}
	if !active {
		_ = h.authSessionStore.RevokeAuthSession(r.Context(), localCookie.Value, record.ID, "idle_timeout", now)
		h.recordAuthIdleTimeout(r, record, claimsUser)
		return authSessionAuthentication{}, errSessionIdleTimeout
	}
	return authSessionAuthentication{identity: identity}, nil
}

func (h *Handler) authNow() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *Handler) authIdleTimeout() time.Duration {
	if h.idleTimeout <= 0 || h.idleTimeout > defaultAuthIdleTimeout {
		return defaultAuthIdleTimeout
	}
	return h.idleTimeout
}

func (h *Handler) authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.auth.Enabled || isAuthGateExemptPath(r.URL.Path) || !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		authentication, err := h.authenticateRequest(r)
		if err != nil {
			h.writeAuthError(w, r, err)
			return
		}
		if authentication.createdNew {
			h.setAuthSessionCookie(w, r, authentication.newToken)
		}
		request := r.WithContext(context.WithValue(r.Context(), authContextKey{}, authentication.identity))
		next.ServeHTTP(w, request)
	})
}

func isAPIPath(path string) bool {
	for _, basePath := range configuredBasePaths() {
		if strings.HasPrefix(path, basePath+"/api/") {
			return true
		}
	}
	return strings.HasPrefix(path, "/api/")
}

func isAuthGateExemptPath(path string) bool {
	for _, basePath := range configuredBasePaths() {
		path = strings.TrimPrefix(path, basePath)
	}
	switch path {
	case "/health", "/_/auth/callback", "/api/auth/logout", "/logout":
		return true
	default:
		return false
	}
}

func (h *Handler) requirePermission(w http.ResponseWriter, r *http.Request, permission string) (AuthUserRecord, bool) {
	record, err := h.currentAuthUser(r)
	if err != nil {
		h.writeAuthError(w, r, err)
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
		if err != nil {
			h.writeAuthError(w, r, err)
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

func (a nvrMonitorAuthorizer) RecordAudit(r *http.Request, event auditlog.AuditEvent) error {
	if r == nil {
		return a.handler.recordMonitorAudit(context.Background(), nil, event)
	}
	return a.handler.recordMonitorAudit(r.Context(), r, event)
}

func (a nvrMonitorAuthorizer) CanViewStore(r *http.Request, externalOrgID string) (bool, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return false, nvrMonitorAuthError(err)
	}
	return a.handler.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
}

func (a nvrMonitorAuthorizer) CanBackfillSnapshot(r *http.Request, externalOrgID string) (bool, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return false, nvrMonitorAuthError(err)
	}
	if !hasPermission(user.permissions(), PermissionStoreWrite) {
		return false, nil
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
	if errors.Is(err, errUnauthorizedAuth) || errors.Is(err, errSessionIdleTimeout) || errors.Is(err, errAuthSessionUnavailable) {
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

func (a h5MonitorAuthorizer) RecordAudit(r *http.Request, event auditlog.AuditEvent) error {
	if r == nil {
		return a.handler.recordMonitorAudit(context.Background(), nil, event)
	}
	return a.handler.recordMonitorAudit(r.Context(), r, event)
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
	if errors.Is(err, errUnauthorizedAuth) || errors.Is(err, errSessionIdleTimeout) || errors.Is(err, errAuthSessionUnavailable) {
		return h5monitor.ErrUnauthorized
	}
	if errors.Is(err, errForbiddenAuth) {
		return h5monitor.ErrForbidden
	}
	return err
}

func (h *Handler) recordMonitorAudit(ctx context.Context, r *http.Request, event auditlog.AuditEvent) error {
	if h == nil || h.auditRecorder == nil {
		return errAuditUnavailable
	}

	actor := auditActor{}
	if r != nil {
		user, err := h.currentAuthUser(r)
		if err != nil {
			if event.Result == "success" {
				return err
			}
			user = AuthUserRecord{}
		}
		actor = actorFromAuthUser(user, AuthUserResponse{
			Email:       user.Email,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		})
		if strings.TrimSpace(event.IPAddress) == "" {
			event.IPAddress = requestIPAddress(r)
		}
		if strings.TrimSpace(event.UserAgent) == "" {
			event.UserAgent = strings.TrimSpace(r.UserAgent())
		}
		if strings.TrimSpace(event.RequestID) == "" {
			event.RequestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
		}
	}
	event.UserID = actor.userID
	event.ActorDisplayName = actor.displayName
	event.UserEmail = actor.email
	return h.auditRecorder.RecordAudit(ctx, event)
}
