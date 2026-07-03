package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"
)

type pgMySQLSourceOrgsResponse struct {
	OK       bool                     `json:"ok"`
	Summary  pgMySQLSourceOrgsSummary `json:"summary,omitempty"`
	Orgs     []pgMySQLSourceOrg       `json:"orgs,omitempty"`
	Error    string                   `json:"error,omitempty"`
	Detail   string                   `json:"detail,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
}

type pgMySQLSourceOrgsSummary struct {
	SourceCount                     int `json:"source_count"`
	MySQLCount                      int `json:"mysql_count"`
	RemainingCount                  int `json:"remaining_count"`
	AllowedCount                    int `json:"allowed_count"`
	BatchableCount                  int `json:"batchable_count"`
	DuplicateSourceExternalOrgCount int `json:"duplicate_source_external_org_count"`
	DuplicateMySQLExternalOrgCount  int `json:"duplicate_mysql_external_org_count"`
}

type pgMySQLSourceOrg struct {
	ExternalOrgID       string `json:"external_org_id"`
	SourceStoreID       int64  `json:"source_store_id"`
	MySQLStoreID        int64  `json:"mysql_store_id,omitempty"`
	City                string `json:"city"`
	Name                string `json:"name"`
	RecorderCount       int    `json:"recorder_count"`
	ChannelCount        int    `json:"channel_count"`
	SnapshotCount       int    `json:"snapshot_count"`
	OperationLogCount   int    `json:"operation_log_count"`
	SourceStoreRefCount int    `json:"source_store_ref_count"`
	Migrated            bool   `json:"migrated"`
	Allowed             bool   `json:"allowed"`
	Batchable           bool   `json:"batchable"`
}

func (h *Handler) pgMySQLSourceOrgsHandler(w http.ResponseWriter, r *http.Request) {
	if !opsEnabled() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	result, err := queryPGMySQLSourceOrgsFromEnv(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, pgMySQLSourceOrgsResponse{
			OK:     false,
			Error:  "postgres mysql source orgs failed",
			Detail: sanitizeOpsError(err.Error()),
		})
		return
	}
	writeJSON(w, http.StatusOK, pgMySQLSourceOrgsResponse{
		OK:      true,
		Summary: result.Summary,
		Orgs:    result.Orgs,
		Warnings: []string{
			"Read-only inventory. This endpoint does not import data, copy assets, or mutate MySQL.",
			"Only batchable orgs are both missing from MySQL and present in the migration allowlist.",
		},
	})
}

func queryPGMySQLSourceOrgsFromEnv(ctx context.Context) (*pgMySQLSourceOrgsResponse, error) {
	pgDSN := envValue("DATABASE_URL", "K8S_SECRET_DATABASE_URL")
	if pgDSN == "" {
		return nil, errors.New("DATABASE_URL or K8S_SECRET_DATABASE_URL is required")
	}
	mysqlDSN := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
	if mysqlDSN == "" {
		return nil, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required")
	}
	pgDB, err := sql.Open("pgx", pgDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres source: %w", err)
	}
	defer pgDB.Close()
	mysqlDB, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql target: %w", err)
	}
	defer mysqlDB.Close()
	if err := pgDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres source: %w", err)
	}
	if err := mysqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql target: %w", err)
	}
	sourceOrgs, duplicateSourceCount, err := queryPostgresSourceOrgs(ctx, pgDB)
	if err != nil {
		return nil, err
	}
	mysqlStores, duplicateMySQLCount, err := queryMySQLTargetOrgStores(ctx, mysqlDB)
	if err != nil {
		return nil, err
	}
	allowed := map[string]struct{}{}
	for _, id := range allowedOpsMigrationOrgIDs() {
		allowed[id] = struct{}{}
	}
	summary := pgMySQLSourceOrgsSummary{
		SourceCount:                     len(sourceOrgs),
		MySQLCount:                      len(mysqlStores),
		DuplicateSourceExternalOrgCount: duplicateSourceCount,
		DuplicateMySQLExternalOrgCount:  duplicateMySQLCount,
	}
	for index := range sourceOrgs {
		org := &sourceOrgs[index]
		if mysqlID, ok := mysqlStores[org.ExternalOrgID]; ok {
			org.MySQLStoreID = mysqlID
			org.Migrated = true
		}
		if _, ok := allowed[org.ExternalOrgID]; ok {
			org.Allowed = true
			summary.AllowedCount++
		}
		if !org.Migrated {
			summary.RemainingCount++
		}
		org.Batchable = !org.Migrated && org.Allowed
		if org.Batchable {
			summary.BatchableCount++
		}
	}
	return &pgMySQLSourceOrgsResponse{Summary: summary, Orgs: sourceOrgs}, nil
}

func queryPostgresSourceOrgs(ctx context.Context, db *sql.DB) ([]pgMySQLSourceOrg, int, error) {
	rows, err := db.QueryContext(ctx, `
		select
			s.id,
			coalesce(s.external_org_id, ''),
			coalesce(s.city, ''),
			coalesce(s.name, ''),
			(select count(*) from video_recorders r where r.store_id = s.id),
			(select count(*) from video_channels c where c.recorder_id in (select r.id from video_recorders r where r.store_id = s.id)),
			(select count(*) from channel_snapshots cs where cs.channel_id in (select c.id from video_channels c where c.recorder_id in (select r.id from video_recorders r where r.store_id = s.id))),
			(select count(*) from operation_logs l where l.store_id = s.id)
		from stores s
		where trim(coalesce(s.external_org_id, '')) <> ''
		order by s.external_org_id, s.id
	`)
	if err != nil {
		return nil, 0, fmt.Errorf("query postgres source orgs: %w", err)
	}
	defer rows.Close()
	byID := map[string]*pgMySQLSourceOrg{}
	order := []string{}
	duplicates := 0
	for rows.Next() {
		var org pgMySQLSourceOrg
		if err := rows.Scan(
			&org.SourceStoreID,
			&org.ExternalOrgID,
			&org.City,
			&org.Name,
			&org.RecorderCount,
			&org.ChannelCount,
			&org.SnapshotCount,
			&org.OperationLogCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan postgres source orgs: %w", err)
		}
		org.SourceStoreRefCount = 1
		if existing, ok := byID[org.ExternalOrgID]; ok {
			duplicates++
			existing.SourceStoreRefCount++
			existing.RecorderCount += org.RecorderCount
			existing.ChannelCount += org.ChannelCount
			existing.SnapshotCount += org.SnapshotCount
			existing.OperationLogCount += org.OperationLogCount
			continue
		}
		byID[org.ExternalOrgID] = &org
		order = append(order, org.ExternalOrgID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read postgres source orgs: %w", err)
	}
	sort.Strings(order)
	orgs := make([]pgMySQLSourceOrg, 0, len(order))
	for _, id := range order {
		orgs = append(orgs, *byID[id])
	}
	return orgs, duplicates, nil
}

func queryMySQLTargetOrgStores(ctx context.Context, db *sql.DB) (map[string]int64, int, error) {
	rows, err := db.QueryContext(ctx, `
		select coalesce(external_org_id, ''), id
		from tb_stores
		where trim(coalesce(external_org_id, '')) <> ''
		order by external_org_id, id
	`)
	if err != nil {
		return nil, 0, fmt.Errorf("query mysql target org stores: %w", err)
	}
	defer rows.Close()
	stores := map[string]int64{}
	duplicates := 0
	for rows.Next() {
		var externalOrgID string
		var storeID int64
		if err := rows.Scan(&externalOrgID, &storeID); err != nil {
			return nil, 0, fmt.Errorf("scan mysql target org stores: %w", err)
		}
		if _, ok := stores[externalOrgID]; ok {
			duplicates++
			continue
		}
		stores[externalOrgID] = storeID
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read mysql target org stores: %w", err)
	}
	return stores, duplicates, nil
}
