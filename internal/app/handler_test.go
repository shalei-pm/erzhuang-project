package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	const wantVersion = "v2"

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.App != AppName {
		t.Fatalf("expected app %q, got %q", AppName, response.App)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
	if response.Version != wantVersion {
		t.Fatalf("expected version %q, got %q", wantVersion, response.Version)
	}
	if response.Database != "memory" {
		t.Fatalf("expected database memory, got %q", response.Database)
	}
	if response.AssetStore != "local" {
		t.Fatalf("expected asset store local, got %q", response.AssetStore)
	}
}

func TestHealthDegradedWhenStorePingFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewHandlerWithStore(failingStore{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Fatalf("expected status degraded, got %q", response.Status)
	}
	if response.Database != "error" {
		t.Fatalf("expected database error, got %q", response.Database)
	}
}

func TestTasks(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []Task
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) == 0 {
		t.Fatal("expected at least one task")
	}
}

type failingStore struct{}

func (failingStore) Name() string {
	return "failing"
}

func (failingStore) Ping(ctx context.Context) error {
	return errors.New("ping failed")
}

func (failingStore) ListTasks(ctx context.Context) ([]Task, error) {
	return nil, errors.New("list failed")
}
