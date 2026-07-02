-- OSS asset migration validation SQL for erzhuang-project.
-- Last updated: 2026-07-02
--
-- Scope:
--   Read-only checks after applying the OSS asset schema patch and after each
--   migration batch. This file contains no credentials.

set @oss_sample_external_org_id := '10030';
set @oss_bucket := 'sy-camera-erzhuang-project';
set @oss_batch_id := '';

-- 1. Overall migration status distribution.
select
  migration_status,
  count(*) as asset_count
from tb_asset_objects
group by migration_status
order by migration_status;

-- 2. Provider/bucket distribution.
select
  storage_provider,
  bucket,
  migration_status,
  count(*) as asset_count
from tb_asset_objects
group by storage_provider, bucket, migration_status
order by storage_provider, bucket, migration_status;

-- 3. Migrated OSS rows that cannot be opened by storage_key.
select
  id,
  logical_key,
  storage_provider,
  bucket,
  storage_key,
  migration_status,
  last_error_code,
  left(last_error_message, 200) as last_error_message
from tb_asset_objects
where storage_provider = 'oss'
  and migration_status = 'migrated'
  and (
    trim(coalesce(bucket, '')) = ''
    or trim(coalesce(storage_key, '')) = ''
    or trim(coalesce(storage_key_hash, '')) = ''
  )
order by updated_at desc, id
limit 100;

-- 4. Duplicate logical keys should be impossible because logical_key_hash is unique.
select
  logical_key_hash,
  count(*) as duplicate_count,
  group_concat(id order by id) as asset_ids
from tb_asset_objects
group by logical_key_hash
having count(*) > 1
order by duplicate_count desc, logical_key_hash
limit 100;

-- 5. Duplicate OSS target keys need review unless they are intentional aliases.
select
  storage_provider,
  bucket,
  storage_key_hash,
  count(*) as duplicate_count,
  group_concat(id order by id) as asset_ids
from tb_asset_objects
where storage_provider = 'oss'
  and trim(coalesce(bucket, '')) <> ''
  and trim(coalesce(storage_key_hash, '')) <> ''
group by storage_provider, bucket, storage_key_hash
having count(*) > 1
order by duplicate_count desc, storage_key_hash
limit 100;

-- 6. Failed/skipped detail for retry planning.
select
  id,
  logical_key,
  source_provider,
  source_bucket,
  source_key,
  storage_provider,
  bucket,
  storage_key,
  migration_status,
  migration_attempts,
  last_attempt_at,
  last_error_code,
  left(last_error_message, 240) as last_error_message,
  updated_at
from tb_asset_objects
where migration_status in ('failed', 'skipped')
order by updated_at desc, migration_attempts desc, id
limit 200;

-- 7. Batch-specific progress. Set @oss_batch_id before running if needed.
select
  coalesce(nullif(migration_batch_id, ''), '(empty)') as migration_batch_id,
  migration_status,
  count(*) as asset_count
from tb_asset_objects
where @oss_batch_id = '' or migration_batch_id = @oss_batch_id
group by coalesce(nullif(migration_batch_id, ''), '(empty)'), migration_status
order by migration_batch_id, migration_status;

-- 8. Sample store 10030 assets. This is the first end-to-end dry-run target.
select
  ao.id,
  ao.logical_key,
  ao.storage_provider,
  ao.bucket,
  ao.storage_key,
  ao.proxy_path,
  ao.content_type,
  ao.size_bytes,
  ao.sensitivity,
  ao.owner_entity_type,
  ao.owner_entity_id,
  ao.migration_status,
  ao.last_error_code,
  left(ao.last_error_message, 200) as last_error_message,
  ao.updated_at
from tb_asset_objects ao
left join tb_store_design_plans p
  on ao.owner_entity_type = 'store_design_plan'
 and ao.owner_entity_id = p.id
left join tb_stores ps
  on ps.id = p.store_id
left join tb_video_channels c
  on ao.owner_entity_type = 'video_channel'
 and ao.owner_entity_id = c.id
left join tb_video_recorders r
  on r.id = c.recorder_id
left join tb_stores cs
  on cs.id = r.store_id
where coalesce(ps.external_org_id, cs.external_org_id, '') = @oss_sample_external_org_id
order by ao.owner_entity_type, ao.owner_entity_id, ao.logical_key;

-- 9. Access proxy readiness: migrated assets should keep backend proxy paths.
select
  count(*) as migrated_without_proxy_path
from tb_asset_objects
where migration_status = 'migrated'
  and trim(coalesce(proxy_path, '')) = '';

-- 10. Bucket mismatch in OSS rows.
select
  bucket,
  count(*) as asset_count
from tb_asset_objects
where storage_provider = 'oss'
  and bucket <> @oss_bucket
group by bucket
order by asset_count desc;
