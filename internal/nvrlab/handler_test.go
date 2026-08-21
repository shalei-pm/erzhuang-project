package nvrlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
)

func TestStreamSessionSetsNoStoreAndDoesNotExposeServiceCredential(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, service, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-lab/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.Contains(response.Body.String(), "service-secret") {
		t.Fatal("response exposed service credential")
	}
	var session StreamSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Mode != ModeLive || session.URL == "" {
		t.Fatalf("session = %#v", session)
	}
}

func TestStreamSessionRequiresFreshAdminGuard(t *testing.T) {
	service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{url: "wss://stream.example.test/session"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, service, func(http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusForbidden, map[string]string{"code": "nvr_lab_forbidden"})
		}
	})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/nvr-lab/10001/cameras", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestStreamSessionReturnsSafeAuthorizationDiagnosticCode(t *testing.T) {
	service := NewService(
		fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}},
		&fakeAuthorizationClient{err: newAuthorizationFailure("upstream_http", http.StatusUnauthorized)},
	)
	mux := http.NewServeMux()
	RegisterRoutes(mux, service, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-lab/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "nvr_stream_authorization_upstream_http_401" {
		t.Fatalf("code = %q", payload["code"])
	}
	if strings.Contains(response.Body.String(), "service-secret") || strings.Contains(response.Body.String(), "private detail") {
		t.Fatal("response exposed sensitive authorization detail")
	}
}
