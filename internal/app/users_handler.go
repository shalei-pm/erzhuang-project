package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type authUsersResponse struct {
	Users []authUserItemResponse `json:"users"`
}

type authUserItemResponse struct {
	ID                     int64                       `json:"id"`
	Email                  string                      `json:"email"`
	Username               string                      `json:"username"`
	DisplayName            string                      `json:"display_name"`
	Role                   string                      `json:"role"`
	Enabled                bool                        `json:"enabled"`
	LastLoginAt            *string                     `json:"last_login_at,omitempty"`
	MonitorStoreScopeCount int                         `json:"monitor_store_scope_count"`
	MonitorStoreScopes     []monitorStoreScopeResponse `json:"monitor_store_scopes,omitempty"`
}

type monitorStoreScopeResponse struct {
	StoreID       int64  `json:"store_id"`
	City          string `json:"city"`
	Name          string `json:"name"`
	ExternalOrgID string `json:"external_org_id"`
}

type monitorStoreScopeCandidatesResponse struct {
	Stores []monitorStoreScopeResponse `json:"stores"`
}

func (h *Handler) listUsersHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	users, err := h.store.ListAuthUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list auth users failed"})
		return
	}
	writeJSON(w, http.StatusOK, authUsersResponse{Users: authUserItemsResponse(users)})
}

func (h *Handler) createUserHandler(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.requirePermission(w, r, PermissionUserManage)
	if !ok {
		return
	}
	var input AuthUserMutation
	if !decodeAuthUserMutation(w, r, &input, true) {
		return
	}
	event := newUserMutationAuditEvent(r, operator, "user.create", "success", 0, input, 0)
	user, err := h.createAuthUserWithAudit(r.Context(), input, event)
	if err != nil {
		h.recordUserMutationFailure(r, operator, "user.create", 0, input, 0)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, authUserItem(user))
}

func (h *Handler) listMonitorStoreScopeCandidatesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	stores, err := h.store.ListMonitorStoreScopeCandidates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list monitor store scope candidates failed"})
		return
	}
	writeJSON(w, http.StatusOK, monitorStoreScopeCandidatesResponse{Stores: monitorStoreScopesResponse(stores)})
}

func (h *Handler) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.requirePermission(w, r, PermissionUserManage)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var input AuthUserMutation
	if !decodeAuthUserMutation(w, r, &input, false) {
		return
	}
	event := newUserMutationAuditEvent(r, operator, "user.update", "success", id, input, len(input.MonitorStoreScopeIDs))
	user, err := h.updateAuthUserWithAudit(r.Context(), id, input, event)
	if errors.Is(err, errAuthUserNotFound) {
		h.recordUserMutationFailure(r, operator, "user.update", id, input, len(input.MonitorStoreScopeIDs))
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if err != nil {
		h.recordUserMutationFailure(r, operator, "user.update", id, input, len(input.MonitorStoreScopeIDs))
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authUserItem(user))
}

func (h *Handler) createAuthUserWithAudit(ctx context.Context, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	if h.auditRecorder == nil {
		return h.store.CreateAuthUser(ctx, input)
	}
	store, ok := h.store.(AuthUserMutationAuditStore)
	if !ok {
		return AuthUserRecord{}, errAuditMutationUnavailable
	}
	return store.CreateAuthUserWithAudit(ctx, input, event)
}

func (h *Handler) updateAuthUserWithAudit(ctx context.Context, id int64, input AuthUserMutation, event auditlog.AuditEvent) (AuthUserRecord, error) {
	if h.auditRecorder == nil {
		return h.store.UpdateAuthUser(ctx, id, input)
	}
	store, ok := h.store.(AuthUserMutationAuditStore)
	if !ok {
		return AuthUserRecord{}, errAuditMutationUnavailable
	}
	return store.UpdateAuthUserWithAudit(ctx, id, input, event)
}

func newUserMutationAuditEvent(r *http.Request, operator AuthUserRecord, action, result string, targetID int64, input AuthUserMutation, scopeCount int) auditlog.AuditEvent {
	if scopeCount == 0 {
		scopeCount = len(input.MonitorStoreScopeIDs)
	}
	detail, _ := json.Marshal(map[string]any{
		"summary":     action,
		"source":      "user_management",
		"role":        normalizeRole(input.Role),
		"scope_count": scopeCount,
		"target_name": firstNonEmpty(input.DisplayName, input.Username),
	})
	event := auditlog.AuditEvent{
		UserID:           int64Pointer(operator.ID),
		ActorDisplayName: firstNonEmpty(operator.DisplayName, operator.Username, operator.Email),
		UserEmail:        operator.Email,
		Action:           action,
		EntityType:       "user",
		IPAddress:        requestIPAddress(r),
		UserAgent:        strings.TrimSpace(r.UserAgent()),
		RequestID:        strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Result:           result,
		DetailJSON:       detail,
	}
	if targetID > 0 {
		event.EntityID = int64Pointer(targetID)
	}
	return event
}

func (h *Handler) recordUserMutationFailure(r *http.Request, operator AuthUserRecord, action string, targetID int64, input AuthUserMutation, scopeCount int) {
	if h.auditRecorder == nil {
		return
	}
	event := newUserMutationAuditEvent(r, operator, action, "failed", targetID, input, scopeCount)
	if err := h.auditRecorder.RecordAudit(r.Context(), event); err != nil {
		// The original mutation error is returned to the caller; keep recorder
		// failures out of the response and rely on server logs/metrics later.
	}
}

func decodeAuthUserMutation(w http.ResponseWriter, r *http.Request, input *AuthUserMutation, requireEmail bool) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return false
	}
	input.Email = normalizeEmail(input.Email)
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Role = normalizeRole(input.Role)
	if requireEmail && input.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return false
	}
	if input.Username == "" && input.Email != "" {
		input.Username = strings.Split(input.Email, "@")[0]
	}
	return true
}

func authUserItemsResponse(users []AuthUserRecord) []authUserItemResponse {
	result := make([]authUserItemResponse, 0, len(users))
	for _, user := range users {
		result = append(result, authUserItem(user))
	}
	return result
}

func authUserItem(user AuthUserRecord) authUserItemResponse {
	var lastLoginAt *string
	if user.LastLoginAt != nil {
		value := user.LastLoginAt.Format("2006-01-02T15:04:05Z07:00")
		lastLoginAt = &value
	}
	return authUserItemResponse{
		ID:                     user.ID,
		Email:                  user.Email,
		Username:               user.Username,
		DisplayName:            user.DisplayName,
		Role:                   normalizeRole(user.Role),
		Enabled:                user.Enabled,
		LastLoginAt:            lastLoginAt,
		MonitorStoreScopeCount: user.MonitorStoreScopeCount,
		MonitorStoreScopes:     monitorStoreScopesResponse(user.MonitorStoreScopes),
	}
}

func monitorStoreScopesResponse(scopes []AuthUserResourceScope) []monitorStoreScopeResponse {
	if len(scopes) == 0 {
		return nil
	}
	result := make([]monitorStoreScopeResponse, 0, len(scopes))
	for _, scope := range scopes {
		result = append(result, monitorStoreScopeResponse{
			StoreID:       scope.StoreID,
			City:          scope.City,
			Name:          scope.Name,
			ExternalOrgID: scope.ExternalOrgID,
		})
	}
	return result
}
