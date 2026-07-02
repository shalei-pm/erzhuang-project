package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
)

var currentOSSSmokeRunner ossSmokeRunner = runOSSSmokeFromEnv

type ossSmokeResponse struct {
	OK          bool   `json:"ok"`
	Key         string `json:"key,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Error       string `json:"error,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

type opsEnvCheckResponse struct {
	OpsEnabled               bool   `json:"ops_enabled"`
	HasOpsEnabled            bool   `json:"has_ops_enabled"`
	HasK8SSecretOpsEnabled   bool   `json:"has_k8s_secret_ops_enabled"`
	AssetStore               string `json:"asset_store"`
	HasAssetStore            bool   `json:"has_asset_store"`
	HasK8SSecretAssetStore   bool   `json:"has_k8s_secret_asset_store"`
	HasOSSBucket             bool   `json:"has_oss_bucket"`
	HasOSSEndpoint           bool   `json:"has_oss_endpoint"`
	HasOSSAccessKeyID        bool   `json:"has_oss_access_key_id"`
	HasOSSAccessKeySecret    bool   `json:"has_oss_access_key_secret"`
	HasK8SOSSBucket          bool   `json:"has_k8s_secret_oss_bucket"`
	HasK8SOSSEndpoint        bool   `json:"has_k8s_secret_oss_endpoint"`
	HasK8SOSSAccessKeyID     bool   `json:"has_k8s_secret_oss_access_key_id"`
	HasK8SOSSAccessKeySecret bool   `json:"has_k8s_secret_oss_access_key_secret"`
}

func (h *Handler) ossEnvCheckHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	writeJSON(w, http.StatusOK, opsEnvCheckResponse{
		OpsEnabled:               opsEnabled(),
		HasOpsEnabled:            envPresent("OPS_ENABLED"),
		HasK8SSecretOpsEnabled:   envPresent("K8S_SECRET_OPS_ENABLED"),
		AssetStore:               envValue("ASSET_STORE", "K8S_SECRET_ASSET_STORE"),
		HasAssetStore:            envPresent("ASSET_STORE"),
		HasK8SSecretAssetStore:   envPresent("K8S_SECRET_ASSET_STORE"),
		HasOSSBucket:             envPresent("OSS_BUCKET") || envPresent("K8S_SECRET_OSS_BUCKET"),
		HasOSSEndpoint:           envPresent("OSS_ENDPOINT") || envPresent("K8S_SECRET_OSS_ENDPOINT"),
		HasOSSAccessKeyID:        envPresent("OSS_ACCESS_KEY_ID") || envPresent("K8S_SECRET_OSS_ACCESS_KEY_ID"),
		HasOSSAccessKeySecret:    envPresent("OSS_ACCESS_KEY_SECRET") || envPresent("K8S_SECRET_OSS_ACCESS_KEY_SECRET"),
		HasK8SOSSBucket:          envPresent("K8S_SECRET_OSS_BUCKET"),
		HasK8SOSSEndpoint:        envPresent("K8S_SECRET_OSS_ENDPOINT"),
		HasK8SOSSAccessKeyID:     envPresent("K8S_SECRET_OSS_ACCESS_KEY_ID"),
		HasK8SOSSAccessKeySecret: envPresent("K8S_SECRET_OSS_ACCESS_KEY_SECRET"),
	})
}

func (h *Handler) ossSmokeHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.ossSmokeRunner(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ossSmokeResponse{
			OK:     false,
			Error:  "oss smoke failed",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, ossSmokeResponse{
		OK:          true,
		Key:         result.Key,
		Bytes:       result.Bytes,
		ContentType: result.ContentType,
	})
}

func opsEnabled() bool {
	return envBool("OPS_ENABLED") || envBool("K8S_SECRET_OPS_ENABLED")
}

func runOSSSmokeFromEnv(ctx context.Context) (*ossSmokeResult, error) {
	if envValue("ASSET_STORE", "K8S_SECRET_ASSET_STORE") == "oss" {
		return runOSSSmokeWithOSSSecretFallback(ctx)
	}
	store, err := assets.NewStoreFromEnv()
	if err != nil {
		return nil, err
	}
	return osssmoke.Run(ctx, store, osssmoke.Options{Apply: true})
}

func runOSSSmokeWithOSSSecretFallback(ctx context.Context) (*ossSmokeResult, error) {
	bucket := envValue("OSS_BUCKET", "K8S_SECRET_OSS_BUCKET")
	endpoint := envValue("OSS_ENDPOINT", "K8S_SECRET_OSS_ENDPOINT")
	accessKeyID := envValue("OSS_ACCESS_KEY_ID", "K8S_SECRET_OSS_ACCESS_KEY_ID")
	accessKeySecret := envValue("OSS_ACCESS_KEY_SECRET", "K8S_SECRET_OSS_ACCESS_KEY_SECRET")
	if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
		return nil, errors.New("ASSET_STORE=oss requires OSS bucket, endpoint, access key id, and access key secret")
	}
	store := assets.NewOSSStore(assets.OSSConfig{
		Bucket:          bucket,
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
	})
	return osssmoke.Run(ctx, store, osssmoke.Options{Apply: true})
}

func envBool(names ...string) bool {
	return strings.EqualFold(envValue(names...), "true")
}

func envPresent(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) != ""
}

func envValue(names ...string) string {
	for _, name := range names {
		value := strings.TrimSpace(os.Getenv(name))
		if value != "" {
			return value
		}
	}
	return ""
}

var opsSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)signature(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)stringtosign(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)oss_access_key_secret(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)oss_access_key_id(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)accesskeysecret(?:[=:][^\s,;]+)?`),
	regexp.MustCompile(`(?i)accesskeyid(?:[=:][^\s,;]+)?`),
}

func sanitizeOpsError(value string) string {
	value = strings.TrimSpace(value)
	for _, pattern := range opsSensitivePatterns {
		value = pattern.ReplaceAllString(value, "[redacted]")
	}
	if len(value) > 320 {
		value = value[:320] + "..."
	}
	if value == "" {
		return "no detail"
	}
	return value
}
