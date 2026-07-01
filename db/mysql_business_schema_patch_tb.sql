-- Business schema patch proposal for existing 14 tb_ tables.
-- This file is a reviewable ALTER/INDEX draft. Do not run blindly in production.
-- Before execution, confirm MySQL version, sql_mode, backup, and stage A/B state.
-- MySQL 8.0.13 does not support fully reliable CHECK enforcement and may not support all
-- modern idempotent DDL syntax. Convert this proposal into a versioned migration after
-- inspecting the target schema with information_schema.

-- Recommended session guard for migration/patch execution.
set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 1. Fix NOT NULL TEXT columns that block current insert paths.
-- Current app syncs/creates ezviz accounts with account_name/status only.
alter table tb_ezviz_accounts
  modify column app_secret_ciphertext text null,
  modify column access_token_ciphertext text null;

-- Current channel insert/upsert paths often omit area_note. Use varchar to allow default.
alter table tb_video_channels
  modify column area_note varchar(1024) not null default '';

-- 2. Reserve external business area/bed identifiers.
alter table tb_store_areas
  add column external_area_id varchar(255) not null default '' after display_name;

alter table tb_video_channels
  add column external_area_id varchar(255) not null default '' after area_id,
  add column external_bed_id varchar(255) not null default '' after external_area_id;

-- 3. Soft-delete columns are a product/implementation decision.
-- Keep these commented until main thread confirms delete semantics and code support.
-- alter table tb_stores
--   add column deleted_at datetime(3) null after updated_at,
--   add column deleted_by bigint null after deleted_at,
--   add key idx_tb_stores_deleted_at (deleted_at);
--
-- alter table tb_video_recorders
--   add column deleted_at datetime(3) null after updated_at,
--   add column deleted_by bigint null after deleted_at,
--   add key idx_tb_video_recorders_deleted_at (deleted_at);
--
-- alter table tb_video_channels
--   add column deleted_at datetime(3) null after updated_at,
--   add column deleted_by bigint null after deleted_at,
--   add key idx_tb_video_channels_deleted_at (deleted_at);

-- 4. Snapshot logical key support. Existing thumbnail_path/full_image_path
-- should be normalized by migration scripts; snapshot_key is a stable canonical key.
alter table tb_channel_snapshots
  add column snapshot_key varchar(1024) not null default '' after channel_id,
  add column snapshot_key_hash char(64) not null default '' after snapshot_key;

-- Backfill strategy draft, execute only after confirming exact path formats.
-- update tb_channel_snapshots
-- set snapshot_key = case
--   when thumbnail_path like '/api/store-space/channel-snapshots/%'
--     then concat('channel-snapshots/', substring_index(thumbnail_path, '/', -1))
--   when thumbnail_path like 'channel-snapshots/%'
--     then thumbnail_path
--   when full_image_path like '/api/store-space/channel-snapshots/%'
--     then concat('channel-snapshots/', substring_index(full_image_path, '/', -1))
--   when full_image_path like 'channel-snapshots/%'
--     then full_image_path
--   else ''
-- end
-- where snapshot_key = '';

-- snapshot_key_hash should be SHA2(snapshot_key, 256) after snapshot_key backfill.
-- update tb_channel_snapshots
-- set snapshot_key_hash = sha2(snapshot_key, 256)
-- where snapshot_key <> '' and snapshot_key_hash = '';

-- 5. Store listing, search/filter, and H5 Monitor indexes.
create index idx_tb_stores_city_updated_at
  on tb_stores (city, updated_at, id);

create index idx_tb_stores_external_org_id
  on tb_stores (external_org_id);

create index idx_tb_stores_city_name_updated_at
  on tb_stores (city, normalized_name, updated_at, id);

create index idx_tb_store_areas_store_type
  on tb_store_areas (store_id, area_type, status);

create index idx_tb_store_areas_external_area
  on tb_store_areas (external_area_id);

create index idx_tb_store_design_plans_store_updated
  on tb_store_design_plans (store_id, updated_at, id);

create index idx_tb_video_recorders_store_status
  on tb_video_recorders (store_id, status, id);

create index idx_tb_video_channels_active_status
  on tb_video_channels (recorder_id, is_active, status, channel_no);

create index idx_tb_video_channels_area_status
  on tb_video_channels (area_id, status, is_active);

create index idx_tb_video_channels_scene_status
  on tb_video_channels (scene_type, status);

create index idx_tb_video_channels_bed_lookup
  on tb_video_channels (area_type, area_number, bed_label);

create index idx_tb_video_channels_external_area_bed
  on tb_video_channels (external_area_id, external_bed_id);

create index idx_tb_operation_logs_entity_time
  on tb_operation_logs (entity_type, entity_id, created_at);

-- Replace or keep existing snapshot index depending on current test DB state.
-- If idx_tb_channel_snapshots_channel_id exists, drop it before creating the latest index.
-- drop index idx_tb_channel_snapshots_channel_id on tb_channel_snapshots;
create index idx_tb_channel_snapshots_latest
  on tb_channel_snapshots (channel_id, created_at, id);

create index idx_tb_channel_snapshots_key_hash
  on tb_channel_snapshots (snapshot_key_hash);

-- 6. Optional FK for deleted_by after tb_users exists.
-- Enable only after db/mysql_governance_schema_tb.sql has been applied.
-- alter table tb_stores
--   add constraint fk_tb_stores_deleted_by foreign key (deleted_by) references tb_users(id);
-- alter table tb_video_recorders
--   add constraint fk_tb_video_recorders_deleted_by foreign key (deleted_by) references tb_users(id);
-- alter table tb_video_channels
--   add constraint fk_tb_video_channels_deleted_by foreign key (deleted_by) references tb_users(id);

-- 7. Deferred decisions that need main-thread confirmation:
-- - Whether formal deletes should be replaced by application-level soft delete in stage A.
-- - Whether snapshot_key should replace thumbnail_path/full_image_path in API responses.
-- - Whether external_area_id/external_bed_id should remain columns or move to a mapping table.
-- - Whether duplicate external_org_id is invalid in all cases.
