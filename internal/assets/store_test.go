package assets

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
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

func TestOSSStoreSaveOpenDeletePrefix(t *testing.T) {
	var requests []string
	var savedContentType string
	var savedBody string
	var listAuthorization string
	var listDate string

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing Authorization header")
		}
		if r.Header.Get("Date") == "" {
			t.Fatalf("missing Date header")
		}
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/uploads/tmp_1/preview.png":
			savedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			savedBody = string(body)
			return textResponse(r, http.StatusOK, "", "text/plain"), nil
		case r.Method == http.MethodGet && r.URL.Path == "/uploads/tmp_1/preview.png":
			return textResponse(r, http.StatusOK, "oss-png", "image/png"), nil
		case r.Method == http.MethodGet && r.URL.Path == "/" && r.URL.Query().Get("list-type") == "2":
			listAuthorization = r.Header.Get("Authorization")
			listDate = r.Header.Get("Date")
			return textResponse(r, http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Contents><Key>uploads/tmp_1/preview.png</Key></Contents></ListBucketResult>`, "application/xml"), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/uploads/tmp_1/preview.png":
			return textResponse(r, http.StatusNoContent, "", "text/plain"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}

	store := NewOSSStore(OSSConfig{
		Bucket:          "sy-camera-erzhuang-project",
		Endpoint:        "sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		HTTPClient:      client,
	})

	if err := store.Save(context.Background(), "uploads/tmp_1/preview.png", strings.NewReader("png-data"), "image/png"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if savedContentType != "image/png" || savedBody != "png-data" {
		t.Fatalf("unexpected save contentType=%q body=%q", savedContentType, savedBody)
	}

	reader, contentType, err := store.Open(context.Background(), "uploads/tmp_1/preview.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	body, _ := io.ReadAll(reader)
	_ = reader.Close()
	if contentType != "image/png" || string(body) != "oss-png" {
		t.Fatalf("unexpected open contentType=%q body=%q", contentType, string(body))
	}

	if err := store.DeletePrefix(context.Background(), "uploads/tmp_1/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}

	if strings.Join(requests, "\n") != strings.Join([]string{
		"PUT /uploads/tmp_1/preview.png",
		"GET /uploads/tmp_1/preview.png",
		"GET /",
		"DELETE /uploads/tmp_1/preview.png",
	}, "\n") {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	expectedListSignature := ossTestSignature("test-secret", "GET\n\n\n"+listDate+"\n/sy-camera-erzhuang-project/?list-type=2&prefix=uploads/tmp_1/")
	if listAuthorization != "OSS test-id:"+expectedListSignature {
		t.Fatalf("unexpected list authorization: got %q want %q", listAuthorization, "OSS test-id:"+expectedListSignature)
	}
}

func TestOSSStoreOpenMissingMapsToErrNotFound(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
	})}
	store := NewOSSStore(OSSConfig{
		Bucket:          "bucket",
		Endpoint:        "bucket.oss-cn-beijing-internal.aliyuncs.com",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		HTTPClient:      client,
	})

	_, _, err := store.Open(context.Background(), "missing.jpg")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOSSStoreDeletePrefixDoesNotDeleteSiblingPrefix(t *testing.T) {
	var deleted []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if r.URL.Query().Get("prefix") != "uploads/tmp_1/" {
				t.Fatalf("expected exact directory prefix, got %q", r.URL.Query().Get("prefix"))
			}
			return textResponse(r, http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Contents><Key>uploads/tmp_1/preview.png</Key></Contents><Contents><Key>uploads/tmp_10/preview.png</Key></Contents></ListBucketResult>`, "application/xml"), nil
		case r.Method == http.MethodDelete:
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/"))
			return textResponse(r, http.StatusNoContent, "", "text/plain"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}
	store := NewOSSStore(OSSConfig{
		Bucket:          "bucket",
		Endpoint:        "bucket.oss-cn-beijing-internal.aliyuncs.com",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		HTTPClient:      client,
	})

	if err := store.DeletePrefix(context.Background(), "uploads/tmp_1/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	if strings.Join(deleted, ",") != "uploads/tmp_1/preview.png" {
		t.Fatalf("unexpected deleted keys: %#v", deleted)
	}
}

func TestOSSStoreDeletePrefixWithoutTrailingSlashDeletesOnlyExactKey(t *testing.T) {
	var deleted []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if r.URL.Query().Get("prefix") != "uploads/tmp_1" {
				t.Fatalf("expected exact key prefix, got %q", r.URL.Query().Get("prefix"))
			}
			return textResponse(r, http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Contents><Key>uploads/tmp_1</Key></Contents><Contents><Key>uploads/tmp_10</Key></Contents><Contents><Key>uploads/tmp_1/preview.png</Key></Contents></ListBucketResult>`, "application/xml"), nil
		case r.Method == http.MethodDelete:
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/"))
			return textResponse(r, http.StatusNoContent, "", "text/plain"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}
	store := NewOSSStore(OSSConfig{
		Bucket:          "bucket",
		Endpoint:        "bucket.oss-cn-beijing-internal.aliyuncs.com",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		HTTPClient:      client,
	})

	if err := store.DeletePrefix(context.Background(), "uploads/tmp_1"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	if strings.Join(deleted, ",") != "uploads/tmp_1" {
		t.Fatalf("unexpected deleted keys: %#v", deleted)
	}
}

func TestOSSHTTPErrorRedactsSensitiveDiagnostics(t *testing.T) {
	response := textResponse(nil, http.StatusForbidden, `<Error><Code>SignatureDoesNotMatch</Code><Message>sig failed</Message><AccessKeyId>AKIA-LEAK</AccessKeyId><SignatureProvided>secret-signature</SignatureProvided><StringToSign>GET
secret
value</StringToSign></Error>`, "application/xml")

	err := ossHTTPError("open asset", response)
	if err == nil {
		t.Fatalf("expected error")
	}
	message := err.Error()
	for _, secret := range []string{"AKIA-LEAK", "secret-signature", "StringToSign", "SignatureProvided", "GET secret value"} {
		if strings.Contains(message, secret) {
			t.Fatalf("error leaked sensitive diagnostic %q in %q", secret, message)
		}
	}
	if !strings.Contains(message, "SignatureDoesNotMatch") {
		t.Fatalf("expected safe oss code in error, got %q", message)
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

func ossTestSignature(secret string, value string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestNewStoreFromEnvRequiresOSSConfig(t *testing.T) {
	t.Setenv("ASSET_STORE", "oss")
	t.Setenv("OSS_BUCKET", "")
	t.Setenv("OSS_ENDPOINT", "")
	t.Setenv("OSS_ACCESS_KEY_ID", "")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "")

	_, err := NewStoreFromEnv()
	if err == nil {
		t.Fatalf("expected missing oss config error")
	}
	if !strings.Contains(err.Error(), "ASSET_STORE=oss requires") {
		t.Fatalf("unexpected error: %v", err)
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
