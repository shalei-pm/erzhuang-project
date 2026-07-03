package mysqlmigration

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeOrgIDs(t *testing.T) {
	got := normalizeOrgIDs([]string{"10047,10030", "10030", "  "})
	want := []string{"10030", "10047"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normalizeOrgIDs()=%v, want %v", got, want)
	}
}

func TestFilterClauseScopesStoreRelatedTables(t *testing.T) {
	orgs := []string{"10030"}
	cases := map[filterKind]string{
		filterStore:                "where external_org_id in ('10030')",
		filterStoreArea:            "where store_id in (select id from stores where external_org_id in ('10030'))",
		filterStoreDesignPlan:      "where store_id in (select id from stores where external_org_id in ('10030'))",
		filterDesignPlanAnnotation: "join stores s on s.id = p.store_id",
		filterRecorder:             "where store_id in (select id from stores where external_org_id in ('10030'))",
		filterChannel:              "join stores s on s.id = r.store_id",
		filterChannelSnapshot:      "join stores s on s.id = r.store_id",
	}
	for kind, wantFragment := range cases {
		t.Run(wantFragment, func(t *testing.T) {
			got := filterClause(kind, orgs)
			if !strings.Contains(got, wantFragment) {
				t.Fatalf("filterClause()=%q, want fragment %q", got, wantFragment)
			}
		})
	}
}

func TestBuildExportQueryUsesTableSpecificOrderColumn(t *testing.T) {
	spec := tableSpec{
		Source:  "app_settings",
		Target:  "tb_app_settings",
		OrderBy: "key",
	}
	got := buildExportQuery(spec, Options{}, map[string]bool{"key": true})
	want := `select * from "app_settings" order by "key"`
	if got != want {
		t.Fatalf("buildExportQuery()=%q, want %q", got, want)
	}
}

func TestBuildExportQueryOmitsOrderWhenColumnMissing(t *testing.T) {
	spec := tableSpec{
		Source: "app_settings",
		Target: "tb_app_settings",
	}
	got := buildExportQuery(spec, Options{}, map[string]bool{"key": true})
	want := `select * from "app_settings"`
	if got != want {
		t.Fatalf("buildExportQuery()=%q, want %q", got, want)
	}
}

func TestMySQLValueFormatsTimeAsShanghaiDateTime(t *testing.T) {
	value := time.Date(2026, 7, 2, 10, 30, 11, 123000000, time.UTC)
	got := mysqlValue(value, columnSpec{Kind: kindDateTime})
	if got != "'2026-07-02 18:30:11.123'" {
		t.Fatalf("mysqlValue(time)=%s", got)
	}
}

func TestMySQLValueEscapesBackslashesInJSONLiteral(t *testing.T) {
	value := `{"raw_notes":"line\nquote \"x\" path C:\\tmp"}`
	got := mysqlValue(value, columnSpec{Kind: kindJSON})
	want := `'{"raw_notes":"line\\nquote \\"x\\" path C:\\\\tmp"}'`
	if got != want {
		t.Fatalf("mysqlValue(json)=%s, want %s", got, want)
	}
}

func TestWriteInsertStatementsPreservesIDAndUsesUpsert(t *testing.T) {
	spec := tableSpec{
		Source: "stores",
		Target: "tb_stores",
		Columns: []columnSpec{
			{Target: "id", Source: "id"},
			{Target: "name", Source: "name", Default: ""},
			{Target: "external_org_id", Source: "external_org_id", Default: ""},
		},
	}
	rows := []rowData{{
		"id":              int64(7),
		"name":            "新氧青春诊所",
		"external_org_id": "10030",
	}}
	var builder strings.Builder
	if err := writeInsertStatements(&builder, spec, rows); err != nil {
		t.Fatalf("writeInsertStatements: %v", err)
	}
	sql := builder.String()
	for _, want := range []string{
		"insert into `tb_stores` (`id`, `name`, `external_org_id`)",
		"'新氧青春诊所'",
		"on duplicate key update",
		"`name` = values(`name`)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("sql missing %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "`id` = values(`id`)") {
		t.Fatalf("id should not be updated on duplicate:\n%s", sql)
	}
}

func TestWriteRoleStatementsMapsEditorToOperator(t *testing.T) {
	var builder strings.Builder
	writeRoleStatements(&builder, []userRoleRow{{UserID: 3, Role: normalizeRole("editor")}})
	sql := builder.String()
	if !strings.Contains(sql, "select 3, r.id") || !strings.Contains(sql, "r.code = 'operator'") {
		t.Fatalf("unexpected role sql:\n%s", sql)
	}
}
