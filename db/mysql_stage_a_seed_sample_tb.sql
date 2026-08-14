-- Stage A sample seed proposal for erzhuang-project MySQL.
-- Review-only draft. Do not run before main thread confirms the target database is a Stage A sandbox.
-- This file contains no real password, access token, app secret, SSO token, or device verification code.
-- Assumptions:
--   1. db/mysql_schema_tb.sql has been applied.
--   2. db/mysql_business_schema_patch_tb.sql has been applied.
--   3. db/mysql_governance_schema_tb.sql has been applied.
--   4. Stage A sample ID range 900001-900199 is available or the database can be rebuilt.
-- Required DDL dependencies:
--   - Base business tables from db/mysql_schema_tb.sql:
--     tb_stores, tb_store_areas, tb_store_design_plans, tb_design_plan_annotations,
--     tb_ezviz_accounts, tb_video_recorders, tb_video_channels, tb_channel_snapshots,
--     tb_operation_logs.
--   - Business patch fields from db/mysql_business_schema_patch_tb.sql:
--     tb_store_areas.external_area_id,
--     tb_video_channels.external_area_id,
--     tb_video_channels.external_bed_id,
--     tb_channel_snapshots.snapshot_key,
--     tb_channel_snapshots.snapshot_key_hash.
--   - Governance tables from db/mysql_governance_schema_tb.sql:
--     tb_users, tb_roles, tb_user_roles, tb_user_store_scopes,
--     tb_asset_objects, tb_audit_logs, tb_asset_access_logs.
-- Missing any prerequisite DDL should block execution.

set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 1. Stores.
insert into tb_stores (
  id, city, name, short_name, normalized_name, external_org_id,
  design_plan_status, overall_status
) values
  (
    900001, '北京', '北京保利实验室门店', '保利实验室',
    'stage_a_beijing_baoli_lab', '10030',
    'completed', 'partial'
  ),
  (
    900002, '上海', '阶段A普通样本门店', '阶段A样本',
    'stage_a_shanghai_sample_store', 'stage-a-org-900002',
    'completed', 'completed'
  );

-- 2. Store areas. external_area_id is reserved for future company business-space objects.
insert into tb_store_areas (
  id, store_id, area_type, area_number, display_name, external_area_id, source, status
) values
  (900011, 900001, 'treatment', 1, '治疗室1', 'stage-a-area-900011', 'manual', 'confirmed'),
  (900012, 900002, 'treatment', 6, '治疗室6', 'stage-a-area-900012', 'design_plan', 'confirmed'),
  (900013, 900002, 'beauty', 2, '美容室2', 'stage-a-area-900013', 'design_plan', 'confirmed'),
  (900014, 900002, 'consultation', 1, '咨询室1', 'stage-a-area-900014', 'design_plan', 'confirmed');

-- 3. Design plans. Asset paths are logical keys or existing backend proxy-style paths, not signed URLs.
insert into tb_store_design_plans (
  id, store_id, upload_id, pdf_file_name,
  original_pdf_path, preview_image_path, thumbnail_path,
  page_count, recognition_status, recognition_result
) values
  (
    900021, 900002, 'stage-a-upload-900021', 'stage-a-sample.pdf',
    'uploads/stage-a-upload-900021/original.pdf',
    'uploads/stage-a-upload-900021/preview.png',
    'uploads/stage-a-upload-900021/thumbnail.png',
    1, 'completed',
    json_object(
      'provider', 'stage_a_mock',
      'areas', json_array(
        json_object('type', 'treatment', 'number', 6, 'label', '治疗室6'),
        json_object('type', 'beauty', 'number', 2, 'label', '美容室2')
      )
    )
  );

-- 4. Design annotations.
insert into tb_design_plan_annotations (
  id, design_plan_id, area_id, box_x, box_y, box_width, box_height, status
) values
  (900031, 900021, 900012, 0.1200000000, 0.1800000000, 0.2200000000, 0.1600000000, 'confirmed'),
  (900032, 900021, 900013, 0.4200000000, 0.1800000000, 0.1800000000, 0.1600000000, 'confirmed'),
  (900033, 900021, 900014, 0.6500000000, 0.1800000000, 0.1600000000, 0.1400000000, 'confirmed');

-- 5. Ezviz account placeholder. Secrets are intentionally NULL after business patch.
insert into tb_ezviz_accounts (
  id, account_name, app_key, app_secret_ciphertext, access_token_ciphertext, status
) values
  (900041, 'stage_a_ezviz_placeholder', '', null, null, 'unverified');

-- 6. Video recorders.
insert into tb_video_recorders (
  id, store_id, ezviz_account_id, device_code, status, effective_channel_count
) values
  (900051, 900001, 900041, 'GN0941203', 'offline', 1),
  (900052, 900002, 900041, 'STAGEA900002', 'offline', 3);

-- 7. Video channels.
insert into tb_video_channels (
  id, recorder_id, channel_no, channel_name, status, is_active,
  scene_type, area_type, area_number, bed_label, area_note, area_id,
  external_area_id, external_bed_id,
  recognition_attempts, recognition_result, confirmed_at
) values
  (
    900061, 900051, 1, '保利实验室通道1', 'confirmed_business', 1,
    'treatment', 'treatment', 1, '', 'stage_a_canary_10030', 900011,
    'stage-a-area-900011', '',
    1, json_object('provider', 'stage_a_mock', 'result', 'treatment_1'), current_timestamp(3)
  ),
  (
    900062, 900052, 1, '样本治疗室6-1', 'confirmed_business', 1,
    'treatment', 'treatment', 6, '1', 'stage_a_business_bed', 900012,
    'stage-a-area-900012', 'stage-a-bed-900062',
    1, json_object('provider', 'stage_a_mock', 'result', 'treatment_6_bed_1'), current_timestamp(3)
  ),
  (
    900063, 900052, 2, '样本美容室2-2', 'confirmed_business', 1,
    'beauty', 'beauty', 2, '2', 'stage_a_beauty_bed', 900013,
    'stage-a-area-900013', 'stage-a-bed-900063',
    1, json_object('provider', 'stage_a_mock', 'result', 'beauty_2_bed_2'), current_timestamp(3)
  ),
  (
    900064, 900052, 3, '样本前台', 'confirmed_non_business', 1,
    'front_desk', null, null, '', 'stage_a_non_business', null,
    '', '',
    1, json_object('provider', 'stage_a_mock', 'result', 'front_desk'), current_timestamp(3)
  );

-- 8. Channel snapshots. Binary files are not inserted; paths are for proxy/diagnostics validation.
insert into tb_channel_snapshots (
  id, channel_id, snapshot_key, snapshot_key_hash,
  thumbnail_path, full_image_path, full_image_expires_at
) values
  (
    900071, 900061,
    'channel-snapshots/stage-a-10030-channel-1.jpg',
    sha2('channel-snapshots/stage-a-10030-channel-1.jpg', 256),
    '/api/store-space/channel-snapshots/stage-a-10030-channel-1.jpg',
    '/api/store-space/channel-snapshots/stage-a-10030-channel-1.jpg',
    null
  ),
  (
    900072, 900062,
    'channel-snapshots/stage-a-sample-treatment-6-1.jpg',
    sha2('channel-snapshots/stage-a-sample-treatment-6-1.jpg', 256),
    '/api/store-space/channel-snapshots/stage-a-sample-treatment-6-1.jpg',
    '/api/store-space/channel-snapshots/stage-a-sample-treatment-6-1.jpg',
    null
  );

-- 9. Asset object mapping sample. Stage A still reads through the existing backend proxy path.
insert into tb_asset_objects (
  id, logical_key, logical_key_hash, storage_provider, bucket, file_id, proxy_path,
  content_type, size_bytes, checksum_sha256, sensitivity,
  owner_entity_type, owner_entity_id, migration_status
) values
  (
    900081,
    'channel-snapshots/stage-a-10030-channel-1.jpg',
    sha2('channel-snapshots/stage-a-10030-channel-1.jpg', 256),
    'supabase', 'design-plan-assets', '', '/api/store-space/channel-snapshots/stage-a-10030-channel-1.jpg',
    'image/jpeg', null, '', 'sensitive',
    'video_channel', 900061, 'pending'
  ),
  (
    900082,
    'uploads/stage-a-upload-900021/preview.png',
    sha2('uploads/stage-a-upload-900021/preview.png', 256),
    'supabase', 'design-plan-assets', '', '/api/design-plan/uploads/stage-a-upload-900021/preview.png',
    'image/png', null, '', 'internal',
    'store_design_plan', 900021, 'pending'
  );

-- 10. Permission users. Emails are non-real stage A samples.
insert into tb_users (
  id, email, username, display_name, feishu_user_id, mobile, department, sso_subject, enabled
) values
  (900091, 'stage-a-admin@example.com', 'stage-a-admin', '阶段A管理员', '', '', 'stage_a', 'stage-a-admin-sub', 1),
  (900092, 'stage-a-viewer-single@example.com', 'stage-a-viewer-single', '阶段A单机构查看', '', '', 'stage_a', 'stage-a-viewer-single-sub', 1),
  (900093, 'stage-a-viewer-multi@example.com', 'stage-a-viewer-multi', '阶段A多机构查看', '', '', 'stage_a', 'stage-a-viewer-multi-sub', 1),
  (900094, 'stage-a-operator-store@example.com', 'stage-a-operator-store', '阶段A单机构运营', '', '', 'stage_a', 'stage-a-operator-store-sub', 1),
  (900095, 'stage-a-disabled@example.com', 'stage-a-disabled', '阶段A禁用用户', '', '', 'stage_a', 'stage-a-disabled-sub', 0);

insert into tb_user_roles (user_id, role_id, created_by)
select 900091, id, 900091 from tb_roles where code = 'admin'
union all
select 900092, id, 900091 from tb_roles where code = 'viewer'
union all
select 900093, id, 900091 from tb_roles where code = 'viewer'
union all
select 900094, id, 900091 from tb_roles where code = 'operator'
union all
select 900095, id, 900091 from tb_roles where code = 'viewer';

-- Scope strategy for Stage A:
-- - Admin uses all.
-- - Viewer/operator use external_org for H5 Monitor compatibility.
-- - store_id is also filled when the target store is known, but scope_key remains the unique business key.
insert into tb_user_store_scopes (
  id, user_id, scope_type, scope_key, store_id, external_org_id, city, region, created_by
) values
  (900101, 900091, 'all', 'all', null, '', '', '', 900091),
  (900102, 900092, 'external_org', '10030', 900001, '10030', '北京', '', 900091),
  (900103, 900093, 'external_org', '10030', 900001, '10030', '北京', '', 900091),
  (900104, 900093, 'external_org', 'stage-a-org-900002', 900002, 'stage-a-org-900002', '上海', '', 900091),
  (900105, 900094, 'external_org', '10030', 900001, '10030', '北京', '', 900091);

-- 11. Operation and audit samples.
insert into tb_operation_logs (
  id, action, entity_type, entity_id, store_id, actor, summary
) values
  (900111, 'stage_a_seed', 'store', 900001, 900001, 'stage-a-admin@example.com', '阶段A金丝雀门店样本'),
  (900112, 'stage_a_seed', 'video_channel', 900062, 900002, 'stage-a-admin@example.com', '阶段A床位拆分样本');

insert into tb_audit_logs (
  id, user_id, user_email, action, entity_type, entity_id,
  store_id, external_org_id, channel_id, asset_logical_key,
  result, detail_json
) values
  (
    900121, 900091, 'stage-a-admin@example.com', 'stage_a_seed', 'video_channel', 900061,
    900001, '10030', 900061, 'channel-snapshots/stage-a-10030-channel-1.jpg',
    'success', json_object('source', 'stage_a_seed')
  );

insert into tb_asset_access_logs (
  id, asset_id, logical_key, user_id, user_email, action, result,
  store_id, external_org_id, channel_id
) values
  (
    900131, 900081, 'channel-snapshots/stage-a-10030-channel-1.jpg',
    900092, 'stage-a-viewer-single@example.com', 'view_snapshot', 'success',
    900001, '10030', 900061
  );

-- 12. Read-only verification snippets to run after seed, if main thread approves execution.
-- select id, city, name, short_name, external_org_id from tb_stores where id in (900001, 900002);
-- select id, recorder_id, channel_no, status, scene_type, area_type, area_number, bed_label from tb_video_channels where id between 900061 and 900064;
-- select u.email, r.code, s.scope_type, s.scope_key, s.external_org_id from tb_users u join tb_user_roles ur on ur.user_id = u.id join tb_roles r on r.id = ur.role_id left join tb_user_store_scopes s on s.user_id = u.id where u.id between 900091 and 900095 order by u.id, s.id;
