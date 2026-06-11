package storespace

import (
	"strings"
	"testing"
)

func TestSchemaEnablesRLSAndRejectsClientAccessForPublicTables(t *testing.T) {
	tables := []string{
		"stores",
		"store_areas",
		"store_design_plans",
		"design_plan_annotations",
		"ezviz_accounts",
		"video_recorders",
		"video_channels",
		"channel_snapshots",
		"operation_logs",
	}

	schema := strings.Join(PostgresSchemaStatements(), "\n")
	for _, table := range tables {
		if !strings.Contains(schema, "alter table "+table+" enable row level security") {
			t.Fatalf("schema does not enable RLS for %s", table)
		}
		policyName := table + "_no_client_access"
		if !strings.Contains(schema, "create policy "+policyName+" on "+table) {
			t.Fatalf("schema does not create no client access policy for %s", table)
		}
		if !strings.Contains(schema, "for all to anon, authenticated") {
			t.Fatalf("schema policy for %s does not target anon/authenticated", table)
		}
	}

	if !strings.Contains(schema, "using (false)") || !strings.Contains(schema, "with check (false)") {
		t.Fatal("schema policies must explicitly deny using/check")
	}
}
