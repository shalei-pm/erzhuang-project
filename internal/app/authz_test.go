package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/auditlog"
)

type monitorAuditRecorder struct {
	event auditlog.AuditEvent
}

func (r *monitorAuditRecorder) RecordAudit(_ context.Context, event auditlog.AuditEvent) error {
	r.event = event
	return nil
}

func newMonitorAuditTestHandler(t *testing.T, recorder auditlog.AuditRecorder) (*Handler, *http.Request) {
	t.Helper()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID:          42,
		Email:       "monitor@example.com",
		Username:    "server-user",
		DisplayName: "服务端姓名",
		Role:        RoleAdmin,
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	handler := &Handler{
		store: store,
		auth: AuthConfig{
			Enabled:      true,
			CookieName:   "sy_sso_token",
			JWTPublicKey: publicKeyPEM(t, &privateKey.PublicKey),
			ClockSkew:    30 * time.Second,
			RequireEmail: true,
		},
		auditRecorder: recorder,
	}
	request := httptest.NewRequest(http.MethodPost, "/api/monitor", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("User-Agent", "monitor-test-agent")
	request.Header.Set("X-Request-ID", "request-42")
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{
			"mail":     "monitor@example.com",
			"username": "前端用户名",
			"display":  "前端姓名",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
	})})
	return handler, request
}

func assertMonitorAuditEvent(t *testing.T, event auditlog.AuditEvent) {
	t.Helper()
	if event.UserID == nil || *event.UserID != 42 {
		t.Fatalf("user id = %v, want 42", event.UserID)
	}
	if event.ActorDisplayName != "服务端姓名" || event.UserEmail != "monitor@example.com" {
		t.Fatalf("actor snapshot = (%q, %q)", event.ActorDisplayName, event.UserEmail)
	}
	if event.IPAddress != "192.0.2.10" || event.UserAgent != "monitor-test-agent" || event.RequestID != "request-42" {
		t.Fatalf("request snapshot = (%q, %q, %q)", event.IPAddress, event.UserAgent, event.RequestID)
	}
}

func TestH5MonitorAuthorizerRecordsAuditWithServerIdentity(t *testing.T) {
	recorder := &monitorAuditRecorder{}
	handler, request := newMonitorAuditTestHandler(t, recorder)
	event := auditlog.AuditEvent{
		Action:        "monitor.live_view",
		EntityType:    "channel",
		ExternalOrgID: "10030",
		EntityID:      auditIDPointer(1540),
		ChannelID:     auditIDPointer(1540),
		Result:        "failed",
		DetailJSON:    []byte(`{"summary":"provider timeout"}`),
	}

	if err := (h5MonitorAuthorizer{handler: handler}).RecordAudit(request, event); err != nil {
		t.Fatalf("record h5 audit: %v", err)
	}

	assertMonitorAuditEvent(t, recorder.event)
	if recorder.event.Action != event.Action || recorder.event.EntityType != event.EntityType ||
		recorder.event.ExternalOrgID != event.ExternalOrgID || recorder.event.Result != event.Result ||
		string(recorder.event.DetailJSON) != string(event.DetailJSON) {
		t.Fatalf("event fields were not preserved: %#v", recorder.event)
	}
}

func TestNVRMonitorAuthorizerRecordsAuditWithServerIdentity(t *testing.T) {
	recorder := &monitorAuditRecorder{}
	handler, request := newMonitorAuditTestHandler(t, recorder)
	authorizer := nvrMonitorAuthorizer{handler: handler}
	if allowed, err := authorizer.CanViewStore(request, "10030"); err != nil || !allowed {
		t.Fatalf("can view store = (%v, %v)", allowed, err)
	}
	event := auditlog.AuditEvent{
		Action:        "snapshot.download",
		EntityType:    "camera",
		ExternalOrgID: "10030",
		EntityID:      auditIDPointer(7),
		Result:        "success",
		DetailJSON:    []byte(`{"summary":"snapshot served"}`),
	}

	if err := authorizer.RecordAudit(request, event); err != nil {
		t.Fatalf("record nvr audit: %v", err)
	}
	assertMonitorAuditEvent(t, recorder.event)
}

func auditIDPointer(value int64) *int64 {
	return &value
}

func TestMonitorAuthorizersReturnErrorWhenAuditRecorderUnavailable(t *testing.T) {
	handler, request := newMonitorAuditTestHandler(t, nil)
	h5 := h5MonitorAuthorizer{handler: handler}
	nvr := nvrMonitorAuthorizer{handler: handler}
	event := auditlog.AuditEvent{Action: "monitor.live_view", Result: "denied"}

	if err := h5.RecordAudit(request, event); !errors.Is(err, errAuditUnavailable) {
		t.Fatalf("h5 error = %v, want audit unavailable", err)
	}
	if err := nvr.RecordAudit(request, event); !errors.Is(err, errAuditUnavailable) {
		t.Fatalf("nvr error = %v, want audit unavailable", err)
	}
}

func TestNVRMonitorAuthorizerRecordsDeniedAuditWhenUnauthenticated(t *testing.T) {
	recorder := &monitorAuditRecorder{}
	handler, request := newMonitorAuditTestHandler(t, recorder)
	request.Header.Set("Cookie", "sy_sso_token=invalid")
	authorizer := nvrMonitorAuthorizer{handler: handler}

	if _, err := authorizer.CanViewStore(request, "10030"); !errors.Is(err, nvrMonitorAuthError(errUnauthorizedAuth)) {
		t.Fatalf("can view error = %v", err)
	}
	if err := authorizer.RecordAudit(request, auditlog.AuditEvent{Action: "monitor.live_view", Result: "denied"}); err != nil {
		t.Fatalf("record denied audit: %v", err)
	}
	if recorder.event.Result != "denied" || recorder.event.UserID != nil {
		t.Fatalf("denied audit = %#v", recorder.event)
	}
}
