package storespace

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateStoreEndpointReturnsValidationFields(t *testing.T) {
	handler := newTestHandler()
	request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", bytes.NewBufferString(`{"name":"深圳壹方城"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	var response struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Fields["resources"] != "请至少上传设计图或填写一个录像机设备编码" {
		t.Fatalf("unexpected fields: %#v", response.Fields)
	}
}

func TestCreateStoreEndpointCreatesRecorderOnlyStore(t *testing.T) {
	handler := newTestHandler()
	body := `{"name":"深圳壹方城","recorders":[{"device_code":"D12345678"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	var store Store
	if err := json.NewDecoder(recorder.Body).Decode(&store); err != nil {
		t.Fatalf("decode store: %v", err)
	}
	if len(store.Recorders) != 1 || store.Recorders[0].DeviceCode != "D12345678" {
		t.Fatalf("unexpected recorders: %#v", store.Recorders)
	}
}

func TestScanRecorderEndpointReturnsStableNotImplemented(t *testing.T) {
	handler := newTestHandler()
	request := httptest.NewRequest(http.MethodPost, "/api/store-space/recorders/1/scan-channels", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, recorder.Code)
	}
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["error"] != "not implemented" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func newTestHandler() http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewService(NewMemoryStore()))
	return mux
}
