package designplan

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerCreateListGetUpdateDeleteStore(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewService(NewMemoryStore()))

	createBody := []byte(`{
		"name": "杭州西湖店",
		"areas": [
			{
				"name": "治疗室 1",
				"type": "treatment",
				"number": "1",
				"box": {"x": 0.1, "y": 0.1, "width": 0.2, "height": 0.2}
			},
			{
				"name": "生美区",
				"type": "beauty",
				"box": {"x": 0.4, "y": 0.1, "width": 0.2, "height": 0.2}
			}
		]
	}`)

	createRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores", createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createRecorder.Code, createRecorder.Body.String())
	}

	var created Store
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created store: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected created id")
	}
	if len(created.Areas) != 2 {
		t.Fatalf("expected 2 areas, got %d", len(created.Areas))
	}

	listRecorder := performRequest(mux, http.MethodGet, "/api/design-plan/stores?q=西湖&page=1&page_size=20", nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listRecorder.Code)
	}

	var list StoreListResult
	if err := json.NewDecoder(listRecorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("expected one list item, got total=%d len=%d", list.Total, len(list.Items))
	}
	if list.Items[0].TreatmentCount != 1 || list.Items[0].BeautyCount != 1 {
		t.Fatalf("unexpected area counts: %+v", list.Items[0])
	}

	getRecorder := performRequest(mux, http.MethodGet, "/api/design-plan/stores/1", nil)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, getRecorder.Code)
	}

	updateBody := []byte(`{
		"name": "杭州西湖旗舰店",
		"areas": [
			{
				"name": "面诊室 3",
				"type": "consultation",
				"number": 3,
				"confidence": "low",
				"box": {"x": 0.2, "y": 0.2, "width": 0.2, "height": 0.2}
			}
		]
	}`)
	updateRecorder := performRequest(mux, http.MethodPut, "/api/design-plan/stores/1", updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d: %s", http.StatusOK, updateRecorder.Code, updateRecorder.Body.String())
	}

	var updated Store
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated store: %v", err)
	}
	if updated.Status != StoreStatusNeedsReview {
		t.Fatalf("expected needs_review status, got %q", updated.Status)
	}
	if updated.Areas[0].Number != "3" {
		t.Fatalf("expected numeric json number to decode as room number 3, got %q", updated.Areas[0].Number)
	}

	deleteRecorder := performRequest(mux, http.MethodDelete, "/api/design-plan/stores/1", nil)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected delete status %d, got %d", http.StatusNoContent, deleteRecorder.Code)
	}
}

func TestHandlerRejectsInvalidStore(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewService(NewMemoryStore()))

	recorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores", []byte(`{"name":"","areas":[]}`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Fields["name"] == "" || response.Fields["areas"] == "" {
		t.Fatalf("expected field errors, got %+v", response.Fields)
	}
}

func TestHandlerReturnsEmptyArrays(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewService(NewMemoryStore()))

	listRecorder := performRequest(mux, http.MethodGet, "/api/design-plan/stores?page=1&page_size=20", nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listRecorder.Code)
	}

	var listResponse struct {
		Items []StoreListItem `json:"items"`
	}
	if err := json.NewDecoder(listRecorder.Body).Decode(&listResponse); err != nil {
		t.Fatalf("decode empty list response: %v", err)
	}
	if listResponse.Items == nil {
		t.Fatal("expected empty list items to be [], got null")
	}

	duplicateRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores/check-duplicate", []byte(`{"name":"不存在门店"}`))
	if duplicateRecorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate status %d, got %d", http.StatusOK, duplicateRecorder.Code)
	}

	var duplicateResponse struct {
		SimilarMatches []DuplicateMatch `json:"similar_matches"`
	}
	if err := json.NewDecoder(duplicateRecorder.Body).Decode(&duplicateResponse); err != nil {
		t.Fatalf("decode empty duplicate response: %v", err)
	}
	if duplicateResponse.SimilarMatches == nil {
		t.Fatal("expected empty similar_matches to be [], got null")
	}
}

func TestDuplicateCheckResultEncodesEmptySimilarMatchesAsArray(t *testing.T) {
	data, err := json.Marshal(DuplicateCheckResult{SimilarMatches: []DuplicateMatch{}})
	if err != nil {
		t.Fatalf("marshal duplicate result: %v", err)
	}
	if !bytes.Contains(data, []byte(`"similar_matches":[]`)) {
		t.Fatalf("expected similar_matches to encode as [], got %s", data)
	}
}

func TestHandlerRejectsExactDuplicateStoreName(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewService(NewMemoryStore()))

	firstRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores", mustJSON(validStoreInput()))
	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf("expected first create status %d, got %d: %s", http.StatusCreated, firstRecorder.Code, firstRecorder.Body.String())
	}

	duplicateInput := validStoreInput()
	duplicateInput.Name = "新氧青春杭州西湖店"
	duplicateRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores", mustJSON(duplicateInput))
	if duplicateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate status %d, got %d: %s", http.StatusBadRequest, duplicateRecorder.Code, duplicateRecorder.Body.String())
	}

	var response struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(duplicateRecorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode duplicate response: %v", err)
	}
	if response.Fields["name"] != "已存在同名门店" {
		t.Fatalf("expected duplicate name error, got %+v", response.Fields)
	}
}

func TestHandlerCheckDuplicateExcludesCurrentStore(t *testing.T) {
	repo := NewMemoryStore()
	service := NewService(repo)
	if _, err := service.CreateStore(context.Background(), validStoreInput()); err != nil {
		t.Fatalf("create store: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, service)

	exactRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores/check-duplicate", []byte(`{"name":"新氧青春杭州西湖门店"}`))
	if exactRecorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate status %d, got %d", http.StatusOK, exactRecorder.Code)
	}

	var exact DuplicateCheckResult
	if err := json.NewDecoder(exactRecorder.Body).Decode(&exact); err != nil {
		t.Fatalf("decode exact duplicate: %v", err)
	}
	if exact.ExactMatch == nil {
		t.Fatal("expected exact duplicate match")
	}

	excludedRecorder := performRequest(mux, http.MethodPost, "/api/design-plan/stores/check-duplicate", []byte(`{"name":"杭州西湖店","exclude_store_id":1}`))
	if excludedRecorder.Code != http.StatusOK {
		t.Fatalf("expected excluded status %d, got %d", http.StatusOK, excludedRecorder.Code)
	}
	var excluded DuplicateCheckResult
	if err := json.NewDecoder(excludedRecorder.Body).Decode(&excluded); err != nil {
		t.Fatalf("decode excluded duplicate: %v", err)
	}
	if excluded.ExactMatch != nil || len(excluded.SimilarMatches) != 0 {
		t.Fatalf("expected no duplicate when excluding current store, got %+v", excluded)
	}
}

func performRequest(handler http.Handler, method string, target string, body []byte) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
