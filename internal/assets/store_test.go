package assets

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStoreSaveOpenDeletePrefix(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	ctx := context.Background()

	if err := store.Save(ctx, "uploads/tmp_1/preview.png", strings.NewReader("png-data"), "image/png"); err != nil {
		t.Fatalf("save: %v", err)
	}

	reader, contentType, err := store.Open(ctx, "uploads/tmp_1/preview.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	body, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
	if contentType != "image/png" {
		t.Fatalf("expected image/png, got %q", contentType)
	}
	if string(body) != "png-data" {
		t.Fatalf("expected saved body, got %q", string(body))
	}

	if err := store.DeletePrefix(ctx, "uploads/tmp_1/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	_, _, err = store.Open(ctx, "uploads/tmp_1/preview.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestLocalStoreRejectsUnsafeKeys(t *testing.T) {
	store := NewLocalStore(t.TempDir())
	ctx := context.Background()

	for _, key := range []string{"", "../secret", "/absolute/path", "uploads/../secret", `uploads\bad.png`} {
		if err := store.Save(ctx, key, strings.NewReader("x"), "text/plain"); err == nil {
			t.Fatalf("expected unsafe key %q to be rejected", key)
		}
	}
}

func TestLocalStoreMapsUploadKeysToHistoricalLocalLayout(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStore(root)
	ctx := context.Background()

	if err := store.Save(ctx, "uploads/tmp_1/preview.png", strings.NewReader("png-data"), "image/png"); err != nil {
		t.Fatalf("save: %v", err)
	}

	historicalPath := filepath.Join(root, "tmp_1", "preview.png")
	if _, err := os.Stat(historicalPath); err != nil {
		t.Fatalf("expected asset in historical local path %s: %v", historicalPath, err)
	}
	unexpectedPath := filepath.Join(root, "uploads", "tmp_1", "preview.png")
	if _, err := os.Stat(unexpectedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no nested uploads path, stat err=%v", err)
	}

	reader, _, err := store.Open(ctx, "uploads/tmp_1/preview.png")
	if err != nil {
		t.Fatalf("open logical uploads key: %v", err)
	}
	_ = reader.Close()
}

func TestSupabaseStorageStoreSaveOpenDeletePrefix(t *testing.T) {
	var savedPath string
	var savedAuth string
	var savedAPIKey string
	var savedContentType string
	var savedBody string
	var deleteBody string

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/object/test-bucket/uploads/tmp_1/preview.png":
			savedPath = r.URL.Path
			savedAuth = r.Header.Get("Authorization")
			savedAPIKey = r.Header.Get("apikey")
			savedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			savedBody = string(body)
			return textResponse(r, http.StatusOK, `{"Key":"uploads/tmp_1/preview.png"}`, "application/json"), nil
		case r.Method == http.MethodGet && r.URL.Path == "/storage/v1/object/test-bucket/uploads/tmp_1/preview.png":
			return textResponse(r, http.StatusOK, "remote-png", "image/png"), nil
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/object/test-bucket/list":
			return textResponse(r, http.StatusOK, `[{"name":"preview.png","id":"asset-id"}]`, "application/json"), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/storage/v1/object/test-bucket":
			body, _ := io.ReadAll(r.Body)
			deleteBody = string(body)
			return textResponse(r, http.StatusOK, `[]`, "application/json"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}

	store := NewSupabaseStorageStore(SupabaseStorageConfig{
		BaseURL:    "https://supabase.test",
		ServiceKey: "service-key",
		Bucket:     "test-bucket",
		HTTPClient: client,
	})
	ctx := context.Background()

	if err := store.Save(ctx, "uploads/tmp_1/preview.png", strings.NewReader("png-data"), "image/png"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if savedPath != "/storage/v1/object/test-bucket/uploads/tmp_1/preview.png" {
		t.Fatalf("unexpected save path %q", savedPath)
	}
	if savedAuth != "Bearer service-key" || savedAPIKey != "service-key" {
		t.Fatalf("missing auth headers auth=%q apikey=%q", savedAuth, savedAPIKey)
	}
	if savedContentType != "image/png" {
		t.Fatalf("unexpected content type %q", savedContentType)
	}
	if savedBody != "png-data" {
		t.Fatalf("unexpected body %q", savedBody)
	}

	reader, contentType, err := store.Open(ctx, "uploads/tmp_1/preview.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	body, _ := io.ReadAll(reader)
	_ = reader.Close()
	if contentType != "image/png" || string(body) != "remote-png" {
		t.Fatalf("unexpected open result contentType=%q body=%q", contentType, string(body))
	}

	if err := store.DeletePrefix(ctx, "uploads/tmp_1/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	if !strings.Contains(deleteBody, `"prefixes":["uploads/tmp_1/preview.png"]`) {
		t.Fatalf("unexpected delete body %q", deleteBody)
	}
}

func TestSupabaseStorageStoreCreatesBucketAndRetriesSaveWhenMissing(t *testing.T) {
	var requests []string
	var bucketBody string

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/object/missing-bucket/channel-snapshots/test.jpg" && len(requests) == 1:
			return textResponse(r, http.StatusBadRequest, `{"statusCode":"404","error":"Bucket not found","message":"Bucket not found"}`, "application/json"), nil
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/bucket":
			body, _ := io.ReadAll(r.Body)
			bucketBody = string(body)
			return textResponse(r, http.StatusOK, `{"name":"missing-bucket"}`, "application/json"), nil
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/object/missing-bucket/channel-snapshots/test.jpg":
			return textResponse(r, http.StatusOK, `{"Key":"channel-snapshots/test.jpg"}`, "application/json"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}

	store := NewSupabaseStorageStore(SupabaseStorageConfig{
		BaseURL:    "https://supabase.test",
		ServiceKey: "service-key",
		Bucket:     "missing-bucket",
		HTTPClient: client,
	})

	if err := store.Save(context.Background(), "channel-snapshots/test.jpg", strings.NewReader("jpg-data"), "image/jpeg"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if strings.Join(requests, "\n") != strings.Join([]string{
		"POST /storage/v1/object/missing-bucket/channel-snapshots/test.jpg",
		"POST /storage/v1/bucket",
		"POST /storage/v1/object/missing-bucket/channel-snapshots/test.jpg",
	}, "\n") {
		t.Fatalf("unexpected request sequence: %#v", requests)
	}
	if !strings.Contains(bucketBody, `"id":"missing-bucket"`) || !strings.Contains(bucketBody, `"public":false`) {
		t.Fatalf("unexpected bucket create body %q", bucketBody)
	}
}

func TestNewStoreFromEnvRequiresSupabaseConfig(t *testing.T) {
	t.Setenv("ASSET_STORE", "supabase")
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	t.Setenv("SUPABASE_STORAGE_BUCKET", "")

	_, err := NewStoreFromEnv()
	if err == nil {
		t.Fatalf("expected missing supabase config error")
	}
}

func TestNewStoreFromEnvAutoSelectsSupabaseWhenStorageConfigExists(t *testing.T) {
	t.Setenv("ASSET_STORE", "")
	t.Setenv("SUPABASE_URL", "https://supabase.test")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-key")
	t.Setenv("SUPABASE_STORAGE_BUCKET", "design-plan-assets")

	store, err := NewStoreFromEnv()
	if err != nil {
		t.Fatalf("new store from env: %v", err)
	}
	if _, ok := store.(*SupabaseStorageStore); !ok {
		t.Fatalf("expected SupabaseStorageStore, got %T", store)
	}
	if mode := ModeFromEnv(); mode != "supabase" {
		t.Fatalf("expected mode supabase, got %q", mode)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func textResponse(request *http.Request, status int, body string, contentType string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
