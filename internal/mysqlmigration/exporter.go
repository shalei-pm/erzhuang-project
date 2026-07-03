package mysqlmigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type Options struct {
	ExternalOrgIDs []string
	IncludeUsers   bool
	BatchID        string
}

type Report struct {
	GeneratedAt    time.Time     `json:"generated_at"`
	ExternalOrgIDs []string      `json:"external_org_ids,omitempty"`
	Tables         []TableReport `json:"tables"`
	Warnings       []string      `json:"warnings,omitempty"`
}

type TableReport struct {
	SourceTable string `json:"source_table"`
	TargetTable string `json:"target_table"`
	Rows        int    `json:"rows"`
	Skipped     bool   `json:"skipped,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type Export struct {
	ImportSQL        string
	AutoIncrementSQL string
	Report           Report
}

type tableSpec struct {
	Source      string
	Target      string
	Columns     []columnSpec
	Filter      filterKind
	OrderBy     string
	IncludeWhen func(Options) bool
}

type columnSpec struct {
	Target   string
	Source   string
	Default  any
	Nullable bool
	Kind     valueKind
}

type filterKind int

const (
	filterNone filterKind = iota
	filterStore
	filterStoreArea
	filterStoreDesignPlan
	filterDesignPlanAnnotation
	filterRecorder
	filterChannel
	filterChannelSnapshot
	filterOperationLog
	filterLegacyDesignStore
	filterLegacyDesignArea
	filterLegacyDesignLog
)

type valueKind int

const (
	kindDefault valueKind = iota
	kindBool
	kindJSON
	kindDateTime
)

var tableSpecs = []tableSpec{
	{
		Source: "tasks",
		Target: "tb_tasks",
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "title", Source: "title", Default: ""},
			{Target: "done", Source: "done", Default: false, Kind: kindBool},
		},
	},
	{
		Source: "app_settings",
		Target: "tb_app_settings",
		OrderBy: "key",
		Columns: []columnSpec{
			{Target: "key", Source: "key"},
			{Target: "value", Source: "value", Default: ""},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "design_plan_stores",
		Target: "tb_design_plan_stores",
		Filter: filterLegacyDesignStore,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "name", Source: "name", Default: ""},
			{Target: "normalized_name", Source: "normalized_name", Default: ""},
			{Target: "pdf_file_name", Source: "pdf_file_name", Default: ""},
			{Target: "original_pdf_path", Source: "original_pdf_path", Default: ""},
			{Target: "preview_image_path", Source: "preview_image_path", Default: ""},
			{Target: "thumbnail_path", Source: "thumbnail_path", Default: ""},
			{Target: "page_count", Source: "page_count", Default: 0},
			{Target: "status", Source: "status", Default: "completed"},
			{Target: "recognition_result", Source: "recognition_result", Nullable: true, Kind: kindJSON},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "design_plan_store_areas",
		Target: "tb_design_plan_store_areas",
		Filter: filterLegacyDesignArea,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "store_id", Source: "store_id"},
			{Target: "display_order", Source: "display_order", Default: 0},
			{Target: "name", Source: "name", Default: ""},
			{Target: "area_type", Source: "area_type", Default: "treatment"},
			{Target: "area_number", Source: "area_number", Nullable: true},
			{Target: "confidence", Source: "confidence", Default: "high"},
			{Target: "needs_review", Source: "needs_review", Default: false, Kind: kindBool},
			{Target: "box_x", Source: "box_x", Default: 0},
			{Target: "box_y", Source: "box_y", Default: 0},
			{Target: "box_width", Source: "box_width", Default: 0},
			{Target: "box_height", Source: "box_height", Default: 0},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "design_plan_operation_logs",
		Target: "tb_design_plan_operation_logs",
		Filter: filterLegacyDesignLog,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "action", Source: "action", Default: "update"},
			{Target: "store_id", Source: "store_id", Nullable: true},
			{Target: "store_name", Source: "store_name", Default: ""},
			{Target: "actor", Source: "actor", Default: "admin"},
			{Target: "summary", Source: "summary", Default: ""},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
		},
	},
	{
		Source: "stores",
		Target: "tb_stores",
		Filter: filterStore,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "city", Source: "city", Default: ""},
			{Target: "name", Source: "name", Default: ""},
			{Target: "short_name", Source: "short_name", Default: ""},
			{Target: "normalized_name", Source: "normalized_name", Default: ""},
			{Target: "external_org_id", Source: "external_org_id", Default: ""},
			{Target: "design_plan_status", Source: "design_plan_status", Default: "not_uploaded"},
			{Target: "overall_status", Source: "overall_status", Default: "partial"},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "store_areas",
		Target: "tb_store_areas",
		Filter: filterStoreArea,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "store_id", Source: "store_id"},
			{Target: "area_type", Source: "area_type", Default: "treatment"},
			{Target: "area_number", Source: "area_number", Default: 1},
			{Target: "display_name", Source: "display_name", Default: ""},
			{Target: "source", Source: "source", Default: "manual"},
			{Target: "status", Source: "status", Default: "confirmed"},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "store_design_plans",
		Target: "tb_store_design_plans",
		Filter: filterStoreDesignPlan,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "store_id", Source: "store_id"},
			{Target: "upload_id", Source: "upload_id", Default: ""},
			{Target: "pdf_file_name", Source: "pdf_file_name", Default: ""},
			{Target: "original_pdf_path", Source: "original_pdf_path", Default: ""},
			{Target: "preview_image_path", Source: "preview_image_path", Default: ""},
			{Target: "thumbnail_path", Source: "thumbnail_path", Default: ""},
			{Target: "page_count", Source: "page_count", Default: 0},
			{Target: "recognition_status", Source: "recognition_status", Default: "not_started"},
			{Target: "recognition_result", Source: "recognition_result", Nullable: true, Kind: kindJSON},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "design_plan_annotations",
		Target: "tb_design_plan_annotations",
		Filter: filterDesignPlanAnnotation,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "design_plan_id", Source: "design_plan_id"},
			{Target: "area_id", Source: "area_id"},
			{Target: "box_x", Source: "box_x", Default: 0},
			{Target: "box_y", Source: "box_y", Default: 0},
			{Target: "box_width", Source: "box_width", Default: 0},
			{Target: "box_height", Source: "box_height", Default: 0},
			{Target: "status", Source: "status", Default: "pending"},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "ezviz_accounts",
		Target: "tb_ezviz_accounts",
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "account_name", Source: "account_name", Default: ""},
			{Target: "app_key", Source: "app_key", Default: ""},
			{Target: "app_secret_ciphertext", Source: "app_secret_ciphertext", Default: ""},
			{Target: "access_token_ciphertext", Source: "access_token_ciphertext", Default: ""},
			{Target: "status", Source: "status", Default: "unverified"},
			{Target: "last_verified_at", Source: "last_verified_at", Nullable: true, Kind: kindDateTime},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "video_recorders",
		Target: "tb_video_recorders",
		Filter: filterRecorder,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "store_id", Source: "store_id"},
			{Target: "ezviz_account_id", Source: "ezviz_account_id", Nullable: true},
			{Target: "device_code", Source: "device_code", Default: ""},
			{Target: "status", Source: "status", Default: "offline"},
			{Target: "effective_channel_count", Source: "effective_channel_count", Default: 0},
			{Target: "last_scanned_at", Source: "last_scanned_at", Nullable: true, Kind: kindDateTime},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "video_channels",
		Target: "tb_video_channels",
		Filter: filterChannel,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "recorder_id", Source: "recorder_id"},
			{Target: "channel_no", Source: "channel_no"},
			{Target: "channel_name", Source: "channel_name", Default: ""},
			{Target: "status", Source: "status", Default: "pending_recognition"},
			{Target: "is_active", Source: "is_active", Default: true, Kind: kindBool},
			{Target: "scene_type", Source: "scene_type", Default: "unknown"},
			{Target: "area_type", Source: "area_type", Nullable: true},
			{Target: "area_number", Source: "area_number", Nullable: true},
			{Target: "bed_label", Source: "bed_label", Default: ""},
			{Target: "area_note", Source: "area_note", Default: ""},
			{Target: "area_id", Source: "area_id", Nullable: true},
			{Target: "recognition_attempts", Source: "recognition_attempts", Default: 0},
			{Target: "recognition_result", Source: "recognition_result", Nullable: true, Kind: kindJSON},
			{Target: "confirmed_at", Source: "confirmed_at", Nullable: true, Kind: kindDateTime},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
	{
		Source: "channel_snapshots",
		Target: "tb_channel_snapshots",
		Filter: filterChannelSnapshot,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "channel_id", Source: "channel_id"},
			{Target: "thumbnail_path", Source: "thumbnail_path", Default: ""},
			{Target: "full_image_path", Source: "full_image_path", Default: ""},
			{Target: "full_image_expires_at", Source: "full_image_expires_at", Nullable: true, Kind: kindDateTime},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
		},
	},
	{
		Source: "operation_logs",
		Target: "tb_operation_logs",
		Filter: filterOperationLog,
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "action", Source: "action", Default: ""},
			{Target: "entity_type", Source: "entity_type", Default: ""},
			{Target: "entity_id", Source: "entity_id", Nullable: true},
			{Target: "store_id", Source: "store_id", Nullable: true},
			{Target: "actor", Source: "actor", Default: "admin"},
			{Target: "summary", Source: "summary", Default: ""},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
		},
	},
	{
		Source: "tb_users",
		Target: "tb_users",
		IncludeWhen: func(options Options) bool {
			return options.IncludeUsers
		},
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "email", Source: "email", Default: ""},
			{Target: "username", Source: "username", Default: ""},
			{Target: "display_name", Source: "display_name", Default: ""},
			{Target: "feishu_user_id", Source: "feishu_user_id", Default: ""},
			{Target: "mobile", Source: "phone", Default: ""},
			{Target: "department", Source: "department", Default: ""},
			{Target: "sso_subject", Source: "sso_subject", Default: ""},
			{Target: "enabled", Source: "enabled", Default: true, Kind: kindBool},
			{Target: "last_login_at", Source: "last_login_at", Nullable: true, Kind: kindDateTime},
			{Target: "created_at", Source: "created_at", Kind: kindDateTime},
			{Target: "updated_at", Source: "updated_at", Kind: kindDateTime},
		},
	},
}

func ExportFromPostgres(ctx context.Context, db *sql.DB, options Options) (*Export, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	options.ExternalOrgIDs = normalizeOrgIDs(options.ExternalOrgIDs)
	report := Report{
		GeneratedAt:    time.Now().UTC(),
		ExternalOrgIDs: options.ExternalOrgIDs,
	}
	var importBuilder strings.Builder
	var autoIncrementBuilder strings.Builder

	writeImportHeader(&importBuilder, options)
	writeAutoIncrementHeader(&autoIncrementBuilder)

	for _, spec := range tableSpecs {
		if spec.IncludeWhen != nil && !spec.IncludeWhen(options) {
			continue
		}
		exists, err := tableExists(ctx, db, spec.Source)
		if err != nil {
			return nil, err
		}
		if !exists {
			report.Tables = append(report.Tables, TableReport{
				SourceTable: spec.Source,
				TargetTable: spec.Target,
				Skipped:     true,
				Reason:      "source_table_missing",
			})
			continue
		}
		rows, err := exportTable(ctx, db, spec, options)
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", spec.Source, err)
		}
		if len(rows) > 0 {
			if err := writeInsertStatements(&importBuilder, spec, rows); err != nil {
				return nil, err
			}
		}
		writeAutoIncrementStatement(&autoIncrementBuilder, spec.Target)
		report.Tables = append(report.Tables, TableReport{
			SourceTable: spec.Source,
			TargetTable: spec.Target,
			Rows:        len(rows),
		})
	}

	if options.IncludeUsers {
		roleRows, err := exportUserRoles(ctx, db)
		if err != nil {
			return nil, err
		}
		if len(roleRows) > 0 {
			writeRoleStatements(&importBuilder, roleRows)
		}
	}
	fmt.Fprintln(&importBuilder, "set foreign_key_checks = 1;")
	fmt.Fprintln(&importBuilder)

	report.Warnings = append(report.Warnings,
		"Import SQL preserves primary IDs and uses ON DUPLICATE KEY UPDATE for repeatable sample imports.",
		"Review row counts and run docs/mysql-validation-sql.md after importing.",
		"Asset binaries are not included. Run OSS asset inventory after MySQL business rows are imported.",
	)

	return &Export{
		ImportSQL:        importBuilder.String(),
		AutoIncrementSQL: autoIncrementBuilder.String(),
		Report:           report,
	}, nil
}

func WriteReportJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

type rowData map[string]any

func exportTable(ctx context.Context, db *sql.DB, spec tableSpec, options Options) ([]rowData, error) {
	sourceColumns, err := tableColumns(ctx, db, spec.Source)
	if err != nil {
		return nil, err
	}
	query := buildExportQuery(spec, options, sourceColumns)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]any, len(names))
	dest := make([]any, len(names))
	for i := range values {
		dest[i] = &values[i]
	}
	result := []rowData{}
	for rows.Next() {
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := rowData{}
		for _, col := range spec.Columns {
			source := col.Source
			if source == "" {
				source = col.Target
			}
			if sourceColumns[source] {
				row[col.Target] = values[indexOf(names, source)]
				continue
			}
			if col.Nullable {
				row[col.Target] = nil
				continue
			}
			row[col.Target] = col.Default
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func buildExportQuery(spec tableSpec, options Options, sourceColumns map[string]bool) string {
	query := "select * from " + pqIdent(spec.Source) + filterClause(spec.Filter, options.ExternalOrgIDs)
	orderBy := strings.TrimSpace(spec.OrderBy)
	if orderBy == "" {
		orderBy = "id"
	}
	if sourceColumns[orderBy] {
		query += " order by " + pqIdent(orderBy)
	}
	return query
}

func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		select exists (
			select 1
			from information_schema.tables
			where table_schema = current_schema()
			  and table_name = $1
		)
	`, name).Scan(&exists)
	return exists, err
}

func tableColumns(ctx context.Context, db *sql.DB, name string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `
		select column_name
		from information_schema.columns
		where table_schema = current_schema()
		  and table_name = $1
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns[column] = true
	}
	return columns, rows.Err()
}

func filterClause(kind filterKind, orgIDs []string) string {
	if len(orgIDs) == 0 {
		return ""
	}
	in := quotedOrgList(orgIDs)
	switch kind {
	case filterStore:
		return " where external_org_id in (" + in + ")"
	case filterStoreArea:
		return " where store_id in (select id from stores where external_org_id in (" + in + "))"
	case filterStoreDesignPlan:
		return " where store_id in (select id from stores where external_org_id in (" + in + "))"
	case filterDesignPlanAnnotation:
		return " where design_plan_id in (select p.id from store_design_plans p join stores s on s.id = p.store_id where s.external_org_id in (" + in + "))"
	case filterRecorder:
		return " where store_id in (select id from stores where external_org_id in (" + in + "))"
	case filterChannel:
		return " where recorder_id in (select r.id from video_recorders r join stores s on s.id = r.store_id where s.external_org_id in (" + in + "))"
	case filterChannelSnapshot:
		return " where channel_id in (select c.id from video_channels c join video_recorders r on r.id = c.recorder_id join stores s on s.id = r.store_id where s.external_org_id in (" + in + "))"
	case filterOperationLog:
		return " where store_id in (select id from stores where external_org_id in (" + in + "))"
	case filterLegacyDesignStore:
		return " where normalized_name in (select normalized_name from stores where external_org_id in (" + in + "))"
	case filterLegacyDesignArea:
		return " where store_id in (select d.id from design_plan_stores d join stores s on s.normalized_name = d.normalized_name where s.external_org_id in (" + in + "))"
	case filterLegacyDesignLog:
		return " where store_id in (select d.id from design_plan_stores d join stores s on s.normalized_name = d.normalized_name where s.external_org_id in (" + in + "))"
	default:
		return ""
	}
}

func writeImportHeader(builder *strings.Builder, options Options) {
	fmt.Fprintln(builder, "-- Generated by cmd/pg-to-mysql-export. Review before executing.")
	fmt.Fprintln(builder, "-- Source: PostgreSQL/Supabase. Target: MySQL tb_ schema.")
	if len(options.ExternalOrgIDs) > 0 {
		fmt.Fprintf(builder, "-- Scope external_org_id: %s\n", strings.Join(options.ExternalOrgIDs, ", "))
	}
	if strings.TrimSpace(options.BatchID) != "" {
		fmt.Fprintf(builder, "-- Batch: %s\n", sanitizeComment(options.BatchID))
	}
	fmt.Fprintln(builder, "set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';")
	fmt.Fprintln(builder, "set foreign_key_checks = 0;")
	fmt.Fprintln(builder)
}

func writeAutoIncrementHeader(builder *strings.Builder) {
	fmt.Fprintln(builder, "-- Generated by cmd/pg-to-mysql-export. Review before executing.")
	fmt.Fprintln(builder, "-- Run after import if auto_increment values need to be advanced.")
}

func writeInsertStatements(builder *strings.Builder, spec tableSpec, rows []rowData) error {
	columns := targetColumns(spec.Columns)
	fmt.Fprintf(builder, "-- %s -> %s (%d rows)\n", spec.Source, spec.Target, len(rows))
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, col := range spec.Columns {
			values = append(values, mysqlValue(row[col.Target], col))
		}
		updates := make([]string, 0, len(columns))
		for _, column := range columns {
			if column == "id" {
				continue
			}
			updates = append(updates, mysqlIdent(column)+" = values("+mysqlIdent(column)+")")
		}
		fmt.Fprintf(builder,
			"insert into %s (%s) values (%s) on duplicate key update %s;\n",
			mysqlIdent(spec.Target),
			joinMySQLIdents(columns),
			strings.Join(values, ", "),
			strings.Join(updates, ", "),
		)
	}
	fmt.Fprintln(builder)
	return nil
}

func writeAutoIncrementStatement(builder *strings.Builder, table string) {
	fmt.Fprintf(builder, "select concat('alter table %s auto_increment = ', coalesce(max(id), 0) + 1, ';') as ddl from %s;\n", table, mysqlIdent(table))
}

type userRoleRow struct {
	UserID int64
	Role   string
}

func exportUserRoles(ctx context.Context, db *sql.DB) ([]userRoleRow, error) {
	exists, err := tableExists(ctx, db, "tb_users")
	if err != nil || !exists {
		return nil, err
	}
	columns, err := tableColumns(ctx, db, "tb_users")
	if err != nil {
		return nil, err
	}
	if !columns["role"] {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `select id, role from tb_users order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []userRoleRow{}
	for rows.Next() {
		var row userRoleRow
		if err := rows.Scan(&row.UserID, &row.Role); err != nil {
			return nil, err
		}
		row.Role = normalizeRole(row.Role)
		result = append(result, row)
	}
	return result, rows.Err()
}

func writeRoleStatements(builder *strings.Builder, rows []userRoleRow) {
	fmt.Fprintf(builder, "-- tb_users.role -> tb_user_roles (%d rows)\n", len(rows))
	for _, row := range rows {
		fmt.Fprintf(builder, `insert ignore into tb_user_roles (user_id, role_id)
select %d, r.id
from tb_roles r
where r.code = %s;
`, row.UserID, mysqlString(row.Role))
	}
	fmt.Fprintln(builder)
}

func targetColumns(columns []columnSpec) []string {
	result := make([]string, 0, len(columns))
	for _, col := range columns {
		result = append(result, col.Target)
	}
	return result
}

func mysqlValue(value any, column columnSpec) string {
	if value == nil {
		if column.Nullable {
			return "null"
		}
		value = column.Default
	}
	switch column.Kind {
	case kindBool:
		if boolValue(value) {
			return "1"
		}
		return "0"
	case kindJSON:
		text := strings.TrimSpace(valueString(value))
		if text == "" {
			return "null"
		}
		return mysqlString(text)
	case kindDateTime:
		if value == nil {
			return "null"
		}
		if t, ok := value.(time.Time); ok {
			return mysqlString(t.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04:05.000"))
		}
		text := strings.TrimSpace(valueString(value))
		if text == "" {
			return "null"
		}
		return mysqlString(text)
	default:
		return mysqlString(valueString(value))
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int:
		return typed != 0
	case []byte:
		return boolString(string(typed))
	case string:
		return boolString(typed)
	default:
		return false
	}
}

func boolString(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func valueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func mysqlString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "'", "''")
	return "'" + value + "'"
}

func mysqlIdent(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func joinMySQLIdents(columns []string) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, mysqlIdent(column))
	}
	return strings.Join(parts, ", ")
}

func pqIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quotedOrgList(orgIDs []string) string {
	parts := make([]string, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		parts = append(parts, mysqlString(orgID))
	}
	return strings.Join(parts, ", ")
}

func normalizeOrgIDs(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
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
	}
	sort.Strings(result)
	return result
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "editor", "operator":
		return "operator"
	default:
		return "viewer"
	}
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

func sanitizeComment(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "\r", " ")
}
