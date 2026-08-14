package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type SupabaseStorageConfig struct {
	BaseURL    string
	ServiceKey string
	Bucket     string
	HTTPClient *http.Client
}

type SupabaseStorageStore struct {
	baseURL    string
	serviceKey string
	bucket     string
	client     *http.Client
}

func NewSupabaseStorageStore(config SupabaseStorageConfig) *SupabaseStorageStore {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &SupabaseStorageStore{
		baseURL:    strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"),
		serviceKey: strings.TrimSpace(config.ServiceKey),
		bucket:     strings.Trim(strings.TrimSpace(config.Bucket), "/"),
		client:     client,
	}
}

func (s *SupabaseStorageStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	clean, err := cleanKey(key)
	if err != nil {
		return err
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	return s.saveBytes(ctx, clean, payload, contentType, true)
}

func (s *SupabaseStorageStore) saveBytes(ctx context.Context, clean string, payload []byte, contentType string, retryMissingBucket bool) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.objectURL(clean), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	s.authorize(request)
	request.Header.Set("Content-Type", contentTypeOrDefault(contentType))
	request.Header.Set("x-upsert", "true")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		storageErr := storageHTTPError("save asset", response)
		if retryMissingBucket && isBucketNotFoundError(storageErr) {
			if err := s.ensureBucket(ctx); err != nil {
				return err
			}
			return s.saveBytes(ctx, clean, payload, contentType, false)
		}
		return storageErr
	}
	return nil
}

func (s *SupabaseStorageStore) ensureBucket(ctx context.Context) error {
	payload, err := json.Marshal(map[string]any{
		"id":     s.bucket,
		"name":   s.bucket,
		"public": false,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/storage/v1/bucket", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	s.authorize(request)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		storageErr := storageHTTPError("create bucket", response)
		if isAlreadyExistsError(storageErr) {
			return nil
		}
		return storageErr
	}
	return nil
}

func (s *SupabaseStorageStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return nil, "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(clean), nil)
	if err != nil {
		return nil, "", err
	}
	s.authorize(request)
	response, err := s.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return nil, "", ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, "", storageHTTPError("open asset", response)
	}
	return response.Body, contentTypeOrDefault(response.Header.Get("Content-Type")), nil
}

func (s *SupabaseStorageStore) DeletePrefix(ctx context.Context, prefix string) error {
	isDirectoryPrefix := strings.HasSuffix(strings.TrimSpace(prefix), "/")
	cleanPrefix, err := cleanKey(prefix)
	if err != nil {
		return err
	}
	keys, err := s.listKeys(ctx, cleanPrefix, isDirectoryPrefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string][]string{"prefixes": keys})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.bucketURL(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	s.authorize(request)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return storageHTTPError("delete assets", response)
	}
	return nil
}

func (s *SupabaseStorageStore) Delete(ctx context.Context, key string) error {
	clean, err := cleanKey(key)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string][]string{"prefixes": []string{clean}})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.bucketURL(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	s.authorize(request)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return storageHTTPError("delete asset", response)
	}
	return nil
}

func (s *SupabaseStorageStore) listKeys(ctx context.Context, prefix string, isDirectoryPrefix bool) ([]string, error) {
	dir, namePrefix := splitPrefix(prefix, isDirectoryPrefix)
	payload, err := json.Marshal(map[string]any{
		"prefix": dir,
		"limit":  1000,
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.bucketURL()+"/list", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	s.authorize(request)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, storageHTTPError("list assets", response)
	}
	var entries []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&entries); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == "" || !strings.HasPrefix(entry.Name, namePrefix) {
			continue
		}
		keys = append(keys, path.Join(dir, entry.Name))
	}
	return keys, nil
}

func splitPrefix(prefix string, isDirectoryPrefix bool) (string, string) {
	clean := strings.TrimSuffix(prefix, "/")
	if isDirectoryPrefix {
		return clean, ""
	}
	dir, file := path.Split(clean)
	return strings.TrimSuffix(dir, "/"), file
}

func (s *SupabaseStorageStore) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+s.serviceKey)
	request.Header.Set("apikey", s.serviceKey)
}

func (s *SupabaseStorageStore) objectURL(key string) string {
	return s.bucketURL() + "/" + escapePath(key)
}

func (s *SupabaseStorageStore) bucketURL() string {
	return s.baseURL + "/storage/v1/object/" + url.PathEscape(s.bucket)
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func storageHTTPError(action string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = response.Status
	}
	return fmt.Errorf("%s failed: http %d %s", action, response.StatusCode, message)
}

func isBucketNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "bucket not found")
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exists") || strings.Contains(message, "duplicate")
}
