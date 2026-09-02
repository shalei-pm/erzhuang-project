package app

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"
)

func TestMonitorScreenshotWatermarkDefaultsToEnabled(t *testing.T) {
	store := NewMemoryStore()

	enabled, err := store.GetMonitorScreenshotWatermarkEnabled(context.Background())

	if err != nil {
		t.Fatalf("get watermark setting: %v", err)
	}
	if !enabled {
		t.Fatal("watermark setting = false, want true when it has not been configured")
	}
}

func TestMonitorScreenshotWatermarkCanBeDisabled(t *testing.T) {
	store := NewMemoryStore()
	if err := store.SetMonitorScreenshotWatermarkEnabled(context.Background(), false); err != nil {
		t.Fatalf("disable watermark setting: %v", err)
	}

	enabled, err := store.GetMonitorScreenshotWatermarkEnabled(context.Background())

	if err != nil {
		t.Fatalf("get watermark setting: %v", err)
	}
	if enabled {
		t.Fatal("watermark setting = true, want false after disabling it")
	}
}

func TestMonitorScreenshotWatermarkSettingsHandlersPersistAndAuditChanges(t *testing.T) {
	store, handler, privateKey := newMonitorScreenshotSettingsTestHandler(t)

	initial := monitorScreenshotSettingsRequest(t, handler, privateKey, http.MethodGet, "")
	if initial.Code != http.StatusOK {
		t.Fatalf("initial settings status = %d body=%s", initial.Code, initial.Body.String())
	}
	assertMonitorScreenshotWatermarkEnabled(t, initial, true)

	updated := monitorScreenshotSettingsRequest(t, handler, privateKey, http.MethodPost, `{"enabled":false}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update settings status = %d body=%s", updated.Code, updated.Body.String())
	}
	assertMonitorScreenshotWatermarkEnabled(t, updated, false)

	persisted, err := store.GetMonitorScreenshotWatermarkEnabled(context.Background())
	if err != nil || persisted {
		t.Fatalf("persisted setting = %t err=%v, want false and nil", persisted, err)
	}
	logs, err := store.ListAuditLogs(context.Background(), AuditLogFilter{StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour), Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs.Items) != 1 || logs.Items[0].Action != "system.monitor_screenshot_watermark.update" {
		t.Fatalf("audit logs = %#v", logs.Items)
	}
	if !strings.Contains(string(logs.Items[0].DetailJSON), `"previous_enabled":true`) || !strings.Contains(string(logs.Items[0].DetailJSON), `"enabled":false`) {
		t.Fatalf("audit detail = %s", logs.Items[0].DetailJSON)
	}
	if summary := auditLogSummary(logs.Items[0]); summary != "监控截图水印：启用 -> 停用" {
		t.Fatalf("audit summary = %q", summary)
	}
}

func TestMonitorScreenshotWatermarkSettingsUpdateRequiresAdminPermission(t *testing.T) {
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 101, Email: "viewer-watermark@soyoung.com", Role: RoleViewer, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	handler := NewHandlerWithStore(store)
	request := httptest.NewRequest(http.MethodPost, "/api/monitor-screenshot-watermark-settings", strings.NewReader(`{"enabled":false}`))
	addAuditLogTestSSOCookie(t, request, privateKey, "viewer-watermark@soyoung.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

func TestNVRMonitorScreenshotMetadataAuthorizesCameraAndWritesAudit(t *testing.T) {
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 102, Email: "camera-watermark@soyoung.com", Username: "camera-user", DisplayName: "摄像头管理员", Role: RoleAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	repository := monitorScreenshotRepository{records: resourceview.StoreRecords{
		Tenant:  resourceview.BusinessTenant{ID: 10001, Name: "测试门店"},
		Devices: []resourceview.BusinessDevice{{ID: 111, TenantID: 10001, Category: "camera", Provider: "HikVisionNvrChannel", Status: 1}},
	}}
	handler := NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVR(
		store,
		designplan.NewService(designplan.NewMemoryStore()),
		storespace.NewService(storespace.NewMemoryStore()),
		nil,
		nil,
		nil,
		nvrmonitor.NewService(repository, nil),
		MonitorPlaybackModeNVR,
	)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/screenshot-metadata", nil)
	addAuditLogTestSSOCookie(t, request, privateKey, "camera-watermark@soyoung.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload monitorScreenshotMetadataResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode screenshot metadata: %v", err)
	}
	if !payload.WatermarkEnabled || payload.DisplayName != "摄像头管理员" || payload.CapturedAt == "" {
		t.Fatalf("metadata = %#v", payload)
	}
	logs, err := store.ListAuditLogs(context.Background(), AuditLogFilter{StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour), Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs.Items) != 1 || logs.Items[0].Action != "monitor.screenshot" || logs.Items[0].ExternalOrgID != "10001" {
		t.Fatalf("audit logs = %#v", logs.Items)
	}
}

type monitorScreenshotRepository struct {
	records resourceview.StoreRecords
}

func (r monitorScreenshotRepository) ListStores(context.Context, resourceview.StoreFilters) ([]resourceview.StoreRecords, error) {
	return []resourceview.StoreRecords{r.records}, nil
}

func (r monitorScreenshotRepository) ListNVRMonitorStores(context.Context) ([]resourceview.StoreRecords, error) {
	return []resourceview.StoreRecords{r.records}, nil
}

func (r monitorScreenshotRepository) GetStoreRecords(_ context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	return r.GetNVRMonitorStoreRecords(context.Background(), tenantID)
}

func (r monitorScreenshotRepository) GetNVRMonitorStoreRecords(_ context.Context, tenantID int64) (resourceview.StoreRecords, error) {
	if tenantID != r.records.Tenant.ID {
		return resourceview.StoreRecords{}, resourceview.ErrNotFound
	}
	return r.records, nil
}

func newMonitorScreenshotSettingsTestHandler(t *testing.T) (*MemoryStore, http.Handler, *rsa.PrivateKey) {
	t.Helper()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{ID: 100, Email: "watermark-admin@soyoung.com", Username: "watermark-admin", DisplayName: "水印管理员", Role: RoleAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))
	return store, NewHandlerWithStore(store), privateKey
}

func monitorScreenshotSettingsRequest(t *testing.T, handler http.Handler, privateKey *rsa.PrivateKey, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "/api/monitor-screenshot-watermark-settings", strings.NewReader(body))
	addAuditLogTestSSOCookie(t, request, privateKey, "watermark-admin@soyoung.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertMonitorScreenshotWatermarkEnabled(t *testing.T, response *httptest.ResponseRecorder, want bool) {
	t.Helper()
	var payload monitorScreenshotWatermarkSettingsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode setting response: %v", err)
	}
	if payload.Enabled != want {
		t.Fatalf("enabled = %t, want %t", payload.Enabled, want)
	}
}
