package storespace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
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

func TestGetStoreDesignPlanDataEndpointReturnsOnlyDesignPlanTabData(t *testing.T) {
	handler := newTestHandler()
	createStoreForHandlerTest(t, handler, `{"city":"上海","name":"上海测试店","recorders":[{"device_code":"D12345678"}],"design_plan_upload_id":"tmp_plan"}`)
	saveBody := `{
		"pdf_file_name":"plan.pdf",
		"preview_image_path":"uploads/tmp_plan/preview.png",
		"thumbnail_path":"uploads/tmp_plan/thumbnail.png",
		"page_count":1,
		"areas":[{"display_name":"治疗室 1","area_type":"treatment","area_number":"1","box":{"x":0.1,"y":0.2,"width":0.3,"height":0.4}}]
	}`
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/store-space/stores/1/design-plan", bytes.NewBufferString(saveBody))
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("expected save status %d, got %d body=%s", http.StatusOK, saveResponse.Code, saveResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores/1/design-plan-data", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var store Store
	if err := json.NewDecoder(response.Body).Decode(&store); err != nil {
		t.Fatalf("decode store: %v", err)
	}
	if len(store.DesignPlans) != 1 || len(store.Areas) != 1 {
		t.Fatalf("expected design plan and areas, got plans=%#v areas=%#v", store.DesignPlans, store.Areas)
	}
	if store.Recorders != nil {
		t.Fatalf("expected recorders omitted from design plan data, got %#v", store.Recorders)
	}
}

func TestGetStoreChannelDataEndpointReturnsOnlyChannelTabData(t *testing.T) {
	handler := newTestHandler()
	createStoreForHandlerTest(t, handler, `{"city":"上海","name":"上海测试店","recorders":[{"device_code":"D12345678"}],"design_plan_upload_id":"tmp_plan"}`)
	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores/1/channel-data", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var store Store
	if err := json.NewDecoder(response.Body).Decode(&store); err != nil {
		t.Fatalf("decode store: %v", err)
	}
	if len(store.Recorders) != 1 || store.Recorders[0].DeviceCode != "D12345678" {
		t.Fatalf("expected recorders, got %#v", store.Recorders)
	}
	if store.DesignPlans != nil || store.Areas != nil {
		t.Fatalf("expected design plan fields omitted from channel data, got plans=%#v areas=%#v", store.DesignPlans, store.Areas)
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

func TestScanRecorderEndpointReturnsDiagnosticForUnexpectedError(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华东"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, unexpectedScanErrorScanner{})
	handler := newTestHandlerWithService(service)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "上海",
		Name: "上海静安",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "K96112775"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/store-space/recorders/"+strconv.FormatInt(store.Recorders[0].ID, 10)+"/scan-channels", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusInternalServerError, response.Code, response.Body.String())
	}
	var payload struct {
		Error  string `json:"error"`
		Code   string `json:"code"`
		Stage  string `json:"stage"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "store space request failed" || payload.Code != "store_space_request_failed" || payload.Stage != "store_space" {
		t.Fatalf("unexpected diagnostic payload: %#v", payload)
	}
	if !strings.Contains(payload.Detail, "scan transport failed") {
		t.Fatalf("expected sanitized detail to include root cause, got %#v", payload)
	}
}

func TestScanRecorderEndpointReturnsEzvizErrorCodeForFallback(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华东"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, planLimitScanner{})
	handler := newTestHandlerWithService(service)
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "上海",
		Name: "上海静安",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "K96112775"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/store-space/recorders/"+strconv.FormatInt(store.Recorders[0].ID, 10)+"/scan-channels", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadGateway, recorder.Code, recorder.Body.String())
	}
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(response["error"], "10026") || !strings.Contains(response["error"], "设备数量超出个人版限制") {
		t.Fatalf("expected fallback-detectable ezviz error, got %#v", response)
	}
}

func TestChannelSnapshotDiagnosticsReportsOpenFailure(t *testing.T) {
	service := NewService(NewMemoryStore())
	service.UseSnapshotStore(failingSnapshotStore{})
	handler := newTestHandlerWithService(service)

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/channel-snapshots/00000000000000000000000000000001.jpg/diagnostics", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var payload struct {
		Code        string `json:"code"`
		Stage       string `json:"stage"`
		AssetStore  string `json:"asset_store"`
		SnapshotKey string `json:"snapshot_key"`
		Exists      bool   `json:"exists"`
		Detail      string `json:"detail"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "snapshot_open_failed" || payload.Stage != "open_snapshot" || payload.AssetStore == "" {
		t.Fatalf("unexpected diagnostics payload: %#v", payload)
	}
	if payload.SnapshotKey != "channel-snapshots/00000000000000000000000000000001.jpg" || payload.Exists {
		t.Fatalf("unexpected snapshot state: %#v", payload)
	}
	if !strings.Contains(payload.Detail, "open asset failed: http 403") || strings.Contains(payload.Detail, "secret-token") {
		t.Fatalf("expected sanitized open failure detail, got %#v", payload)
	}
}

func TestChannelSnapshotResponseUsesBrowserCacheHeaders(t *testing.T) {
	service := NewService(NewMemoryStore())
	service.UseSnapshotStore(staticSnapshotStore{data: "jpg-data", contentType: "image/jpeg"})
	handler := newTestHandlerWithService(service)
	name := "00000000000000000000000000000001.jpg"

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/channel-snapshots/"+name, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, max-age=604800, immutable" {
		t.Fatalf("unexpected cache header: %q", response.Header().Get("Cache-Control"))
	}
	etag := response.Header().Get("ETag")
	if etag != strconv.Quote(name) {
		t.Fatalf("unexpected etag: %q", etag)
	}
	if response.Body.String() != "jpg-data" {
		t.Fatalf("unexpected body: %q", response.Body.String())
	}

	cachedRequest := httptest.NewRequest(http.MethodGet, "/api/store-space/channel-snapshots/"+name, nil)
	cachedRequest.Header.Set("If-None-Match", etag)
	cachedResponse := httptest.NewRecorder()
	handler.ServeHTTP(cachedResponse, cachedRequest)
	if cachedResponse.Code != http.StatusNotModified {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotModified, cachedResponse.Code, cachedResponse.Body.String())
	}
	if cachedResponse.Body.Len() != 0 {
		t.Fatalf("expected empty 304 body, got %q", cachedResponse.Body.String())
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

func TestListStoresReportsChannelsFullyConfirmed(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华北"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{channels: []ScannedChannel{
		{ChannelNo: 1, ChannelName: "通道1", Active: true},
		{ChannelNo: 2, ChannelName: "通道2", Active: true},
	}})
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

	before := listStoresForTest(t, handler)
	if len(before.Items) != 1 {
		t.Fatalf("expected one store, got %d", len(before.Items))
	}
	if before.Items[0].ChannelsFullyConfirmed {
		t.Fatalf("expected scanned but unconfirmed channels to be reported as not fully confirmed")
	}

	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{AreaType: AreaTypeTreatment, AreaNumber: "1"}); err != nil {
		t.Fatalf("confirm channel 1: %v", err)
	}
	if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[1].ID, ChannelConfirmationInput{SceneType: SceneTypeMachineRoom, AreaNote: "机房"}); err != nil {
		t.Fatalf("confirm channel 2: %v", err)
	}

	after := listStoresForTest(t, handler)
	if len(after.Items) != 1 {
		t.Fatalf("expected one store, got %d", len(after.Items))
	}
	if !after.Items[0].ChannelsFullyConfirmed {
		t.Fatalf("expected all active confirmed channels to be reported as fully confirmed")
	}
}

func TestListStoresSummaryCountsAllFilteredStoresAcrossPages(t *testing.T) {
	repo := NewMemoryStore()
	service := NewService(repo)
	handler := newTestHandlerWithService(service)
	for _, input := range []struct {
		name     string
		areaType AreaType
	}{
		{name: "分页测试一号店", areaType: AreaTypeTreatment},
		{name: "分页测试二号店", areaType: AreaTypeConsultation},
	} {
		store, err := service.CreateStore(context.Background(), CreateStoreInput{
			City:               "深圳",
			Name:               input.name,
			DesignPlanUploadID: "upload_123",
		})
		if err != nil {
			t.Fatalf("create store %s: %v", input.name, err)
		}
		if _, err := service.SaveDesignPlan(context.Background(), store.ID, SaveDesignPlanInput{
			UploadID:         "upload_123",
			PDFFileName:      "store.pdf",
			PreviewImagePath: "uploads/upload_123/preview.png",
			ThumbnailPath:    "uploads/upload_123/thumbnail.png",
			PageCount:        1,
			Areas: []DesignAreaInput{
				{
					DisplayName: string(input.areaType) + "1号",
					Type:        input.areaType,
					NumberText:  "1",
					Box:         &AreaBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
				},
			},
		}); err != nil {
			t.Fatalf("save design plan: %v", err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores?page=1&page_size=1&q=%E5%88%86%E9%A1%B5%E6%B5%8B%E8%AF%95", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var result StoreListResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode list stores: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one paged item, got %d", len(result.Items))
	}
	if result.Total != 2 {
		t.Fatalf("expected total stores = 2, got %d", result.Total)
	}
	if result.Summary.StoreCount != 2 || result.Summary.TreatmentCount != 1 || result.Summary.ConsultationCount != 1 {
		t.Fatalf("expected summary across all filtered stores, got %#v", result.Summary)
	}
}

func TestListStoresFiltersCityBeforePagination(t *testing.T) {
	repo := NewMemoryStore()
	service := NewService(repo)
	handler := newTestHandlerWithService(service)
	for _, input := range []CreateStoreInput{
		{City: "北京", Name: "北京一号店", DesignPlanUploadID: "upload_123"},
		{City: "上海", Name: "上海一号店", DesignPlanUploadID: "upload_123"},
		{City: "上海", Name: "上海二号店", DesignPlanUploadID: "upload_123"},
	} {
		if _, err := service.CreateStore(context.Background(), input); err != nil {
			t.Fatalf("create store %s: %v", input.Name, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores?page=1&page_size=1&city=%E4%B8%8A%E6%B5%B7", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var result StoreListResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode list stores: %v", err)
	}
	if result.Total != 2 || result.Summary.StoreCount != 2 {
		t.Fatalf("expected all Shanghai stores to be counted before pagination, total=%d summary=%#v", result.Total, result.Summary)
	}
	cityOptions := map[string]bool{}
	for _, city := range result.Cities {
		cityOptions[city] = true
	}
	if !cityOptions["北京"] || !cityOptions["上海"] {
		t.Fatalf("expected city options to include all cities for the current search before city filter, got %q", strings.Join(result.Cities, ","))
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one paged item, got %d", len(result.Items))
	}
	if result.Items[0].City != "上海" {
		t.Fatalf("expected paged item city 上海, got %q", result.Items[0].City)
	}
}

func listStoresForTest(t *testing.T, handler http.Handler) StoreListResult {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/store-space/stores?page=1&page_size=20", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}
	var result StoreListResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode list stores: %v", err)
	}
	return result
}

func createStoreForHandlerTest(t *testing.T, handler http.Handler, body string) Store {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusCreated, response.Code, response.Body.String())
	}
	var store Store
	if err := json.NewDecoder(response.Body).Decode(&store); err != nil {
		t.Fatalf("decode created store: %v", err)
	}
	return store
}

func newTestHandler() http.Handler {
	return newTestHandlerWithService(NewService(NewMemoryStore()))
}

func newTestHandlerWithService(service *Service) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, service)
	return mux
}

type planLimitScanner struct{}

func (planLimitScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return nil, &ezviz.Error{Code: "10026", Msg: "设备数量超出个人版限制，当前设备无法操作"}
}

func (planLimitScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	return ChannelSnapshotInput{}, nil
}

func (planLimitScanner) LiveAddress(ctx context.Context, account EzvizAccount, recorder Recorder, channelNo int, code string) (LiveAddressResult, error) {
	return LiveAddressResult{}, nil
}

type unexpectedScanErrorScanner struct{}

func (unexpectedScanErrorScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	return nil, errors.New("scan transport failed: upstream reset")
}

func (unexpectedScanErrorScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	return ChannelSnapshotInput{}, nil
}

func (unexpectedScanErrorScanner) LiveAddress(ctx context.Context, account EzvizAccount, recorder Recorder, channelNo int, code string) (LiveAddressResult, error) {
	return LiveAddressResult{}, nil
}

type failingSnapshotStore struct{}

func (failingSnapshotStore) SaveRemote(ctx context.Context, imageURL string) (string, error) {
	return "", errors.New("not used")
}

func (failingSnapshotStore) Open(ctx context.Context, name string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("open asset failed: http 403 access_token=secret-token")
}

type staticSnapshotStore struct {
	data        string
	contentType string
}

func (s staticSnapshotStore) SaveRemote(ctx context.Context, imageURL string) (string, error) {
	return "", errors.New("not used")
}

func (s staticSnapshotStore) Open(ctx context.Context, name string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader(s.data)), s.contentType, nil
}
