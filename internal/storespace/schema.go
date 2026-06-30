package storespace

import (
	"context"
	"database/sql"
)

func EnsurePostgresSchema(ctx context.Context, db *sql.DB) error {
	for _, statement := range PostgresSchemaStatements() {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func PostgresSchemaStatements() []string {
	return []string{
		`create table if not exists stores (
			id bigserial primary key,
			city text not null default '',
			name text not null,
			normalized_name text not null unique,
			external_org_id text not null default '',
			design_plan_status text not null default 'not_uploaded',
			overall_status text not null default 'partial',
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint stores_design_plan_status_check
				check (design_plan_status in ('not_uploaded', 'pending_recognition', 'pending_annotation', 'completed')),
			constraint stores_overall_status_check
				check (overall_status in ('incomplete', 'partial', 'completed', 'exception'))
		)`,
		`alter table stores add column if not exists city text not null default ''`,
		`alter table stores add column if not exists short_name text not null default ''`,
		`create index if not exists stores_updated_at_idx on stores (updated_at desc)`,
		`create table if not exists store_areas (
			id bigserial primary key,
			store_id bigint not null references stores(id) on delete cascade,
			area_type text not null,
			area_number integer not null,
			display_name text not null,
			source text not null default 'manual',
			status text not null default 'confirmed',
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint store_areas_type_check
				check (area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')),
			constraint store_areas_source_check
				check (source in ('manual', 'design_plan', 'video_channel', 'multiple')),
			constraint store_areas_status_check
				check (status in ('candidate', 'confirmed')),
			constraint store_areas_area_number_check
				check ((area_type = 'vip_treatment' and area_number >= 0) or (area_type <> 'vip_treatment' and area_number > 0))
		)`,
		`alter table store_areas drop constraint if exists store_areas_type_check`,
		`alter table store_areas add constraint store_areas_type_check
			check (area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty'))`,
		`alter table store_areas drop constraint if exists store_areas_area_number_check`,
		`alter table store_areas add constraint store_areas_area_number_check
			check ((area_type = 'vip_treatment' and area_number >= 0) or (area_type <> 'vip_treatment' and area_number > 0))`,
		`create unique index if not exists store_areas_unique_number_per_type
			on store_areas (store_id, area_type, area_number)`,
		`create table if not exists store_design_plans (
			id bigserial primary key,
			store_id bigint not null references stores(id) on delete cascade,
			upload_id text not null default '',
			pdf_file_name text not null default '',
			original_pdf_path text not null default '',
			preview_image_path text not null default '',
			thumbnail_path text not null default '',
			page_count integer not null default 0,
			recognition_status text not null default 'not_started',
			recognition_result jsonb,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint store_design_plans_recognition_status_check
				check (recognition_status in ('not_started', 'running', 'failed', 'completed'))
		)`,
		`create index if not exists store_design_plans_store_id_idx on store_design_plans (store_id)`,
		`create table if not exists design_plan_annotations (
			id bigserial primary key,
			design_plan_id bigint not null references store_design_plans(id) on delete cascade,
			area_id bigint not null references store_areas(id) on delete cascade,
			box_x numeric not null,
			box_y numeric not null,
			box_width numeric not null,
			box_height numeric not null,
			status text not null default 'pending',
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint design_plan_annotations_status_check
				check (status in ('pending', 'confirmed')),
			constraint design_plan_annotations_box_check check (
				box_x >= 0 and box_x <= 1 and
				box_y >= 0 and box_y <= 1 and
				box_width > 0 and box_width <= 1 and
				box_height > 0 and box_height <= 1 and
				box_x + box_width <= 1 and
				box_y + box_height <= 1
			)
		)`,
		`create unique index if not exists design_plan_annotations_unique_area
			on design_plan_annotations (design_plan_id, area_id)`,
		`create table if not exists ezviz_accounts (
			id bigserial primary key,
			account_name text not null unique,
			app_key text not null default '',
			app_secret_ciphertext text not null default '',
			access_token_ciphertext text not null default '',
			status text not null default 'unverified',
			last_verified_at timestamptz,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint ezviz_accounts_status_check
				check (status in ('unverified', 'available', 'unavailable'))
		)`,
		`create table if not exists video_recorders (
			id bigserial primary key,
			store_id bigint not null references stores(id) on delete cascade,
			ezviz_account_id bigint references ezviz_accounts(id),
			device_code text not null unique,
			status text not null default 'offline',
			effective_channel_count integer not null default 0,
			last_scanned_at timestamptz,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint video_recorders_status_check
				check (status in ('online', 'offline')),
			constraint video_recorders_effective_channel_count_check
				check (effective_channel_count >= 0)
		)`,
		`create index if not exists video_recorders_store_id_idx on video_recorders (store_id)`,
		`create table if not exists video_channels (
			id bigserial primary key,
			recorder_id bigint not null references video_recorders(id) on delete cascade,
			channel_no integer not null,
			channel_name text not null default '',
			status text not null default 'pending_recognition',
			is_active boolean not null default true,
			scene_type text not null default 'unknown',
			area_type text,
			area_number integer,
			area_note text not null default '',
			area_id bigint references store_areas(id),
			recognition_attempts integer not null default 0,
			recognition_result jsonb,
			confirmed_at timestamptz,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			constraint video_channels_status_check
				check (status in ('pending_recognition', 'pending_confirmation', 'confirmed_business', 'confirmed_non_business', 'recognition_failed', 'inactive')),
			constraint video_channels_scene_type_check
				check (scene_type in ('treatment', 'vip_treatment', 'consultation', 'beauty', 'front_desk', 'corridor', 'passage', 'waiting_area', 'hall', 'entrance', 'storage', 'pharmacy', 'machine_room', 'unknown')),
			constraint video_channels_area_type_check
				check (area_type is null or area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')),
			constraint video_channels_channel_no_check
				check (channel_no > 0),
			constraint video_channels_recognition_attempts_check
				check (recognition_attempts >= 0)
		)`,
		`alter table video_channels add column if not exists area_note text not null default ''`,
		`alter table video_channels add column if not exists bed_label text not null default ''`,
		`alter table video_channels drop constraint if exists video_channels_scene_type_check`,
		`alter table video_channels add constraint video_channels_scene_type_check
			check (scene_type in ('treatment', 'vip_treatment', 'consultation', 'beauty', 'front_desk', 'corridor', 'passage', 'waiting_area', 'hall', 'entrance', 'storage', 'pharmacy', 'machine_room', 'unknown'))`,
		`alter table video_channels drop constraint if exists video_channels_area_type_check`,
		`alter table video_channels add constraint video_channels_area_type_check
			check (area_type is null or area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty'))`,
		`create unique index if not exists video_channels_unique_channel
			on video_channels (recorder_id, channel_no)`,
		`create table if not exists channel_snapshots (
			id bigserial primary key,
			channel_id bigint not null references video_channels(id) on delete cascade,
			thumbnail_path text not null default '',
			full_image_path text not null default '',
			full_image_expires_at timestamptz,
			created_at timestamptz not null default now()
		)`,
		`create index if not exists channel_snapshots_channel_id_idx on channel_snapshots (channel_id, created_at desc)`,
		`create table if not exists operation_logs (
			id bigserial primary key,
			action text not null,
			entity_type text not null,
			entity_id bigint,
			store_id bigint,
			actor text not null default 'admin',
			summary text not null,
			created_at timestamptz not null default now()
		)`,
		`create index if not exists operation_logs_store_id_idx
			on operation_logs (store_id, created_at desc)`,
		`alter table stores enable row level security`,
		`alter table store_areas enable row level security`,
		`alter table store_design_plans enable row level security`,
		`alter table design_plan_annotations enable row level security`,
		`alter table ezviz_accounts enable row level security`,
		`alter table video_recorders enable row level security`,
		`alter table video_channels enable row level security`,
		`alter table channel_snapshots enable row level security`,
		`alter table operation_logs enable row level security`,
		`drop policy if exists stores_no_client_access on stores`,
		`create policy stores_no_client_access on stores
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists store_areas_no_client_access on store_areas`,
		`create policy store_areas_no_client_access on store_areas
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists store_design_plans_no_client_access on store_design_plans`,
		`create policy store_design_plans_no_client_access on store_design_plans
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists design_plan_annotations_no_client_access on design_plan_annotations`,
		`create policy design_plan_annotations_no_client_access on design_plan_annotations
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists ezviz_accounts_no_client_access on ezviz_accounts`,
		`create policy ezviz_accounts_no_client_access on ezviz_accounts
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists video_recorders_no_client_access on video_recorders`,
		`create policy video_recorders_no_client_access on video_recorders
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists video_channels_no_client_access on video_channels`,
		`create policy video_channels_no_client_access on video_channels
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists channel_snapshots_no_client_access on channel_snapshots`,
		`create policy channel_snapshots_no_client_access on channel_snapshots
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`drop policy if exists operation_logs_no_client_access on operation_logs`,
		`create policy operation_logs_no_client_access on operation_logs
			for all to anon, authenticated
			using (false)
			with check (false)`,
		`insert into stores (city, name, normalized_name, design_plan_status, overall_status, created_at, updated_at)
		select
			'',
			dps.name,
			dps.normalized_name,
			case
				when dps.preview_image_path <> '' then 'completed'
				else 'not_uploaded'
			end,
			case
				when dps.status = 'completed' then 'completed'
				when dps.status = 'incomplete' then 'incomplete'
				else 'partial'
			end,
			dps.created_at,
			dps.updated_at
		from design_plan_stores dps
		on conflict (normalized_name) do nothing`,
		`insert into store_design_plans (
			store_id, upload_id, pdf_file_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, recognition_status, recognition_result, created_at, updated_at
		)
		select
			s.id,
			'legacy-' || dps.id::text,
			dps.pdf_file_name,
			dps.original_pdf_path,
			dps.preview_image_path,
			dps.thumbnail_path,
			dps.page_count,
			case
				when dps.status = 'completed' then 'completed'
				else 'not_started'
			end,
			dps.recognition_result,
			dps.created_at,
			dps.updated_at
		from design_plan_stores dps
		join stores s on s.normalized_name = dps.normalized_name
		where not exists (
			select 1
			from store_design_plans sdp
			where sdp.store_id = s.id
				and sdp.upload_id = 'legacy-' || dps.id::text
		)`,
		`with legacy_area_rows as (
			select
				s.id as store_id,
				dps.id as legacy_store_id,
				dpa.area_type,
				dpa.area_number,
				max(coalesce(nullif(dpa.area_number, 0), 0)) over (
					partition by s.id, dpa.area_type
				) as max_existing_area_number,
				sum(case when dpa.area_number is null or dpa.area_number <= 0 then 1 else 0 end) over (
					partition by s.id, dpa.area_type
					order by dpa.display_order, dpa.id
				) as missing_area_index,
				dpa.name,
				dpa.box_x,
				dpa.box_y,
				dpa.box_width,
				dpa.box_height,
				dpa.created_at,
				dpa.updated_at
			from design_plan_store_areas dpa
			join design_plan_stores dps on dps.id = dpa.store_id
			join stores s on s.normalized_name = dps.normalized_name
			where dpa.area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')
		),
		legacy_areas as (
			select
				store_id,
				legacy_store_id,
				area_type,
				case
					when area_number is not null and area_number > 0 then area_number
					else max_existing_area_number + missing_area_index
				end::integer as area_number,
				name,
				box_x,
				box_y,
				box_width,
				box_height,
				created_at,
				updated_at
			from legacy_area_rows
		),
		inserted_areas as (
			insert into store_areas (store_id, area_type, area_number, display_name, source, status, created_at, updated_at)
			select
				store_id,
				area_type,
				area_number,
				name,
				'design_plan',
				'confirmed',
				created_at,
				updated_at
			from legacy_areas
			on conflict (store_id, area_type, area_number) do nothing
			returning id
		)
		select count(*) from inserted_areas`,
		`with legacy_area_rows as (
			select
				s.id as store_id,
				dps.id as legacy_store_id,
				dpa.area_type,
				dpa.area_number,
				max(coalesce(nullif(dpa.area_number, 0), 0)) over (
					partition by s.id, dpa.area_type
				) as max_existing_area_number,
				sum(case when dpa.area_number is null or dpa.area_number <= 0 then 1 else 0 end) over (
					partition by s.id, dpa.area_type
					order by dpa.display_order, dpa.id
				) as missing_area_index,
				dpa.name,
				dpa.box_x,
				dpa.box_y,
				dpa.box_width,
				dpa.box_height,
				dpa.created_at,
				dpa.updated_at
			from design_plan_store_areas dpa
			join design_plan_stores dps on dps.id = dpa.store_id
			join stores s on s.normalized_name = dps.normalized_name
			where dpa.area_type in ('treatment', 'vip_treatment', 'consultation', 'beauty')
		),
		legacy_areas as (
			select
				store_id,
				legacy_store_id,
				area_type,
				case
					when area_number is not null and area_number > 0 then area_number
					else max_existing_area_number + missing_area_index
				end::integer as area_number,
				box_x,
				box_y,
				box_width,
				box_height,
				created_at,
				updated_at
			from legacy_area_rows
		)
		insert into design_plan_annotations (
			design_plan_id, area_id, box_x, box_y, box_width, box_height, status, created_at, updated_at
		)
		select
			sdp.id,
			sa.id,
			la.box_x,
			la.box_y,
			la.box_width,
			la.box_height,
			'confirmed',
			la.created_at,
			la.updated_at
		from legacy_areas la
		join store_areas sa
			on sa.store_id = la.store_id
			and sa.area_type = la.area_type
			and sa.area_number = la.area_number
		join store_design_plans sdp
			on sdp.store_id = la.store_id
			and sdp.upload_id = 'legacy-' || la.legacy_store_id::text
		on conflict (design_plan_id, area_id) do nothing`,
	}
}
