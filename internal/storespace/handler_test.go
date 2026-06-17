package storespace

import (
	"bytes"
	"context"
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

func TestUpdateStoreBasicInfoEndpoint(t *testing.T) {
	handler := newTestHandler()
	createBody := `{"city":"深圳","name":"深圳壹方城","recorders":[{"device_code":"D12345678"}]}`
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

	updateBody := `{"city":"广州","name":"广州天河店","external_org_id":"888001"}`
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/store-space/stores/"+strconv.FormatInt(created.ID, 10), bytes.NewBufferString(updateBody))
	updateRecorder := httptest.NewRecorder()

	handler.ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated Store
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated store: %v", err)
	}
	if updated.City != "广州" || updated.Name != "广州天河店" || updated.ExternalOrgID != "888001" {
		t.Fatalf("unexpected updated store: %#v", updated)
	}
	if len(updated.Recorders) != 1 || updated.Recorders[0].DeviceCode != "D12345678" {
		t.Fatalf("expected existing recorder to remain unchanged, got %#v", updated.Recorders)
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

func TestAddRecorderEndpointAddsRecorderAfterDelete(t *testing.T) {
	handler := newTestHandler()
	createBody := `{"city":"深圳","name":"深圳壹方城","recorders":[{"device_code":"D12345678"}]}`
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
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/store-space/recorders/"+strconv.FormatInt(created.Recorders[0].ID, 10), nil)
	handler.ServeHTTP(httptest.NewRecorder(), deleteRequest)

	addRequest := httptest.NewRequest(http.MethodPost, "/api/store-space/stores/"+strconv.FormatInt(created.ID, 10)+"/recorders", bytes.NewBufferString(`{"device_code":"D87654321"}`))
	addRecorder := httptest.NewRecorder()
	handler.ServeHTTP(addRecorder, addRequest)

	if addRecorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, addRecorder.Code, addRecorder.Body.String())
	}
	var updated Store
	if err := json.NewDecoder(addRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated store: %v", err)
	}
	if len(updated.Recorders) != 1 || updated.Recorders[0].DeviceCode != "D87654321" {
		t.Fatalf("unexpected recorders after add: %#v", updated.Recorders)
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

func TestExportChannelMappingsEndpointDownloadsExcel(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "通道1", Active: true}}})
	handler := newTestHandlerWithService(service)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "深圳",
		Name: "深圳壹方城",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "GN0941203"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	recorder, err := service.ScanRecorderChannels(context.Background(), store.Recorders[0].ID)
	if err != nil {
		t.Fatalf("scan channels: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{AreaType: AreaTypeConsultation, AreaNumber: "1"}); err != nil {
		t.Fatalf("confirm channel: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores/"+strconv.FormatInt(store.ID, 10)+"/channel-mappings/export.xlsx", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != channelMappingExcelContentType {
		t.Fatalf("unexpected content type: %s", response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Header().Get("Content-Disposition"), "xlsx") {
		t.Fatalf("unexpected content disposition: %s", response.Header().Get("Content-Disposition"))
	}
	if !bytes.HasPrefix(response.Body.Bytes(), []byte("PK")) {
		t.Fatalf("expected xlsx zip payload, got %q", response.Body.String())
	}
}

func newTestHandler() http.Handler {
	return newTestHandlerWithService(NewService(NewMemoryStore()))
}

func newTestHandlerWithService(service *Service) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, service)
	return mux
}
