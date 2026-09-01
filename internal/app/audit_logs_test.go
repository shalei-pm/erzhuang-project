package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAuditLogMemoryStoreUsesLeftClosedRightOpenTimeRange(t *testing.T) {
	store := NewMemoryStore()
	createdAt := []time.Time{
		time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 31, 11, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC),
	}
	for _, created := range createdAt {
		store.now = func() time.Time { return created }
		if err := store.CreateAuditLog(context.Background(), AuditLog{Action: "monitor.live_view"}); err != nil {
			t.Fatalf("create audit log: %v", err)
		}
	}

	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt:  createdAt[1],
		EndAt:    createdAt[2],
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || !page.Items[0].CreatedAt.Equal(createdAt[1]) {
		t.Fatalf("expected only lower-bound log, got %#v", page)
	}
}

func TestAuditLogMemoryStorePaginatesByCreatedAtThenID(t *testing.T) {
	store := NewMemoryStore()
	created := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return created }
	for i := 0; i < 4; i++ {
		if err := store.CreateAuditLog(context.Background(), AuditLog{Action: "user.update"}); err != nil {
			t.Fatalf("create audit log: %v", err)
		}
	}

	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt:  created.Add(-time.Hour),
		EndAt:    created.Add(time.Hour),
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if page.Total != 4 || len(page.Items) != 2 || page.Items[0].ID != 2 || page.Items[1].ID != 1 {
		t.Fatalf("unexpected second page: %#v", page)
	}
}

func TestAuditLogMemoryStorePaginatesPastLastPage(t *testing.T) {
	store := NewMemoryStore()
	created := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return created }
	for i := 0; i < 5; i++ {
		if err := store.CreateAuditLog(context.Background(), AuditLog{Action: "user.update"}); err != nil {
			t.Fatalf("create audit log: %v", err)
		}
	}
	filter := AuditLogFilter{
		StartAt:  created.Add(-time.Hour),
		EndAt:    created.Add(time.Hour),
		PageSize: 2,
	}

	filter.Page = 3
	lastPage, err := store.ListAuditLogs(context.Background(), filter)
	if err != nil {
		t.Fatalf("list last audit log page: %v", err)
	}
	if lastPage.Page != 3 || lastPage.PageSize != 2 || lastPage.Total != 5 || len(lastPage.Items) != 1 || lastPage.Items[0].ID != 1 {
		t.Fatalf("unexpected final page: %#v", lastPage)
	}

	filter.Page = 4
	beyondLastPage, err := store.ListAuditLogs(context.Background(), filter)
	if err != nil {
		t.Fatalf("list beyond last audit log page: %v", err)
	}
	if beyondLastPage.Page != 4 || beyondLastPage.PageSize != 2 || beyondLastPage.Total != 5 || len(beyondLastPage.Items) != 0 {
		t.Fatalf("unexpected page beyond final page: %#v", beyondLastPage)
	}
}

func TestAuditLogMemoryStoreFiltersAction(t *testing.T) {
	store := NewMemoryStore()
	created := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return created }
	for _, action := range []string{"auth.login", "user.update", "auth.login"} {
		if err := store.CreateAuditLog(context.Background(), AuditLog{Action: action}); err != nil {
			t.Fatalf("create audit log: %v", err)
		}
	}

	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt:  created.Add(-time.Hour),
		EndAt:    created.Add(time.Hour),
		Action:   "auth.login",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("expected two login logs, got %#v", page)
	}
	for _, log := range page.Items {
		if log.Action != "auth.login" {
			t.Fatalf("unexpected action %q", log.Action)
		}
	}
}

func TestAuditLogMemoryStorePreservesAssetLogicalKeyWithoutJSONExposure(t *testing.T) {
	store := NewMemoryStore()
	created := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return created }
	if err := store.CreateAuditLog(context.Background(), AuditLog{
		Action:          "snapshot.download",
		AssetLogicalKey: " channel-snapshots/private-object.jpg ",
	}); err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt:  created.Add(-time.Hour),
		EndAt:    created.Add(time.Hour),
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].AssetLogicalKey != "channel-snapshots/private-object.jpg" {
		t.Fatalf("unexpected asset logical key: %#v", page)
	}
	encoded, err := json.Marshal(page.Items[0])
	if err != nil {
		t.Fatalf("marshal audit log: %v", err)
	}
	if strings.Contains(string(encoded), "asset_logical_key") || strings.Contains(string(encoded), "private-object.jpg") {
		t.Fatalf("asset logical key must not be exposed: %s", encoded)
	}
}

func TestMySQLAuditLogWhereUsesBoundParameters(t *testing.T) {
	userID := int64(42)
	filter := AuditLogFilter{
		StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		UserID:  &userID,
		Action:  "auth.login' or 1=1 --",
	}
	where, args := mysqlAuditLogWhere(filter)
	if !strings.Contains(where, "created_at >= ?") || !strings.Contains(where, "created_at < ?") || !strings.Contains(where, "user_id = ?") || !strings.Contains(where, "action = ?") {
		t.Fatalf("unexpected query conditions: %q", where)
	}
	if strings.Contains(where, filter.Action) {
		t.Fatalf("action must not be interpolated into query: %q", where)
	}
	if len(args) != 4 || args[2] != userID || args[3] != filter.Action {
		t.Fatalf("unexpected query args: %#v", args)
	}
}

func TestMySQLStoreCreatesAuditLogWithAssetLogicalKey(t *testing.T) {
	recorder := newRecordingSQLDriver(t)
	db, err := sql.Open(recorder.driverName, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	defer db.Close()

	if err := NewMySQLStore(db).CreateAuditLog(context.Background(), AuditLog{
		Action:          "snapshot.download",
		AssetLogicalKey: "channel-snapshots/private-object.jpg",
	}); err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	queries := recorder.queries()
	if len(queries) != 1 || !strings.Contains(queries[0], "asset_logical_key") {
		t.Fatalf("expected asset logical key insert column, got %#v", queries)
	}
}

func TestScanAuditLogReadsAssetLogicalKey(t *testing.T) {
	created := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	log, err := scanAuditLog(auditLogScanner(func(dest ...any) error {
		*dest[0].(*int64) = 7
		*dest[1].(*sql.NullInt64) = sql.NullInt64{Int64: 3, Valid: true}
		*dest[2].(*string) = "operator"
		*dest[3].(*string) = "operator@example.com"
		*dest[4].(*string) = "snapshot.download"
		*dest[5].(*string) = "asset"
		*dest[6].(*sql.NullInt64) = sql.NullInt64{Int64: 11, Valid: true}
		*dest[7].(*sql.NullInt64) = sql.NullInt64{Int64: 12, Valid: true}
		*dest[8].(*string) = "10030"
		*dest[9].(*sql.NullInt64) = sql.NullInt64{Int64: 13, Valid: true}
		*dest[10].(*string) = " channel-snapshots/private-object.jpg "
		*dest[11].(*string) = "192.0.2.1"
		*dest[12].(*string) = "test-agent"
		*dest[13].(*string) = "request-123"
		*dest[14].(*string) = "success"
		*dest[15].(*sql.NullString) = sql.NullString{String: `{"summary":"downloaded snapshot"}`, Valid: true}
		*dest[16].(*time.Time) = created
		return nil
	}))
	if err != nil {
		t.Fatalf("scan audit log: %v", err)
	}
	if log.AssetLogicalKey != "channel-snapshots/private-object.jpg" {
		t.Fatalf("unexpected asset logical key: %#v", log)
	}
}

func TestSanitizeAuditDetailDropsSensitiveFields(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"updated store scope",
		"source":"user_management",
		"scope_count":2,
		"token":"secret-token",
		"authorization":"Bearer secret-token",
		"wss_url":"wss://private.example/stream",
		"signed_url":"https://private.example/object?signature=secret",
		"media":"base64-media",
		"nested":{"password":"secret"}
	}`)
	sanitized := sanitizeAuditDetail(raw)
	var detail map[string]any
	if err := json.Unmarshal(sanitized, &detail); err != nil {
		t.Fatalf("decode sanitized detail: %v", err)
	}
	if len(detail) != 3 || detail["summary"] != "updated store scope" || detail["source"] != "user_management" || detail["scope_count"] != float64(2) {
		t.Fatalf("unexpected sanitized detail: %#v", detail)
	}
	for _, sensitiveKey := range []string{"token", "authorization", "wss_url", "signed_url", "media", "nested"} {
		if _, ok := detail[sensitiveKey]; ok {
			t.Fatalf("sensitive key %q was retained", sensitiveKey)
		}
	}

	if detail := sanitizeAuditDetail(json.RawMessage(`{"summary":"wss://private.example/stream"}`)); detail != nil {
		t.Fatalf("expected sensitive summary value to be dropped, got %s", detail)
	}
	for _, value := range []string{
		"password: secret-value",
		"token = secret-value",
		"authorization : Bearer secret-value",
		"media: data:image/jpeg;base64,SGVsbG8=",
		strings.Repeat("QUJD", 32),
	} {
		if detail := sanitizeAuditDetail(json.RawMessage(`{"summary":` + mustMarshalAuditDetailString(t, value) + `}`)); detail != nil {
			t.Fatalf("expected sensitive summary value to be dropped: %q, got %s", value, detail)
		}
	}
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "access token", value: "access_token=secret-value"},
		{name: "client secret", value: "client_secret: secret-value"},
		{name: "user password", value: "user_password = secret-value"},
		{name: "quoted access token JSON fragment", value: `{"access_token":"secret-value"}`},
		{name: "quoted client secret JSON fragment", value: `{"client_secret" : "secret-value"}`},
		{name: "quoted user password JSON fragment", value: `{"user_password" = "secret-value"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !containsSensitiveAuditValue(test.value) {
				t.Fatalf("expected value detection to reject %q", test.value)
			}
			detail := sanitizeAuditDetail(json.RawMessage(`{"summary":` + mustMarshalAuditDetailString(t, test.value) + `}`))
			if detail != nil {
				t.Fatalf("expected sanitized summary to be dropped for %q, got %s", test.value, detail)
			}
		})
	}
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "bearer credential", value: "Bearer abcdefghijklmnopqrst"},
		{name: "signature URL", value: "https://private.example/object?signature=secret-value"},
		{name: "bare data URI", value: "data;base64,SGVsbG8="},
		{name: "data URI with MIME type", value: "data:image/jpeg;base64,SGVsbG8="},
		{name: "bare data URI prefix", value: "data;base64"},
		{name: "bare colon data URI prefix", value: "data:;base64"},
		{name: "bare MIME data URI prefix", value: "data:image/png;base64"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !containsSensitiveAuditValue(test.value) {
				t.Fatalf("expected value detection to reject %q", test.value)
			}
			detail := sanitizeAuditDetail(json.RawMessage(`{"summary":` + mustMarshalAuditDetailString(t, test.value) + `}`))
			if detail != nil {
				t.Fatalf("expected sanitized summary to be dropped for %q, got %s", test.value, detail)
			}
		})
	}
	for _, value := range []string{"token policy updated", "password policy updated", "media retention changed"} {
		detail := sanitizeAuditDetail(json.RawMessage(`{"summary":` + mustMarshalAuditDetailString(t, value) + `}`))
		if !strings.Contains(string(detail), value) {
			t.Fatalf("expected safe business summary to be retained: %q, got %s", value, detail)
		}
	}

	encoded, err := json.Marshal(AuditLog{
		DetailJSON: raw,
		IPAddress:  "192.0.2.1",
		UserAgent:  "test-agent",
		RequestID:  "request-123",
	})
	if err != nil {
		t.Fatalf("marshal audit log: %v", err)
	}
	for _, unexpected := range []string{"detail_json", "192.0.2.1", "test-agent", "request-123"} {
		if strings.Contains(string(encoded), unexpected) {
			t.Fatalf("unexpected direct JSON value %q", unexpected)
		}
	}
}

type auditLogScanner func(dest ...any) error

func (scanner auditLogScanner) Scan(dest ...any) error {
	return scanner(dest...)
}

func mustMarshalAuditDetailString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal audit detail string: %v", err)
	}
	return string(encoded)
}

func TestAuditLogMemoryStoreReturnsEmptyPageAndNormalizesPaging(t *testing.T) {
	store := NewMemoryStore()
	page, err := store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt:  time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		EndAt:    time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		Page:     0,
		PageSize: 101,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if page.Page != 1 || page.PageSize != maxAuditLogPageSize || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("unexpected empty page: %#v", page)
	}

	page, err = store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		Page:    -1,
	})
	if err != nil {
		t.Fatalf("list audit logs with default page size: %v", err)
	}
	if page.Page != 1 || page.PageSize != defaultAuditLogPageSize {
		t.Fatalf("unexpected normalized page: %#v", page)
	}
}
