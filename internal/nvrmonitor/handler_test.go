package nvrmonitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

type fakeAuthorizer struct {
	allow map[string]bool
}

func (a fakeAuthorizer) CanViewStore(_ *http.Request, externalOrgID string) (bool, error) {
	return a.allow[externalOrgID], nil
}

func (a fakeAuthorizer) FilterStores(_ *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error) {
	filtered := MonitorStoresResponse{}
	for _, city := range response.Cities {
		next := StoreCityGroup{City: city.City}
		for _, store := range city.Stores {
			if a.allow[store.ExternalOrgID] {
				next.Stores = append(next.Stores, store)
			}
		}
		if len(next.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, next)
		}
	}
	return filtered, nil
}

func newTestHandler(t *testing.T, allow map[string]bool) http.Handler {
	t.Helper()
	service := NewService(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"})
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeAuthorizer{allow: allow})
	return mux
}

func newTestHandlerWithSnapshots(t *testing.T, allow map[string]bool, snapshots SnapshotStore) http.Handler {
	t.Helper()
	service := NewServiceWithSnapshotStore(fakeRepository{stores: map[int64]resourceview.StoreRecords{10001: nvrMonitorRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"}, snapshots)
	mux := http.NewServeMux()
	RegisterRoutesWithAuthorizer(mux, service, fakeAuthorizer{allow: allow})
	return mux
}

func TestStoresFiltersToAuthorizedMonitorScope(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Cities) != 1 || len(payload.Cities[0].Stores) != 1 || payload.Cities[0].Stores[0].ExternalOrgID != "10001" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCameraListRejectsUnauthorizedStore(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": false})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamSessionSetsNoStoreAndDoesNotExposeServiceCredential(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.Contains(response.Body.String(), "service-secret") {
		t.Fatalf("response leaked service credential: %s", response.Body.String())
	}
}

func TestStreamSessionRejectsCameraOutsideTenantEligibleSet(t *testing.T) {
	handler := newTestHandler(t, map[string]bool{"10001": true})
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-monitor/orgs/10001/cameras/112/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestSnapshotServesBackfilledImageOnlyToAuthorizedStore(t *testing.T) {
	handler := newTestHandlerWithSnapshots(t, map[string]bool{"10001": true}, fakeSnapshotStore{
		data: map[int64]string{111: "jpeg-data"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != "jpeg-data" || response.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected snapshot response: headers=%#v body=%q", response.Header(), response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, max-age=3600" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestSnapshotDoesNotLeakToUnauthorizedStore(t *testing.T) {
	handler := newTestHandlerWithSnapshots(t, map[string]bool{"10001": false}, fakeSnapshotStore{
		data: map[int64]string{111: "jpeg-data"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || response.Body.String() == "jpeg-data" {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}
