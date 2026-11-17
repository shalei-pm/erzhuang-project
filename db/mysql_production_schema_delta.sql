-- Erzhuang incremental production schema DDL.
-- Prerequisite: the original 14 base tb_ tables already exist in db_pm_erzhuang.
-- This package contains later-added tables plus later changes to the original tables.
-- Before execution, compare information_schema and apply only the ALTER/INDEX statements that are missing.
-- No tb_crm_* DDL and no tb_nvr_camera_snapshots DDL are included.


-- ============================================================================
-- Source: db/mysql_business_schema_patch_tb.sql
-- ============================================================================
-- Business schema patch proposal for existing 14 tb_ tables.
-- This file is a reviewable ALTER/INDEX draft. It is intended for a first run against
-- the current empty 14-table Stage A test schema, not as a repeatable/idempotent migration.
-- Repeated runs need a rebuilt schema or an information_schema-guarded migration.
-- Do not run blindly in production.
-- Before execution, confirm MySQL version, sql_mode, backup, and stage A/B state.
-- MySQL 8.0.13 does not support fully reliable CHECK enforcement and may not support all
-- modern idempotent DDL syntax. Convert this proposal into a versioned migration after
-- inspecting the target schema with information_schema.

-- Recommended session guard for migration/patch execution.
set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 1. Fix NOT NULL TEXT columns that block current insert paths.
-- Current app stores and displays institution short names.
alter table tb_stores
  add column short_name varchar(255) not null default '' after name;

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
  add column bed_label varchar(64) not null default '' after area_number;

alter table tb_video_channels
  add column external_area_id varchar(255) not null default '' after area_id;

alter table tb_video_channels
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
  add column snapshot_key varchar(1024) not null default '' after channel_id;

alter table tb_channel_snapshots
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

-- ============================================================================
-- Source: db/mysql_governance_schema_tb.sql
-- ============================================================================
-- MySQL governance schema for erzhuang-project.
-- This file is a DDL proposal. Do not store secrets in this schema.
-- Target baseline: MySQL 8.0.13+, InnoDB, utf8mb4.
-- Apply after business tables exist, because audit/scope/asset logs reference tb_stores and tb_video_channels.
-- MySQL 8.0.13 parses CHECK constraints but does not enforce them. Application code and migration scripts
-- must validate enum values, scope consistency, and sensitive fields explicitly.

create table if not exists tb_users (
  id bigint not null auto_increment,
  email varchar(255) not null,
  username varchar(255) not null default '',
  display_name varchar(255) not null default '',
  feishu_user_id varchar(255) not null default '',
  phone varchar(64) not null default '',
  mobile varchar(64) not null default '',
  department varchar(255) not null default '',
  sso_subject varchar(255) not null default '',
  role varchar(32) not null default 'viewer',
  enabled tinyint(1) not null default 1,
  last_login_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_users_email (email),
  key idx_tb_users_feishu_user_id (feishu_user_id),
  key idx_tb_users_phone (phone),
  key idx_tb_users_mobile (mobile),
  key idx_tb_users_sso_subject (sso_subject),
  key idx_tb_users_role_enabled (role, enabled),
  key idx_tb_users_enabled (enabled)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_roles (
  id bigint not null auto_increment,
  code varchar(64) not null,
  name varchar(128) not null,
  description varchar(512) not null default '',
  is_system tinyint(1) not null default 0,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_roles_code (code)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_permissions (
  id bigint not null auto_increment,
  code varchar(128) not null,
  name varchar(128) not null,
  category varchar(64) not null,
  description varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  unique key uq_tb_permissions_code (code),
  key idx_tb_permissions_category (category)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_user_roles (
  user_id bigint not null,
  role_id bigint not null,
  created_at datetime(3) not null default current_timestamp(3),
  created_by bigint null,
  primary key (user_id, role_id),
  key idx_tb_user_roles_role (role_id, user_id),
  key idx_tb_user_roles_created_by (created_by),
  constraint fk_tb_user_roles_user
    foreign key (user_id) references tb_users(id),
  constraint fk_tb_user_roles_role
    foreign key (role_id) references tb_roles(id),
  constraint fk_tb_user_roles_created_by
    foreign key (created_by) references tb_users(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_role_permissions (
  role_id bigint not null,
  permission_id bigint not null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (role_id, permission_id),
  key idx_tb_role_permissions_permission (permission_id, role_id),
  constraint fk_tb_role_permissions_role
    foreign key (role_id) references tb_roles(id),
  constraint fk_tb_role_permissions_permission
    foreign key (permission_id) references tb_permissions(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_user_store_scopes (
  id bigint not null auto_increment,
  user_id bigint not null,
  scope_type varchar(32) not null default 'store',
  scope_key varchar(255) not null default '',
  store_id bigint null,
  external_org_id varchar(255) not null default '',
  city varchar(128) not null default '',
  region varchar(128) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  created_by bigint null,
  primary key (id),
  unique key uq_tb_user_store_scopes_scope (user_id, scope_type, scope_key),
  key idx_tb_user_store_scopes_user_scope (user_id, scope_type, store_id),
  key idx_tb_user_store_scopes_external_org (user_id, external_org_id),
  key idx_tb_user_store_scopes_city (user_id, city),
  key idx_tb_user_store_scopes_region (user_id, region),
  key idx_tb_user_store_scopes_created_by (created_by),
  constraint fk_tb_user_store_scopes_user
    foreign key (user_id) references tb_users(id),
  constraint fk_tb_user_store_scopes_store
    foreign key (store_id) references tb_stores(id),
  constraint fk_tb_user_store_scopes_created_by
    foreign key (created_by) references tb_users(id),
  constraint chk_tb_user_store_scopes_type
    check (scope_type in ('all', 'store', 'external_org', 'city', 'region'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_user_resource_scopes (
  id bigint primary key auto_increment,
  user_id bigint not null,
  resource_type varchar(32) not null,
  resource_id bigint not null,
  external_key varchar(128) not null,
  scope varchar(64) not null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  constraint fk_user_resource_scopes_user foreign key (user_id) references tb_users(id) on delete cascade,
  unique key uk_user_resource_scope (user_id, resource_type, resource_id, scope),
  key idx_user_scope (user_id, resource_type, scope),
  key idx_resource_external_scope (resource_type, external_key, scope)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_general_ci;

create table if not exists tb_auth_sessions (
  id bigint not null auto_increment,
  session_token_hash char(64) not null,
  user_id bigint not null,
  sso_subject varchar(255) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  last_activity_at datetime(3) not null,
  expires_at datetime(3) not null,
  revoked_at datetime(3) null,
  revoked_reason varchar(255) not null default '',
  primary key (id),
  unique key uq_tb_auth_sessions_token_hash (session_token_hash),
  key idx_tb_auth_sessions_user (user_id, created_at),
  key idx_tb_auth_sessions_user_activity (user_id, last_activity_at),
  key idx_tb_auth_sessions_expires_at (expires_at),
  constraint fk_tb_auth_sessions_user
    foreign key (user_id) references tb_users(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_audit_logs (
  id bigint not null auto_increment,
  user_id bigint null,
  actor_display_name varchar(255) not null default '',
  user_email varchar(255) not null default '',
  action varchar(128) not null,
  entity_type varchar(64) not null,
  entity_id bigint null,
  store_id bigint null,
  external_org_id varchar(255) not null default '',
  channel_id bigint null,
  asset_logical_key varchar(1024) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  request_id varchar(128) not null default '',
  result varchar(32) not null default 'success',
  detail_json json null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_audit_logs_user_time (user_id, created_at),
  key idx_tb_audit_logs_email_time (user_email, created_at),
  key idx_tb_audit_logs_store_time (store_id, created_at),
  key idx_tb_audit_logs_external_org_time (external_org_id, created_at),
  key idx_tb_audit_logs_channel_time (channel_id, created_at),
  key idx_tb_audit_logs_action_time (action, created_at),
  key idx_tb_audit_logs_request_id (request_id),
  constraint fk_tb_audit_logs_user
    foreign key (user_id) references tb_users(id),
  constraint fk_tb_audit_logs_store
    foreign key (store_id) references tb_stores(id),
  constraint fk_tb_audit_logs_channel
    foreign key (channel_id) references tb_video_channels(id),
  constraint chk_tb_audit_logs_result
    check (result in ('success', 'denied', 'failed'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_asset_objects (
  id bigint not null auto_increment,
  logical_key varchar(1024) not null,
  logical_key_hash char(64) not null,
  storage_provider varchar(32) not null default 'supabase',
  bucket varchar(255) not null default '',
  file_id varchar(255) not null default '',
  proxy_path varchar(1024) not null default '',
  content_type varchar(128) not null default '',
  size_bytes bigint null,
  checksum_sha256 varchar(64) not null default '',
  sensitivity varchar(32) not null default 'internal',
  owner_entity_type varchar(64) not null default '',
  owner_entity_id bigint null,
  migration_status varchar(32) not null default 'pending',
  migrated_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_asset_objects_logical_key_hash (logical_key_hash),
  key idx_tb_asset_objects_file_id (file_id),
  key idx_tb_asset_objects_owner (owner_entity_type, owner_entity_id),
  key idx_tb_asset_objects_sensitivity (sensitivity),
  key idx_tb_asset_objects_migration_status (migration_status),
  constraint chk_tb_asset_objects_sensitivity
    check (sensitivity in ('public', 'internal', 'sensitive')),
  constraint chk_tb_asset_objects_migration_status
    check (migration_status in ('pending', 'migrated', 'failed', 'skipped'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_asset_access_logs (
  id bigint not null auto_increment,
  asset_id bigint null,
  logical_key varchar(1024) not null default '',
  user_id bigint null,
  user_email varchar(255) not null default '',
  action varchar(64) not null,
  result varchar(32) not null default 'success',
  store_id bigint null,
  external_org_id varchar(255) not null default '',
  channel_id bigint null,
  ip_address varchar(64) not null default '',
  request_id varchar(128) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_asset_access_logs_asset_time (asset_id, created_at),
  key idx_tb_asset_access_logs_user_time (user_id, created_at),
  key idx_tb_asset_access_logs_store_time (store_id, created_at),
  key idx_tb_asset_access_logs_channel_time (channel_id, created_at),
  key idx_tb_asset_access_logs_request_id (request_id),
  constraint fk_tb_asset_access_logs_asset
    foreign key (asset_id) references tb_asset_objects(id),
  constraint fk_tb_asset_access_logs_user
    foreign key (user_id) references tb_users(id),
  constraint fk_tb_asset_access_logs_store
    foreign key (store_id) references tb_stores(id),
  constraint fk_tb_asset_access_logs_channel
    foreign key (channel_id) references tb_video_channels(id),
  constraint chk_tb_asset_access_logs_result
    check (result in ('success', 'denied', 'failed', 'not_found'))
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

insert ignore into tb_roles (code, name, description, is_system) values
  ('admin', '管理员', '全量机构和系统管理权限', 1),
  ('editor', '编辑运维', '维护门店、设计图、录像机和通道', 1),
  ('operator', '运营人员', '按授权范围维护门店、设计图和通道', 1),
  ('viewer', '只读用户', '按授权范围查看门店、设计图、通道和监控', 1);

insert ignore into tb_permissions (code, name, category, description) values
  ('store_space.view', '查看门店空间', 'page', '查看门店空间后台'),
  ('store_space.design_plan.view', '查看设计图 Tab', 'tab', '查看设计图和标注'),
  ('store_space.channels.view', '查看通道映射 Tab', 'tab', '查看录像机、通道和截图'),
  ('h5_monitor.view', '查看 H5 Monitor', 'page', '查看授权机构的 H5 监控'),
  ('store.create', '新增门店', 'operation', '创建门店'),
  ('store.update', '编辑门店', 'operation', '编辑门店基础信息'),
  ('store.delete', '删除门店', 'operation', '删除或停用门店'),
  ('design_plan.upload', '上传设计图', 'operation', '上传或替换设计图'),
  ('design_plan.annotate', '保存设计图标注', 'operation', '保存设计图区域和标注'),
  ('recorder.manage', '管理录像机', 'operation', '新增、删除和扫描录像机'),
  ('channel.scan', '扫描通道', 'operation', '扫描或刷新录像机通道'),
  ('channel.recognize', '识别通道', 'operation', '调用 AI 识别通道业务区域'),
  ('channel.confirm', '确认通道', 'operation', '保存通道业务/非业务确认结果'),
  ('snapshot.refresh', '刷新截图', 'operation', '刷新通道截图'),
  ('h5_monitor.play_live', '获取实时视频', 'sensitive', '获取实时视频播放地址'),
  ('h5_monitor.playback', '获取录像回放', 'sensitive', '查询录像片段和回放地址'),
  ('asset.sensitive.view', '查看敏感资产', 'sensitive', '查看摄像头截图、设计图等敏感资产'),
  ('audit.view', '查看审计日志', 'admin', '查看审计日志'),
  ('permission.manage', '管理权限', 'admin', '管理用户、角色和机构范围');

insert ignore into tb_role_permissions (role_id, permission_id)
select r.id, p.id
from tb_roles r
cross join tb_permissions p
where r.code = 'admin';

insert ignore into tb_role_permissions (role_id, permission_id)
select r.id, p.id
from tb_roles r
join tb_permissions p on p.code in (
  'store_space.view',
  'store_space.design_plan.view',
  'store_space.channels.view',
  'h5_monitor.view',
  'store.create',
  'store.update',
  'design_plan.upload',
  'design_plan.annotate',
  'recorder.manage',
  'channel.scan',
  'channel.recognize',
  'channel.confirm',
  'snapshot.refresh',
  'h5_monitor.play_live',
  'h5_monitor.playback',
  'asset.sensitive.view'
)
where r.code in ('editor', 'operator');

insert ignore into tb_role_permissions (role_id, permission_id)
select r.id, p.id
from tb_roles r
join tb_permissions p on p.code in (
  'store_space.view',
  'store_space.design_plan.view',
  'store_space.channels.view',
  'h5_monitor.view',
  'h5_monitor.play_live',
  'h5_monitor.playback',
  'asset.sensitive.view'
)
where r.code = 'viewer';

-- ============================================================================
-- Source: db/mysql_auth_sessions.sql
-- ============================================================================
-- DBA migration for tb_auth_sessions.
-- The preflight queries are read-only. The procedure below is idempotent for
-- the table, column, and index existence cases described by the preflight.
-- This file is never executed by application startup.

-- ============================================================================
-- 1. Read-only preflight
-- ============================================================================

select table_name, engine, table_collation
from information_schema.tables
where table_schema = database()
  and table_name = 'tb_auth_sessions';

select column_name, column_type, is_nullable, column_default, ordinal_position
from information_schema.columns
where table_schema = database()
  and table_name = 'tb_auth_sessions'
order by ordinal_position;

select index_name, non_unique, seq_in_index, column_name
from information_schema.statistics
where table_schema = database()
  and table_name = 'tb_auth_sessions'
order by index_name, seq_in_index;

-- ============================================================================
-- 2. Conditional create/patch migration
-- ============================================================================
-- Run this whole file in a MySQL client that supports DELIMITER commands,
-- such as the mysql CLI. The procedure is removed after it is called.
-- All existing-table blockers are checked before ALTER/UPDATE statements;
-- MySQL DDL may implicitly commit, so this migration does not promise rollback.

drop procedure if exists erzhuang_migrate_tb_auth_sessions_tmp;

delimiter $$

create procedure erzhuang_migrate_tb_auth_sessions_tmp()
begin
  declare v_table_exists bigint default 0;
  declare v_users_exists bigint default 0;
  declare v_missing_columns bigint default 0;
  declare v_nullable_governance_columns bigint default 0;
  declare v_table_engine varchar(64) default '';
  declare v_null_created_at bigint default 0;
  declare v_nullable_token_hash bigint default 0;
  declare v_null_token_hash bigint default 0;
  declare v_null_expires_at bigint default 0;
  declare v_nullable_expires_at bigint default 0;
  declare v_column_exists bigint default 0;
  declare v_index_entries bigint default 0;
  declare v_index_matches bigint default 0;
  declare v_duplicate_hashes bigint default 0;
  declare v_primary_index_entries bigint default 0;
  declare v_primary_index_matches bigint default 0;
  declare v_user_index_entries bigint default 0;
  declare v_user_index_matches bigint default 0;
  declare v_user_index_missing bigint default 0;
  declare v_token_index_missing bigint default 0;
  declare v_user_activity_index_missing bigint default 0;
  declare v_expiry_index_missing bigint default 0;

  select count(*) into v_table_exists
  from information_schema.tables
  where table_schema = database()
    and table_name = 'tb_auth_sessions';

  if v_table_exists = 0 then
    select count(*) into v_users_exists
    from information_schema.tables
    where table_schema = database()
      and table_name = 'tb_users';

    if v_users_exists = 0 then
      signal sqlstate '45000'
        set message_text = 'tb_users is required before creating tb_auth_sessions';
    end if;

    -- Create branch: all required fields and indexes are created together.
    create table tb_auth_sessions (
      id bigint not null auto_increment,
      session_token_hash char(64) not null,
      user_id bigint not null,
      sso_subject varchar(255) not null default '',
      ip_address varchar(64) not null default '',
      user_agent varchar(512) not null default '',
      created_at datetime(3) not null default current_timestamp(3),
      last_activity_at datetime(3) not null,
      expires_at datetime(3) not null,
      revoked_at datetime(3) null,
      revoked_reason varchar(255) not null default '',
      primary key (id),
      unique key uq_tb_auth_sessions_token_hash (session_token_hash),
      key idx_tb_auth_sessions_user (user_id, created_at),
      key idx_tb_auth_sessions_user_activity (user_id, last_activity_at),
      key idx_tb_auth_sessions_expires_at (expires_at),
      constraint fk_tb_auth_sessions_user
        foreign key (user_id) references tb_users(id)
    ) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
  else
    -- Patch branch: block before any ALTER if the existing base is incomplete.
    select engine into v_table_engine
    from information_schema.tables
    where table_schema = database()
      and table_name = 'tb_auth_sessions';

    if upper(coalesce(v_table_engine, '')) <> 'INNODB' then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions must use the InnoDB engine before migration';
    end if;

    select count(*) into v_missing_columns
    from (
      select 'id' as column_name
      union all select 'session_token_hash'
      union all select 'user_id'
      union all select 'sso_subject'
      union all select 'ip_address'
      union all select 'user_agent'
      union all select 'created_at'
      union all select 'expires_at'
      union all select 'revoked_at'
      union all select 'revoked_reason'
    ) required_columns
    left join information_schema.columns c
      on c.table_schema = database()
      and c.table_name = 'tb_auth_sessions'
      and c.column_name = required_columns.column_name
    where c.column_name is null;

    if v_missing_columns > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions is missing required base columns; inspect information_schema.columns';
    end if;

    select count(*) into v_nullable_governance_columns
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name in (
        'id', 'user_id', 'created_at', 'sso_subject', 'ip_address',
        'user_agent', 'revoked_reason'
      )
      and is_nullable <> 'NO';

    if v_nullable_governance_columns > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions governance columns id, user_id, created_at, sso_subject, ip_address, user_agent, and revoked_reason must be NOT NULL before migration';
    end if;

    select count(*) into v_nullable_token_hash
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name = 'session_token_hash'
      and is_nullable <> 'NO';

    if v_nullable_token_hash > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions.session_token_hash must be NOT NULL before migration';
    end if;

    select count(*) into v_nullable_expires_at
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name = 'expires_at'
      and is_nullable <> 'NO';

    if v_nullable_expires_at > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions.expires_at must be NOT NULL before migration';
    end if;

    select count(*) into v_null_created_at
    from tb_auth_sessions
    where created_at is null;

    if v_null_created_at > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL created_at; repair data before migration';
    end if;

    select count(*) into v_null_token_hash
    from tb_auth_sessions
    where session_token_hash is null;

    if v_null_token_hash > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL session_token_hash; repair data before migration';
    end if;

    select count(*) into v_null_expires_at
    from tb_auth_sessions
    where expires_at is null;

    if v_null_expires_at > 0 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions contains rows with NULL expires_at; repair data before migration';
    end if;

    select count(*) into v_duplicate_hashes
    from (
      select session_token_hash
      from tb_auth_sessions
      group by session_token_hash
      having count(*) > 1
    ) duplicate_hashes;

    if v_duplicate_hashes > 0 then
      signal sqlstate '45000'
        set message_text = 'duplicate session_token_hash values must be repaired before migration';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 0
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'id'
        then 1 else 0 end), 0)
      into v_primary_index_entries, v_primary_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'PRIMARY';

    if v_primary_index_entries <> 1 or v_primary_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'tb_auth_sessions PRIMARY index must be a single BTREE on id without a prefix';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and ((seq_in_index = 1 and column_name = 'user_id')
          or (seq_in_index = 2 and column_name = 'created_at'))
        then 1 else 0 end), 0)
      into v_user_index_entries, v_user_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_user';

    if v_user_index_entries = 0 then
      set v_user_index_missing = 1;
    elseif v_user_index_entries <> 2 or v_user_index_matches <> 2 then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_user has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    -- Validate existing named indexes before any ALTER or UPDATE. A wrong
    -- definition is blocked rather than silently treated as missing.
    select count(*), coalesce(sum(
      case when non_unique = 0
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'session_token_hash'
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'uq_tb_auth_sessions_token_hash';

    if v_index_entries = 0 then
      set v_token_index_missing = 1;
    elseif v_index_entries <> 1 or v_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'uq_tb_auth_sessions_token_hash has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and ((seq_in_index = 1 and column_name = 'user_id')
          or (seq_in_index = 2 and column_name = 'last_activity_at'))
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_user_activity';

    if v_index_entries = 0 then
      set v_user_activity_index_missing = 1;
    elseif v_index_entries <> 2 or v_index_matches <> 2 then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_user_activity has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    select count(*), coalesce(sum(
      case when non_unique = 1
        and index_type = 'BTREE'
        and (sub_part = 0 or sub_part is null)
        and seq_in_index = 1
        and column_name = 'expires_at'
        then 1 else 0 end), 0)
      into v_index_entries, v_index_matches
    from information_schema.statistics
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and index_name = 'idx_tb_auth_sessions_expires_at';

    if v_index_entries = 0 then
      set v_expiry_index_missing = 1;
    elseif v_index_entries <> 1 or v_index_matches <> 1 then
      signal sqlstate '45000'
        set message_text = 'idx_tb_auth_sessions_expires_at has the wrong uniqueness, BTREE type, prefix, or column order';
    end if;

    select count(*) into v_column_exists
    from information_schema.columns
    where table_schema = database()
      and table_name = 'tb_auth_sessions'
      and column_name = 'last_activity_at';

    if v_column_exists = 0 then
      alter table tb_auth_sessions
        add column last_activity_at datetime(3) null after created_at;
    end if;

    update tb_auth_sessions
    set last_activity_at = created_at
    where last_activity_at is null;

    alter table tb_auth_sessions
      modify column last_activity_at datetime(3) not null;

    if v_token_index_missing = 1 then
      alter table tb_auth_sessions
        add unique key uq_tb_auth_sessions_token_hash (session_token_hash);
    end if;

    if v_user_index_missing = 1 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_user (user_id, created_at);
    end if;

    if v_user_activity_index_missing = 1 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_user_activity (user_id, last_activity_at);
    end if;

    if v_expiry_index_missing = 1 then
      alter table tb_auth_sessions
        add key idx_tb_auth_sessions_expires_at (expires_at);
    end if;
  end if;
end$$

call erzhuang_migrate_tb_auth_sessions_tmp()$$

drop procedure erzhuang_migrate_tb_auth_sessions_tmp$$

delimiter ;

-- ============================================================================
-- Source: db/oss_asset_schema_patch_tb.sql
-- ============================================================================
-- OSS asset object schema patch for erzhuang-project.
-- Last updated: 2026-07-02
--
-- Scope:
--   Patch tb_asset_objects for OSS historical asset migration tracking.
--   This file does not contain OSS AK/SK, temporary credentials, endpoints, or secrets.
--
-- Execution prerequisites:
--   1. Confirm target database and backup. Do not run against production without a
--      reviewed migration window and rollback plan.
--   2. Confirm db/mysql_governance_schema_tb.sql has already created tb_asset_objects.
--   3. Confirm MySQL version. This patch uses dynamic SQL and information_schema checks
--      so repeated execution is safer on MySQL versions where ALTER TABLE ADD COLUMN
--      IF NOT EXISTS is unavailable or inconsistent.
--   4. OSS phase 1 uses storage_key as the primary object locator. Keep file_id as
--      an optional legacy locator; do not rely on it for OSS rows.
--
-- Preflight inspection:
--
-- select column_name, column_type, is_nullable, column_default
-- from information_schema.columns
-- where table_schema = database()
--   and table_name = 'tb_asset_objects'
-- order by ordinal_position;
--
-- select index_name, group_concat(column_name order by seq_in_index) as index_columns
-- from information_schema.statistics
-- where table_schema = database()
--   and table_name = 'tb_asset_objects'
-- group by index_name
-- order by index_name;

-- ============================================================================
-- Later audit-log field addition for databases that created tb_audit_logs
-- before the actor nickname enhancement.
-- ============================================================================
set @erzhuang_schema_name := database();
set @erzhuang_audit_actor_column_sql := (
  select if(
    exists (
      select 1
      from information_schema.tables
      where table_schema = @erzhuang_schema_name
        and table_name = 'tb_audit_logs'
    ) and not exists (
      select 1
      from information_schema.columns
      where table_schema = @erzhuang_schema_name
        and table_name = 'tb_audit_logs'
        and column_name = 'actor_display_name'
    ),
    'alter table tb_audit_logs add column actor_display_name varchar(255) not null default '''' after user_id',
    'select ''skip tb_audit_logs.actor_display_name'' as info'
  )
);
prepare erzhuang_audit_actor_column_stmt from @erzhuang_audit_actor_column_sql;
execute erzhuang_audit_actor_column_stmt;
deallocate prepare erzhuang_audit_actor_column_stmt;

set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

set @schema_name := database();

-- Default newly migrated rows to oss, while historical rows keep their existing value.
set @ddl := (
  select if(
    exists (
      select 1
      from information_schema.columns
      where table_schema = @schema_name
        and table_name = 'tb_asset_objects'
        and column_name = 'storage_provider'
        and column_default <> 'oss'
    ),
    'alter table tb_asset_objects modify column storage_provider varchar(32) not null default ''oss''',
    'select ''skip storage_provider default patch'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'source_provider'
    ),
    'alter table tb_asset_objects add column source_provider varchar(32) not null default ''supabase'' after logical_key_hash',
    'select ''skip source_provider'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'source_bucket'
    ),
    'alter table tb_asset_objects add column source_bucket varchar(255) not null default '''' after source_provider',
    'select ''skip source_bucket'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'source_key'
    ),
    'alter table tb_asset_objects add column source_key varchar(1024) not null default '''' after source_bucket',
    'select ''skip source_key'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'storage_key'
    ),
    'alter table tb_asset_objects add column storage_key varchar(1024) not null default '''' after bucket',
    'select ''skip storage_key'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'storage_key_hash'
    ),
    'alter table tb_asset_objects add column storage_key_hash char(64) not null default '''' after storage_key',
    'select ''skip storage_key_hash'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'migration_batch_id'
    ),
    'alter table tb_asset_objects add column migration_batch_id varchar(64) not null default '''' after migration_status',
    'select ''skip migration_batch_id'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'migration_attempts'
    ),
    'alter table tb_asset_objects add column migration_attempts int not null default 0 after migration_batch_id',
    'select ''skip migration_attempts'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'last_attempt_at'
    ),
    'alter table tb_asset_objects add column last_attempt_at datetime(3) null after migration_attempts',
    'select ''skip last_attempt_at'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'last_error_code'
    ),
    'alter table tb_asset_objects add column last_error_code varchar(64) not null default '''' after last_attempt_at',
    'select ''skip last_error_code'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.columns
      where table_schema = @schema_name and table_name = 'tb_asset_objects' and column_name = 'last_error_message'
    ),
    'alter table tb_asset_objects add column last_error_message varchar(512) not null default '''' after last_error_code',
    'select ''skip last_error_message'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.statistics
      where table_schema = @schema_name
        and table_name = 'tb_asset_objects'
        and index_name = 'idx_tb_asset_objects_source_key_hash'
    ),
    'create index idx_tb_asset_objects_source_key_hash on tb_asset_objects (source_provider, source_bucket, logical_key_hash)',
    'select ''skip idx_tb_asset_objects_source_key_hash'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.statistics
      where table_schema = @schema_name
        and table_name = 'tb_asset_objects'
        and index_name = 'idx_tb_asset_objects_storage_key_hash'
    ),
    'create index idx_tb_asset_objects_storage_key_hash on tb_asset_objects (storage_provider, bucket, storage_key_hash)',
    'select ''skip idx_tb_asset_objects_storage_key_hash'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.statistics
      where table_schema = @schema_name
        and table_name = 'tb_asset_objects'
        and index_name = 'idx_tb_asset_objects_batch_status'
    ),
    'create index idx_tb_asset_objects_batch_status on tb_asset_objects (migration_batch_id, migration_status, updated_at)',
    'select ''skip idx_tb_asset_objects_batch_status'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

set @ddl := (
  select if(
    not exists (
      select 1 from information_schema.statistics
      where table_schema = @schema_name
        and table_name = 'tb_asset_objects'
        and index_name = 'idx_tb_asset_objects_status_attempt'
    ),
    'create index idx_tb_asset_objects_status_attempt on tb_asset_objects (migration_status, migration_attempts, updated_at)',
    'select ''skip idx_tb_asset_objects_status_attempt'' as info'
  )
);
prepare stmt from @ddl;
execute stmt;
deallocate prepare stmt;

-- Post-apply verification:
--
-- select column_name, column_type, is_nullable, column_default
-- from information_schema.columns
-- where table_schema = database()
--   and table_name = 'tb_asset_objects'
--   and column_name in (
--     'file_id', 'storage_provider',
--     'source_provider', 'source_bucket', 'source_key',
--     'storage_key', 'storage_key_hash',
--     'migration_batch_id', 'migration_attempts',
--     'last_attempt_at', 'last_error_code', 'last_error_message'
--   )
-- order by ordinal_position;
--
-- select index_name, group_concat(column_name order by seq_in_index) as index_columns
-- from information_schema.statistics
-- where table_schema = database()
--   and table_name = 'tb_asset_objects'
--   and index_name in (
--     'idx_tb_asset_objects_source_key_hash',
--     'idx_tb_asset_objects_storage_key_hash',
--     'idx_tb_asset_objects_batch_status',
--     'idx_tb_asset_objects_status_attempt'
--   )
-- group by index_name
-- order by index_name;
