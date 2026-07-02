package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assetmigration"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
)

var currentOSSSmokeRunner ossSmokeRunner = runOSSSmokeFromEnv
var currentAssetMigrationRunner assetMigrationRunner = runAssetMigrationFromEnv
var currentStageASourceSampleRunner stageASourceSampleRunner = runStageASourceSampleFromEnv

const (
	defaultOpsMigrationOrgID   = "10030"
	defaultOpsMigrationMaxRows = 20
	maxOpsMigrationRows        = 100
	maxOpsMigrationBodyBytes   = 2 << 20
	stageASourceSampleKey      = "channel-snapshots/stage-a-10030-channel-1.jpg"
	stageASourceSampleType     = "image/jpeg"
)

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

type assetMigrationRequest struct {
	ManifestCSV   string `json:"manifest_csv"`
	Apply         bool   `json:"apply"`
	ExternalOrgID string `json:"external_org_id"`
	MaxRows       int    `json:"max_rows"`
	BatchID       string `json:"batch_id"`
}

type assetMigrationResponse struct {
	OK            bool                   `json:"ok"`
	Apply         bool                   `json:"apply"`
	ExternalOrgID string                 `json:"external_org_id"`
	MaxRows       int                    `json:"max_rows"`
	Summary       assetmigration.Summary `json:"summary"`
	Results       []assetMigrationResult `json:"results"`
	ResultCSV     string                 `json:"result_csv,omitempty"`
	ResultSQL     string                 `json:"result_sql,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Detail        string                 `json:"detail,omitempty"`
}

type assetMigrationResult struct {
	Action      string `json:"action"`
	ExternalID  string `json:"external_org_id"`
	LogicalKey  string `json:"logical_key"`
	TargetKey   string `json:"target_oss_key"`
	Bytes       int64  `json:"bytes"`
	ContentType string `json:"content_type,omitempty"`
	Error       string `json:"error,omitempty"`
}

type assetMigrationRunRequest struct {
	ManifestCSV   string
	Apply         bool
	ExternalOrgID string
	MaxRows       int
	BatchID       string
}

type assetMigrationRunResult struct {
	Summary   assetmigration.Summary
	Results   []assetmigration.RowResult
	ResultCSV string
	ResultSQL string
	Warnings  []string
}

type stageASourceSampleRequest struct {
	Action string `json:"action"`
}

type stageASourceSampleResponse struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action,omitempty"`
	Key         string `json:"key,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Error       string `json:"error,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

type stageASourceSampleResult struct {
	Action      string
	Key         string
	Bytes       int
	ContentType string
}

func (h *Handler) ossEnvCheckHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	writeJSON(w, http.StatusOK, opsEnvCheckResponse{
		OpsEnabled:               opsEnabled(),
		HasOpsEnabled:            envPresent("OPS_ENABLED"),
		HasK8SSecretOpsEnabled:   envPresent("K8S_SECRET_OPS_ENABLED"),
		AssetStore:               opsAssetStoreMode(),
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

func (h *Handler) assetMigrationHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input assetMigrationRequest
	reader := http.MaxBytesReader(w, r.Body, maxOpsMigrationBodyBytes)
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, assetMigrationResponse{
			OK:     false,
			Error:  "invalid migration request",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	request, err := normalizeAssetMigrationRequest(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, assetMigrationResponse{
			OK:     false,
			Error:  "invalid migration request",
			Detail: err.Error(),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := h.assetMigrationRunner(ctx, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, assetMigrationResponse{
			OK:            false,
			Apply:         request.Apply,
			ExternalOrgID: request.ExternalOrgID,
			MaxRows:       request.MaxRows,
			Error:         "asset migration failed",
			Detail:        sanitizeOpsError(err.Error()),
		})
		return
	}
	status := http.StatusOK
	if result.Summary.Errors > 0 {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, assetMigrationResponse{
		OK:            result.Summary.Errors == 0,
		Apply:         request.Apply,
		ExternalOrgID: request.ExternalOrgID,
		MaxRows:       request.MaxRows,
		Summary:       result.Summary,
		Results:       assetMigrationResultsResponse(result.Results),
		ResultCSV:     result.ResultCSV,
		ResultSQL:     result.ResultSQL,
		Warnings:      result.Warnings,
	})
}

func (h *Handler) stageASourceSampleHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input stageASourceSampleRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, stageASourceSampleResponse{
			OK:     false,
			Error:  "invalid source sample request",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "seed" && action != "cleanup" {
		writeJSON(w, http.StatusBadRequest, stageASourceSampleResponse{
			OK:     false,
			Error:  "invalid source sample request",
			Detail: "action must be seed or cleanup",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.stageASampleRunner(ctx, action)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, stageASourceSampleResponse{
			OK:     false,
			Error:  "stage-a source sample failed",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, stageASourceSampleResponse{
		OK:          true,
		Action:      result.Action,
		Key:         result.Key,
		Bytes:       result.Bytes,
		ContentType: result.ContentType,
	})
}

func opsEnabled() bool {
	return envBool("OPS_ENABLED") || envBool("K8S_SECRET_OPS_ENABLED")
}

func runOSSSmokeFromEnv(ctx context.Context) (*ossSmokeResult, error) {
	if opsAssetStoreMode() == "oss" {
		return runOSSSmokeWithOSSSecretFallback(ctx)
	}
	store, err := assets.NewStoreFromEnv()
	if err != nil {
		return nil, err
	}
	return osssmoke.Run(ctx, store, osssmoke.Options{Apply: true})
}

func opsAssetStoreMode() string {
	return envValue("K8S_SECRET_ASSET_STORE", "ASSET_STORE")
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

func normalizeAssetMigrationRequest(input assetMigrationRequest) (assetMigrationRunRequest, error) {
	manifest := strings.TrimSpace(input.ManifestCSV)
	if manifest == "" {
		return assetMigrationRunRequest{}, errors.New("manifest_csv is required")
	}
	externalOrgID := strings.TrimSpace(input.ExternalOrgID)
	if externalOrgID == "" {
		externalOrgID = defaultOpsMigrationOrgID
	}
	maxRows := input.MaxRows
	if maxRows <= 0 {
		maxRows = defaultOpsMigrationMaxRows
	}
	if maxRows > maxOpsMigrationRows {
		return assetMigrationRunRequest{}, fmt.Errorf("max_rows must be <= %d", maxOpsMigrationRows)
	}
	if input.Apply && externalOrgID != defaultOpsMigrationOrgID {
		return assetMigrationRunRequest{}, fmt.Errorf("apply is currently limited to external_org_id %s", defaultOpsMigrationOrgID)
	}
	return assetMigrationRunRequest{
		ManifestCSV:   manifest,
		Apply:         input.Apply,
		ExternalOrgID: externalOrgID,
		MaxRows:       maxRows,
		BatchID:       strings.TrimSpace(input.BatchID),
	}, nil
}

func runAssetMigrationFromEnv(ctx context.Context, request assetMigrationRunRequest) (*assetMigrationRunResult, error) {
	rows, err := assetmigration.ReadManifest(strings.NewReader(request.ManifestCSV))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	source, err := sourceAssetStoreForMigration()
	if err != nil {
		return nil, fmt.Errorf("create source store: %w", err)
	}
	target, bucket, err := targetOSSStoreForMigration()
	if err != nil {
		return nil, fmt.Errorf("create target store: %w", err)
	}
	summary, results := assetmigration.CopyManifest(ctx, source, target, rows, assetmigration.Options{
		Apply:         request.Apply,
		ExternalOrgID: request.ExternalOrgID,
		MaxRows:       request.MaxRows,
	})
	resultCSV, err := assetMigrationResultsCSV(results)
	if err != nil {
		return nil, fmt.Errorf("write result csv: %w", err)
	}
	var resultSQL string
	warnings := []string{
		"Result SQL only updates existing tb_asset_objects rows. Confirm pending rows exist before executing it.",
	}
	if request.Apply {
		var sqlBuffer bytes.Buffer
		if err := assetmigration.WriteResultSQL(&sqlBuffer, results, assetmigration.SQLUpdateOptions{
			Bucket:  bucket,
			BatchID: request.BatchID,
		}); err != nil {
			return nil, fmt.Errorf("write result sql: %w", err)
		}
		resultSQL = sqlBuffer.String()
		warnings = append(warnings, "Review result_sql before running it in MySQL. The migration endpoint does not write database state.")
	}
	return &assetMigrationRunResult{
		Summary:   summary,
		Results:   results,
		ResultCSV: resultCSV,
		ResultSQL: resultSQL,
		Warnings:  warnings,
	}, nil
}

func sourceAssetStoreForMigration() (assets.Store, error) {
	mode := strings.ToLower(envValue("SOURCE_ASSET_STORE", "ASSET_STORE"))
	if mode == "" {
		mode = assets.ModeFromEnv()
	}
	switch mode {
	case "", "local":
		root := envValue("SOURCE_UPLOAD_DIR", "UPLOAD_DIR")
		if root == "" {
			root = "uploads"
		}
		return assets.NewLocalStore(root), nil
	case "supabase":
		baseURL := envValue("SOURCE_SUPABASE_URL", "SUPABASE_URL")
		serviceKey := envValue("SOURCE_SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_SERVICE_ROLE_KEY")
		bucket := envValue("SOURCE_SUPABASE_STORAGE_BUCKET", "SUPABASE_STORAGE_BUCKET")
		if bucket == "" {
			bucket = "design-plan-assets"
		}
		if baseURL == "" || serviceKey == "" || bucket == "" {
			return nil, errors.New("SOURCE_ASSET_STORE=supabase requires Supabase URL, service key, and bucket")
		}
		return assets.NewSupabaseStorageStore(assets.SupabaseStorageConfig{
			BaseURL:    baseURL,
			ServiceKey: serviceKey,
			Bucket:     bucket,
		}), nil
	case "oss":
		bucket := envValue("SOURCE_OSS_BUCKET", "OSS_BUCKET", "K8S_SECRET_OSS_BUCKET")
		endpoint := envValue("SOURCE_OSS_ENDPOINT", "OSS_ENDPOINT", "K8S_SECRET_OSS_ENDPOINT")
		accessKeyID := envValue("SOURCE_OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID", "K8S_SECRET_OSS_ACCESS_KEY_ID")
		accessKeySecret := envValue("SOURCE_OSS_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET", "K8S_SECRET_OSS_ACCESS_KEY_SECRET")
		if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
			return nil, errors.New("SOURCE_ASSET_STORE=oss requires OSS bucket, endpoint, access key id, and access key secret")
		}
		return assets.NewOSSStore(assets.OSSConfig{
			Bucket:          bucket,
			Endpoint:        endpoint,
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported source asset store %q", mode)
	}
}

func targetOSSStoreForMigration() (assets.Store, string, error) {
	mode := strings.ToLower(envValue("TARGET_ASSET_STORE", "K8S_SECRET_ASSET_STORE"))
	if mode == "" {
		mode = "oss"
	}
	if mode != "oss" {
		return nil, "", fmt.Errorf("target asset store must be oss, got %q", mode)
	}
	bucket := envValue("TARGET_OSS_BUCKET", "K8S_SECRET_OSS_BUCKET", "OSS_BUCKET")
	endpoint := envValue("TARGET_OSS_ENDPOINT", "K8S_SECRET_OSS_ENDPOINT", "OSS_ENDPOINT")
	accessKeyID := envValue("TARGET_OSS_ACCESS_KEY_ID", "K8S_SECRET_OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID")
	accessKeySecret := envValue("TARGET_OSS_ACCESS_KEY_SECRET", "K8S_SECRET_OSS_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET")
	if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
		return nil, "", errors.New("target OSS requires bucket, endpoint, access key id, and access key secret")
	}
	return assets.NewOSSStore(assets.OSSConfig{
		Bucket:          bucket,
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
	}), bucket, nil
}

func runStageASourceSampleFromEnv(ctx context.Context, action string) (*stageASourceSampleResult, error) {
	store, err := sourceAssetStoreForMigration()
	if err != nil {
		return nil, err
	}
	switch action {
	case "seed":
		payload := stageASourceSampleJPEG()
		if err := store.Save(ctx, stageASourceSampleKey, bytes.NewReader(payload), stageASourceSampleType); err != nil {
			return nil, fmt.Errorf("seed source sample: %w", err)
		}
		return &stageASourceSampleResult{
			Action:      "seeded",
			Key:         stageASourceSampleKey,
			Bytes:       len(payload),
			ContentType: stageASourceSampleType,
		}, nil
	case "cleanup":
		if err := deleteExactAsset(ctx, store, stageASourceSampleKey); err != nil {
			return nil, fmt.Errorf("cleanup source sample: %w", err)
		}
		return &stageASourceSampleResult{
			Action: "cleaned",
			Key:    stageASourceSampleKey,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported source sample action %q", action)
	}
}

type exactAssetDeleter interface {
	Delete(ctx context.Context, key string) error
}

func deleteExactAsset(ctx context.Context, store assets.Store, key string) error {
	if deleter, ok := store.(exactAssetDeleter); ok {
		return deleter.Delete(ctx, key)
	}
	return store.DeletePrefix(ctx, key)
}

func stageASourceSampleJPEG() []byte {
	return []byte{
		0xff, 0xd8,
		0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00,
		0xff, 0xdb, 0x00, 0x43, 0x00,
		0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0a, 0x0c, 0x14,
		0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12, 0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a,
		0x1c, 0x1c, 0x20, 0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29, 0x2c,
		0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32, 0x3c, 0x2e, 0x33, 0x34, 0x32,
		0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01, 0x00, 0x01, 0x01, 0x01, 0x11, 0x00,
		0xff, 0xc4, 0x00, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
		0xff, 0xc4, 0x00, 0x14, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0x37,
		0xff, 0xd9,
	}
}

func assetMigrationResultsCSV(results []assetmigration.RowResult) (string, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"action", "external_org_id", "logical_key", "target_oss_key", "bytes", "content_type", "error"}); err != nil {
		return "", err
	}
	for _, result := range results {
		if err := writer.Write([]string{
			result.Action,
			result.Row.ExternalOrgID,
			result.Row.LogicalKey,
			result.Row.TargetOSSKey,
			fmt.Sprintf("%d", result.Bytes),
			result.ContentType,
			sanitizeOptionalOpsError(result.Error),
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return buffer.String(), writer.Error()
}

func assetMigrationResultsResponse(results []assetmigration.RowResult) []assetMigrationResult {
	response := make([]assetMigrationResult, 0, len(results))
	for _, result := range results {
		response = append(response, assetMigrationResult{
			Action:      result.Action,
			ExternalID:  result.Row.ExternalOrgID,
			LogicalKey:  result.Row.LogicalKey,
			TargetKey:   result.Row.TargetOSSKey,
			Bytes:       result.Bytes,
			ContentType: result.ContentType,
			Error:       sanitizeOptionalOpsError(result.Error),
		})
	}
	return response
}

func sanitizeOptionalOpsError(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return sanitizeOpsError(value)
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
