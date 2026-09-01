package app

import (
	"bytes"
	"context"
	"crypto/rsa"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

func TestAPISIXSSOLogoutGetCanRedirectToGatewayLogoutAfterClearingCookies(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	gatewayLogout := "https://security-test.sy.soyoung.com/api/g/sso/logouttogether?from_host=lite.sy.soyoung.com&from_uri=https%3A%2F%2Flite.sy.soyoung.com%2Ferzhuang-project%2F"
	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/logout?redirect="+url.QueryEscape(gatewayLogout), nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: "token-value"})
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != gatewayLogout {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
	cookies := recorder.Result().Cookies()
	if !hasClearedAuthCookie(cookies, "sy_sso_token", "") {
		t.Fatalf("expected logout to clear host-only sso cookie, got %#v", cookies)
	}
	if !hasClearedAuthCookie(cookies, "sy_sso_token", "sy.soyoung.com") {
		t.Fatalf("expected logout to clear parent-domain sso cookie, got %#v", cookies)
	}
}

func TestAPISIXSSOLogoutGetRejectsUnsafeRedirect(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	t.Setenv("SSO_ENABLED", "true")

	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/logout?redirect="+url.QueryEscape("https://example.com/logout"), nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected redirect location: %s", recorder.Header().Get("Location"))
	}
}

func TestAPISIXSSOLogoutRecordsServerSideActorSnapshot(t *testing.T) {
	store, handler, privateKey := newAuthAuditTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "https://lite.sy.soyoung.com/api/auth/logout", nil)
	request.RemoteAddr = "203.0.113.42:54321"
	request.Header.Set("User-Agent", "audit-test-agent/1.0")
	request.Header.Set("X-Request-ID", "request-logout-123")
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{
			"mail":     "logout@example.com",
			"username": "claims-name",
			"display":  "Claims Name",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
	})})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	logs := readAuthAuditLogs(t, store)
	if len(logs) != 1 {
		t.Fatalf("expected one audit log, got %#v", logs)
	}
	event := logs[0]
	if event.Action != "auth.logout" || event.Result != "success" {
		t.Fatalf("unexpected logout event: %#v", event)
	}
	if event.UserID == nil || *event.UserID != 42 {
		t.Fatalf("expected server-side user id 42, got %#v", event.UserID)
	}
	if event.ActorDisplayName != "服务端姓名" || event.UserEmail != "logout@example.com" {
		t.Fatalf("unexpected actor snapshot: %#v", event)
	}
	if event.IPAddress != "203.0.113.42" || event.UserAgent != "audit-test-agent/1.0" || event.RequestID != "request-logout-123" {
		t.Fatalf("unexpected request snapshot: %#v", event)
	}
}

func TestAPISIXSSOLogoutWithoutVerifiedIdentityDoesNotRecordSuccess(t *testing.T) {
	store, handler, _ := newAuthAuditTestHandler(t)
	for _, token := range []string{"", "invalid-token"} {
		request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/logout?redirect="+url.QueryEscape("https://security-test.sy.soyoung.com/api/g/sso/logouttogether"), nil)
		if token != "" {
			request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: token})
		}
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusFound {
			t.Fatalf("token %q: expected status %d, got %d", token, http.StatusFound, recorder.Code)
		}
		if recorder.Header().Get("Location") != "https://security-test.sy.soyoung.com/api/g/sso/logouttogether" {
			t.Fatalf("token %q: unexpected redirect location: %s", token, recorder.Header().Get("Location"))
		}
	}
	logs := readAuthAuditLogs(t, store)
	if len(logs) != 2 {
		t.Fatalf("expected two anonymous logout audit logs, got %#v", logs)
	}
	for _, event := range logs {
		if event.Action != "auth.logout" || (event.Result != "denied" && event.Result != "failed") {
			t.Fatalf("unexpected anonymous logout event: %#v", event)
		}
		if event.UserID != nil || event.UserEmail != "" || event.ActorDisplayName != "" {
			t.Fatalf("anonymous logout event identified an actor: %#v", event)
		}
	}
}

func TestAPISIXSSOLogoutRecordsDeniedForDisabledOrMissingUser(t *testing.T) {
	store, handler, privateKey := newAuthAuditTestHandler(t)
	for _, email := range []string{"disabled@example.com", "missing@example.com"} {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
			"data": map[string]string{"mail": email, "display": "Claims Name"},
			"exp":  time.Now().Add(time.Hour).Unix(),
		})})
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)
	}
	logs := readAuthAuditLogs(t, store)
	if len(logs) != 2 {
		t.Fatalf("expected two denied audit logs, got %#v", logs)
	}
	for _, event := range logs {
		if event.Action != "auth.logout" || event.Result != "denied" {
			t.Fatalf("expected denied logout event, got %#v", event)
		}
	}
}

func TestAuthCallbackRecordsLoginForEnabledServerSideUserAndRedirects(t *testing.T) {
	store, handler, privateKey := newAuthAuditTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/_/auth/callback", nil)
	request.RemoteAddr = "203.0.113.42:54321"
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{"mail": "logout@example.com", "display": "Claims Name"},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected callback redirect: status=%d location=%s", recorder.Code, recorder.Header().Get("Location"))
	}
	logs := readAuthAuditLogs(t, store)
	if len(logs) != 1 || logs[0].Action != "auth.login" || logs[0].Result != "success" {
		t.Fatalf("expected one successful login audit log, got %#v", logs)
	}
	if logs[0].UserID == nil || *logs[0].UserID != 42 || logs[0].ActorDisplayName != "服务端姓名" || logs[0].UserEmail != "logout@example.com" {
		t.Fatalf("unexpected login actor snapshot: %#v", logs[0])
	}
}

func TestAuthCallbackWithInvalidTokenDoesNotRecordLoginSuccessAndRedirects(t *testing.T) {
	store, handler, _ := newAuthAuditTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/_/auth/callback", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: "invalid-token"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("unexpected callback redirect: status=%d location=%s", recorder.Code, recorder.Header().Get("Location"))
	}
	logs := readAuthAuditLogs(t, store)
	if len(logs) != 1 || logs[0].Action != "auth.login" || logs[0].Result == "success" {
		t.Fatalf("invalid callback recorded login success: %#v", logs)
	}
	if logs[0].UserID != nil || logs[0].UserEmail != "" || logs[0].ActorDisplayName != "" {
		t.Fatalf("invalid callback audit event identified an actor: %#v", logs[0])
	}
}

func TestAuthCallbackWithValidTokenForUnknownOrDisabledUserRedirectsWithoutLoginSuccess(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")

	tests := []struct {
		name  string
		email string
	}{
		{name: "unknown user", email: "missing@example.com"},
		{name: "disabled user", email: "disabled@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, handler, privateKey := newAuthAuditTestHandler(t)
			request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/_/auth/callback", nil)
			request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
				"data": map[string]string{"mail": tt.email, "display": "Claims Name"},
				"exp":  time.Now().Add(time.Hour).Unix(),
			})})
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/erzhuang-project/" {
				t.Fatalf("unexpected callback redirect: status=%d location=%s", recorder.Code, recorder.Header().Get("Location"))
			}
			logs := readAuthAuditLogs(t, store)
			if len(logs) != 1 || logs[0].Action != "auth.login" || logs[0].Result == "success" {
				t.Fatalf("callback recorded login success for %s: %#v", tt.email, logs)
			}
		})
	}
}

func TestAuthCallbackAuditFailureStillRedirectsWithoutLeakingToken(t *testing.T) {
	store, _, privateKey := newAuthAuditTestHandler(t)
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	request := httptest.NewRequest(http.MethodGet, "https://lite.sy.soyoung.com/erzhuang-project/_/auth/callback", nil)
	token := signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{"mail": "logout@example.com"},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: token})
	recorder := httptest.NewRecorder()

	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)
	NewHandlerWithStore(failingAuditStore{MemoryStore: store, err: errors.New("audit failure token=" + token)}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/erzhuang-project/" {
		t.Fatalf("callback did not preserve redirect after audit failure: status=%d location=%s", recorder.Code, recorder.Header().Get("Location"))
	}
	if bytes.Contains(logs.Bytes(), []byte(token)) {
		t.Fatalf("callback audit failure log leaked token: %s", logs.String())
	}
}

func TestAuthMeDoesNotRecordLogin(t *testing.T) {
	store, handler, privateKey := newAuthAuditTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{"mail": "logout@example.com"},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	for _, event := range readAuthAuditLogs(t, store) {
		if event.Action == "auth.login" {
			t.Fatalf("auth login must not be recorded by auth.me: %#v", event)
		}
	}
}

func TestAPISIXSSOLogoutAuditFailureDoesNotBlockCookieClearingOrLeakToken(t *testing.T) {
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 42, Email: "logout@example.com", DisplayName: "服务端姓名", Role: RoleAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	token := signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{"mail": "logout@example.com"},
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: token})
	recorder := httptest.NewRecorder()

	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)
	NewHandlerWithStore(failingAuditStore{MemoryStore: store, err: errors.New("audit failure token=" + token)}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !hasClearedAuthCookie(recorder.Result().Cookies(), "sy_sso_token", "") {
		t.Fatalf("logout did not complete after audit failure: status=%d cookies=%#v", recorder.Code, recorder.Result().Cookies())
	}
	if bytes.Contains(logs.Bytes(), []byte(token)) {
		t.Fatalf("audit failure log leaked token: %s", logs.String())
	}
}

func newAuthAuditTestHandler(t *testing.T) (*MemoryStore, http.Handler, *rsa.PrivateKey) {
	t.Helper()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 42, Email: "logout@example.com", DisplayName: "服务端姓名", Role: RoleAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 43, Email: "disabled@example.com", DisplayName: "已禁用用户", Role: RoleViewer, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	return store, NewHandlerWithStore(store), privateKey
}

func readAuthAuditLogs(t *testing.T, store *MemoryStore) []AuditLog {
	t.Helper()
	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{StartAt: time.Unix(0, 0), EndAt: time.Now().Add(time.Hour), PageSize: 100})
	if err != nil {
		t.Fatalf("read audit logs: %v", err)
	}
	return page.Items
}

type failingAuditStore struct {
	*MemoryStore
	err error
}

func (s failingAuditStore) RecordAudit(context.Context, auditlog.AuditEvent) error {
	return s.err
}
