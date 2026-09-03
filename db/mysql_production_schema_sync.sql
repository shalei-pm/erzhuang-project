-- Erzhuang production MySQL schema sync entrypoint.
--
-- Scope:
--   - db_pm_erzhuang only.
--   - Creates missing Erzhuang-owned tb_ tables and applies additive patches.
--   - Does not drop tables, columns, indexes, or data.
--   - Does not touch synced tb_crm_* business source tables.
--   - Does not create the retired tb_nvr_camera_snapshots table.
--
-- Run with the MySQL CLI from the repository root after connecting to the
-- intended database. Review the referenced SQL files and take the standard
-- schema backup before execution.
--
-- Example (credentials must be provided through the approved operations path):
--   mysql --default-character-set=utf8mb4 < db/mysql_production_schema_sync.sql

select database() as target_database;

-- Base store, recorder, channel, snapshot, settings, and legacy compatibility
-- tables. All table DDL uses CREATE TABLE IF NOT EXISTS.
source db/mysql_schema_tb.sql;

-- Users, roles, monitor permissions, idle-session storage, audit logs, and
-- OSS asset inventory/access-log tables. This also seeds system roles and
-- permissions with INSERT IGNORE.
source db/mysql_governance_schema_tb.sql;

-- Idempotent migration for tb_auth_sessions. It adds last_activity_at and the
-- required indexes when the table was created by an older schema revision.
source db/mysql_auth_sessions.sql;

-- Additive, information_schema-guarded extension for OSS asset metadata.
source db/oss_asset_schema_patch_tb.sql;

-- Existing older audit tables need this field for the displayed SSO nickname.
-- New tables created by mysql_governance_schema_tb.sql already include it.
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

-- Post-apply minimum verification. The result must show zero missing tables.
select required.table_name
from (
  select 'tb_tasks' as table_name union all
  select 'tb_app_settings' union all
  select 'tb_design_plan_stores' union all
  select 'tb_design_plan_store_areas' union all
  select 'tb_design_plan_operation_logs' union all
  select 'tb_stores' union all
  select 'tb_store_areas' union all
  select 'tb_store_design_plans' union all
  select 'tb_design_plan_annotations' union all
  select 'tb_ezviz_accounts' union all
  select 'tb_video_recorders' union all
  select 'tb_video_channels' union all
  select 'tb_channel_snapshots' union all
  select 'tb_operation_logs' union all
  select 'tb_users' union all
  select 'tb_roles' union all
  select 'tb_permissions' union all
  select 'tb_user_roles' union all
  select 'tb_role_permissions' union all
  select 'tb_user_store_scopes' union all
  select 'tb_user_resource_scopes' union all
  select 'tb_auth_sessions' union all
  select 'tb_audit_logs' union all
  select 'tb_asset_objects' union all
  select 'tb_asset_access_logs'
) required
left join information_schema.tables actual
  on actual.table_schema = database()
 and actual.table_name = required.table_name
where actual.table_name is null
order by required.table_name;

select column_name, column_type, is_nullable, column_default
from information_schema.columns
where table_schema = database()
  and table_name = 'tb_audit_logs'
  and column_name = 'actor_display_name';
