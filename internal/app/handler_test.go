package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestHealthUnderConfiguredBasePath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/health", nil)
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

func TestTasksUnderErzhuangAPIPrefix(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/erzhuang/api/tasks", nil)
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

func TestTasksUnderConfiguredBasePath(t *testing.T) {
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project/api/tasks", nil)
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

func TestServesFrontendIndexUnderErzhuang(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html>container frontend</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang/", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "container frontend") {
		t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
	}
}

func TestServesFrontendIndexUnderConfiguredBasePath(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	t.Setenv("APP_BASE_PATH", "/erzhuang-project")
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html>project frontend</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang-project", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "project frontend") {
		t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
	}
}

func TestServesFrontendAssetUnderErzhuang(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("FRONTEND_DIR", frontendDir)
	assetsDir := filepath.Join(frontendDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log('container asset')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/erzhuang/assets/app.js", nil)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "container asset") {
		t.Fatalf("expected frontend asset body, got %q", recorder.Body.String())
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
