package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input AuthUserMutation
	if !decodeAuthUserMutation(w, r, &input, true) {
		return
	}
	user, err := h.store.CreateAuthUser(r.Context(), input)
	if err != nil {
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
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
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
	user, err := h.store.UpdateAuthUser(r.Context(), id, input)
	if errors.Is(err, errAuthUserNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authUserItem(user))
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
