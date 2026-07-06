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

create table if not exists tb_auth_sessions (
  id bigint not null auto_increment,
  session_token_hash char(64) not null,
  user_id bigint not null,
  sso_subject varchar(255) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  expires_at datetime(3) not null,
  revoked_at datetime(3) null,
  revoked_reason varchar(255) not null default '',
  primary key (id),
  unique key uq_tb_auth_sessions_token_hash (session_token_hash),
  key idx_tb_auth_sessions_user (user_id, created_at),
  key idx_tb_auth_sessions_expires_at (expires_at),
  constraint fk_tb_auth_sessions_user
    foreign key (user_id) references tb_users(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists tb_audit_logs (
  id bigint not null auto_increment,
  user_id bigint null,
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
