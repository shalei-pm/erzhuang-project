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
	if mode != "supabase" {
		return nil, fmt.Errorf("unsupported ASSET_STORE %q", mode)
	}
	baseURL := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	serviceKey := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	bucket := strings.TrimSpace(os.Getenv("SUPABASE_STORAGE_BUCKET"))
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
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ASSET_STORE")))
	if mode != "" {
		return mode
	}
	if strings.TrimSpace(os.Getenv("SUPABASE_URL")) != "" &&
		strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("SUPABASE_STORAGE_BUCKET")) != "" {
		return "supabase"
	}
	return "local"
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
