package app

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assetmigration"
)

func TestHealth(t *testing.T) {
	const wantVersion = "v2"

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.App != AppName {
		t.Fatalf("expected app %q, got %q", AppName, response.App)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
	if response.Version != wantVersion {
		t.Fatalf("expected version %q, got %q", wantVersion, response.Version)
	}
	if response.Database != "memory" {
		t.Fatalf("expected database memory, got %q", response.Database)
	}
	if response.AssetStore != "local" {
		t.Fatalf("expected asset store local, got %q", response.AssetStore)
	}
}

func TestHealthDegradedWhenStorePingFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandlerWithStore(failingStore{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Fatalf("expected status degraded, got %q", response.Status)
	}
	if response.Database != "error" {
		t.Fatalf("expected database error, got %q", response.Database)
	}
}

func TestHealthUnderConfiguredBasePath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/health", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.App != AppName {
		t.Fatalf("expected app %q, got %q", AppName, response.App)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
}

func TestTasks(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []Task
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) == 0 {
		t.Fatal("expected at least one task")
	}
}

func TestTasksUnderErzhuangAPIPrefix(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/erzhuang/api/tasks", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []Task
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) == 0 {
		t.Fatal("expected at least one task")
	}
}

func TestTasksUnderConfiguredBasePath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/api/tasks", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []Task
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) == 0 {
		t.Fatal("expected at least one task")
	}
}

func TestServesFrontendIndexUnderErzhuang(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	if err := os.WriteFile(frontendDir+string(os.PathSeparator)+"index.html", []byte("<html>container frontend</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang/", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "container frontend") {
		t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
	}
}

func TestServesFrontendIndexUnderConfiguredBasePath(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	if err := os.WriteFile(frontendDir+string(os.PathSeparator)+"index.html", []byte("<html>project frontend</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "project frontend") {
		t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
	}
}

func TestServesFrontendIndexForNestedRouteUnderConfiguredBasePath(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	if err := os.WriteFile(frontendDir+string(os.PathSeparator)+"index.html", []byte("<html>project nested frontend</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/h5/orgs/10030/monitor", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "project nested frontend") {
		t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
	}
}

func TestServesFrontendAssetUnderErzhuang(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	assetsDir := frontendDir + string(os.PathSeparator) + "assets"
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("create assets dir: %v", err)
	}
	if err := os.WriteFile(assetsDir+string(os.PathSeparator)+"app.js", []byte("console.log('container asset')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang/assets/app.js", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "container asset") {
		t.Fatalf("expected frontend asset body, got %q", recorder.Body.String())
	}
}

func TestAISettingsToggle(t *testing.T) {
	handler := NewHandler()

	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/ai-settings", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, getRecorder.Code)
	}
	var initial AISettings
	if err := json.NewDecoder(getRecorder.Body).Decode(&initial); err != nil {
		t.Fatalf("decode initial settings: %v", err)
	}
	if initial.Provider != "openai" {
		t.Fatalf("expected default openai provider, got %#v", initial)
	}

	toggleRecorder := httptest.NewRecorder()
	handler.ServeHTTP(toggleRecorder, httptest.NewRequest(http.MethodPost, "/api/ai-settings/toggle", nil))
	if toggleRecorder.Code != http.StatusOK {
		t.Fatalf("expected toggle status %d, got %d", http.StatusOK, toggleRecorder.Code)
	}
	var toggled AISettings
	if err := json.NewDecoder(toggleRecorder.Body).Decode(&toggled); err != nil {
		t.Fatalf("decode toggled settings: %v", err)
	}
	if toggled.Provider != "minimax" {
		t.Fatalf("expected minimax provider after toggle, got %#v", toggled)
	}

	secondToggleRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondToggleRecorder, httptest.NewRequest(http.MethodPost, "/api/ai-settings/toggle", nil))
	if secondToggleRecorder.Code != http.StatusOK {
		t.Fatalf("expected second toggle status %d, got %d", http.StatusOK, secondToggleRecorder.Code)
	}
	var secondToggled AISettings
	if err := json.NewDecoder(secondToggleRecorder.Body).Decode(&secondToggled); err != nil {
		t.Fatalf("decode second toggled settings: %v", err)
	}
	if secondToggled.Provider != "openai" {
		t.Fatalf("expected openai provider after second toggle, got %#v", secondToggled)
	}
}

func TestAuthMeDisabledByDefaultAllowsExistingAdmin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["enabled"] != false {
		t.Fatalf("expected disabled auth, got %#v", response)
	}
	if response["authenticated"] != true {
		t.Fatalf("expected existing admin to pass while sso disabled, got %#v", response)
	}
}

func TestAuthMePrefersValidAPISIXSSOJWTWhenCompatibilityModeIsDisabled(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "沙磊",
			"mail":     "shalei@soyoung.com",
			"phone":    "13800138000",
			"username": "shalei",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.User == nil {
		t.Fatalf("expected sso user, got %#v", response)
	}
	if response.User.Email != "shalei@soyoung.com" || response.User.DisplayName != "沙磊" {
		t.Fatalf("expected real sso user instead of local admin, got %#v", response.User)
	}
}

func TestAuthMeRequiresLoginWhenSSOEnabled(t *testing.T) {
	t.Setenv("SSO_ENABLED", "true")

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["enabled"] != true || response["authenticated"] != false {
		t.Fatalf("unexpected auth response: %#v", response)
	}
	if response["login_url"] != "/erzhuang-project/_/auth/callback" {
		t.Fatalf("unexpected login url: %#v", response["login_url"])
	}
}

func TestAuthMeAcceptsValidAPISIXSSOJWT(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	t.Setenv("SSO_EXPECTED_SUB", "lite.sy.soyoung.com")

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":   "四喜（测试）",
			"mail":      "shalei@soyoung.com",
			"open_id":   "ou_test_open_id",
			"user_id":   "feishu_user_id",
			"phone":     "13800112233",
			"username":  "shalei",
			"login_way": "lark",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Authenticated || response.User == nil {
		t.Fatalf("expected authenticated response, got %#v", response)
	}
	if response.User.Email != "shalei@soyoung.com" || response.User.Username != "shalei" || response.User.DisplayName != "四喜（测试）" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
	if response.User.OpenID != "ou_test_open_id" || response.User.FeishuUserID != "feishu_user_id" || response.User.Phone != "13800112233" || response.User.LoginWay != "lark" {
		t.Fatalf("unexpected sso fields: %#v", response.User)
	}
}

func TestAuthMeUsesProvisionedAdminUserFromSSOMail(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":   "沙磊",
			"mail":      "shalei@soyoung.com",
			"phone":     "13800138000",
			"username":  "shalei",
			"login_way": "lark",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.User == nil {
		t.Fatalf("expected auth user, got %#v", response)
	}
	if response.User.Email != "shalei@soyoung.com" || response.User.DisplayName != "沙磊" || response.User.Phone != "13800138000" {
		t.Fatalf("expected sso profile to update provisioned user, got %#v", response.User)
	}
	if response.User.Role != "admin" || !containsString(response.Permissions, "admin") {
		t.Fatalf("expected provisioned admin permissions, got user=%#v permissions=%#v", response.User, response.Permissions)
	}
}

func TestAuthUserPermissionsForAdminEditorViewer(t *testing.T) {
	tests := []struct {
		role string
		want []string
	}{
		{role: "admin", want: []string{"admin", "store:read", "store:write", "user:manage"}},
		{role: "editor", want: []string{"editor", "store:read", "store:write"}},
		{role: "viewer", want: []string{"viewer", "store:read"}},
		{role: "", want: []string{"viewer", "store:read"}},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			record := AuthUserRecord{Role: tt.role, Enabled: true}
			if got := record.permissions(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("permissions()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAuthUsersRequiresAdmin(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "编辑用户",
			"mail":     "changwenxia@soyoung.com",
			"username": "changwenxia",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestListAuthUsersUsesSSOTokenWhileSSODisabled(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "false")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "编辑用户",
			"mail":     "wangxiaofan@soyoung.com",
			"username": "wangxiaofan",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestListAuthUsersReturnsSeededUsersForAdmin(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "沙磊",
			"mail":     "shalei@soyoung.com",
			"username": "shalei",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Users []AuthUserRecord `json:"users"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Users) < 4 {
		t.Fatalf("expected seeded users, got %#v", response.Users)
	}
	if !authUsersContain(response.Users, "maming@soyoung.com", RoleAdmin) {
		t.Fatalf("expected maming admin in %#v", response.Users)
	}
	if !authUsersContain(response.Users, "changwenxia@soyoung.com", RoleEditor) {
		t.Fatalf("expected changwenxia editor in %#v", response.Users)
	}
}

func TestStoreSpaceWriteRequiresStoreWritePermission(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID:       10,
		Email:    "viewer@example.com",
		Username: "viewer",
		Role:     RoleViewer,
		Enabled:  true,
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", strings.NewReader(`{}`))
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "只读用户",
			"mail":     "viewer@example.com",
			"username": "viewer",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandlerWithStore(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}

func TestStoreSpaceWriteAllowsEditorPastPermissionGuard(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", strings.NewReader(`{}`))
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "编辑用户",
			"mail":     "changwenxia@soyoung.com",
			"username": "changwenxia",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code == http.StatusForbidden || recorder.Code == http.StatusUnauthorized {
		t.Fatalf("expected editor to pass permission guard, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAISettingsToggleRequiresAdminPermission(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodPost, "/api/ai-settings/toggle", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "编辑用户",
			"mail":     "wangxiaofan@soyoung.com",
			"username": "wangxiaofan",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}

func TestOSSSmokeEndpointHiddenUnlessOpsEnabled(t *testing.T) {
	called := false
	restore := setOSSSmokeRunnerForTest(func(ctx context.Context) (*ossSmokeResult, error) {
		called = true
		return &ossSmokeResult{Key: "smoke-tests/test.txt", ContentType: "text/plain", Bytes: 12}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/oss-smoke", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("expected disabled ops endpoint not to call smoke runner")
	}
}

func TestOSSSmokeEndpointRunsWhenOpsEnabledForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setOSSSmokeRunnerForTest(func(ctx context.Context) (*ossSmokeResult, error) {
		return &ossSmokeResult{Key: "smoke-tests/test.txt", ContentType: "text/plain; charset=utf-8", Bytes: 24}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/oss-smoke", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["ok"] != true || response["key"] != "smoke-tests/test.txt" || response["bytes"] != float64(24) {
		t.Fatalf("unexpected smoke response: %#v", response)
	}
}

func TestOSSSmokeEndpointAcceptsK8SSecretOpsEnabled(t *testing.T) {
	t.Setenv("K8S_SECRET_OPS_ENABLED", "true")
	restore := setOSSSmokeRunnerForTest(func(ctx context.Context) (*ossSmokeResult, error) {
		return &ossSmokeResult{Key: "smoke-tests/k8s.txt", ContentType: "text/plain; charset=utf-8", Bytes: 32}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/oss-smoke", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestOSSSmokeEndpointPrefersK8SSecretAssetStoreOverRuntimeAssetStore(t *testing.T) {
	t.Setenv("ASSET_STORE", "supabase")
	t.Setenv("K8S_SECRET_ASSET_STORE", "oss")

	if got := opsAssetStoreMode(); got != "oss" {
		t.Fatalf("opsAssetStoreMode()=%q, want oss", got)
	}
}

func TestOpsEnvCheckReturnsSanitizedRuntimeConfig(t *testing.T) {
	t.Setenv("K8S_SECRET_OPS_ENABLED", "true")
	t.Setenv("K8S_SECRET_ASSET_STORE", "oss")
	t.Setenv("K8S_SECRET_OSS_BUCKET", "secret-bucket")
	t.Setenv("K8S_SECRET_OSS_ENDPOINT", "secret-endpoint")
	t.Setenv("K8S_SECRET_OSS_ACCESS_KEY_ID", "secret-id")
	t.Setenv("K8S_SECRET_OSS_ACCESS_KEY_SECRET", "secret-key")

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/env-check", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{"secret-bucket", "secret-endpoint", "secret-id", "secret-key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("expected env check to hide %q, got %s", secret, body)
		}
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["ops_enabled"] != true || response["asset_store"] != "oss" {
		t.Fatalf("unexpected env check response: %#v", response)
	}
	if response["has_oss_bucket"] != true || response["has_oss_endpoint"] != true || response["has_oss_access_key_id"] != true || response["has_oss_access_key_secret"] != true {
		t.Fatalf("expected oss vars to be present, got %#v", response)
	}
}

func TestMySQLCanaryImportEndpointHiddenUnlessOpsEnabled(t *testing.T) {
	called := false
	restore := setMySQLCanaryImportRunnerForTest(func(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error) {
		called = true
		return &mysqlCanaryImportRunResult{}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/mysql-canary-import", strings.NewReader(`{"import_sql":"-- Scope external_org_id: 10030"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("expected disabled ops endpoint not to call runner")
	}
}

func TestMySQLCanaryImportEndpointRunsDryRunForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLCanaryImportRunnerForTest(func(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error) {
		if request.Apply {
			t.Fatal("expected dry-run request")
		}
		if request.ExternalOrgID != "10030" || !strings.Contains(request.ImportSQL, "insert into `tb_stores`") {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &mysqlCanaryImportRunResult{
			Summary: mysqlCanaryImportSummary{
				StoreCount:    1,
				RecorderCount: 1,
				ChannelCount:  4,
				SnapshotCount: 4,
			},
			Warnings: []string{"dry-run did not execute import sql"},
		}, nil
	})
	defer restore()

	body := `{"import_sql":"-- Scope external_org_id: 10030\ninsert into ` + "`tb_stores`" + ` (` + "`external_org_id`" + `) values ('10030');"}`
	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/mysql-canary-import", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mysqlCanaryImportResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Apply || response.Summary.ChannelCount != 4 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestMySQLCanaryImportEndpointRejectsNonCanaryScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLCanaryImportRunnerForTest(func(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error) {
		t.Fatal("invalid import request should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/mysql-canary-import", strings.NewReader(`{"external_org_id":"10047","import_sql":"-- Scope external_org_id: 10047"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestMySQLCanaryImportEndpointRejectsOtherStoreExternalOrgID(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLCanaryImportRunnerForTest(func(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error) {
		t.Fatal("invalid import request should not call runner")
		return nil, nil
	})
	defer restore()

	body := `{"import_sql":"-- Scope external_org_id: 10030\ninsert into ` + "`tb_stores`" + ` (` + "`external_org_id`" + `) values ('10047');"}`
	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/mysql-canary-import", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestMySQLCanaryValidateEndpointRunsReadOnlySummaryForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLCanaryValidateRunnerForTest(func(ctx context.Context, externalOrgID string) (*mysqlCanaryValidateResult, error) {
		if externalOrgID != "10030" {
			t.Fatalf("unexpected external org id %q", externalOrgID)
		}
		return &mysqlCanaryValidateResult{
			Summary: mysqlCanaryImportSummary{
				StoreCount:        1,
				RecorderCount:     1,
				ChannelCount:      4,
				SnapshotCount:     4,
				OperationLogCount: 6,
				UserCount:         6,
			},
			Warnings: []string{"read-only validation"},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-canary-validate?external_org_id=10030", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mysqlCanaryValidateResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.ExternalOrgID != "10030" || response.Summary.ChannelCount != 4 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestMySQLCanaryValidateEndpointRejectsNonCanaryScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLCanaryValidateRunnerForTest(func(ctx context.Context, externalOrgID string) (*mysqlCanaryValidateResult, error) {
		t.Fatal("invalid validate request should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-canary-validate?external_org_id=10047", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestMySQLCanaryValidateEndpointAllowsConfiguredScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	t.Setenv("OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS", "10030,10047")
	restore := setMySQLCanaryValidateRunnerForTest(func(ctx context.Context, externalOrgID string) (*mysqlCanaryValidateResult, error) {
		if externalOrgID != "10047" {
			t.Fatalf("unexpected external org id: %s", externalOrgID)
		}
		return &mysqlCanaryValidateResult{
			Summary: mysqlCanaryImportSummary{StoreCount: 1},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-canary-validate?external_org_id=10047", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestMySQLAssetInventoryEndpointReturnsManifestCSV(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLAssetInventoryRunnerForTest(func(ctx context.Context, request mysqlAssetInventoryRunRequest) (*mysqlAssetInventoryRunResult, error) {
		if request.ExternalOrgID != "10030" {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &mysqlAssetInventoryRunResult{
			Summary:     mysqlAssetInventorySummary{Total: 2, Pending: 1, Skipped: 1, Sensitive: 1, SnapshotRows: 2},
			ManifestCSV: "logical_key,target_oss_key,suggested_migration_status\nchannel-snapshots/a.jpg,channel-snapshots/a.jpg,pending\n",
			Warnings:    []string{"read-only inventory"},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-asset-inventory?external_org_id=10030", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response mysqlAssetInventoryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Summary.Pending != 1 || !strings.Contains(response.ManifestCSV, "channel-snapshots/a.jpg") {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestMySQLAssetInventoryEndpointRejectsNonCanaryApplyScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setMySQLAssetInventoryRunnerForTest(func(ctx context.Context, request mysqlAssetInventoryRunRequest) (*mysqlAssetInventoryRunResult, error) {
		t.Fatal("invalid inventory request should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-asset-inventory?external_org_id=10047", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestMySQLAssetInventoryEndpointAllowsConfiguredScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	t.Setenv("OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS", "10030,10047")
	restore := setMySQLAssetInventoryRunnerForTest(func(ctx context.Context, request mysqlAssetInventoryRunRequest) (*mysqlAssetInventoryRunResult, error) {
		if request.ExternalOrgID != "10047" {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &mysqlAssetInventoryRunResult{
			Summary: mysqlAssetInventorySummary{Total: 1, Pending: 1},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ops/mysql-asset-inventory?external_org_id=10047", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestBuildMySQLAssetInventoryUsesLatestSnapshotPerChannel(t *testing.T) {
	rows := []mysqlAssetInventoryRawRow{
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", ChannelID: "1540", OldPath: "/api/store-space/channel-snapshots/old-1.jpg", SourceKey: "/api/store-space/channel-snapshots/old-1.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", ChannelID: "1540", OldPath: "/api/store-space/channel-snapshots/old-1.jpg", SourceKey: "/api/store-space/channel-snapshots/old-1.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "2", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", ChannelID: "1540", OldPath: "/api/store-space/channel-snapshots/new-1.jpg", SourceKey: "/api/store-space/channel-snapshots/new-1.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "2", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", ChannelID: "1540", OldPath: "/api/store-space/channel-snapshots/new-1.jpg", SourceKey: "/api/store-space/channel-snapshots/new-1.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "3", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", ChannelID: "1541", OldPath: "/api/store-space/channel-snapshots/old-2.jpg", SourceKey: "/api/store-space/channel-snapshots/old-2.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "3", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", ChannelID: "1541", OldPath: "/api/store-space/channel-snapshots/old-2.jpg", SourceKey: "/api/store-space/channel-snapshots/old-2.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "4", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", ChannelID: "1541", OldPath: "/api/store-space/channel-snapshots/new-2.jpg", SourceKey: "/api/store-space/channel-snapshots/new-2.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "4", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", ChannelID: "1541", OldPath: "/api/store-space/channel-snapshots/new-2.jpg", SourceKey: "/api/store-space/channel-snapshots/new-2.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
	}
	result, err := buildMySQLAssetInventory(rows, nil)
	if err != nil {
		t.Fatalf("buildMySQLAssetInventory: %v", err)
	}
	if result.Summary.Total != 4 || result.Summary.SnapshotRows != 4 || result.Summary.Pending != 4 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if strings.Contains(result.ManifestCSV, "old-1.jpg") || strings.Contains(result.ManifestCSV, "old-2.jpg") {
		t.Fatalf("manifest should not include stale snapshots:\n%s", result.ManifestCSV)
	}
	if !strings.Contains(result.ManifestCSV, "new-1.jpg") || !strings.Contains(result.ManifestCSV, "new-2.jpg") {
		t.Fatalf("manifest should include latest snapshots:\n%s", result.ManifestCSV)
	}
}

func TestBuildMySQLAssetInventorySkipsAlreadyMigratedAssets(t *testing.T) {
	rows := []mysqlAssetInventoryRawRow{
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", OldPath: "/api/store-space/channel-snapshots/a.jpg", SourceKey: "/api/store-space/channel-snapshots/a.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", OldPath: "/api/store-space/channel-snapshots/a.jpg", SourceKey: "/api/store-space/channel-snapshots/a.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
	}
	result, err := buildMySQLAssetInventory(rows, map[string]mysqlAssetState{
		"channel-snapshots/a.jpg": {
			MigrationStatus: "migrated",
			StorageProvider: "oss",
			Bucket:          "sy-camera-erzhuang-project",
			StorageKey:      "channel-snapshots/a.jpg",
		},
	})
	if err != nil {
		t.Fatalf("buildMySQLAssetInventory: %v", err)
	}
	if result.Summary.Total != 2 || result.Summary.Pending != 0 || result.Summary.Skipped != 2 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if strings.Contains(result.ManifestCSV, ",pending,") {
		t.Fatalf("already migrated manifest rows should not remain pending:\n%s", result.ManifestCSV)
	}
	if !strings.Contains(result.ManifestCSV, "already_migrated") {
		t.Fatalf("manifest should explain migrated skip reason:\n%s", result.ManifestCSV)
	}
}

func TestBuildMySQLAssetInventoryNormalizesSnapshotProxyPathsAndDuplicates(t *testing.T) {
	rows := []mysqlAssetInventoryRawRow{
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", OldPath: "/api/store-space/channel-snapshots/a.jpg", SourceKey: "/api/store-space/channel-snapshots/a.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "1", SourceColumn: "full_image_path", AssetRole: "snapshot_full_image", ExternalOrgID: "10030", OldPath: "/api/store-space/channel-snapshots/a.jpg", SourceKey: "/api/store-space/channel-snapshots/a.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
		{SourceTable: "tb_channel_snapshots", SourceID: "2", SourceColumn: "thumbnail_path", AssetRole: "snapshot_thumbnail", ExternalOrgID: "10030", OldPath: "https://example.com/signed.jpg", SourceKey: "https://example.com/signed.jpg", ExpectedContentType: "image/jpeg", Sensitivity: "sensitive"},
	}
	result, err := buildMySQLAssetInventory(rows, nil)
	if err != nil {
		t.Fatalf("buildMySQLAssetInventory: %v", err)
	}
	if result.Summary.Total != 3 || result.Summary.Pending != 2 || result.Summary.Skipped != 1 || result.Summary.DuplicateRefs != 2 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if !strings.Contains(result.ManifestCSV, "channel-snapshots/a.jpg") || !strings.Contains(result.ManifestCSV, "remote_http_url") {
		t.Fatalf("unexpected manifest csv:\n%s", result.ManifestCSV)
	}
}

func TestNormalizeOpsExportOrgIDs(t *testing.T) {
	got := normalizeOpsExportOrgIDs("10047, 10030,10047,,")
	want := []string{"10047", "10030"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeOpsExportOrgIDs()=%v, want %v", got, want)
	}
}

func TestOSSSmokeEndpointRequiresAdminPermission(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("OPS_ENABLED", "true")
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	restore := setOSSSmokeRunnerForTest(func(ctx context.Context) (*ossSmokeResult, error) {
		t.Fatal("viewer should not call smoke runner")
		return nil, nil
	})
	defer restore()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID:      99,
		Email:   "viewer@example.com",
		Role:    RoleViewer,
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/oss-smoke", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display": "只读用户",
			"mail":    "viewer@example.com",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandlerWithStore(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}

func TestOSSSmokeEndpointSanitizesFailureDetails(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setOSSSmokeRunnerForTest(func(ctx context.Context) (*ossSmokeResult, error) {
		return nil, errors.New("Authorization=abc OSS_ACCESS_KEY_SECRET=very-secret Signature=bad StringToSign=hidden endpoint failed")
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/oss-smoke", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadGateway, recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, sensitive := range []string{"Authorization", "OSS_ACCESS_KEY_SECRET", "very-secret", "Signature", "StringToSign"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("expected response to redact %q, got %s", sensitive, body)
		}
	}
	if !strings.Contains(body, "oss smoke failed") {
		t.Fatalf("expected generic smoke failure body, got %s", body)
	}
}

func TestAssetMigrationEndpointHiddenUnlessOpsEnabled(t *testing.T) {
	called := false
	restore := setAssetMigrationRunnerForTest(func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
		called = true
		return &assetMigrationRunResult{}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-migrate", strings.NewReader(`{"manifest_csv":"x"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("expected disabled ops endpoint not to call migration runner")
	}
}

func TestAssetMigrationEndpointRunsDryRunForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setAssetMigrationRunnerForTest(func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
		if request.Apply {
			t.Fatal("expected dry-run request")
		}
		if request.ExternalOrgID != "10030" || request.MaxRows != defaultOpsMigrationMaxRows {
			t.Fatalf("unexpected request defaults: %#v", request)
		}
		return &assetMigrationRunResult{
			Summary: assetmigration.Summary{Total: 2, WouldCopy: 1, Skipped: 1},
			Results: []assetmigration.RowResult{
				{
					Action: "would_copy",
					Row: assetmigration.ManifestRow{
						ExternalOrgID: "10030",
						LogicalKey:    "channel-snapshots/sample.jpg",
						TargetOSSKey:  "channel-snapshots/sample.jpg",
					},
				},
				{
					Action: "skipped",
					Error:  "duplicate_logical_key",
					Row: assetmigration.ManifestRow{
						ExternalOrgID:  "10030",
						LogicalKey:     "channel-snapshots/sample.jpg",
						TargetOSSKey:   "channel-snapshots/sample.jpg",
						LogicalKeyRank: 2,
					},
				},
			},
			ResultCSV: "action,external_org_id,logical_key,target_oss_key,bytes,content_type,error\nwould_copy,10030,channel-snapshots/sample.jpg,channel-snapshots/sample.jpg,0,,\n",
			Warnings:  []string{"check pending rows"},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-migrate", strings.NewReader(`{"manifest_csv":"logical_key,target_oss_key,suggested_migration_status\nchannel-snapshots/sample.jpg,channel-snapshots/sample.jpg,pending\n"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response assetMigrationResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Apply || response.Summary.WouldCopy != 1 || len(response.Results) != 2 {
		t.Fatalf("unexpected migration response: %#v", response)
	}
	if response.ResultSQL != "" {
		t.Fatalf("dry-run should not return result SQL: %s", response.ResultSQL)
	}
}

func TestAssetMigrationEndpointLimitsApplyScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setAssetMigrationRunnerForTest(func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
		t.Fatal("invalid apply request should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-migrate", strings.NewReader(`{"manifest_csv":"logical_key,target_oss_key,suggested_migration_status\nx,x,pending\n","external_org_id":"10047","apply":true}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestAssetMigrationEndpointAllowsConfiguredApplyScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	t.Setenv("OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS", "10030,10047")
	restore := setAssetMigrationRunnerForTest(func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
		if request.ExternalOrgID != "10047" || !request.Apply {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &assetMigrationRunResult{
			Summary:   assetmigration.Summary{Total: 1, Copied: 1},
			ResultCSV: "action,external_org_id,logical_key,target_oss_key,bytes,content_type,error\ncopied,10047,x,x,1,image/jpeg,\n",
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-migrate", strings.NewReader(`{"manifest_csv":"logical_key,target_oss_key,suggested_migration_status\nx,x,pending\n","external_org_id":"10047","apply":true}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestAssetMigrationEndpointReturnsSanitizedApplySQL(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setAssetMigrationRunnerForTest(func(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
		if !request.Apply || request.BatchID != "stage-a-test" {
			t.Fatalf("unexpected apply request: %#v", request)
		}
		return &assetMigrationRunResult{
			Summary: assetmigration.Summary{Total: 1, Copied: 1},
			Results: []assetmigration.RowResult{
				{
					Action:      "copied",
					Bytes:       12,
					ContentType: "image/jpeg",
					Row: assetmigration.ManifestRow{
						ExternalOrgID: "10030",
						LogicalKey:    "channel-snapshots/sample.jpg",
						TargetOSSKey:  "channel-snapshots/sample.jpg",
					},
				},
			},
			ResultCSV: "action,external_org_id,logical_key,target_oss_key,bytes,content_type,error\ncopied,10030,channel-snapshots/sample.jpg,channel-snapshots/sample.jpg,12,image/jpeg,\n",
			ResultSQL: "update tb_asset_objects set storage_provider = 'oss' where logical_key_hash = sha2('channel-snapshots/sample.jpg', 256);\n",
		}, nil
	})
	defer restore()

	body := fmt.Sprintf(`{"manifest_csv":%q,"apply":true,"batch_id":"stage-a-test"}`, "logical_key,target_oss_key,suggested_migration_status\nchannel-snapshots/sample.jpg,channel-snapshots/sample.jpg,pending\n")
	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-migrate", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	responseBody := recorder.Body.String()
	for _, sensitive := range []string{"Authorization", "OSS_ACCESS_KEY_SECRET", "very-secret", "Signature", "StringToSign"} {
		if strings.Contains(responseBody, sensitive) {
			t.Fatalf("expected response to redact %q, got %s", sensitive, responseBody)
		}
	}
	if !strings.Contains(responseBody, "update tb_asset_objects") {
		t.Fatalf("expected result SQL in response, got %s", responseBody)
	}
}

func TestAssetStateBackfillEndpointUpsertsCopiedRows(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setAssetStateBackfillRunnerForTest(func(ctx context.Context, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error) {
		if request.ExternalOrgID != "10030" || request.BatchID != "canary-test" {
			t.Fatalf("unexpected request: %#v", request)
		}
		if !strings.Contains(request.ManifestCSV, "channel-snapshots/sample.jpg") || !strings.Contains(request.ResultCSV, "copied") {
			t.Fatalf("unexpected payload: %#v", request)
		}
		return &assetStateBackfillRunResult{
			Summary:  assetStateBackfillSummary{Total: 1, Migrated: 1, Upserted: 1},
			Warnings: []string{"idempotent backfill"},
		}, nil
	})
	defer restore()

	body := `{
		"external_org_id":"10030",
		"batch_id":"canary-test",
		"manifest_csv":"logical_key,target_oss_key,suggested_migration_status,logical_key_rank\nchannel-snapshots/sample.jpg,channel-snapshots/sample.jpg,pending,1\n",
		"result_csv":"action,external_org_id,logical_key,target_oss_key,bytes,content_type,error\ncopied,10030,channel-snapshots/sample.jpg,channel-snapshots/sample.jpg,12,image/jpeg,\n"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-state-backfill", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response assetStateBackfillResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Summary.Migrated != 1 || response.Summary.Upserted != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestAssetStateBackfillEndpointLimitsScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setAssetStateBackfillRunnerForTest(func(ctx context.Context, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error) {
		t.Fatal("invalid backfill request should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-state-backfill", strings.NewReader(`{"external_org_id":"10047","manifest_csv":"x","result_csv":"x"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestAssetStateBackfillEndpointAllowsConfiguredScope(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	t.Setenv("OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS", "10030,10047")
	restore := setAssetStateBackfillRunnerForTest(func(ctx context.Context, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error) {
		if request.ExternalOrgID != "10047" {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &assetStateBackfillRunResult{
			Summary: assetStateBackfillSummary{Total: 1, Migrated: 1, Upserted: 1},
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/asset-state-backfill", strings.NewReader(`{"external_org_id":"10047","manifest_csv":"logical_key,target_oss_key,suggested_migration_status,logical_key_rank\nx,x,pending,1\n","result_csv":"action,external_org_id,logical_key,target_oss_key,bytes,content_type,error\ncopied,10047,x,x,1,image/jpeg,\n"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestStageASourceSampleEndpointHiddenUnlessOpsEnabled(t *testing.T) {
	called := false
	restore := setStageASourceSampleRunnerForTest(func(ctx context.Context, action string) (*stageASourceSampleResult, error) {
		called = true
		return &stageASourceSampleResult{}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-source-sample", strings.NewReader(`{"action":"seed"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("expected disabled ops endpoint not to call source sample runner")
	}
}

func TestStageASourceSampleEndpointSeedsFixedSampleForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setStageASourceSampleRunnerForTest(func(ctx context.Context, action string) (*stageASourceSampleResult, error) {
		if action != "seed" {
			t.Fatalf("unexpected action %q", action)
		}
		return &stageASourceSampleResult{
			Key:         "channel-snapshots/stage-a-10030-channel-1.jpg",
			Action:      "seeded",
			Bytes:       128,
			ContentType: "image/jpeg",
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-source-sample", strings.NewReader(`{"action":"seed"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response stageASourceSampleResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Action != "seeded" || response.Key != "channel-snapshots/stage-a-10030-channel-1.jpg" || response.Bytes != 128 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestStageASourceSampleEndpointCleansFixedSampleForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setStageASourceSampleRunnerForTest(func(ctx context.Context, action string) (*stageASourceSampleResult, error) {
		if action != "cleanup" {
			t.Fatalf("unexpected action %q", action)
		}
		return &stageASourceSampleResult{
			Key:    "channel-snapshots/stage-a-10030-channel-1.jpg",
			Action: "cleaned",
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-source-sample", strings.NewReader(`{"action":"cleanup"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response stageASourceSampleResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Action != "cleaned" || response.Key != "channel-snapshots/stage-a-10030-channel-1.jpg" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestStageASourceSampleEndpointRejectsUnknownAction(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setStageASourceSampleRunnerForTest(func(ctx context.Context, action string) (*stageASourceSampleResult, error) {
		t.Fatal("invalid action should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-source-sample", strings.NewReader(`{"action":"full-migrate"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestStageATargetSampleEndpointHiddenUnlessOpsEnabled(t *testing.T) {
	called := false
	restore := setStageATargetSampleRunnerForTest(func(ctx context.Context) (*stageATargetSampleResult, error) {
		called = true
		return &stageATargetSampleResult{}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-target-sample", strings.NewReader(`{"action":"cleanup"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("expected disabled ops endpoint not to call target sample runner")
	}
}

func TestStageATargetSampleEndpointCleansFixedOSSSampleForAdmin(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setStageATargetSampleRunnerForTest(func(ctx context.Context) (*stageATargetSampleResult, error) {
		return &stageATargetSampleResult{
			Action: "cleaned",
			Key:    "channel-snapshots/stage-a-10030-channel-1.jpg",
		}, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-target-sample", strings.NewReader(`{"action":"cleanup"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response stageATargetSampleResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || response.Action != "cleaned" || response.Key != "channel-snapshots/stage-a-10030-channel-1.jpg" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestStageATargetSampleEndpointRejectsUnknownAction(t *testing.T) {
	t.Setenv("OPS_ENABLED", "true")
	restore := setStageATargetSampleRunnerForTest(func(ctx context.Context) (*stageATargetSampleResult, error) {
		t.Fatal("invalid action should not call runner")
		return nil, nil
	})
	defer restore()

	request := httptest.NewRequest(http.MethodPost, "/api/admin/ops/stage-a-target-sample", strings.NewReader(`{"action":"seed"}`))
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestAuthMeRejectsUnprovisionedSSOUser(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "未授权用户",
			"mail":     "unknown@soyoung.com",
			"username": "unknown",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	var response AuthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Authenticated || response.User != nil {
		t.Fatalf("expected rejected auth response, got %#v", response)
	}
}

func TestAuthMeRejectsInvalidAPISIXSSOSignature(t *testing.T) {
	privateKey := newTestRSAKey(t)
	otherKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, otherKey, map[string]any{
		"data": map[string]any{"mail": "sixi@soyoung.com"},
		"exp":  time.Now().Add(time.Hour).Unix(),
		"sub":  "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAuthMeRejectsExpiredAPISIXSSOJWT(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{"mail": "sixi@soyoung.com"},
		"exp":  time.Now().Add(-time.Minute).Unix(),
		"sub":  "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAuthMeRejectsAPISIXSSOJWTWithoutMail(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{"username": "sixi"},
		"exp":  time.Now().Add(time.Hour).Unix(),
		"sub":  "lite.sy.soyoung.com",
	})})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAPISIXSSOCallbackUnderConfiguredBasePathRedirectsHome(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/_/auth/callback", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
}

func TestAPISIXSSOLogoutGetUnderConfiguredBasePathRedirectsHome(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/logout", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: "token-value"})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected logout to clear one cookie, got %#v", cookies)
	}
	if cookies[0].Name != "sy_sso_token" || cookies[0].MaxAge != -1 {
		t.Fatalf("expected cleared sso cookie, got %#v", cookies[0])
	}
}

func TestAuthLogoutPostKeepsJSONResponse(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response map[string]bool
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response["ok"] {
		t.Fatalf("unexpected logout response: %#v", response)
	}
}

type failingStore struct{}

func (failingStore) Name() string {
	return "failing"
}

func (failingStore) Ping(ctx context.Context) error {
	return errors.New("ping failed")
}

func (failingStore) ListTasks(ctx context.Context) ([]Task, error) {
	return nil, errors.New("list failed")
}

func (failingStore) GetAIProvider(ctx context.Context) (string, error) {
	return "", errors.New("settings failed")
}

func (failingStore) SetAIProvider(ctx context.Context, provider string) error {
	return errors.New("settings failed")
}

func (failingStore) GetAuthUserByEmail(ctx context.Context, email string) (AuthUserRecord, error) {
	return AuthUserRecord{}, errors.New("auth user failed")
}

func (failingStore) UpdateAuthUserProfile(ctx context.Context, patch AuthUserPatch) (AuthUserRecord, error) {
	return AuthUserRecord{}, errors.New("auth user failed")
}

func (failingStore) ListAuthUsers(ctx context.Context) ([]AuthUserRecord, error) {
	return nil, errors.New("auth user failed")
}

func (failingStore) CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error) {
	return AuthUserRecord{}, errors.New("auth user failed")
}

func (failingStore) UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error) {
	return AuthUserRecord{}, errors.New("auth user failed")
}

func newTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func publicKeyPEM(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func signAPISIXSSOToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal jwt header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal jwt claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func authUsersContain(users []AuthUserRecord, email string, role string) bool {
	for _, user := range users {
		if user.Email == email && user.Role == role {
			return true
		}
	}
	return false
}

func setOSSSmokeRunnerForTest(runner ossSmokeRunner) func() {
	previous := currentOSSSmokeRunner
	currentOSSSmokeRunner = runner
	return func() {
		currentOSSSmokeRunner = previous
	}
}

func setAssetMigrationRunnerForTest(runner assetMigrationRunner) func() {
	previous := currentAssetMigrationRunner
	currentAssetMigrationRunner = runner
	return func() {
		currentAssetMigrationRunner = previous
	}
}

func setAssetStateBackfillRunnerForTest(runner assetStateBackfillRunner) func() {
	previous := currentAssetStateBackfillRunner
	currentAssetStateBackfillRunner = runner
	return func() {
		currentAssetStateBackfillRunner = previous
	}
}

func setStageASourceSampleRunnerForTest(runner stageASourceSampleRunner) func() {
	previous := currentStageASourceSampleRunner
	currentStageASourceSampleRunner = runner
	return func() {
		currentStageASourceSampleRunner = previous
	}
}

func setStageATargetSampleRunnerForTest(runner stageATargetSampleRunner) func() {
	previous := currentStageATargetSampleRunner
	currentStageATargetSampleRunner = runner
	return func() {
		currentStageATargetSampleRunner = previous
	}
}

func setMySQLCanaryImportRunnerForTest(runner mysqlCanaryImportRunner) func() {
	previous := currentMySQLCanaryImportRunner
	currentMySQLCanaryImportRunner = runner
	return func() {
		currentMySQLCanaryImportRunner = previous
	}
}

func setMySQLCanaryValidateRunnerForTest(runner mysqlCanaryValidateRunner) func() {
	previous := currentMySQLCanaryValidateRunner
	currentMySQLCanaryValidateRunner = runner
	return func() {
		currentMySQLCanaryValidateRunner = previous
	}
}

func setMySQLAssetInventoryRunnerForTest(runner mysqlAssetInventoryRunner) func() {
	previous := currentMySQLAssetInventoryRunner
	currentMySQLAssetInventoryRunner = runner
	return func() {
		currentMySQLAssetInventoryRunner = previous
	}
}
