package storespace

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMySQLStoreListWhereOmitsEmptyFilters(t *testing.T) {
	whereSQL, args := mysqlStoreListWhere(StoreFilters{Query: "", City: " "})
	if whereSQL != "" {
		t.Fatalf("whereSQL = %q, want empty", whereSQL)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, want empty", args)
	}
}

func TestMySQLStoreListWhereAddsRealFiltersOnly(t *testing.T) {
	whereSQL, args := mysqlStoreListWhere(StoreFilters{Query: " 北京 ", City: "上海"})
	if !strings.Contains(whereSQL, "where binary coalesce") {
		t.Fatalf("whereSQL = %q, want city condition", whereSQL)
	}
	if !strings.Contains(whereSQL, "and (replace") {
		t.Fatalf("whereSQL = %q, want search condition", whereSQL)
	}
	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3: %#v", len(args), args)
	}
	if args[0] != "上海" || args[1] != "%北京%" || args[2] != "%北京%" {
		t.Fatalf("args = %#v, want city and search args", args)
	}
}

func TestMySQLStoreSearchLikeEmptyQuery(t *testing.T) {
	for _, query := range []string{"", "   "} {
		pattern, ok := mysqlStoreSearchLike(query)
		if ok {
			t.Fatalf("mysqlStoreSearchLike(%q) ok = true, want false", query)
		}
		if pattern != "" {
			t.Fatalf("mysqlStoreSearchLike(%q) pattern = %q, want empty", query, pattern)
		}
	}
}

func TestMySQLStoreSearchLikeNormalizesQuery(t *testing.T) {
	pattern, ok := mysqlStoreSearchLike(" 北 京 10030 ")
	if !ok {
		t.Fatal("mysqlStoreSearchLike returned ok = false, want true")
	}
	if pattern != "%北京10030%" {
		t.Fatalf("mysqlStoreSearchLike pattern = %q, want %%北京10030%%", pattern)
	}
}

func TestMySQLDateTimeTextScansBytes(t *testing.T) {
	var value mysqlDateTimeText
	if err := value.Scan([]byte("2026-07-03 18:46:11.889000")); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	got := value.Time()
	if got.IsZero() {
		t.Fatal("Time returned zero, want parsed time")
	}
	if got.Format("2006-01-02 15:04:05.000") != "2026-07-03 18:46:11.889" {
		t.Fatalf("Time = %s, want 2026-07-03 18:46:11.889", got.Format("2006-01-02 15:04:05.000"))
	}
}

func TestMySQLDateTimeTextIgnoresZeroDate(t *testing.T) {
	var value mysqlDateTimeText
	if err := value.Scan("0000-00-00 00:00:00.000000"); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if !value.Time().IsZero() {
		t.Fatalf("Time = %s, want zero", value.Time().Format(time.RFC3339Nano))
	}
}

func TestMySQLChannelSnapshotUpdateArgsForRefresh(t *testing.T) {
	args := mysqlChannelSnapshotUpdateArgs(ChannelSnapshotInput{
		ThumbnailPath: "/api/store-space/channel-snapshots/fresh.jpg",
		FullImagePath: "/api/store-space/channel-snapshots/fresh.jpg",
	}, 607)

	if len(args) != 18 {
		t.Fatalf("len(args) = %d, want 18: %#v", len(args), args)
	}
	if args[0] != false {
		t.Fatalf("count attempt arg = %#v, want false", args[0])
	}
	if args[1] != false || args[2] != "" || args[5] != "" || args[8] != "" || args[10] != "" || args[13] != 0 || args[15] != "" || args[17] != int64(607) {
		t.Fatalf("unexpected refresh args: %#v", args)
	}
}

func TestMySQLUpsertRecorderChannelRejectsInvalidChannel(t *testing.T) {
	if _, err := mysqlValidateScannedChannel(ChannelInput{ChannelNo: 0, IsActive: true}); err == nil {
		t.Fatal("expected invalid channel to return an error")
	}
	if _, err := mysqlValidateScannedChannel(ChannelInput{ChannelNo: 1, IsActive: false}); err == nil {
		t.Fatal("expected inactive channel to return an error")
	}
}

func TestMySQLRecorderStatusForActiveCount(t *testing.T) {
	if got := mysqlRecorderStatusForActiveCount(0); got != RecorderStatusOffline {
		t.Fatalf("status for no active channels = %q, want %q", got, RecorderStatusOffline)
	}
	if got := mysqlRecorderStatusForActiveCount(3); got != RecorderStatusOnline {
		t.Fatalf("status for active channels = %q, want %q", got, RecorderStatusOnline)
	}
}

func TestMySQLStoreWriteMethodsAreImplemented(t *testing.T) {
	source := readMySQLStoreSourceForTest(t)
	for _, method := range []string{
		"CreateEzvizAccount",
		"CreateStore",
		"UpdateStoreBasicInfo",
		"SaveDesignPlan",
		"AddRecorder",
		"DeleteStore",
		"DeleteRecorder",
		"DeleteChannel",
	} {
		body := mysqlStoreMethodSourceForTest(source, method)
		if body == "" {
			t.Fatalf("MySQLStore.%s source not found", method)
		}
		if strings.Contains(body, "ErrNot"+"Implemented") {
			t.Fatalf("MySQLStore.%s still returns ErrNotImplemented", method)
		}
	}
}

func TestMySQLListStoresSummaryUsesFilteredDataset(t *testing.T) {
	source := readMySQLStoreSourceForTest(t)
	body := mysqlStoreMethodSourceForTest(source, "ListStores")
	if body == "" {
		t.Fatal("MySQLStore.ListStores source not found")
	}
	if !strings.Contains(body, "storeListSummary(ctx, filters)") {
		t.Fatalf("MySQLStore.ListStores must summarize the full filtered dataset, body=%s", body)
	}
	if strings.Contains(body, "summarizeStoreListItems(items)") {
		t.Fatalf("MySQLStore.ListStores still summarizes only current page items, body=%s", body)
	}
	if !strings.Contains(source, "binary coalesce(nullif(trim(s.city), ''), '未设置') = binary ?") {
		t.Fatal("MySQL store list filters must use binary city comparison to avoid mixed collation failures")
	}
	summaryBody := mysqlStoreMethodSourceForTest(source, "storeListSummary")
	for _, want := range []string{
		"mysqlStoreListWhere(filters)",
		"select count(*) from tb_stores s",
		"exists (select 1 from tb_stores s",
		"binary a.area_type",
	} {
		if !strings.Contains(summaryBody, want) {
			t.Fatalf("MySQLStore.storeListSummary missing %q, body=%s", want, summaryBody)
		}
	}
}

func TestMySQLStoreAvoidsStringNullifCollationMismatch(t *testing.T) {
	source := readMySQLStoreSourceForTest(t)
	if strings.Contains(source, "nullif(?, '')") {
		t.Fatal("mysql_store.go uses nullif on string parameters, which can fail with mixed MySQL collations")
	}
}

func readMySQLStoreSourceForTest(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("mysql_store.go")
	if err != nil {
		t.Fatalf("read mysql store source: %v", err)
	}
	return string(content)
}

func mysqlStoreMethodSourceForTest(source string, method string) string {
	marker := "func (s *MySQLStore) " + method + "("
	start := strings.Index(source, marker)
	if start < 0 {
		return ""
	}
	rest := source[start+len(marker):]
	next := strings.Index(rest, "\nfunc ")
	if next < 0 {
		return source[start:]
	}
	return source[start : start+len(marker)+next]
}
