-- Stage A sample cleanup proposal for erzhuang-project MySQL.
-- Review-only draft. Do not run before main thread confirms the target database is a Stage A sandbox.
-- This cleanup only targets Stage A sample IDs plus explicit stage_a_/stage-a-/canary_ markers.
-- Do not run after Stage B historical data import. Do not use truncate. Do not clean real historical data.
-- Assumes sample ID range 900001-900199 from db/mysql_stage_a_seed_sample_tb.sql.

set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 1. Asset and audit logs first.
delete from tb_asset_access_logs
where id between 900001 and 900199
  and (
    logical_key like 'channel-snapshots/stage-a-%'
    or user_email like 'stage-a-%@example.com'
    or asset_id between 900001 and 900199
  );

delete from tb_audit_logs
where id between 900001 and 900199
  and (
    user_email like 'stage-a-%@example.com'
    or asset_logical_key like 'channel-snapshots/stage-a-%'
    or action = 'stage_a_seed'
  );

delete from tb_operation_logs
where id between 900001 and 900199
  and (
    actor like 'stage-a-%@example.com'
    or action = 'stage_a_seed'
    or summary like '阶段A%'
  );

-- 2. Permission scope and role bindings before users.
delete from tb_user_store_scopes
where id between 900001 and 900199
  and (
    user_id between 900001 and 900199
    or scope_key like 'stage-a-%'
    or external_org_id like 'stage-a-%'
    or created_by between 900001 and 900199
  );

delete from tb_user_roles
where user_id between 900001 and 900199;

delete from tb_auth_sessions
where user_id between 900001 and 900199
  and sso_subject like 'stage-a-%';

delete from tb_users
where id between 900001 and 900199
  and (
    email like 'stage-a-%@example.com'
    or username like 'stage-a-%'
    or department = 'stage_a'
    or sso_subject like 'stage-a-%'
  );

-- 3. Asset object mappings before business rows they describe.
delete from tb_asset_objects
where id between 900001 and 900199
  and (
    logical_key like 'channel-snapshots/stage-a-%'
    or logical_key like 'uploads/stage-a-%'
    or proxy_path like '/api/%stage-a-%'
    or owner_entity_id between 900001 and 900199
  );

-- 4. Snapshots and channels before recorders/stores.
delete from tb_channel_snapshots
where id between 900001 and 900199
  and (
    channel_id between 900001 and 900199
    or snapshot_key like 'channel-snapshots/stage-a-%'
    or thumbnail_path like '/api/store-space/channel-snapshots/stage-a-%'
    or full_image_path like '/api/store-space/channel-snapshots/stage-a-%'
  );

delete from tb_video_channels
where id between 900001 and 900199
  and (
    area_note like 'stage_a_%'
    or external_area_id like 'stage-a-%'
    or external_bed_id like 'stage-a-%'
    or recorder_id between 900001 and 900199
  );

delete from tb_video_recorders
where id between 900001 and 900199
  and (
    device_code in ('GN0941203', 'STAGEA900002')
    or store_id between 900001 and 900199
  );

delete from tb_ezviz_accounts
where id between 900001 and 900199
  and account_name like 'stage_a_%';

-- 5. Design annotations and design plans before areas/stores.
delete from tb_design_plan_annotations
where id between 900001 and 900199
   or design_plan_id between 900001 and 900199
   or area_id between 900001 and 900199;

delete from tb_store_design_plans
where id between 900001 and 900199
  and (
    upload_id like 'stage-a-%'
    or original_pdf_path like 'uploads/stage-a-%'
    or preview_image_path like 'uploads/stage-a-%'
    or thumbnail_path like 'uploads/stage-a-%'
  );

delete from tb_store_areas
where id between 900001 and 900199
  and (
    external_area_id like 'stage-a-%'
    or store_id between 900001 and 900199
  );

-- 6. Stores last. Require both ID range and stage marker to avoid cleaning real historical stores.
delete from tb_stores
where id between 900001 and 900199
  and (
    normalized_name like 'stage_a_%'
    or external_org_id like 'stage-a-%'
    or name like '阶段A%'
    or short_name like '阶段A%'
  );

-- 7. Optional read-only verification after cleanup.
-- select 'tb_stores' table_name, count(*) cnt from tb_stores where id between 900001 and 900199
-- union all select 'tb_store_areas', count(*) from tb_store_areas where id between 900001 and 900199
-- union all select 'tb_store_design_plans', count(*) from tb_store_design_plans where id between 900001 and 900199
-- union all select 'tb_design_plan_annotations', count(*) from tb_design_plan_annotations where id between 900001 and 900199
-- union all select 'tb_ezviz_accounts', count(*) from tb_ezviz_accounts where id between 900001 and 900199
-- union all select 'tb_video_recorders', count(*) from tb_video_recorders where id between 900001 and 900199
-- union all select 'tb_video_channels', count(*) from tb_video_channels where id between 900001 and 900199
-- union all select 'tb_channel_snapshots', count(*) from tb_channel_snapshots where id between 900001 and 900199
-- union all select 'tb_users', count(*) from tb_users where id between 900001 and 900199
-- union all select 'tb_asset_objects', count(*) from tb_asset_objects where id between 900001 and 900199
-- union all select 'tb_audit_logs', count(*) from tb_audit_logs where id between 900001 and 900199
-- union all select 'tb_asset_access_logs', count(*) from tb_asset_access_logs where id between 900001 and 900199;
