package assetmigration

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

type ManifestRow struct {
	SourceTable              string
	SourceID                 string
	SourceColumn             string
	AssetRole                string
	StoreID                  string
	ExternalOrgID            string
	RecorderID               string
	ChannelID                string
	OwnerEntityType          string
	OwnerEntityID            string
	LogicalKey               string
	TargetOSSKey             string
	ProxyPath                string
	ExpectedContentType      string
	Sensitivity              string
	SuggestedMigrationStatus string
	SkipReason               string
	LogicalKeyRank           int
}

type Options struct {
	Apply              bool
	ExternalOrgID      string
	IncludeSkipped     bool
	MaxRows            int
	DefaultContentType string
}

type Summary struct {
	Total     int
	Copied    int
	Skipped   int
	WouldCopy int
	Errors    int
}

type RowResult struct {
	Row         ManifestRow
	Action      string
	Error       string
	Bytes       int64
	ContentType string
}

func ReadManifest(reader io.Reader) ([]ManifestRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("manifest is empty")
	}
	header := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		header[strings.TrimSpace(name)] = index
	}
	required := []string{"logical_key", "target_oss_key", "suggested_migration_status"}
	for _, name := range required {
		if _, ok := header[name]; !ok {
			return nil, fmt.Errorf("manifest missing required column %q", name)
		}
	}
	rows := make([]ManifestRow, 0, len(records)-1)
	for _, record := range records[1:] {
		row := ManifestRow{
			SourceTable:              value(record, header, "source_table"),
			SourceID:                 value(record, header, "source_id"),
			SourceColumn:             value(record, header, "source_column"),
			AssetRole:                value(record, header, "asset_role"),
			StoreID:                  value(record, header, "store_id"),
			ExternalOrgID:            value(record, header, "external_org_id"),
			RecorderID:               value(record, header, "recorder_id"),
			ChannelID:                value(record, header, "channel_id"),
			OwnerEntityType:          value(record, header, "owner_entity_type"),
			OwnerEntityID:            value(record, header, "owner_entity_id"),
			LogicalKey:               value(record, header, "logical_key"),
			TargetOSSKey:             value(record, header, "target_oss_key"),
			ProxyPath:                value(record, header, "proxy_path"),
			ExpectedContentType:      value(record, header, "expected_content_type"),
			Sensitivity:              value(record, header, "sensitivity"),
			SuggestedMigrationStatus: value(record, header, "suggested_migration_status"),
			SkipReason:               value(record, header, "skip_reason"),
			LogicalKeyRank:           intValue(value(record, header, "logical_key_rank")),
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func CopyManifest(ctx context.Context, source assets.Store, target assets.Store, rows []ManifestRow, options Options) (Summary, []RowResult) {
	summary := Summary{Total: len(rows)}
	results := make([]RowResult, 0, len(rows))
	processed := 0
	for _, row := range rows {
		result := RowResult{Row: row}
		if options.MaxRows > 0 && processed >= options.MaxRows {
			result.Action = "skipped"
			result.Error = "max_rows_reached"
			summary.Skipped++
			results = append(results, result)
			continue
		}
		if !rowMatchesOptions(row, options) {
			result.Action = "skipped"
			result.Error = "filtered"
			summary.Skipped++
			results = append(results, result)
			continue
		}
		if !rowReady(row, options.IncludeSkipped) {
			result.Action = "skipped"
			result.Error = skipReason(row)
			summary.Skipped++
			results = append(results, result)
			continue
		}
		processed++
		if !options.Apply {
			result.Action = "would_copy"
			summary.WouldCopy++
			results = append(results, result)
			continue
		}
		bytes, contentType, err := copyOne(ctx, source, target, row, options)
		if err != nil {
			result.Action = "failed"
			result.Error = err.Error()
			summary.Errors++
			results = append(results, result)
			continue
		}
		result.Action = "copied"
		result.Bytes = bytes
		result.ContentType = contentType
		summary.Copied++
		results = append(results, result)
	}
	return summary, results
}

func copyOne(ctx context.Context, source assets.Store, target assets.Store, row ManifestRow, options Options) (int64, string, error) {
	if source == nil || target == nil {
		return 0, "", errors.New("source and target stores are required")
	}
	reader, contentType, err := source.Open(ctx, row.LogicalKey)
	if err != nil {
		return 0, "", fmt.Errorf("open source %q: %w", row.LogicalKey, err)
	}
	defer reader.Close()
	if strings.TrimSpace(contentType) == "" {
		contentType = strings.TrimSpace(row.ExpectedContentType)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = strings.TrimSpace(options.DefaultContentType)
	}
	counting := &countingReader{reader: reader}
	if err := target.Save(ctx, row.TargetOSSKey, counting, contentType); err != nil {
		return counting.bytes, contentType, fmt.Errorf("save target %q: %w", row.TargetOSSKey, err)
	}
	return counting.bytes, contentType, nil
}

func rowMatchesOptions(row ManifestRow, options Options) bool {
	externalOrgID := strings.TrimSpace(options.ExternalOrgID)
	if externalOrgID == "" {
		return true
	}
	return strings.TrimSpace(row.ExternalOrgID) == externalOrgID
}

func rowReady(row ManifestRow, includeSkipped bool) bool {
	if strings.TrimSpace(row.LogicalKey) == "" || strings.TrimSpace(row.TargetOSSKey) == "" {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(row.SuggestedMigrationStatus))
	if status == "" || status == "pending" {
		if row.LogicalKeyRank == 0 || row.LogicalKeyRank == 1 {
			return true
		}
		return false
	}
	return includeSkipped && status == "skipped" && strings.TrimSpace(row.SkipReason) == ""
}

func skipReason(row ManifestRow) string {
	if strings.TrimSpace(row.LogicalKey) == "" {
		return "empty_logical_key"
	}
	if strings.TrimSpace(row.TargetOSSKey) == "" {
		return "empty_target_oss_key"
	}
	if row.LogicalKeyRank > 1 {
		return "duplicate_logical_key"
	}
	if strings.TrimSpace(row.SkipReason) != "" {
		return row.SkipReason
	}
	status := strings.TrimSpace(row.SuggestedMigrationStatus)
	if status != "" && status != "pending" {
		return "status_" + status
	}
	return "not_ready"
}

func value(record []string, header map[string]int, name string) string {
	index, ok := header[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func intValue(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

type countingReader struct {
	reader io.Reader
	bytes  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytes += int64(n)
	return n, err
}
