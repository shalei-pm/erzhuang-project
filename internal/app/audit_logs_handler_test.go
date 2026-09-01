package app

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuditLogsHandlerRequiresAuditViewPermission(t *testing.T) {
	store := NewMemoryStore()
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	for _, user := range []AuthUserRecord{
		{ID: 101, Email: "audit-admin@soyoung.com", Role: RoleAdmin, Enabled: true},
		{ID: 102, Email: "audit-editor@soyoung.com", Role: RoleEditor, Enabled: true},
		{ID: 103, Email: "audit-viewer@soyoung.com", Role: RoleViewer, Enabled: true},
	} {
		if err := store.setAuthUserForTest(user); err != nil {
			t.Fatalf("set auth user %q: %v", user.Email, err)
		}
	}

	handler := NewHandlerWithStore(store)
	path := "/api/admin/audit-logs?start_time=2026-08-01&end_time=2026-08-01"
	tests := []struct {
		name   string
		email  string
		status int
	}{
		{name: "unauthenticated", status: http.StatusUnauthorized},
		{name: "editor", email: "audit-editor@soyoung.com", status: http.StatusForbidden},
		{name: "viewer", email: "audit-viewer@soyoung.com", status: http.StatusForbidden},
		{name: "admin", email: "audit-admin@soyoung.com", status: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			if tt.email != "" {
				addAuditLogTestSSOCookie(t, request, privateKey, tt.email)
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.status, recorder.Body.String())
			}
		})
	}
}

func TestAuditLogsHandlerSupportsConfiguredBasePath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	_, handler, privateKey := newAuditLogTestHandler(t)

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/api/admin/audit-logs?start_time=2026-08-01&end_time=2026-08-01", nil)
	addAuditLogTestSSOCookie(t, request, privateKey, "audit-admin@soyoung.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestAuditLogsHandlerRejectsInvalidQuery(t *testing.T) {
	_, handler, privateKey := newAuditLogTestHandler(t)
	tests := []struct {
		name  string
		query string
	}{
		{name: "missing start time", query: "end_time=2026-08-01"},
		{name: "missing end time", query: "start_time=2026-08-01"},
		{name: "invalid start time", query: "start_time=2026-08-32&end_time=2026-08-31"},
		{name: "invalid end time", query: "start_time=2026-08-01&end_time=not-a-date"},
		{name: "range exceeds three months", query: "start_time=2026-01-01&end_time=2026-04-02"},
		{name: "inverted range", query: "start_time=2026-08-02&end_time=2026-08-01"},
		{name: "zero user id", query: "start_time=2026-08-01&end_time=2026-08-01&user_id=0"},
		{name: "negative user id", query: "start_time=2026-08-01&end_time=2026-08-01&user_id=-1"},
		{name: "non numeric user id", query: "start_time=2026-08-01&end_time=2026-08-01&user_id=abc"},
		{name: "invalid action", query: "start_time=2026-08-01&end_time=2026-08-01&action=DROP+TABLE"},
		{name: "zero page", query: "start_time=2026-08-01&end_time=2026-08-01&page=0"},
		{name: "negative page", query: "start_time=2026-08-01&end_time=2026-08-01&page=-1"},
		{name: "non numeric page", query: "start_time=2026-08-01&end_time=2026-08-01&page=one"},
		{name: "zero page size", query: "start_time=2026-08-01&end_time=2026-08-01&page_size=0"},
		{name: "negative page size", query: "start_time=2026-08-01&end_time=2026-08-01&page_size=-1"},
		{name: "non numeric page size", query: "start_time=2026-08-01&end_time=2026-08-01&page_size=large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?"+tt.query, nil)
			addAuditLogTestSSOCookie(t, request, privateKey, "audit-admin@soyoung.com")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
}

func TestAuditLogsHandlerUsesShanghaiDateBoundary(t *testing.T) {
	store, handler, privateKey := newAuditLogTestHandler(t)
	boundary := []struct {
		actor     string
		createdAt time.Time
	}{
		{actor: "previous-day", createdAt: time.Date(2026, time.August, 30, 15, 59, 59, 0, time.UTC)},
		{actor: "included", createdAt: time.Date(2026, time.August, 30, 16, 0, 0, 0, time.UTC)},
		{actor: "next-day", createdAt: time.Date(2026, time.August, 31, 16, 0, 0, 0, time.UTC)},
	}
	for _, entry := range boundary {
		store.now = func() time.Time { return entry.createdAt }
		if err := store.CreateAuditLog(context.Background(), AuditLog{
			ActorDisplayName: entry.actor,
			Action:           "auth.login",
		}); err != nil {
			t.Fatalf("create %s audit log: %v", entry.actor, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?start_time=2026-08-31&end_time=2026-08-31", nil)
	addAuditLogTestSSOCookie(t, request, privateKey, "audit-admin@soyoung.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Items []struct {
			ActorDisplayName string    `json:"actor_display_name"`
			CreatedAt        time.Time `json:"created_at"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Items) != 1 {
		t.Fatalf("response = %#v, want only the Shanghai midnight entry", response)
	}
	if response.Items[0].ActorDisplayName != "included" || !response.Items[0].CreatedAt.Equal(boundary[1].createdAt) {
		t.Fatalf("item = %#v, want included UTC boundary %#v", response.Items[0], boundary[1])
	}
}

func TestAuditLogsHandlerNormalizesPageSizeAndReturnsSafeFields(t *testing.T) {
	store, handler, privateKey := newAuditLogTestHandler(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai: %v", err)
	}
	createdAt := time.Date(2026, time.August, 31, 23, 59, 59, 0, shanghai)
	store.now = func() time.Time { return createdAt }
	userID := int64(42)
	entityID := int64(7)
	storeID := int64(9)
	channelID := int64(11)
	if err := store.CreateAuditLog(context.Background(), AuditLog{
		UserID:           &userID,
		ActorDisplayName: "审计管理员",
		UserEmail:        "audit-admin@soyoung.com",
		Action:           "user.update",
		EntityType:       "user",
		EntityID:         &entityID,
		StoreID:          &storeID,
		ExternalOrgID:    "org-9",
		ChannelID:        &channelID,
		AssetLogicalKey:  "private/audit-object.jpg",
		IPAddress:        "203.0.113.42",
		UserAgent:        "private user agent",
		RequestID:        "private-request-id",
		Result:           "success",
		DetailJSON:       json.RawMessage(`{"token":"secret-value","summary":"safe audit summary"}`),
	}); err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?start_time=2026-08-31&end_time=2026-08-31&user_id=42&action=user.update&page=1&page_size=101", nil)
	addAuditLogTestSSOCookie(t, request, privateKey, "audit-admin@soyoung.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Items    []map[string]any `json:"items"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Page != 1 || response.PageSize != maxAuditLogPageSize || response.Total != 1 || len(response.Items) != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
	item := response.Items[0]
	if item["summary"] != "safe audit summary" {
		t.Fatalf("summary = %#v, want safe detail summary", item["summary"])
	}
	for _, key := range []string{"actor_display_name", "user_email", "action", "entity_type", "entity_id", "store_id", "external_org_id", "channel_id", "result", "created_at", "summary"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("response item is missing allowed field %q: %#v", key, item)
		}
	}
	for _, key := range []string{"id", "user_id", "detail_json", "ip_address", "user_agent", "request_id", "asset_logical_key"} {
		if _, ok := item[key]; ok {
			t.Fatalf("response item leaks internal field %q: %#v", key, item)
		}
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	for _, value := range []string{"private/audit-object.jpg", "203.0.113.42", "private user agent", "private-request-id", "secret-value"} {
		if strings.Contains(string(encoded), value) {
			t.Fatalf("response leaks internal value %q: %s", value, encoded)
		}
	}
}

func TestAuditLogsHandlerReturnsServerErrorWhenStoreDoesNotSupportAuditLogs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?start_time=2026-08-01&end_time=2026-08-01", nil)
	recorder := httptest.NewRecorder()

	NewHandlerWithStore(failingStore{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "audit log store unavailable") {
		t.Fatalf("expected clear audit store error, got %s", recorder.Body.String())
	}
}

func TestAuditLogsHandlerDoesNotLeakStoreError(t *testing.T) {
	store, _, privateKey := newAuditLogTestHandler(t)
	handler := NewHandlerWithStore(auditLogListErrorStore{MemoryStore: store})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?start_time=2026-08-01&end_time=2026-08-01", nil)
	addAuditLogTestSSOCookie(t, request, privateKey, "audit-admin@soyoung.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "list audit logs failed") {
		t.Fatalf("expected generic list error, got %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "mysql password=not-for-clients") {
		t.Fatalf("response leaks store error detail: %s", recorder.Body.String())
	}
}

func newAuditLogTestHandler(t *testing.T) (*MemoryStore, http.Handler, *rsa.PrivateKey) {
	t.Helper()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID:      101,
		Email:   "audit-admin@soyoung.com",
		Role:    RoleAdmin,
		Enabled: true,
	}); err != nil {
		t.Fatalf("set admin: %v", err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	return store, NewHandlerWithStore(store), privateKey
}

func addAuditLogTestSSOCookie(t *testing.T, request *http.Request, privateKey *rsa.PrivateKey, email string) {
	t.Helper()
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{"mail": email, "username": strings.TrimSuffix(email, "@soyoung.com")},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})})
}

type auditLogListErrorStore struct {
	*MemoryStore
}

func (auditLogListErrorStore) ListAuditLogs(context.Context, AuditLogFilter) (AuditLogPage, error) {
	return AuditLogPage{}, errors.New("mysql password=not-for-clients")
}
