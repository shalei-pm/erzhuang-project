-- OSS historical asset inventory SQL for erzhuang-project.
-- Last updated: 2026-07-02
--
-- Scope:
--   Read-only inventory for design-plan files and channel snapshots.
--   Phase 1 target_oss_key intentionally equals logical_key to reduce migration risk.
--   http(s) URLs are marked skipped and must not be force-migrated without a stable
--   source object key.
--
-- Default sample:
--   external_org_id = '10030'
--
-- Usage:
--   1. Run the "sample dry-run inventory" query first.
--   2. Remove the final where external_org_id = '10030' filter only after sample review.
--   3. This file performs no writes and contains no OSS credentials.

-- Sample dry-run inventory for store external_org_id = '10030'.
with raw_refs as (
  select
    'tb_store_design_plans' as source_table,
    p.id as source_id,
    'original_pdf_path' as source_column,
    'design_original' as asset_role,
    p.store_id,
    s.external_org_id,
    cast(null as signed) as recorder_id,
    cast(null as signed) as channel_id,
    'store_design_plan' as owner_entity_type,
    p.id as owner_entity_id,
    p.upload_id,
    p.original_pdf_path as old_path,
    p.original_pdf_path as source_key,
    concat('/api/design-plan/uploads/', p.upload_id, '/original') as proxy_path,
    'application/pdf' as expected_content_type,
    'internal' as sensitivity
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id

  union all
  select
    'tb_store_design_plans',
    p.id,
    'preview_image_path',
    'design_preview',
    p.store_id,
    s.external_org_id,
    null,
    null,
    'store_design_plan',
    p.id,
    p.upload_id,
    p.preview_image_path,
    p.preview_image_path,
    concat('/api/design-plan/uploads/', p.upload_id, '/preview'),
    'image/png',
    'internal'
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id

  union all
  select
    'tb_store_design_plans',
    p.id,
    'thumbnail_path',
    'design_thumbnail',
    p.store_id,
    s.external_org_id,
    null,
    null,
    'store_design_plan',
    p.id,
    p.upload_id,
    p.thumbnail_path,
    p.thumbnail_path,
    concat('/api/design-plan/uploads/', p.upload_id, '/thumbnail'),
    'image/png',
    'internal'
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id

  union all
  select
    'tb_channel_snapshots',
    cs.id,
    'thumbnail_path',
    'snapshot_thumbnail',
    r.store_id,
    s.external_org_id,
    r.id,
    c.id,
    'video_channel',
    c.id,
    '',
    cs.thumbnail_path,
    coalesce(nullif(cs.snapshot_key, ''), cs.thumbnail_path),
    cs.thumbnail_path,
    'image/jpeg',
    'sensitive'
  from tb_channel_snapshots cs
  join tb_video_channels c on c.id = cs.channel_id
  join tb_video_recorders r on r.id = c.recorder_id
  join tb_stores s on s.id = r.store_id

  union all
  select
    'tb_channel_snapshots',
    cs.id,
    'full_image_path',
    'snapshot_full_image',
    r.store_id,
    s.external_org_id,
    r.id,
    c.id,
    'video_channel',
    c.id,
    '',
    cs.full_image_path,
    coalesce(nullif(cs.snapshot_key, ''), cs.full_image_path),
    cs.full_image_path,
    'image/jpeg',
    'sensitive'
  from tb_channel_snapshots cs
  join tb_video_channels c on c.id = cs.channel_id
  join tb_video_recorders r on r.id = c.recorder_id
  join tb_stores s on s.id = r.store_id
),
clean_refs as (
  select
    raw_refs.*,
    trim(coalesce(old_path, '')) as old_path_trimmed,
    trim(coalesce(source_key, '')) as source_key_trimmed
  from raw_refs
),
normalized_refs as (
  select
    clean_refs.*,
    regexp_replace(old_path_trimmed, '[?#].*$', '') as clean_old_path,
    regexp_replace(source_key_trimmed, '[?#].*$', '') as clean_source_key
  from clean_refs
),
logical_refs as (
  select
    normalized_refs.*,
    case
      when old_path_trimmed = '' then ''
      when clean_source_key regexp '^https?://' then ''
      when clean_old_path regexp '^https?://' then ''
      when clean_source_key like '/api/design-plan/uploads/%/original%' then concat('uploads/', substring_index(substring_index(clean_source_key, '/api/design-plan/uploads/', -1), '/original', 1), '/original.pdf')
      when clean_source_key like '/api/design-plan/uploads/%/preview%' then concat('uploads/', substring_index(substring_index(clean_source_key, '/api/design-plan/uploads/', -1), '/preview', 1), '/preview.png')
      when clean_source_key like '/api/design-plan/uploads/%/thumbnail%' then concat('uploads/', substring_index(substring_index(clean_source_key, '/api/design-plan/uploads/', -1), '/thumbnail', 1), '/thumbnail.png')
      when clean_source_key like 'uploads/%' then clean_source_key
      when clean_source_key like '/api/store-space/channel-snapshots/%' then concat('channel-snapshots/', substring_index(clean_source_key, '/api/store-space/channel-snapshots/', -1))
      when clean_source_key like 'channel-snapshots/%' then clean_source_key
      when clean_old_path like '/api/design-plan/uploads/%/original%' then concat('uploads/', substring_index(substring_index(clean_old_path, '/api/design-plan/uploads/', -1), '/original', 1), '/original.pdf')
      when clean_old_path like '/api/design-plan/uploads/%/preview%' then concat('uploads/', substring_index(substring_index(clean_old_path, '/api/design-plan/uploads/', -1), '/preview', 1), '/preview.png')
      when clean_old_path like '/api/design-plan/uploads/%/thumbnail%' then concat('uploads/', substring_index(substring_index(clean_old_path, '/api/design-plan/uploads/', -1), '/thumbnail', 1), '/thumbnail.png')
      when clean_old_path like 'uploads/%' then clean_old_path
      when clean_old_path like '/api/store-space/channel-snapshots/%' then concat('channel-snapshots/', substring_index(clean_old_path, '/api/store-space/channel-snapshots/', -1))
      when clean_old_path like 'channel-snapshots/%' then clean_old_path
      else ''
    end as logical_key
  from normalized_refs
),
inventory as (
  select
    source_table,
    source_id,
    source_column,
    asset_role,
    store_id,
    external_org_id,
    recorder_id,
    channel_id,
    owner_entity_type,
    owner_entity_id,
    upload_id,
    old_path_trimmed as old_path,
    source_key_trimmed as source_key,
    logical_key,
    logical_key as target_oss_key,
    sha2(logical_key, 256) as logical_key_hash,
    sha2(logical_key, 256) as target_oss_key_hash,
    proxy_path,
    expected_content_type,
    sensitivity,
    case
      when old_path_trimmed = '' then 'skipped'
      when clean_source_key regexp '^https?://' or clean_old_path regexp '^https?://' then 'skipped'
      when logical_key = '' then 'skipped'
      else 'pending'
    end as suggested_migration_status,
    case
      when old_path_trimmed = '' then 'empty_path'
      when clean_source_key regexp '^https?://' or clean_old_path regexp '^https?://' then 'remote_http_url'
      when logical_key = '' then 'unrecognized_path'
      else ''
    end as skip_reason
  from logical_refs
),
deduped_inventory as (
  select
    inventory.*,
    row_number() over (
      partition by logical_key_hash
      order by
        case when suggested_migration_status = 'pending' then 0 else 1 end,
        source_table,
        source_id,
        source_column
    ) as logical_key_rank,
    count(*) over (partition by logical_key_hash) as logical_key_ref_count
  from inventory
  where old_path <> ''
)
select
  source_table,
  source_id,
  source_column,
  asset_role,
  store_id,
  external_org_id,
  recorder_id,
  channel_id,
  owner_entity_type,
  owner_entity_id,
  upload_id,
  old_path,
  source_key,
  logical_key,
  target_oss_key,
  logical_key_hash,
  target_oss_key_hash,
  proxy_path,
  expected_content_type,
  sensitivity,
  suggested_migration_status,
  skip_reason,
  logical_key_rank,
  logical_key_ref_count
from deduped_inventory
where external_org_id = '10030'
order by suggested_migration_status, source_table, source_id, source_column;

-- Full inventory:
--   To review all stores, copy the query above and remove:
--     where external_org_id = '10030'
--
-- Migration program rule:
--   Only rows where suggested_migration_status = 'pending' and logical_key_rank = 1
--   should produce one OSS copy operation. Rows sharing the same logical_key_hash are
--   duplicate references to the same historical object, such as thumbnail/full image
--   pointing to one channel snapshot.
--
-- Legacy optional inventory:
--   Some older local schemas include tb_design_plan_stores, but the current company
--   schema may not. Keep this as a manually reviewed optional query so the main
--   inventory does not fail when the table is absent.
--
-- select
--   'tb_design_plan_stores' as source_table,
--   d.id as source_id,
--   'original_pdf_path' as source_column,
--   'legacy_design_original' as asset_role,
--   cast(null as signed) as store_id,
--   '' as external_org_id,
--   cast(null as signed) as recorder_id,
--   cast(null as signed) as channel_id,
--   'legacy_design_plan_store' as owner_entity_type,
--   d.id as owner_entity_id,
--   '' as upload_id,
--   d.original_pdf_path as old_path,
--   d.original_pdf_path as source_key,
--   case
--     when regexp_replace(trim(d.original_pdf_path), '[?#].*$', '') regexp '^https?://' then ''
--     when regexp_replace(trim(d.original_pdf_path), '[?#].*$', '') like 'uploads/%'
--       then regexp_replace(trim(d.original_pdf_path), '[?#].*$', '')
--     else ''
--   end as logical_key,
--   case
--     when regexp_replace(trim(d.original_pdf_path), '[?#].*$', '') like 'uploads/%'
--       then regexp_replace(trim(d.original_pdf_path), '[?#].*$', '')
--     else ''
--   end as target_oss_key,
--   'application/pdf' as expected_content_type,
--   'internal' as sensitivity
-- from tb_design_plan_stores d
-- where trim(d.original_pdf_path) <> '';
