package storespace

import (
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
	if !strings.Contains(whereSQL, "where coalesce") {
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
