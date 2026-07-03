package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultLocalRoot = "uploads"
	defaultBucket    = "design-plan-assets"
)

var ErrNotFound = errors.New("asset not found")

type Store interface {
	Save(ctx context.Context, key string, body io.Reader, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	DeletePrefix(ctx context.Context, prefix string) error
}

func NewStoreFromEnv() (Store, error) {
	mode := ModeFromEnv()
	if mode == "" || mode == "local" {
		root := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
		if root == "" {
			root = defaultLocalRoot
		}
		return NewLocalStore(root), nil
	}
	if mode == "oss" {
		bucket := envValue("OSS_BUCKET", "K8S_SECRET_OSS_BUCKET")
		endpoint := envValue("OSS_ENDPOINT", "K8S_SECRET_OSS_ENDPOINT")
		accessKeyID := envValue("OSS_ACCESS_KEY_ID", "K8S_SECRET_OSS_ACCESS_KEY_ID")
		accessKeySecret := envValue("OSS_ACCESS_KEY_SECRET", "K8S_SECRET_OSS_ACCESS_KEY_SECRET")
		if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
			return nil, errors.New("ASSET_STORE=oss requires OSS_BUCKET, OSS_ENDPOINT, OSS_ACCESS_KEY_ID, and OSS_ACCESS_KEY_SECRET")
		}
		return NewOSSStore(OSSConfig{
			Bucket:          bucket,
			Endpoint:        endpoint,
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
		}), nil
	}
	if mode != "supabase" {
		return nil, fmt.Errorf("unsupported ASSET_STORE %q", mode)
	}
	baseURL := envValue("SUPABASE_URL", "K8S_SECRET_SUPABASE_URL")
	serviceKey := envValue("SUPABASE_SERVICE_ROLE_KEY", "K8S_SECRET_SUPABASE_SERVICE_ROLE_KEY")
	bucket := envValue("SUPABASE_STORAGE_BUCKET", "K8S_SECRET_SUPABASE_STORAGE_BUCKET")
	if bucket == "" {
		bucket = defaultBucket
	}
	if baseURL == "" || serviceKey == "" || bucket == "" {
		return nil, errors.New("ASSET_STORE=supabase requires SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, and SUPABASE_STORAGE_BUCKET")
	}
	return NewSupabaseStorageStore(SupabaseStorageConfig{
		BaseURL:    baseURL,
		ServiceKey: serviceKey,
		Bucket:     bucket,
	}), nil
}

func ModeFromEnv() string {
	mode := strings.ToLower(envValue("ASSET_STORE", "K8S_SECRET_ASSET_STORE"))
	if mode != "" {
		return mode
	}
	if envValue("SUPABASE_URL", "K8S_SECRET_SUPABASE_URL") != "" &&
		envValue("SUPABASE_SERVICE_ROLE_KEY", "K8S_SECRET_SUPABASE_SERVICE_ROLE_KEY") != "" &&
		envValue("SUPABASE_STORAGE_BUCKET", "K8S_SECRET_SUPABASE_STORAGE_BUCKET") != "" {
		return "supabase"
	}
	return "local"
}

func envValue(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func cleanKey(key string) (string, error) {
	clean := filepath.ToSlash(strings.TrimSpace(key))
	clean = strings.TrimSuffix(clean, "/")
	if clean == "" || strings.HasPrefix(clean, "/") || strings.Contains(clean, `\`) {
		return "", fmt.Errorf("invalid asset key")
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid asset key")
		}
	}
	return clean, nil
}

func contentTypeOrDefault(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
