package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/digitaltwin"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"
)

func TestDigitalTwinWhitelistFiltersEvenAdministrator(t *testing.T) {
	store := NewMemoryStore()
	h := &Handler{store: store}
	response := nvrmonitor.MonitorStoresResponse{Cities: []nvrmonitor.StoreCityGroup{{City: "北京", Stores: []nvrmonitor.StoreInfo{{ExternalOrgID: "10001"}, {ExternalOrgID: "10042"}}}}}
	filtered, err := h.filterDigitalTwinStores(context.Background(), response)
	if err != nil || len(filtered.Cities) != 1 || len(filtered.Cities[0].Stores) != 1 || filtered.Cities[0].Stores[0].ExternalOrgID != "10001" {
		t.Fatalf("filter = %#v, %v", filtered, err)
	}
}

func TestDigitalTwinSettingsAndGuardsThroughHTTP(t *testing.T) {
	store := NewMemoryStore()
	for _, user := range []AuthUserRecord{
		{ID: 100, Email: "twin-admin@example.invalid", DisplayName: "联调管理员", Role: RoleAdmin, Enabled: true},
		{ID: 101, Email: "twin-viewer@example.invalid", DisplayName: "联调查看者", Role: RoleViewer, Enabled: true},
	} {
		if err := store.setAuthUserForTest(user); err != nil {
			t.Fatal(err)
		}
	}
	key := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &key.PublicKey))
	repo := monitorScreenshotRepository{records: resourceview.StoreRecords{
		Tenant:  resourceview.BusinessTenant{ID: 10001, Name: "北京保利总部店"},
		Devices: []resourceview.BusinessDevice{{ID: 111, TenantID: 10001, Name: "测试摄像头", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1}},
	}}
	handler := NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVR(store, designplan.NewService(designplan.NewMemoryStore()), storespace.NewService(storespace.NewMemoryStore()), nil, nil, nil, nvrmonitor.NewService(repo, nil), MonitorPlaybackModeNVR)
	cookies := map[string][]*http.Cookie{}
	for _, who := range []string{"admin", "viewer"} {
		req := httptest.NewRequest("GET", "/_/auth/callback", nil)
		addAuditLogTestSSOCookie(t, req, key, "twin-"+who+"@example.invalid")
		req.AddCookie(loginTestSession(t, handler, req))
		cookies[who] = req.Cookies()
	}
	request := func(who, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		for _, c := range cookies[who] {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		return out
	}
	for _, tc := range []struct {
		who, method, path string
		status            int
	}{
		{"admin", "GET", "/api/admin/digital-twin-settings", 200},
		{"viewer", "GET", "/api/admin/digital-twin-settings", 403},
		{"viewer", "GET", "/api/admin/digital-twin-candidates", 403},
		{"viewer", "PUT", "/api/admin/digital-twin-settings", 403},
		{"", "GET", "/api/digitaltwin/stores", 401},
		{"admin", "GET", "/api/digitaltwin/orgs/10042/cameras", 403},
		{"admin", "POST", "/api/digitaltwin/orgs/10042/cameras/111/stream-session", 403},
		{"viewer", "GET", "/api/digitaltwin/orgs/10001/cameras", 403},
		{"admin", "GET", "/api/digitaltwin/orgs/10001/cameras", 200},
	} {
		out := request(tc.who, tc.method, tc.path, "")
		if out.Code != tc.status {
			t.Fatalf("%s %s %s: %d %s", tc.who, tc.method, tc.path, out.Code, out.Body.String())
		}
	}
	initial, err := store.GetDigitalTwinSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	empty, _ := json.Marshal(map[string]any{"version": initial.Version, "store_ids": []string{}})
	out := request("admin", "PUT", "/api/admin/digital-twin-settings", string(empty))
	if out.Code != 200 {
		t.Fatalf("save: %d %s", out.Code, out.Body.String())
	}
	if again := request("admin", "PUT", "/api/admin/digital-twin-settings", string(empty)); again.Code != 409 {
		t.Fatalf("stale save: %d", again.Code)
	}
	if denied := request("admin", "GET", "/api/digitaltwin/orgs/10001/cameras", ""); denied.Code != 403 {
		t.Fatalf("removed store: %d", denied.Code)
	}
	if normal := request("admin", "GET", "/api/h5/nvr-monitor/orgs/10001/cameras", ""); normal.Code != 200 {
		t.Fatalf("ordinary monitor changed: %d", normal.Code)
	}
	logs, err := store.ListAuditLogs(context.Background(), AuditLogFilter{Action: "system.digital_twin_whitelist.update", StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour), Page: 1, PageSize: 20})
	if err != nil || len(logs.Items) != 1 || logs.Items[0].ActorDisplayName != "联调管理员" || !strings.Contains(string(logs.Items[0].DetailJSON), "10001") {
		t.Fatalf("audit: %#v %v", logs, err)
	}
}

func TestDigitalTwinRejectsNonWhitelistedStoreBeforeService(t *testing.T) {
	h := &Handler{store: NewMemoryStore()}
	a := digitalTwinAuthorizer{handler: h}
	ok, err := a.CanViewStore(httptest.NewRequest(http.MethodGet, "/", nil), "10042")
	if ok || err != nil {
		t.Fatalf("allowed=%t err=%v", ok, err)
	}
}

type digitalTwinMetricsProviderStub struct{}

func (digitalTwinMetricsProviderStub) GetOverview(context.Context, digitaltwin.Request) (digitaltwin.Overview, error) {
	return digitaltwin.Overview{ExpectedArrival: 12, Arrived: 8, NoConsult: 3, NeedConsult: 4, NonQuick: 1}, nil
}

func (digitalTwinMetricsProviderStub) GetDutyStaff(context.Context, digitaltwin.Request) (digitaltwin.DutyStaff, error) {
	return digitaltwin.DutyStaff{Consultants: 2, Nurses: 1, Doctors: 1}, nil
}

func (digitalTwinMetricsProviderStub) GetTrafficFlow(context.Context, digitaltwin.Request) (digitaltwin.TrafficFlow, error) {
	return digitaltwin.TrafficFlow{ReceptionCurrent: 2, Waiting: 3, TreatmentServed: 5}, nil
}

func TestDigitalTwinDashboardRequiresWhitelistAndReturnsRPCMetrics(t *testing.T) {
	store := NewMemoryStore()
	repo := monitorScreenshotRepository{records: resourceview.StoreRecords{
		Tenant:  resourceview.BusinessTenant{ID: 10001, Name: "北京保利总部店"},
		Devices: []resourceview.BusinessDevice{{ID: 111, TenantID: 10001, Name: "测试摄像头", Category: "camera", Provider: "HikVisionNvrChannel", Status: 1}},
	}}
	handler := NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVRAndDigitalTwin(
		store,
		designplan.NewService(designplan.NewMemoryStore()),
		storespace.NewService(storespace.NewMemoryStore()),
		nil,
		nil,
		nil,
		nvrmonitor.NewService(repo, nil),
		digitaltwin.NewService(digitalTwinMetricsProviderStub{}),
		MonitorPlaybackModeNVR,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/digitaltwin/orgs/10001/dashboard?date=2026-09-14", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var got digitaltwin.Dashboard
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Overview.ExpectedArrival != 12 || got.Traffic.TreatmentServed != 5 {
		t.Fatalf("dashboard = %#v", got)
	}

	forbidden := httptest.NewRequest(http.MethodGet, "/api/digitaltwin/orgs/10042/dashboard", nil)
	forbiddenResponse := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, body = %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}
}
