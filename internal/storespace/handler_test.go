package storespace

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestListEzvizAccountsEndpointReturnsSafeFieldsOnly(t *testing.T) {
	handler := newTestHandler()
	request := httptest.NewRequest(http.MethodGet, "/api/store-space/ezviz-accounts", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"app_key", "app_secret_ciphertext", "access_token_ciphertext", "access_token", "app_secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaks forbidden field %q: %s", forbidden, body)
		}
	}
	var accounts []EzvizAccount
	if err := json.Unmarshal([]byte(body), &accounts); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected empty account list from memory store, got %d", len(accounts))
	}
}

func TestCreateEzvizAccountEndpointReturnsSafeFieldsOnly(t *testing.T) {
	handler := newTestHandler()
	request := httptest.NewRequest(http.MethodPost, "/api/store-space/ezviz-accounts", bytes.NewBufferString(`{"account_name":"华南测试账号"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"app_key", "app_secret_ciphertext", "access_token_ciphertext", "access_token", "app_secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaks forbidden field %q: %s", forbidden, body)
		}
	}
	var account EzvizAccount
	if err := json.Unmarshal([]byte(body), &account); err != nil {
		t.Fatalf("decode account: %v", err)
	}
	if account.AccountName != "华南测试账号" || account.Status != "unverified" {
		t.Fatalf("unexpected account: %#v", account)
	}
}

func TestCreateEzvizAccountEndpointRejectsDuplicateName(t *testing.T) {
	handler := newTestHandler()
	body := `{"account_name":"华南测试账号"}`
	first := httptest.NewRequest(http.MethodPost, "/api/store-space/ezviz-accounts", bytes.NewBufferString(body))
	handler.ServeHTTP(httptest.NewRecorder(), first)

	request := httptest.NewRequest(http.MethodPost, "/api/store-space/ezviz-accounts", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Fields["account_name"] != "萤石云账号名称已存在" {
		t.Fatalf("unexpected fields: %#v", response.Fields)
	}
}

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
	body := `{"city":"深圳","name":"深圳壹方城","recorders":[{"device_code":"D12345678"}]}`
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
	if store.City != "深圳" {
		t.Fatalf("expected city to be saved, got %q", store.City)
	}
}

func TestDeleteRecorderEndpointRemovesRecorder(t *testing.T) {
	handler := newTestHandler()
	createBody := `{"city":"深圳","name":"深圳壹方城","recorders":[{"device_code":"D12345678"},{"device_code":"D87654321"}]}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", bytes.NewBufferString(createBody))
	createRecorder := httptest.NewRecorder()

	handler.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, createRecorder.Code, createRecorder.Body.String())
	}
	var created Store
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created store: %v", err)
	}
	if len(created.Recorders) != 2 {
		t.Fatalf("expected two recorders, got %d", len(created.Recorders))
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/store-space/recorders/"+strconv.FormatInt(created.Recorders[0].ID, 10), nil)
	deleteRecorder := httptest.NewRecorder()
	handler.ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, deleteRecorder.Code, deleteRecorder.Body.String())
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/store-space/stores/"+strconv.FormatInt(created.ID, 10), nil)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, getRequest)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, getRecorder.Code, getRecorder.Body.String())
	}
	var updated Store
	if err := json.NewDecoder(getRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated store: %v", err)
	}
	if len(updated.Recorders) != 1 || updated.Recorders[0].DeviceCode != "D87654321" {
		t.Fatalf("unexpected recorders after delete: %#v", updated.Recorders)
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
