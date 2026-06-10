package designplan

import (
	"context"
	"database/sql"
)

func EnsurePostgresSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`create table if not exists design_plan_stores (
			id bigserial primary key,
			name text not null,
			normalized_name text not null unique,
			pdf_file_name text not null default '',
			original_pdf_path text not null default '',
			preview_image_path text not null default '',
			thumbnail_path text not null default '',
			page_count integer not null default 0,
			status text not null default 'completed',
			recognition_result jsonb,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint design_plan_stores_status_check
				check (status in ('completed', 'needs_review', 'incomplete'))
		)`,
		`create table if not exists design_plan_store_areas (
			id bigserial primary key,
			store_id bigint not null references design_plan_stores(id) on delete cascade,
			display_order integer not null,
			name text not null,
			area_type text not null,
			area_number integer,
			confidence text not null default 'high',
			needs_review boolean not null default false,
			box_x numeric not null,
			box_y numeric not null,
			box_width numeric not null,
			box_height numeric not null,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint design_plan_store_areas_type_check
				check (area_type in ('treatment', 'consultation', 'beauty')),
			constraint design_plan_store_areas_confidence_check
				check (confidence in ('high', 'medium', 'low')),
			constraint design_plan_store_areas_box_check check (
				box_x >= 0 and box_x <= 1 and
				box_y >= 0 and box_y <= 1 and
				box_width > 0 and box_width <= 1 and
				box_height > 0 and box_height <= 1 and
				box_x + box_width <= 1 and
				box_y + box_height <= 1
			)
		)`,
		`create unique index if not exists design_plan_store_areas_unique_number_per_type
			on design_plan_store_areas (store_id, area_type, area_number)
			where area_number is not null`,
		`create index if not exists design_plan_stores_updated_at_idx
			on design_plan_stores (updated_at desc)`,
		`alter table design_plan_stores
			add column if not exists pdf_file_name text not null default ''`,
		`create index if not exists design_plan_store_areas_store_id_idx
			on design_plan_store_areas (store_id, display_order)`,
		`create table if not exists design_plan_operation_logs (
			id bigserial primary key,
			action text not null,
			store_id bigint,
			store_name text not null,
			actor text not null default 'admin',
			summary text not null,
			created_at timestamptz not null default now(),
			constraint design_plan_operation_logs_action_check
				check (action in ('create', 'update', 'delete', 'replace'))
		)`,
		`create index if not exists design_plan_operation_logs_store_id_idx
			on design_plan_operation_logs (store_id, created_at desc)`,
		`alter table design_plan_stores enable row level security`,
		`alter table design_plan_store_areas enable row level security`,
		`alter table design_plan_operation_logs enable row level security`,
		`drop policy if exists design_plan_stores_no_client_access on design_plan_stores`,
		`create policy design_plan_stores_no_client_access on design_plan_stores
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists design_plan_store_areas_no_client_access on design_plan_store_areas`,
		`create policy design_plan_store_areas_no_client_access on design_plan_store_areas
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists design_plan_operation_logs_no_client_access on design_plan_operation_logs`,
		`create policy design_plan_operation_logs_no_client_access on design_plan_operation_logs
			for all to anon, authenticated
			using (false)
			with check (false)`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
