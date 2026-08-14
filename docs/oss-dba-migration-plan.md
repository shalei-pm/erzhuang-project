# OSS 对象存储迁移 DBA 方案

最后更新：2026-07-02

本文只覆盖数据库、历史引用清单、校验 SQL、断点续跑和样本 dry-run 验收口径。主会话负责 OSS provider 代码、运行环境变量和发布。本方案不包含 OSS AK/SK，不要求真实密钥，不执行正式数据写入。

## 1. 结论

现有 `tb_asset_objects` / `tb_asset_access_logs` 已经能支撑第一版“按 logical key 查映射、后端代理读取、访问留痕”的主链路，但还不足以支撑可审计的历史对象迁移作业。

当前足够的字段：

- `tb_asset_objects.logical_key` / `logical_key_hash`：保留旧路径归一化后的兼容 key。
- `storage_provider`、`bucket`、`file_id`、`proxy_path`：可记录目标 provider、桶、文件标识和前端稳定代理路径。
- `content_type`、`size_bytes`、`checksum_sha256`：可做迁移后内容校验。
- `sensitivity`、`owner_entity_type`、`owner_entity_id`：可表达资产敏感级别和业务归属。
- `migration_status`、`migrated_at`：可表达迁移终态。
- `tb_asset_access_logs` 的 `asset_id`、`logical_key`、`user_id`、`store_id`、`external_org_id`、`channel_id`、`request_id` 足够支撑第一版访问审计。

建议补齐的字段：

- `storage_key`：OSS 目标对象 key。不要把它和旧 `logical_key` 混在一起，因为历史兼容 key 可能是 `uploads/{upload_id}/preview.png`，目标 OSS key 可能是 `design-plans/{store_id}/{upload_id}/preview.png`。
- `storage_key_hash`：避免对 `varchar(1024)` 建长唯一索引。
- `source_provider`、`source_bucket`、`source_key`：记录来源，便于回放和核对。
- `migration_batch_id`、`migration_attempts`、`last_attempt_at`、`last_error_code`、`last_error_message`：支撑断点续跑、失败重试和迁移报告。
- 将 `storage_provider` 默认值从 `supabase` 改成 `oss`。历史来源用 `source_provider` 表达。

## 2. DDL Patch 建议

以下 SQL 是评审草案，不代表允许直接在正式库执行。执行前必须确认目标库、备份、阶段状态、MySQL 版本和现有字段是否已存在。若目标库不支持 `add column if not exists`，需要先用 `information_schema.columns` 生成版本化 migration。

```sql
set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

alter table tb_asset_objects
  modify column storage_provider varchar(32) not null default 'oss',
  add column source_provider varchar(32) not null default 'supabase' after logical_key_hash,
  add column source_bucket varchar(255) not null default '' after source_provider,
  add column source_key varchar(1024) not null default '' after source_bucket,
  add column storage_key varchar(1024) not null default '' after bucket,
  add column storage_key_hash char(64) not null default '' after storage_key,
  add column migration_batch_id varchar(64) not null default '' after migration_status,
  add column migration_attempts int not null default 0 after migration_batch_id,
  add column last_attempt_at datetime(3) null after migration_attempts,
  add column last_error_code varchar(64) not null default '' after last_attempt_at,
  add column last_error_message varchar(512) not null default '' after last_error_code;

create index idx_tb_asset_objects_storage_key_hash
  on tb_asset_objects (storage_provider, bucket, storage_key_hash);

create index idx_tb_asset_objects_status_attempt
  on tb_asset_objects (migration_status, migration_attempts, updated_at);

create index idx_tb_asset_objects_batch_status
  on tb_asset_objects (migration_batch_id, migration_status);
```

如果需要增强访问审计，`tb_asset_access_logs` 可以后续补 `content_type`、`size_bytes`、`duration_ms`、`http_status`。这不阻断本轮 OSS 迁移。

## 3. 路径归一化口径

历史字段允许混存 API 路径、logical key、带 query 的临时 URL。本轮迁移只迁可归一化为内部 logical key 的对象：

- 设计图：`uploads/{upload_id}/original.pdf`、`uploads/{upload_id}/preview.png`、`uploads/{upload_id}/thumbnail.png`
- 设计图 API 路径：`/api/design-plan/uploads/{upload_id}/original`、`/api/design-plan/uploads/{upload_id}/preview`、`/api/design-plan/uploads/{upload_id}/thumbnail`
- 通道截图：`channel-snapshots/{name}`
- 通道截图 API 路径：`/api/store-space/channel-snapshots/{name}`

无法归一化的 `http://` / `https://` 临时 URL 不直接迁移，应标记 `skipped`，原因建议为 `remote_signed_url`，除非主会话提供稳定来源对象 key。

## 4. 历史对象引用清单 SQL

以下查询用于 dry-run 生成迁移清单。它覆盖新门店设计图、旧设计图表和通道截图缩略图/大图。`logical_key` 是兼容查找 key；`target_oss_key` 是建议写入 OSS 的目标 key。

```sql
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
    concat('/api/design-plan/uploads/', p.upload_id, '/original') as proxy_path,
    'application/pdf' as expected_content_type,
    'internal' as sensitivity
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id
  where trim(p.original_pdf_path) <> ''

  union all
  select 'tb_store_design_plans', p.id, 'preview_image_path', 'design_preview',
         p.store_id, s.external_org_id, null, null,
         'store_design_plan', p.id, p.upload_id, p.preview_image_path,
         concat('/api/design-plan/uploads/', p.upload_id, '/preview'),
         'image/png', 'internal'
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id
  where trim(p.preview_image_path) <> ''

  union all
  select 'tb_store_design_plans', p.id, 'thumbnail_path', 'design_thumbnail',
         p.store_id, s.external_org_id, null, null,
         'store_design_plan', p.id, p.upload_id, p.thumbnail_path,
         concat('/api/design-plan/uploads/', p.upload_id, '/thumbnail'),
         'image/png', 'internal'
  from tb_store_design_plans p
  join tb_stores s on s.id = p.store_id
  where trim(p.thumbnail_path) <> ''

  union all
  select
    'tb_design_plan_stores', d.id, 'original_pdf_path', 'legacy_design_original',
    null, '', null, null,
    'legacy_design_plan_store', d.id, '', d.original_pdf_path,
    '', 'application/pdf', 'internal'
  from tb_design_plan_stores d
  where trim(d.original_pdf_path) <> ''

  union all
  select 'tb_design_plan_stores', d.id, 'preview_image_path', 'legacy_design_preview',
         null, '', null, null,
         'legacy_design_plan_store', d.id, '', d.preview_image_path,
         '', 'image/png', 'internal'
  from tb_design_plan_stores d
  where trim(d.preview_image_path) <> ''

  union all
  select 'tb_design_plan_stores', d.id, 'thumbnail_path', 'legacy_design_thumbnail',
         null, '', null, null,
         'legacy_design_plan_store', d.id, '', d.thumbnail_path,
         '', 'image/png', 'internal'
  from tb_design_plan_stores d
  where trim(d.thumbnail_path) <> ''

  union all
  select
    'tb_channel_snapshots', cs.id, 'thumbnail_path', 'snapshot_thumbnail',
    r.store_id, s.external_org_id, r.id, c.id,
    'video_channel', c.id, '', coalesce(nullif(cs.snapshot_key, ''), cs.thumbnail_path),
    cs.thumbnail_path, 'image/jpeg', 'sensitive'
  from tb_channel_snapshots cs
  join tb_video_channels c on c.id = cs.channel_id
  join tb_video_recorders r on r.id = c.recorder_id
  join tb_stores s on s.id = r.store_id
  where trim(coalesce(nullif(cs.snapshot_key, ''), cs.thumbnail_path)) <> ''

  union all
  select
    'tb_channel_snapshots', cs.id, 'full_image_path', 'snapshot_full_image',
    r.store_id, s.external_org_id, r.id, c.id,
    'video_channel', c.id, '', coalesce(nullif(cs.snapshot_key, ''), cs.full_image_path),
    cs.full_image_path, 'image/jpeg', 'sensitive'
  from tb_channel_snapshots cs
  join tb_video_channels c on c.id = cs.channel_id
  join tb_video_recorders r on r.id = c.recorder_id
  join tb_stores s on s.id = r.store_id
  where trim(coalesce(nullif(cs.snapshot_key, ''), cs.full_image_path)) <> ''
),
clean_refs as (
  select
    raw_refs.*,
    regexp_replace(trim(old_path), '[?#].*$', '') as clean_path
  from raw_refs
),
normalized_refs as (
  select
    clean_refs.*,
    case
      when clean_path regexp '^https?://' then ''
      when clean_path like '/api/design-plan/uploads/%/original%' then concat('uploads/', substring_index(substring_index(clean_path, '/api/design-plan/uploads/', -1), '/original', 1), '/original.pdf')
      when clean_path like '/api/design-plan/uploads/%/preview%' then concat('uploads/', substring_index(substring_index(clean_path, '/api/design-plan/uploads/', -1), '/preview', 1), '/preview.png')
      when clean_path like '/api/design-plan/uploads/%/thumbnail%' then concat('uploads/', substring_index(substring_index(clean_path, '/api/design-plan/uploads/', -1), '/thumbnail', 1), '/thumbnail.png')
      when clean_path like 'uploads/%' then clean_path
      when clean_path like '/api/store-space/channel-snapshots/%' then concat('channel-snapshots/', substring_index(clean_path, '/api/store-space/channel-snapshots/', -1))
      when clean_path like 'channel-snapshots/%' then clean_path
      else ''
    end as logical_key
  from clean_refs
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
  old_path,
  logical_key,
  case
    when logical_key = '' then ''
    when asset_role in ('design_original', 'design_preview', 'design_thumbnail')
      then concat('design-plans/', store_id, '/', substring_index(substring_index(logical_key, 'uploads/', -1), '/', 1), '/', substring_index(logical_key, '/', -1))
    when asset_role like 'legacy_design_%'
      then concat('legacy-design-plans/', source_id, '/', substring_index(logical_key, '/', -1))
    when asset_role in ('snapshot_thumbnail', 'snapshot_full_image')
      then concat('channel-snapshots/', coalesce(nullif(external_org_id, ''), concat('store-', store_id)), '/', recorder_id, '/', channel_id, '/', substring_index(logical_key, '/', -1))
    else logical_key
  end as target_oss_key,
  sha2(logical_key, 256) as logical_key_hash,
  proxy_path,
  expected_content_type,
  sensitivity,
  case
    when logical_key = '' then 'skipped'
    else 'pending'
  end as suggested_migration_status,
  case
    when clean_path regexp '^https?://' then 'remote_signed_url'
    when logical_key = '' then 'unrecognized_path'
    else ''
  end as skip_reason
from normalized_refs
order by source_table, source_id, source_column;
```

若主会话决定第一波迁移完全保留旧 key 以降低代码改动，`target_oss_key` 可临时改为 `logical_key`；但仍建议保留 `storage_key` 字段，给后续新对象 key 规范留余地。

## 5. 迁移 Upsert SQL 模板

迁移程序应使用参数化 SQL，不拼接路径字符串。以下模板表达写入语义，变量名仅用于说明。

```sql
insert into tb_asset_objects (
  logical_key, logical_key_hash,
  source_provider, source_bucket, source_key,
  storage_provider, bucket, storage_key, storage_key_hash, file_id, proxy_path,
  content_type, size_bytes, checksum_sha256, sensitivity,
  owner_entity_type, owner_entity_id,
  migration_status, migration_batch_id, migration_attempts,
  last_attempt_at, last_error_code, last_error_message, migrated_at
) values (
  ?, sha2(?, 256),
  ?, ?, ?,
  'oss', ?, ?, sha2(?, 256), '', ?,
  ?, ?, ?, ?,
  ?, ?,
  ?, ?, ?,
  current_timestamp(3), ?, ?,
  case when ? = 'migrated' then current_timestamp(3) else null end
)
on duplicate key update
  source_provider = values(source_provider),
  source_bucket = values(source_bucket),
  source_key = values(source_key),
  storage_provider = case
    when migration_status = 'migrated' then storage_provider
    else values(storage_provider)
  end,
  bucket = case
    when migration_status = 'migrated' then bucket
    else values(bucket)
  end,
  storage_key = case
    when migration_status = 'migrated' then storage_key
    else values(storage_key)
  end,
  storage_key_hash = case
    when migration_status = 'migrated' then storage_key_hash
    else values(storage_key_hash)
  end,
  proxy_path = values(proxy_path),
  content_type = case
    when values(content_type) <> '' then values(content_type)
    else content_type
  end,
  size_bytes = coalesce(values(size_bytes), size_bytes),
  checksum_sha256 = case
    when values(checksum_sha256) <> '' then values(checksum_sha256)
    else checksum_sha256
  end,
  sensitivity = values(sensitivity),
  owner_entity_type = values(owner_entity_type),
  owner_entity_id = values(owner_entity_id),
  migration_status = case
    when migration_status = 'migrated' then migration_status
    else values(migration_status)
  end,
  migration_batch_id = values(migration_batch_id),
  migration_attempts = case
    when migration_status = 'migrated' then migration_attempts
    else migration_attempts + 1
  end,
  last_attempt_at = current_timestamp(3),
  last_error_code = case
    when values(migration_status) = 'failed' then values(last_error_code)
    else ''
  end,
  last_error_message = case
    when values(migration_status) = 'failed' then left(values(last_error_message), 512)
    else ''
  end,
  migrated_at = case
    when migration_status = 'migrated' then migrated_at
    when values(migration_status) = 'migrated' then current_timestamp(3)
    else migrated_at
  end;
```

## 6. 迁移校验 SQL

### 6.1 资产引用总数

将第 4 节清单 SQL 保存为视图或临时表 `tmp_oss_asset_inventory` 后执行：

```sql
select asset_role, count(*) as ref_count
from tmp_oss_asset_inventory
group by asset_role
order by asset_role;

select count(*) as total_refs,
       count(distinct logical_key) as distinct_logical_keys,
       count(distinct target_oss_key) as distinct_target_oss_keys
from tmp_oss_asset_inventory
where logical_key <> '';
```

### 6.2 空路径与不可归一化路径

```sql
select 'tb_store_design_plans.original_pdf_path' as field_name, count(*) as empty_count
from tb_store_design_plans
where trim(original_pdf_path) = ''
union all
select 'tb_store_design_plans.preview_image_path', count(*)
from tb_store_design_plans
where trim(preview_image_path) = ''
union all
select 'tb_store_design_plans.thumbnail_path', count(*)
from tb_store_design_plans
where trim(thumbnail_path) = ''
union all
select 'tb_channel_snapshots.thumbnail_path', count(*)
from tb_channel_snapshots
where trim(thumbnail_path) = ''
union all
select 'tb_channel_snapshots.full_image_path', count(*)
from tb_channel_snapshots
where trim(full_image_path) = '';

select source_table, source_id, source_column, old_path, skip_reason
from tmp_oss_asset_inventory
where logical_key = ''
order by source_table, source_id, source_column;
```

### 6.3 重复 key

```sql
select logical_key, count(*) as ref_count,
       group_concat(concat(source_table, '#', source_id, '.', source_column) order by source_table, source_id separator ', ') as refs
from tmp_oss_asset_inventory
where logical_key <> ''
group by logical_key
having count(*) > 1
order by ref_count desc, logical_key;

select target_oss_key, count(*) as ref_count,
       group_concat(concat(source_table, '#', source_id, '.', source_column) order by source_table, source_id separator ', ') as refs
from tmp_oss_asset_inventory
where target_oss_key <> ''
group by target_oss_key
having count(*) > 1
order by ref_count desc, target_oss_key;
```

重复 `logical_key` 不一定是错误，例如同一截图的缩略图和大图当前可能相同；重复 `target_oss_key` 必须确认是同一物理对象，不能把不同内容覆盖到同一个 OSS key。

### 6.4 已迁移、失败、跳过

```sql
select migration_status, storage_provider, bucket, count(*) as cnt
from tb_asset_objects
group by migration_status, storage_provider, bucket
order by migration_status, storage_provider, bucket;

select migration_batch_id, migration_status, count(*) as cnt,
       min(updated_at) as first_updated_at,
       max(updated_at) as last_updated_at
from tb_asset_objects
where migration_batch_id <> ''
group by migration_batch_id, migration_status
order by migration_batch_id, migration_status;

select id, logical_key, storage_key, migration_attempts, last_error_code, last_error_message, updated_at
from tb_asset_objects
where migration_status = 'failed'
order by updated_at desc, id desc
limit 100;

select id, logical_key, last_error_code, last_error_message, updated_at
from tb_asset_objects
where migration_status = 'skipped'
order by updated_at desc, id desc
limit 100;
```

### 6.5 孤儿引用

```sql
select count(*) as orphan_store_design_plans
from tb_store_design_plans p
left join tb_stores s on s.id = p.store_id
where s.id is null;

select count(*) as orphan_snapshots
from tb_channel_snapshots cs
left join tb_video_channels c on c.id = cs.channel_id
where c.id is null;

select count(*) as orphan_snapshot_recorders
from tb_channel_snapshots cs
join tb_video_channels c on c.id = cs.channel_id
left join tb_video_recorders r on r.id = c.recorder_id
where r.id is null;

select count(*) as orphan_snapshot_stores
from tb_channel_snapshots cs
join tb_video_channels c on c.id = cs.channel_id
join tb_video_recorders r on r.id = c.recorder_id
left join tb_stores s on s.id = r.store_id
where s.id is null;
```

### 6.6 业务引用与资产映射一致性

```sql
select inv.source_table, inv.source_id, inv.source_column, inv.logical_key, inv.target_oss_key
from tmp_oss_asset_inventory inv
left join tb_asset_objects ao on ao.logical_key_hash = sha2(inv.logical_key, 256)
where inv.logical_key <> ''
  and ao.id is null
order by inv.source_table, inv.source_id, inv.source_column
limit 200;

select inv.source_table, inv.source_id, inv.source_column,
       inv.logical_key, inv.target_oss_key,
       ao.storage_provider, ao.bucket, ao.storage_key, ao.migration_status
from tmp_oss_asset_inventory inv
join tb_asset_objects ao on ao.logical_key_hash = sha2(inv.logical_key, 256)
where inv.logical_key <> ''
  and (
    ao.storage_provider <> 'oss'
    or trim(ao.bucket) = ''
    or trim(ao.storage_key) = ''
    or ao.storage_key <> inv.target_oss_key
    or ao.migration_status <> 'migrated'
  )
order by inv.source_table, inv.source_id, inv.source_column
limit 200;

select ao.id, ao.logical_key, ao.storage_key, ao.migration_status
from tb_asset_objects ao
left join tmp_oss_asset_inventory inv on inv.logical_key_hash = ao.logical_key_hash
where ao.storage_provider = 'oss'
  and inv.logical_key_hash is null
order by ao.updated_at desc, ao.id desc
limit 200;

select count(*) as missing_logical_key_hash
from tb_asset_objects
where trim(logical_key) <> ''
  and trim(logical_key_hash) = '';

select count(*) as missing_storage_key_hash
from tb_asset_objects
where trim(storage_key) <> ''
  and trim(storage_key_hash) = '';
```

## 7. 断点续跑策略

迁移程序应按 `logical_key_hash` 做幂等，不按业务行 ID 做幂等。一个对象可能被多个业务字段引用，但只能上传和登记一次。

推荐流程：

1. 先执行历史清单 SQL，生成候选对象；按 `logical_key` 去重后再迁移。
2. 对每个候选对象查询 `tb_asset_objects`。
3. 如果已存在 `migration_status='migrated'` 且 `storage_provider='oss'`、`bucket`、`storage_key` 非空，则跳过上传；可选做 OSS HEAD 校验。
4. 如果不存在，或状态是 `pending` / `failed`，才允许重试。
5. 上传前使用确定性的 `target_oss_key`。OSS Put 建议启用禁止覆盖能力，例如 Aliyun OSS `x-oss-forbid-overwrite: true`；如 provider 未实现该能力，必须先 HEAD 目标 key。
6. 如果目标 key 已存在，读取元数据或下载计算 checksum；一致则标记 `migrated`，不一致则标记 `failed`，错误码 `target_key_conflict`。
7. 上传成功后计算并写入 `size_bytes`、`checksum_sha256`、`content_type`、`migration_status='migrated'`、`migrated_at`。
8. 上传失败时写 `migration_status='failed'`、`migration_attempts+1`、`last_error_code`、`last_error_message`，不得记录密钥、签名 URL、Authorization header。
9. 对无法归一化、远程临时 URL、业务孤儿数据，写 `migration_status='skipped'`，保留 `last_error_code` 方便复盘。
10. 重复执行同一批次不会重复上传成功对象，不覆盖成功对象状态；失败对象可按 `migration_attempts < 3` 或指定 `--retry-failed` 重新进入队列。

并发迁移第一版建议不开启。如果后续需要多 worker，需要补充租约字段，例如 `lease_owner`、`lease_until`，避免两个 worker 同时上传同一个 key。

## 8. 样本门店 10030 Dry-run 验收口径

样本门店以 `tb_stores.external_org_id='10030'` 为准。dry-run 只读，不应修改 `tb_asset_objects`、业务表或 OSS。

### 8.1 样本清单

```sql
select id, city, name, short_name, external_org_id
from tb_stores
where external_org_id = '10030';

select asset_role, count(*) as ref_count,
       count(distinct logical_key) as distinct_logical_keys,
       count(distinct target_oss_key) as distinct_target_oss_keys
from tmp_oss_asset_inventory
where external_org_id = '10030'
group by asset_role
order by asset_role;

select source_table, source_id, source_column, asset_role,
       store_id, external_org_id, recorder_id, channel_id,
       old_path, logical_key, target_oss_key, proxy_path,
       suggested_migration_status, skip_reason
from tmp_oss_asset_inventory
where external_org_id = '10030'
order by source_table, source_id, source_column
limit 200;
```

### 8.2 Dry-run 通过标准

- 能找到且只找到预期的 `external_org_id='10030'` 样本门店。
- 设计图 original / preview / thumbnail 如样本门店已有设计图，均能生成非空 `logical_key` 和 `target_oss_key`。
- 通道截图 thumbnail / full image 能归一化为 `channel-snapshots/{name}`。
- `target_oss_key` 不为空，通道截图路径优先包含 `external_org_id`，缺失时用 `store-{store_id}` 兜底，并包含 `recorder_id`、`channel_id`。
- `skip_reason` 为空；如果不为空，必须能解释为远程临时 URL、空历史数据或业务孤儿，不进入 apply。
- dry-run 前后 `tb_asset_objects` 行数不变。

dry-run 前后行数检查：

```sql
select count(*) as asset_object_count_before
from tb_asset_objects;

-- 执行 dry-run 命令或只读清单 SQL。

select count(*) as asset_object_count_after
from tb_asset_objects;
```

### 8.3 样本 apply 后可选校验

如果主会话后续明确允许对测试库样本 apply，可执行：

```sql
select inv.logical_key, inv.target_oss_key,
       ao.storage_provider, ao.bucket, ao.storage_key,
       ao.content_type, ao.size_bytes, ao.checksum_sha256,
       ao.migration_status, ao.migrated_at
from tmp_oss_asset_inventory inv
join tb_asset_objects ao on ao.logical_key_hash = sha2(inv.logical_key, 256)
where inv.external_org_id = '10030'
order by inv.asset_role, inv.logical_key;
```

通过标准：

- 样本对象均为 `storage_provider='oss'`。
- `bucket` 非空、`storage_key=target_oss_key`。
- `migration_status='migrated'`，`migrated_at` 非空。
- `size_bytes > 0`，`checksum_sha256` 为 64 位十六进制字符串。
- 前端/API 仍走原代理路径，不暴露 OSS 真实地址。

## 9. 需要主会话确认的问题

- 第一波历史迁移的 `target_oss_key` 是否采用新规范，还是临时等于旧 `logical_key`。
- `tb_asset_objects.file_id` 在 OSS provider 下是否保留为空，或由后端写入 OSS ETag / version id。
- 公司 OSS 是否开启版本控制、禁止覆盖、服务端加密和内网 endpoint；这些会影响重试和校验策略，但不需要在本文记录密钥。
- 无法归一化的 `http(s)` 临时 URL 是否跳过，还是由主会话提供 legacy storage 的稳定 key 再迁。
- 通道截图缩略图和大图当前如果指向同一个对象，是否只迁一次并让两个业务字段共享同一 `logical_key`。
- 访问审计是否本轮同步接入 `tb_asset_access_logs`，还是 OSS 切换稳定后再接入。
