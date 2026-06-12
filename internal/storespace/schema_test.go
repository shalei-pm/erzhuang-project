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

func TestSchemaMigratesLegacyDesignPlanStores(t *testing.T) {
	schema := strings.Join(PostgresSchemaStatements(), "\n")
	required := []string{
		"from design_plan_stores dps",
		"insert into stores",
		"insert into store_design_plans",
		"insert into store_areas",
		"insert into design_plan_annotations",
		"legacy-",
		"on conflict (normalized_name) do nothing",
		"on conflict (store_id, area_type, area_number) do nothing",
		"where not exists",
		"and sdp.upload_id = 'legacy-' || dps.id::text",
	}

	for _, part := range required {
		if !strings.Contains(schema, part) {
			t.Fatalf("schema does not include legacy migration fragment %q", part)
		}
	}

	if strings.Contains(schema, "from design_plan_stores dps\n\t\tjoin stores s on s.normalized_name = dps.normalized_name\n\t\ton conflict do nothing") {
		t.Fatal("store_design_plans legacy migration must use not exists because it has no unique conflict target")
	}
}
