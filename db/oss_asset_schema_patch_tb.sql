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
