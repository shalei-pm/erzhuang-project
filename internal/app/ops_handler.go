package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assetmigration"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"

	_ "github.com/go-sql-driver/mysql"
)

var currentOSSSmokeRunner ossSmokeRunner = runOSSSmokeFromEnv
var currentAssetMigrationRunner assetMigrationRunner = runAssetMigrationFromEnv
var currentAssetStateBackfillRunner assetStateBackfillRunner = runAssetStateBackfillFromEnv
var currentStageASourceSampleRunner stageASourceSampleRunner = runStageASourceSampleFromEnv
var currentStageATargetSampleRunner stageATargetSampleRunner = runStageATargetSampleFromEnv
var currentMySQLCanaryImportRunner mysqlCanaryImportRunner = runMySQLCanaryImportFromEnv
var currentMySQLCanaryValidateRunner mysqlCanaryValidateRunner = runMySQLCanaryValidateFromEnv
var currentMySQLAssetInventoryRunner mysqlAssetInventoryRunner = runMySQLAssetInventoryFromEnv

const (
	defaultOpsMigrationOrgID   = "10030"
	defaultOpsMigrationMaxRows = 20
	maxOpsMigrationRows        = 100
	maxOpsMigrationBodyBytes   = 2 << 20
	maxOpsExportOrgCount       = 5
	maxOpsCanaryImportBytes    = 1 << 20
	stageASourceSampleKey      = "channel-snapshots/stage-a-10030-channel-1.jpg"
	stageASourceSampleType     = "image/jpeg"
)

func allowedOpsMigrationOrgIDs() []string {
	ids := []string{defaultOpsMigrationOrgID}
	configured := envValue("OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS", "K8S_SECRET_OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS")
	for _, id := range strings.Split(configured, ",") {
		clean := strings.TrimSpace(id)
		if clean != "" {
			ids = append(ids, clean)
		}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func opsMigrationOrgAllowed(externalOrgID string) bool {
	clean := strings.TrimSpace(externalOrgID)
	for _, allowed := range allowedOpsMigrationOrgIDs() {
		if clean == allowed {
			return true
		}
	}
	return false
}

func allowedOpsMigrationOrgIDsText() string {
	allowed := allowedOpsMigrationOrgIDs()
	if len(allowed) == 0 {
		return ""
	}
	text := allowed[0]
	for _, id := range allowed[1:] {
		text += "," + id
	}
	return text
}

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

type assetStateBackfillRequest struct {
	ExternalOrgID string `json:"external_org_id"`
	ManifestCSV   string `json:"manifest_csv"`
	ResultCSV     string `json:"result_csv"`
	BatchID       string `json:"batch_id"`
}

type assetStateBackfillResponse struct {
	OK            bool                      `json:"ok"`
	ExternalOrgID string                    `json:"external_org_id,omitempty"`
	Summary       assetStateBackfillSummary `json:"summary,omitempty"`
	Warnings      []string                  `json:"warnings,omitempty"`
	Error         string                    `json:"error,omitempty"`
	Detail        string                    `json:"detail,omitempty"`
}

type assetStateBackfillSummary struct {
	Total    int `json:"total"`
	Migrated int `json:"migrated"`
	Skipped  int `json:"skipped"`
	Upserted int `json:"upserted"`
	Errors   int `json:"errors"`
}

type assetStateBackfillRunRequest struct {
	ExternalOrgID string
	ManifestCSV   string
	ResultCSV     string
	BatchID       string
}

type assetStateBackfillRunResult struct {
	Summary  assetStateBackfillSummary
	Warnings []string
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

type stageATargetSampleRequest struct {
	Action string `json:"action"`
}

type stageATargetSampleResponse struct {
	OK     bool   `json:"ok"`
	Action string `json:"action,omitempty"`
	Key    string `json:"key,omitempty"`
	Error  string `json:"error,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type stageATargetSampleResult struct {
	Action string
	Key    string
}

type mysqlCanaryImportRequest struct {
	ExternalOrgID string `json:"external_org_id"`
	ImportSQL     string `json:"import_sql"`
	Apply         bool   `json:"apply"`
	BatchID       string `json:"batch_id"`
}

type mysqlCanaryImportResponse struct {
	OK            bool                     `json:"ok"`
	Apply         bool                     `json:"apply"`
	ExternalOrgID string                   `json:"external_org_id,omitempty"`
	Summary       mysqlCanaryImportSummary `json:"summary,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Error         string                   `json:"error,omitempty"`
	Detail        string                   `json:"detail,omitempty"`
}

type mysqlCanaryValidateResponse struct {
	OK            bool                     `json:"ok"`
	ExternalOrgID string                   `json:"external_org_id,omitempty"`
	Summary       mysqlCanaryImportSummary `json:"summary,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Error         string                   `json:"error,omitempty"`
	Detail        string                   `json:"detail,omitempty"`
}

type mysqlCanaryImportSummary struct {
	StoreCount        int `json:"store_count"`
	RecorderCount     int `json:"recorder_count"`
	ChannelCount      int `json:"channel_count"`
	SnapshotCount     int `json:"snapshot_count"`
	OperationLogCount int `json:"operation_log_count"`
	UserCount         int `json:"user_count"`
	OrphanCount       int `json:"orphan_count"`
	InvalidJSONCount  int `json:"invalid_json_count"`
}

type mysqlCanaryImportRunRequest struct {
	ExternalOrgID string
	ImportSQL     string
	Apply         bool
	BatchID       string
}

type mysqlCanaryImportRunResult struct {
	Summary  mysqlCanaryImportSummary
	Warnings []string
}

type mysqlCanaryValidateResult struct {
	Summary  mysqlCanaryImportSummary
	Warnings []string
}

type mysqlAssetInventoryResponse struct {
	OK            bool                       `json:"ok"`
	ExternalOrgID string                     `json:"external_org_id,omitempty"`
	Summary       mysqlAssetInventorySummary `json:"summary,omitempty"`
	ManifestCSV   string                     `json:"manifest_csv,omitempty"`
	Warnings      []string                   `json:"warnings,omitempty"`
	Error         string                     `json:"error,omitempty"`
	Detail        string                     `json:"detail,omitempty"`
}

type mysqlAssetInventorySummary struct {
	Total         int `json:"total"`
	Pending       int `json:"pending"`
	Skipped       int `json:"skipped"`
	Sensitive     int `json:"sensitive"`
	DesignRows    int `json:"design_rows"`
	SnapshotRows  int `json:"snapshot_rows"`
	DuplicateRefs int `json:"duplicate_refs"`
}

type mysqlAssetInventoryRunRequest struct {
	ExternalOrgID string
}

type mysqlAssetInventoryRunResult struct {
	Summary     mysqlAssetInventorySummary
	ManifestCSV string
	Warnings    []string
}

type mysqlAssetInventoryRawRow struct {
	SourceTable         string
	SourceID            string
	SourceColumn        string
	AssetRole           string
	StoreID             string
	ExternalOrgID       string
	RecorderID          string
	ChannelID           string
	OwnerEntityType     string
	OwnerEntityID       string
	UploadID            string
	OldPath             string
	SourceKey           string
	ProxyPath           string
	ExpectedContentType string
	Sensitivity         string
}

type mysqlAssetState struct {
	MigrationStatus string
	StorageProvider string
	Bucket          string
	StorageKey      string
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

func (h *Handler) assetStateBackfillHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input assetStateBackfillRequest
	reader := http.MaxBytesReader(w, r.Body, maxOpsMigrationBodyBytes)
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, assetStateBackfillResponse{
			OK:     false,
			Error:  "invalid backfill request",
			Detail: err.Error(),
		})
		return
	}
	request, err := normalizeAssetStateBackfillRequest(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, assetStateBackfillResponse{
			OK:            false,
			ExternalOrgID: strings.TrimSpace(input.ExternalOrgID),
			Error:         "invalid backfill request",
			Detail:        err.Error(),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.assetStateBackfillRunner(ctx, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, assetStateBackfillResponse{
			OK:            false,
			ExternalOrgID: request.ExternalOrgID,
			Error:         "asset state backfill failed",
			Detail:        sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, assetStateBackfillResponse{
		OK:            true,
		ExternalOrgID: request.ExternalOrgID,
		Summary:       result.Summary,
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

func (h *Handler) stageATargetSampleHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input stageATargetSampleRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, stageATargetSampleResponse{
			OK:     false,
			Error:  "invalid target sample request",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "cleanup" {
		writeJSON(w, http.StatusBadRequest, stageATargetSampleResponse{
			OK:     false,
			Error:  "invalid target sample request",
			Detail: "action must be cleanup",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.stageATargetRunner(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, stageATargetSampleResponse{
			OK:     false,
			Error:  "stage-a target sample failed",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, stageATargetSampleResponse{
		OK:     true,
		Action: result.Action,
		Key:    result.Key,
	})
}

func (h *Handler) mysqlCanaryImportHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	var input mysqlCanaryImportRequest
	reader := http.MaxBytesReader(w, r.Body, maxOpsCanaryImportBytes)
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, mysqlCanaryImportResponse{
			OK:     false,
			Error:  "invalid canary import request",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	request, err := normalizeMySQLCanaryImportRequest(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, mysqlCanaryImportResponse{
			OK:            false,
			Apply:         input.Apply,
			ExternalOrgID: strings.TrimSpace(input.ExternalOrgID),
			Error:         "invalid canary import request",
			Detail:        err.Error(),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := h.mysqlCanaryRunner(ctx, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, mysqlCanaryImportResponse{
			OK:            false,
			Apply:         request.Apply,
			ExternalOrgID: request.ExternalOrgID,
			Error:         "mysql canary import failed",
			Detail:        sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, mysqlCanaryImportResponse{
		OK:            true,
		Apply:         request.Apply,
		ExternalOrgID: request.ExternalOrgID,
		Summary:       result.Summary,
		Warnings:      result.Warnings,
	})
}

func (h *Handler) mysqlCanaryValidateHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	externalOrgID := strings.TrimSpace(r.URL.Query().Get("external_org_id"))
	if externalOrgID == "" {
		externalOrgID = defaultOpsMigrationOrgID
	}
	if !opsMigrationOrgAllowed(externalOrgID) {
		writeJSON(w, http.StatusBadRequest, mysqlCanaryValidateResponse{
			OK:            false,
			ExternalOrgID: externalOrgID,
			Error:         "invalid canary validate request",
			Detail:        fmt.Sprintf("mysql canary validation is limited to external_org_id in %s", allowedOpsMigrationOrgIDsText()),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.mysqlValidateRunner(ctx, externalOrgID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, mysqlCanaryValidateResponse{
			OK:            false,
			ExternalOrgID: externalOrgID,
			Error:         "mysql canary validation failed",
			Detail:        sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, mysqlCanaryValidateResponse{
		OK:            true,
		ExternalOrgID: externalOrgID,
		Summary:       result.Summary,
		Warnings:      result.Warnings,
	})
}

func (h *Handler) mysqlAssetInventoryHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	externalOrgID := strings.TrimSpace(r.URL.Query().Get("external_org_id"))
	if externalOrgID == "" {
		externalOrgID = defaultOpsMigrationOrgID
	}
	if !opsMigrationOrgAllowed(externalOrgID) {
		writeJSON(w, http.StatusBadRequest, mysqlAssetInventoryResponse{
			OK:            false,
			ExternalOrgID: externalOrgID,
			Error:         "invalid asset inventory request",
			Detail:        fmt.Sprintf("mysql asset inventory is limited to external_org_id in %s", allowedOpsMigrationOrgIDsText()),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.mysqlInventoryRunner(ctx, mysqlAssetInventoryRunRequest{ExternalOrgID: externalOrgID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, mysqlAssetInventoryResponse{
			OK:            false,
			ExternalOrgID: externalOrgID,
			Error:         "mysql asset inventory failed",
			Detail:        sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, mysqlAssetInventoryResponse{
		OK:            true,
		ExternalOrgID: externalOrgID,
		Summary:       result.Summary,
		ManifestCSV:   result.ManifestCSV,
		Warnings:      result.Warnings,
	})
}

func opsEnabled() bool {
	return envBool("OPS_ENABLED") || envBool("K8S_SECRET_OPS_ENABLED")
}

func normalizeOpsExportOrgIDs(value string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, part := range strings.Split(value, ",") {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func normalizeMySQLCanaryImportRequest(input mysqlCanaryImportRequest) (mysqlCanaryImportRunRequest, error) {
	externalOrgID := strings.TrimSpace(input.ExternalOrgID)
	if externalOrgID == "" {
		externalOrgID = defaultOpsMigrationOrgID
	}
	if !opsMigrationOrgAllowed(externalOrgID) {
		return mysqlCanaryImportRunRequest{}, fmt.Errorf("mysql canary import is limited to external_org_id in %s", allowedOpsMigrationOrgIDsText())
	}
	importSQL := strings.TrimSpace(input.ImportSQL)
	if importSQL == "" {
		return mysqlCanaryImportRunRequest{}, errors.New("import_sql is required")
	}
	if !strings.Contains(importSQL, "-- Scope external_org_id: "+externalOrgID) {
		return mysqlCanaryImportRunRequest{}, fmt.Errorf("import_sql must include scope comment for external_org_id %s", externalOrgID)
	}
	if mentionsOtherStoreExternalOrgID(importSQL, externalOrgID) {
		return mysqlCanaryImportRunRequest{}, errors.New("import_sql appears to include a non-canary store external_org_id")
	}
	return mysqlCanaryImportRunRequest{
		ExternalOrgID: externalOrgID,
		ImportSQL:     importSQL,
		Apply:         input.Apply,
		BatchID:       strings.TrimSpace(input.BatchID),
	}, nil
}

var tbStoresExternalOrgPattern = regexp.MustCompile("insert into `tb_stores`[^\\n]*values \\([^\\n]*'([0-9]{5,})'[^\\n]*\\)")

func mentionsOtherStoreExternalOrgID(importSQL string, expected string) bool {
	for _, match := range tbStoresExternalOrgPattern.FindAllStringSubmatch(importSQL, -1) {
		if len(match) > 1 && match[1] != expected {
			return true
		}
	}
	return false
}

func runMySQLCanaryImportFromEnv(ctx context.Context, request mysqlCanaryImportRunRequest) (*mysqlCanaryImportRunResult, error) {
	dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if err := ensureMySQLCanaryTables(ctx, db); err != nil {
		return nil, err
	}
	if request.Apply {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin mysql import transaction: %w", err)
		}
		if _, err := tx.ExecContext(ctx, request.ImportSQL); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("execute mysql canary import: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit mysql canary import: %w", err)
		}
	}
	summary, err := queryMySQLCanarySummary(ctx, db, request.ExternalOrgID)
	if err != nil {
		return nil, err
	}
	warnings := []string{
		"Canary endpoint is limited to external_org_id 10030.",
	}
	if !request.Apply {
		warnings = append(warnings, "Dry-run connected to MySQL and validated schema but did not execute import_sql.")
	}
	return &mysqlCanaryImportRunResult{Summary: summary, Warnings: warnings}, nil
}

func runMySQLCanaryValidateFromEnv(ctx context.Context, externalOrgID string) (*mysqlCanaryValidateResult, error) {
	dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if err := ensureMySQLCanaryTables(ctx, db); err != nil {
		return nil, err
	}
	summary, err := queryMySQLCanarySummary(ctx, db, externalOrgID)
	if err != nil {
		return nil, err
	}
	return &mysqlCanaryValidateResult{
		Summary: summary,
		Warnings: []string{
			"Read-only validation. This endpoint does not execute import SQL or mutate MySQL.",
			"Canary endpoint is limited to external_org_id 10030.",
		},
	}, nil
}

func runMySQLAssetInventoryFromEnv(ctx context.Context, request mysqlAssetInventoryRunRequest) (*mysqlAssetInventoryRunResult, error) {
	dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if err := ensureMySQLCanaryTables(ctx, db); err != nil {
		return nil, err
	}
	rows, err := queryMySQLAssetInventoryRows(ctx, db, request.ExternalOrgID)
	if err != nil {
		return nil, err
	}
	assetStates, err := queryMySQLAssetStates(ctx, db, rows)
	if err != nil {
		return nil, err
	}
	result, err := buildMySQLAssetInventory(rows, assetStates)
	if err != nil {
		return nil, err
	}
	result.Warnings = append(result.Warnings,
		"Read-only inventory. This endpoint does not copy assets or mutate MySQL.",
		"Review manifest_csv before passing it to asset-migrate.",
	)
	return result, nil
}

func queryMySQLAssetInventoryRows(ctx context.Context, db *sql.DB, externalOrgID string) ([]mysqlAssetInventoryRawRow, error) {
	rows := []mysqlAssetInventoryRawRow{}
	queries := []string{
		`select
			'tb_store_design_plans', cast(p.id as char), 'original_pdf_path', 'design_original',
			cast(p.store_id as char), s.external_org_id, '', '',
			'store_design_plan', cast(p.id as char), p.upload_id,
			p.original_pdf_path, p.original_pdf_path,
			concat('/api/design-plan/uploads/', p.upload_id, '/original'),
			'application/pdf', 'internal'
		from tb_store_design_plans p, tb_stores s
		where s.id = p.store_id and s.external_org_id = ? and coalesce(p.original_pdf_path, '') <> ''`,
		`select
			'tb_store_design_plans', cast(p.id as char), 'preview_image_path', 'design_preview',
			cast(p.store_id as char), s.external_org_id, '', '',
			'store_design_plan', cast(p.id as char), p.upload_id,
			p.preview_image_path, p.preview_image_path,
			concat('/api/design-plan/uploads/', p.upload_id, '/preview'),
			'image/png', 'internal'
		from tb_store_design_plans p, tb_stores s
		where s.id = p.store_id and s.external_org_id = ? and coalesce(p.preview_image_path, '') <> ''`,
		`select
			'tb_store_design_plans', cast(p.id as char), 'thumbnail_path', 'design_thumbnail',
			cast(p.store_id as char), s.external_org_id, '', '',
			'store_design_plan', cast(p.id as char), p.upload_id,
			p.thumbnail_path, p.thumbnail_path,
			concat('/api/design-plan/uploads/', p.upload_id, '/thumbnail'),
			'image/png', 'internal'
		from tb_store_design_plans p, tb_stores s
		where s.id = p.store_id and s.external_org_id = ? and coalesce(p.thumbnail_path, '') <> ''`,
		`select
			'tb_channel_snapshots', cast(cs.id as char), 'thumbnail_path', 'snapshot_thumbnail',
			cast(r.store_id as char), s.external_org_id, cast(r.id as char), cast(c.id as char),
			'video_channel', cast(c.id as char), '',
			cs.thumbnail_path, cs.thumbnail_path, cs.thumbnail_path,
			'image/jpeg', 'sensitive'
		from tb_channel_snapshots cs, tb_video_channels c, tb_video_recorders r, tb_stores s
		where c.id = cs.channel_id and r.id = c.recorder_id and s.id = r.store_id and s.external_org_id = ? and coalesce(cs.thumbnail_path, '') <> ''
			and not exists (
				select 1 from tb_channel_snapshots newer
				where newer.channel_id = cs.channel_id
					and (newer.created_at > cs.created_at or (newer.created_at = cs.created_at and newer.id > cs.id))
			)`,
		`select
			'tb_channel_snapshots', cast(cs.id as char), 'full_image_path', 'snapshot_full_image',
			cast(r.store_id as char), s.external_org_id, cast(r.id as char), cast(c.id as char),
			'video_channel', cast(c.id as char), '',
			cs.full_image_path, cs.full_image_path, cs.full_image_path,
			'image/jpeg', 'sensitive'
		from tb_channel_snapshots cs, tb_video_channels c, tb_video_recorders r, tb_stores s
		where c.id = cs.channel_id and r.id = c.recorder_id and s.id = r.store_id and s.external_org_id = ? and coalesce(cs.full_image_path, '') <> ''
			and not exists (
				select 1 from tb_channel_snapshots newer
				where newer.channel_id = cs.channel_id
					and (newer.created_at > cs.created_at or (newer.created_at = cs.created_at and newer.id > cs.id))
			)`,
	}
	for _, query := range queries {
		queryRows, err := db.QueryContext(ctx, query, externalOrgID)
		if err != nil {
			return nil, fmt.Errorf("query mysql asset inventory: %w", err)
		}
		for queryRows.Next() {
			var row mysqlAssetInventoryRawRow
			if err := queryRows.Scan(
				&row.SourceTable,
				&row.SourceID,
				&row.SourceColumn,
				&row.AssetRole,
				&row.StoreID,
				&row.ExternalOrgID,
				&row.RecorderID,
				&row.ChannelID,
				&row.OwnerEntityType,
				&row.OwnerEntityID,
				&row.UploadID,
				&row.OldPath,
				&row.SourceKey,
				&row.ProxyPath,
				&row.ExpectedContentType,
				&row.Sensitivity,
			); err != nil {
				queryRows.Close()
				return nil, fmt.Errorf("scan mysql asset inventory: %w", err)
			}
			rows = append(rows, row)
		}
		if err := queryRows.Err(); err != nil {
			queryRows.Close()
			return nil, fmt.Errorf("read mysql asset inventory: %w", err)
		}
		queryRows.Close()
	}
	return rows, nil
}

func queryMySQLAssetStates(ctx context.Context, db *sql.DB, rows []mysqlAssetInventoryRawRow) (map[string]mysqlAssetState, error) {
	logicalKeys := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		logicalKey, status, _ := normalizeMySQLAssetLogicalKey(row)
		if status != "pending" || logicalKey == "" {
			continue
		}
		if _, ok := seen[logicalKey]; ok {
			continue
		}
		seen[logicalKey] = struct{}{}
		logicalKeys = append(logicalKeys, logicalKey)
	}
	states := make(map[string]mysqlAssetState, len(logicalKeys))
	for _, logicalKey := range logicalKeys {
		var state mysqlAssetState
		err := db.QueryRowContext(ctx, `select migration_status, storage_provider, bucket, storage_key
from tb_asset_objects
where logical_key_hash = sha2(?, 256)
limit 1`, logicalKey).Scan(&state.MigrationStatus, &state.StorageProvider, &state.Bucket, &state.StorageKey)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("query mysql asset state: %w", err)
		}
		states[logicalKey] = state
	}
	return states, nil
}

func buildMySQLAssetInventory(rows []mysqlAssetInventoryRawRow, assetStates map[string]mysqlAssetState) (*mysqlAssetInventoryRunResult, error) {
	type inventoryRow struct {
		raw                      mysqlAssetInventoryRawRow
		logicalKey               string
		targetOSSKey             string
		suggestedMigrationStatus string
		skipReason               string
		logicalKeyRank           int
		logicalKeyRefCount       int
	}
	rows = latestMySQLSnapshotRows(rows)
	inventory := make([]inventoryRow, 0, len(rows))
	counts := map[string]int{}
	for _, raw := range rows {
		logicalKey, status, reason := normalizeMySQLAssetLogicalKey(raw)
		if isMigratedMySQLAsset(assetStates[logicalKey]) {
			status = "skipped"
			reason = "already_migrated"
		}
		item := inventoryRow{
			raw:                      raw,
			logicalKey:               logicalKey,
			targetOSSKey:             logicalKey,
			suggestedMigrationStatus: status,
			skipReason:               reason,
		}
		inventory = append(inventory, item)
		if logicalKey != "" {
			counts[logicalKey]++
		}
	}
	ranks := map[string]int{}
	summary := mysqlAssetInventorySummary{Total: len(inventory)}
	for index := range inventory {
		item := &inventory[index]
		if item.logicalKey != "" {
			ranks[item.logicalKey]++
			item.logicalKeyRank = ranks[item.logicalKey]
			item.logicalKeyRefCount = counts[item.logicalKey]
			if item.logicalKeyRefCount > 1 {
				summary.DuplicateRefs++
			}
		}
		if item.suggestedMigrationStatus == "pending" {
			summary.Pending++
		} else {
			summary.Skipped++
		}
		if strings.TrimSpace(item.raw.Sensitivity) == "sensitive" {
			summary.Sensitive++
		}
		if item.raw.SourceTable == "tb_store_design_plans" {
			summary.DesignRows++
		}
		if item.raw.SourceTable == "tb_channel_snapshots" {
			summary.SnapshotRows++
		}
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := []string{
		"source_table",
		"source_id",
		"source_column",
		"asset_role",
		"store_id",
		"external_org_id",
		"recorder_id",
		"channel_id",
		"owner_entity_type",
		"owner_entity_id",
		"upload_id",
		"old_path",
		"source_key",
		"logical_key",
		"target_oss_key",
		"proxy_path",
		"expected_content_type",
		"sensitivity",
		"suggested_migration_status",
		"skip_reason",
		"logical_key_rank",
		"logical_key_ref_count",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for _, item := range inventory {
		row := []string{
			item.raw.SourceTable,
			item.raw.SourceID,
			item.raw.SourceColumn,
			item.raw.AssetRole,
			item.raw.StoreID,
			item.raw.ExternalOrgID,
			item.raw.RecorderID,
			item.raw.ChannelID,
			item.raw.OwnerEntityType,
			item.raw.OwnerEntityID,
			item.raw.UploadID,
			item.raw.OldPath,
			item.raw.SourceKey,
			item.logicalKey,
			item.targetOSSKey,
			item.raw.ProxyPath,
			item.raw.ExpectedContentType,
			item.raw.Sensitivity,
			item.suggestedMigrationStatus,
			item.skipReason,
			fmt.Sprint(item.logicalKeyRank),
			fmt.Sprint(item.logicalKeyRefCount),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return &mysqlAssetInventoryRunResult{Summary: summary, ManifestCSV: buffer.String()}, nil
}

func latestMySQLSnapshotRows(rows []mysqlAssetInventoryRawRow) []mysqlAssetInventoryRawRow {
	latestIDByChannel := map[string]int64{}
	for _, row := range rows {
		if row.SourceTable != "tb_channel_snapshots" || strings.TrimSpace(row.ChannelID) == "" {
			continue
		}
		sourceID, err := strconv.ParseInt(strings.TrimSpace(row.SourceID), 10, 64)
		if err != nil {
			continue
		}
		if sourceID > latestIDByChannel[row.ChannelID] {
			latestIDByChannel[row.ChannelID] = sourceID
		}
	}
	if len(latestIDByChannel) == 0 {
		return rows
	}
	filtered := make([]mysqlAssetInventoryRawRow, 0, len(rows))
	for _, row := range rows {
		if row.SourceTable != "tb_channel_snapshots" || strings.TrimSpace(row.ChannelID) == "" {
			filtered = append(filtered, row)
			continue
		}
		sourceID, err := strconv.ParseInt(strings.TrimSpace(row.SourceID), 10, 64)
		if err != nil || sourceID == latestIDByChannel[row.ChannelID] {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func isMigratedMySQLAsset(state mysqlAssetState) bool {
	return strings.TrimSpace(state.MigrationStatus) == "migrated" &&
		strings.TrimSpace(state.StorageProvider) == "oss" &&
		strings.TrimSpace(state.Bucket) != "" &&
		strings.TrimSpace(state.StorageKey) != ""
}

func normalizeMySQLAssetLogicalKey(row mysqlAssetInventoryRawRow) (string, string, string) {
	oldPath := stripAssetQuery(strings.TrimSpace(row.OldPath))
	sourceKey := stripAssetQuery(strings.TrimSpace(row.SourceKey))
	if oldPath == "" && sourceKey == "" {
		return "", "skipped", "empty_path"
	}
	if strings.HasPrefix(sourceKey, "http://") || strings.HasPrefix(sourceKey, "https://") || strings.HasPrefix(oldPath, "http://") || strings.HasPrefix(oldPath, "https://") {
		return "", "skipped", "remote_http_url"
	}
	candidates := []string{sourceKey, oldPath}
	for _, candidate := range candidates {
		if key := normalizeAssetPathCandidate(candidate); key != "" {
			return key, "pending", ""
		}
	}
	return "", "skipped", "unrecognized_path"
}

func stripAssetQuery(value string) string {
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		return value[:index]
	}
	return value
}

func normalizeAssetPathCandidate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "uploads/") || strings.HasPrefix(value, "channel-snapshots/") {
		return value
	}
	if strings.HasPrefix(value, "/api/store-space/channel-snapshots/") {
		return "channel-snapshots/" + strings.TrimPrefix(value, "/api/store-space/channel-snapshots/")
	}
	if strings.HasPrefix(value, "/api/design-plan/uploads/") {
		rest := strings.TrimPrefix(value, "/api/design-plan/uploads/")
		parts := strings.Split(rest, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
			return ""
		}
		switch parts[1] {
		case "original":
			return "uploads/" + parts[0] + "/original.pdf"
		case "preview":
			return "uploads/" + parts[0] + "/preview.png"
		case "thumbnail":
			return "uploads/" + parts[0] + "/thumbnail.png"
		default:
			return ""
		}
	}
	return ""
}

func ensureMySQLCanaryTables(ctx context.Context, db *sql.DB) error {
	required := []string{
		"tb_tasks",
		"tb_app_settings",
		"tb_stores",
		"tb_ezviz_accounts",
		"tb_video_recorders",
		"tb_video_channels",
		"tb_channel_snapshots",
		"tb_operation_logs",
		"tb_users",
		"tb_roles",
		"tb_user_roles",
	}
	for _, table := range required {
		var exists int
		if err := db.QueryRowContext(ctx, `
			select count(*)
			from information_schema.tables
			where table_schema = database()
			  and table_name = ?
		`, table).Scan(&exists); err != nil {
			return fmt.Errorf("check mysql table %s: %w", table, err)
		}
		if exists == 0 {
			return fmt.Errorf("mysql table %s is missing", table)
		}
	}
	return nil
}

func queryMySQLCanarySummary(ctx context.Context, db *sql.DB, externalOrgID string) (mysqlCanaryImportSummary, error) {
	var summary mysqlCanaryImportSummary
	queries := []struct {
		name string
		dest *int
		sql  string
		args []any
	}{
		{
			name: "stores",
			dest: &summary.StoreCount,
			sql:  `select count(*) from tb_stores where external_org_id = ?`,
			args: []any{externalOrgID},
		},
		{
			name: "recorders",
			dest: &summary.RecorderCount,
			sql:  `select count(*) from tb_video_recorders where store_id in (select id from tb_stores where external_org_id = ?)`,
			args: []any{externalOrgID},
		},
		{
			name: "channels",
			dest: &summary.ChannelCount,
			sql:  `select count(*) from tb_video_channels where recorder_id in (select id from tb_video_recorders where store_id in (select id from tb_stores where external_org_id = ?))`,
			args: []any{externalOrgID},
		},
		{
			name: "snapshots",
			dest: &summary.SnapshotCount,
			sql:  `select count(*) from tb_channel_snapshots where channel_id in (select id from tb_video_channels where recorder_id in (select id from tb_video_recorders where store_id in (select id from tb_stores where external_org_id = ?)))`,
			args: []any{externalOrgID},
		},
		{
			name: "operation logs",
			dest: &summary.OperationLogCount,
			sql:  `select count(*) from tb_operation_logs where store_id in (select id from tb_stores where external_org_id = ?)`,
			args: []any{externalOrgID},
		},
		{
			name: "users",
			dest: &summary.UserCount,
			sql:  `select count(*) from tb_users`,
		},
		{
			name: "orphan rows",
			dest: &summary.OrphanCount,
			sql: `select
				(select count(*) from tb_video_recorders r where not exists (select 1 from tb_stores s where s.id = r.store_id)) +
				(select count(*) from tb_video_channels c where not exists (select 1 from tb_video_recorders r where r.id = c.recorder_id)) +
				(select count(*) from tb_channel_snapshots cs where not exists (select 1 from tb_video_channels c where c.id = cs.channel_id))`,
		},
		{
			name: "invalid json",
			dest: &summary.InvalidJSONCount,
			sql: `select
				(select count(*) from tb_store_design_plans where recognition_result is not null and json_valid(recognition_result) = 0) +
				(select count(*) from tb_video_channels where recognition_result is not null and json_valid(recognition_result) = 0)`,
		},
	}
	for _, query := range queries {
		if err := db.QueryRowContext(ctx, query.sql, query.args...).Scan(query.dest); err != nil {
			return mysqlCanaryImportSummary{}, fmt.Errorf("query mysql canary %s: %w", query.name, err)
		}
	}
	return summary, nil
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
	if input.Apply && !opsMigrationOrgAllowed(externalOrgID) {
		return assetMigrationRunRequest{}, fmt.Errorf("apply is currently limited to external_org_id in %s", allowedOpsMigrationOrgIDsText())
	}
	return assetMigrationRunRequest{
		ManifestCSV:   manifest,
		Apply:         input.Apply,
		ExternalOrgID: externalOrgID,
		MaxRows:       maxRows,
		BatchID:       strings.TrimSpace(input.BatchID),
	}, nil
}

func normalizeAssetStateBackfillRequest(input assetStateBackfillRequest) (assetStateBackfillRunRequest, error) {
	manifest := strings.TrimSpace(input.ManifestCSV)
	if manifest == "" {
		return assetStateBackfillRunRequest{}, errors.New("manifest_csv is required")
	}
	resultCSV := strings.TrimSpace(input.ResultCSV)
	if resultCSV == "" {
		return assetStateBackfillRunRequest{}, errors.New("result_csv is required")
	}
	externalOrgID := strings.TrimSpace(input.ExternalOrgID)
	if externalOrgID == "" {
		externalOrgID = defaultOpsMigrationOrgID
	}
	if !opsMigrationOrgAllowed(externalOrgID) {
		return assetStateBackfillRunRequest{}, fmt.Errorf("backfill is currently limited to external_org_id in %s", allowedOpsMigrationOrgIDsText())
	}
	return assetStateBackfillRunRequest{
		ExternalOrgID: externalOrgID,
		ManifestCSV:   manifest,
		ResultCSV:     resultCSV,
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

type assetMigrationCSVResult struct {
	Action      string
	ExternalID  string
	LogicalKey  string
	TargetKey   string
	Bytes       int64
	ContentType string
	Error       string
}

func runAssetStateBackfillFromEnv(ctx context.Context, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error) {
	manifestRows, err := assetmigration.ReadManifest(strings.NewReader(request.ManifestCSV))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	resultRows, err := readAssetMigrationResultCSV(strings.NewReader(request.ResultCSV))
	if err != nil {
		return nil, fmt.Errorf("read result csv: %w", err)
	}
	dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	return runAssetStateBackfill(ctx, db, manifestRows, resultRows, request)
}

func runAssetStateBackfill(ctx context.Context, db *sql.DB, manifestRows []assetmigration.ManifestRow, resultRows []assetMigrationCSVResult, request assetStateBackfillRunRequest) (*assetStateBackfillRunResult, error) {
	if db == nil {
		return nil, errors.New("mysql db is required")
	}
	manifestByKey := map[string]assetmigration.ManifestRow{}
	for _, row := range manifestRows {
		key := strings.TrimSpace(row.LogicalKey)
		if key == "" || row.LogicalKeyRank > 1 {
			continue
		}
		manifestByKey[key] = row
	}
	bucket := envValue("OSS_BUCKET", "K8S_SECRET_OSS_BUCKET")
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("OSS_BUCKET or K8S_SECRET_OSS_BUCKET is required")
	}
	batchID := strings.TrimSpace(request.BatchID)
	if batchID == "" {
		batchID = "asset-state-backfill-" + time.Now().UTC().Format("20060102T150405Z")
	}
	summary := assetStateBackfillSummary{Total: len(resultRows)}
	for _, result := range resultRows {
		if result.Action != "copied" {
			summary.Skipped++
			continue
		}
		if strings.TrimSpace(result.ExternalID) != request.ExternalOrgID {
			summary.Skipped++
			continue
		}
		manifest, ok := manifestByKey[strings.TrimSpace(result.LogicalKey)]
		if !ok {
			summary.Errors++
			return nil, fmt.Errorf("missing manifest row for copied logical key %q", result.LogicalKey)
		}
		ownerID, err := parseNullableInt64(manifest.OwnerEntityID)
		if err != nil {
			summary.Errors++
			return nil, fmt.Errorf("invalid owner_entity_id for %q: %w", result.LogicalKey, err)
		}
		contentType := strings.TrimSpace(result.ContentType)
		if contentType == "" {
			contentType = strings.TrimSpace(manifest.ExpectedContentType)
		}
		_, err = db.ExecContext(ctx, `insert into tb_asset_objects (
  logical_key, logical_key_hash,
  storage_provider, bucket, storage_key, storage_key_hash,
  proxy_path, content_type, size_bytes, sensitivity,
  owner_entity_type, owner_entity_id,
  migration_status, migration_batch_id, migration_attempts,
  last_attempt_at, last_error_code, last_error_message, migrated_at
) values (
  ?, sha2(?, 256),
  'oss', ?, ?, sha2(?, 256),
  ?, ?, ?, ?,
  ?, ?,
  'migrated', ?, 1,
  current_timestamp(3), '', '', current_timestamp(3)
) on duplicate key update
  storage_provider = values(storage_provider),
  bucket = values(bucket),
  storage_key = values(storage_key),
  storage_key_hash = values(storage_key_hash),
  proxy_path = values(proxy_path),
  content_type = values(content_type),
  size_bytes = values(size_bytes),
  sensitivity = values(sensitivity),
  owner_entity_type = values(owner_entity_type),
  owner_entity_id = values(owner_entity_id),
  migration_status = 'migrated',
  migration_batch_id = values(migration_batch_id),
  migration_attempts = migration_attempts + 1,
  last_attempt_at = current_timestamp(3),
  last_error_code = '',
  last_error_message = '',
  migrated_at = current_timestamp(3)`,
			result.LogicalKey, result.LogicalKey,
			bucket, result.TargetKey, result.TargetKey,
			manifest.ProxyPath, contentType, nullablePositiveInt64(result.Bytes), normalizedAssetSensitivity(manifest.Sensitivity),
			strings.TrimSpace(manifest.OwnerEntityType), ownerID,
			batchID,
		)
		if err != nil {
			summary.Errors++
			return nil, fmt.Errorf("upsert asset object %q: %w", result.LogicalKey, err)
		}
		summary.Migrated++
		summary.Upserted++
	}
	return &assetStateBackfillRunResult{
		Summary: summary,
		Warnings: []string{
			"Backfill only upserts rows with action=copied and manifest logical_key_rank=1.",
			"Duplicate thumbnail/full references remain represented by one logical asset row.",
		},
	}, nil
}

func readAssetMigrationResultCSV(reader io.Reader) ([]assetMigrationCSVResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("result csv is empty")
	}
	header := map[string]int{}
	for index, name := range records[0] {
		header[strings.TrimSpace(name)] = index
	}
	for _, name := range []string{"action", "external_org_id", "logical_key", "target_oss_key"} {
		if _, ok := header[name]; !ok {
			return nil, fmt.Errorf("result csv missing required column %q", name)
		}
	}
	results := make([]assetMigrationCSVResult, 0, len(records)-1)
	for _, record := range records[1:] {
		bytes, _ := strconv.ParseInt(strings.TrimSpace(csvValue(record, header, "bytes")), 10, 64)
		results = append(results, assetMigrationCSVResult{
			Action:      strings.TrimSpace(csvValue(record, header, "action")),
			ExternalID:  strings.TrimSpace(csvValue(record, header, "external_org_id")),
			LogicalKey:  strings.TrimSpace(csvValue(record, header, "logical_key")),
			TargetKey:   strings.TrimSpace(csvValue(record, header, "target_oss_key")),
			Bytes:       bytes,
			ContentType: strings.TrimSpace(csvValue(record, header, "content_type")),
			Error:       strings.TrimSpace(csvValue(record, header, "error")),
		})
	}
	return results, nil
}

func csvValue(record []string, header map[string]int, name string) string {
	index, ok := header[name]
	if !ok || index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func parseNullableInt64(value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func normalizedAssetSensitivity(value string) string {
	switch strings.TrimSpace(value) {
	case "public", "internal", "sensitive":
		return strings.TrimSpace(value)
	default:
		return "internal"
	}
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

func runStageATargetSampleFromEnv(ctx context.Context) (*stageATargetSampleResult, error) {
	store, _, err := targetOSSStoreForMigration()
	if err != nil {
		return nil, err
	}
	if err := deleteExactAsset(ctx, store, stageASourceSampleKey); err != nil {
		return nil, fmt.Errorf("cleanup target sample: %w", err)
	}
	return &stageATargetSampleResult{
		Action: "cleaned",
		Key:    stageASourceSampleKey,
	}, nil
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
