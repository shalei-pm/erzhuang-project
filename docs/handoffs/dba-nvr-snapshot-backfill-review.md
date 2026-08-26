# Superseded DBA Draft: NVR Snapshot One-Time Backfill

> Do not execute this document. Its initial DDL draft conflicts with the
> finalized ownership decision: `camera_id` is globally unique and the live
> implementation uses `oss_object_key`, not `object_key`. The executable
> DBA material is `db/nvr_camera_snapshots.sql`; the current review outcome is
> recorded in `docs/codex-learning-state.md`. This file remains only as an
> audit trail for the earlier review.

Date: 2026-08-26

Scope: DBA review for the one-time NVR camera thumbnail backfill. This document is a reviewable handoff only. It contains no credentials and must not be treated as permission to execute database changes.

## 1. Review Conclusion

- The proposed ownership boundary is sound: the Job may read synchronized `tb_crm_*` tables, but must write only private OSS objects and the erzhuang-owned table `tb_nvr_camera_snapshots`.
- Web service startup must not create `tb_nvr_camera_snapshots`. The table must be created by DBA/operations DDL before the Job or Web read preference is released in each environment.
- `tb_nvr_camera_snapshots` should store one current thumbnail per `(tenant_id, camera_id)`. It must not store signed OSS URLs, NVR tokens, WSS URLs, raw upstream responses, or image bytes.
- The implementation plan DDL is close, but DBA recommends tightening naming, status/error constraints, object-key validation, validation SQL, and privilege separation before test or production execution.
- `tb_crm_*` tables must remain read-only for this feature. Rollback must not alter `tb_crm_*`.

## 2. Recommended DDL

Use the same DDL in test and production after environment-specific approval. Execute manually through DBA/operations path, not from Web service startup.

### Test Environment

Execute in company test MySQL database `db_pm_erzhuang` only after main-thread approval.

```sql
-- Environment: test
-- Purpose: create erzhuang-owned NVR snapshot result table.
-- Preconditions:
--   1. Target database confirmed as test db_pm_erzhuang.
--   2. Web service will not execute CREATE TABLE for this table.
--   3. Job database account will not have write privileges on tb_crm_*.

create table if not exists tb_nvr_camera_snapshots (
  id bigint unsigned not null auto_increment,
  tenant_id bigint unsigned not null,
  camera_id bigint unsigned not null,
  status varchar(32) not null,
  object_key varchar(512) not null default '',
  object_key_hash char(64) not null default '',
  content_type varchar(64) not null default '',
  width int unsigned not null default 0,
  height int unsigned not null default 0,
  byte_size int unsigned not null default 0,
  captured_at datetime(3) null,
  attempted_at datetime(3) not null,
  error_code varchar(64) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_nvr_camera_snapshots_camera (tenant_id, camera_id),
  key idx_tb_nvr_camera_snapshots_status_attempted (status, attempted_at),
  key idx_tb_nvr_camera_snapshots_camera (camera_id),
  key idx_tb_nvr_camera_snapshots_object_hash (object_key_hash),
  key idx_tb_nvr_camera_snapshots_updated (updated_at),
  constraint chk_tb_nvr_camera_snapshots_status
    check (status in (
      'succeeded',
      'authorization_failed',
      'wss_connect_timeout',
      'media_timeout',
      'demux_failed',
      'decode_failed',
      'thumbnail_invalid',
      'oss_upload_failed',
      'database_write_failed'
    )),
  constraint chk_tb_nvr_camera_snapshots_content_type
    check (content_type in ('', 'image/jpeg')),
  constraint chk_tb_nvr_camera_snapshots_size
    check (width <= 640 and height <= 640 and byte_size <= 1048576)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

### Production Environment

Execute the same DDL in production `db_pm_erzhuang` only after:

- test single-camera spike is accepted,
- test store batch is accepted,
- production release window and rollback point are confirmed,
- production Job Secret references and database privileges are reviewed by operations/security.

Do not reuse test OSS object prefix, test database connection, or test Job Secret references in production.

## 3. DBA Notes On The DDL

- `object_key_hash` is recommended for diagnostics and future dedup/search without indexing a long object key. Job should write `sha2(object_key, 256)` equivalent.
- `status='succeeded'` is the only successful state. Failure states reuse the stable error codes so `status` alone can drive resume logic.
- `error_code` should equal `status` for terminal failure rows and be empty for `succeeded`. MySQL 8.0.13 does not reliably enforce this with `CHECK`; enforce in Job code and validation SQL.
- `object_key` must be empty on failed rows and must be `nvr-camera-snapshots/{tenant_id}/{camera_id}.jpg` on successful rows. Enforce in Job code and validation SQL.
- `content_type` must be `image/jpeg` only for success, empty for failure.
- Use `utf8mb4_unicode_ci` to match most existing project-owned MySQL DDL. If operations requires database-default `utf8mb4_general_ci`, confirm before execution and keep test/production identical.
- No foreign key to `tb_crm_iot_device` is recommended. Those are synchronized business tables; this feature should not couple owned write table DDL to sync-table lifecycle.
- MySQL 8.0.13 parses but may not enforce `CHECK`, so all enum/size/key validation must be repeated in application code and read-only validation SQL.

## 4. Structure Validation SQL

Run after DDL in the same environment where DDL was executed.

```sql
-- Environment: test or production, matching the DDL target.
-- Purpose: confirm table shape, indexes, and collation.

select
  table_name,
  engine,
  table_collation
from information_schema.tables
where table_schema = database()
  and table_name = 'tb_nvr_camera_snapshots';

select
  ordinal_position,
  column_name,
  column_type,
  is_nullable,
  column_default,
  extra
from information_schema.columns
where table_schema = database()
  and table_name = 'tb_nvr_camera_snapshots'
order by ordinal_position;

select
  index_name,
  non_unique,
  seq_in_index,
  column_name
from information_schema.statistics
where table_schema = database()
  and table_name = 'tb_nvr_camera_snapshots'
order by index_name, seq_in_index;

select
  constraint_name,
  constraint_type
from information_schema.table_constraints
where table_schema = database()
  and table_name = 'tb_nvr_camera_snapshots'
order by constraint_type, constraint_name;
```

Expected:

- table exists with `InnoDB`;
- one unique key on `(tenant_id, camera_id)`;
- indexes on `(status, attempted_at)`, `camera_id`, `object_key_hash`, `updated_at`;
- no foreign key to `tb_crm_*`.

## 5. Pre-Job Read-Only Data Validation SQL

Run before each Job stage. These queries read only `tb_crm_*` and the owned snapshot table.

### Candidate Count

```sql
-- Environment: test for tenant 10001 spike/batch; production only after separate approval.
-- Purpose: count eligible cameras without modifying source tables.

select
  tenant_id,
  count(*) as eligible_camera_count
from tb_crm_iot_device
where category = 'camera'
  and provider = 'HikVisionNvrChannel'
  and status = 1
  and deleted_at is null
  and tenant_id = 10001
group by tenant_id;
```

### Single Camera Gate

```sql
-- Environment: test first; production only after approval.
-- Purpose: verify the chosen camera belongs to the approved tenant and is eligible.

select
  id as camera_id,
  tenant_id,
  parent_id,
  name,
  hardware_id,
  category,
  provider,
  status,
  deleted_at
from tb_crm_iot_device
where tenant_id = 10001
  and id = 111
  and category = 'camera'
  and provider = 'HikVisionNvrChannel'
  and status = 1
  and deleted_at is null;
```

### Existing Snapshot Rows

```sql
-- Environment: test or production.
-- Purpose: understand resume/skip behavior before running the Job.

select
  status,
  count(*) as row_count
from tb_nvr_camera_snapshots
where tenant_id = 10001
group by status
order by status;

select
  tenant_id,
  camera_id,
  status,
  object_key,
  content_type,
  width,
  height,
  byte_size,
  captured_at,
  attempted_at,
  error_code,
  updated_at
from tb_nvr_camera_snapshots
where tenant_id = 10001
order by camera_id;
```

## 6. Post-Job Data Validation SQL

Run immediately after each Job stage in the same environment.

### Summary

```sql
-- Environment: same as Job target.
-- Purpose: compare selected cameras with durable result rows.

select
  c.tenant_id,
  count(*) as eligible_camera_count,
  sum(case when s.status = 'succeeded' then 1 else 0 end) as succeeded_count,
  sum(case when s.status is not null and s.status <> 'succeeded' then 1 else 0 end) as failed_count,
  sum(case when s.camera_id is null then 1 else 0 end) as missing_result_count
from tb_crm_iot_device c
left join tb_nvr_camera_snapshots s
  on s.tenant_id = c.tenant_id
 and s.camera_id = c.id
where c.category = 'camera'
  and c.provider = 'HikVisionNvrChannel'
  and c.status = 1
  and c.deleted_at is null
  and c.tenant_id = 10001
group by c.tenant_id;
```

### Failure Distribution

```sql
-- Environment: same as Job target.
-- Purpose: review stable failure codes only.

select
  status,
  error_code,
  count(*) as row_count
from tb_nvr_camera_snapshots
where tenant_id = 10001
group by status, error_code
order by row_count desc, status, error_code;
```

### Success Row Integrity

```sql
-- Environment: same as Job target.
-- Purpose: success rows must reference private OSS keys and bounded JPEG metadata.

select
  tenant_id,
  camera_id,
  status,
  object_key,
  content_type,
  width,
  height,
  byte_size,
  captured_at,
  attempted_at,
  error_code
from tb_nvr_camera_snapshots
where status = 'succeeded'
  and (
    object_key <> concat('nvr-camera-snapshots/', tenant_id, '/', camera_id, '.jpg')
    or object_key_hash <> sha2(object_key, 256)
    or content_type <> 'image/jpeg'
    or width = 0
    or height = 0
    or width > 640
    or height > 640
    or byte_size = 0
    or byte_size > 1048576
    or captured_at is null
    or error_code <> ''
  );
```

Expected: zero rows.

### Failure Row Integrity

```sql
-- Environment: same as Job target.
-- Purpose: failed rows must not retain object metadata.

select
  tenant_id,
  camera_id,
  status,
  object_key,
  content_type,
  width,
  height,
  byte_size,
  captured_at,
  error_code
from tb_nvr_camera_snapshots
where status <> 'succeeded'
  and (
    object_key <> ''
    or object_key_hash <> ''
    or content_type <> ''
    or width <> 0
    or height <> 0
    or byte_size <> 0
    or captured_at is not null
    or error_code <> status
  );
```

Expected: zero rows.

### Source Table Write Guard

```sql
-- Environment: same as Job target.
-- Purpose: verify the Job did not add columns or mutate schema on synchronized source tables.

select
  table_name,
  update_time
from information_schema.tables
where table_schema = database()
  and table_name in (
    'tb_crm_admin_tenant',
    'tb_crm_iot_device',
    'tb_crm_consulting_room',
    'tb_crm_iot_area_device_relation'
  )
order by table_name;
```

This is a coarse signal only. Operations should also review DB audit logs if available.

## 7. Rollback SQL

Rollback priority should be:

1. stop/delete temporary Job,
2. roll back Web read preference code if released,
3. keep `tb_nvr_camera_snapshots` and OSS objects for audit unless data owner approves deletion.

### Non-Destructive Disable

No SQL required. Stop the Job and deploy Web rollback. Existing rows are inert if Web no longer reads them.

### Review Rows Before Any Deletion

```sql
-- Environment: test or production, only after rollback owner approval.
-- Purpose: inspect owned snapshot rows before any destructive operation.

select
  tenant_id,
  status,
  count(*) as row_count,
  min(created_at) as first_created_at,
  max(updated_at) as last_updated_at
from tb_nvr_camera_snapshots
group by tenant_id, status
order by tenant_id, status;
```

### Delete A Test Tenant Result Set

```sql
-- Environment: test only unless separately approved for production.
-- Purpose: remove result rows for an explicitly scoped tenant.
-- Do not delete tb_crm_* rows. Do not delete OSS objects with this SQL.

delete from tb_nvr_camera_snapshots
where tenant_id = 10001;
```

### Drop Table

```sql
-- Environment: test or production only after DBA/operations/data-owner approval.
-- Purpose: remove the owned result table after Job and Web read path are both rolled back.
-- OSS object deletion is a separate operation and is not covered by this SQL.

drop table tb_nvr_camera_snapshots;
```

## 8. Minimal Privilege Recommendations

### Web Service Account

The normal Web service account should have:

- `select` on `tb_nvr_camera_snapshots`;
- existing read permissions needed for `tb_crm_*` monitor/resource-view queries;
- no `insert`, `update`, `delete`, `alter`, `drop`, or `create` on `tb_nvr_camera_snapshots`;
- no write privilege on `tb_crm_*`.

The Web service must not run `create table if not exists tb_nvr_camera_snapshots`.

### Backfill Job Account

The one-shot Job account should have:

- `select` on required `tb_crm_*` source tables;
- `select`, `insert`, `update` on `tb_nvr_camera_snapshots`;
- preferably no `delete` on `tb_nvr_camera_snapshots` for normal runs;
- no `insert`, `update`, `delete`, `alter`, `drop`, or `create` on `tb_crm_*`;
- no DDL privileges in production unless operations intentionally uses a DBA account for the DDL step.

### DBA/Operations Account

The DDL account is separate from Web and Job runtime accounts. It may execute create/validation/rollback SQL only through the approved test or production change path.

## 9. Secret And Job Boundary Checklist

Must be confirmed by DBA/operations/security before execution. Do not write actual values in Git, docs, terminal logs, or Job args.

- Test Job uses test MySQL DSN, test NVR authorization Secret, and test OSS Secret references only.
- Production Job uses production MySQL DSN, production NVR authorization Secret, and production OSS Secret references only.
- Test and production OSS prefixes/buckets are not accidentally shared, or sharing is explicitly approved by data/security owner.
- Job logs do not print DSN, OSS credentials, NVR token, signed WSS URL, request headers, raw upstream response, or image bytes.
- Job manifest is temporary: `restartPolicy: Never`, `backoffLimit: 0`, `parallelism: 1`, `completions: 1`, finite TTL.
- Job flags are explicit: tenant/camera scope, `missing-only`, concurrency fixed to `1`, timeout per camera fixed or reviewed.
- Web deployment receives no new NVR/OSS/MySQL variables for this feature beyond existing runtime config.
- Any production run requires a separate approval from the test run.

## 10. Confirmation Against Design Boundaries

- This plan does not write `tb_crm_*`.
- This plan does not require Web service to create `tb_nvr_camera_snapshots`.
- The table stores private OSS `object_key`, not a signed URL.
- The read path must reuse existing monitor permission checks before returning image bytes.
- Rollback does not require mutating synchronized business tables.

## 11. Risks And Blockers

- Blocker: DBA/operations has not yet approved and executed the DDL in test.
- Blocker: runtime DB privilege split between Web and Job must be confirmed. Current broad shared accounts may be convenient but are not the desired production boundary.
- Blocker: server-side WSS/RTP/H.265/ffmpeg spike may fail even if browser playback works. Do not batch until one-camera technical gate passes.
- Risk: MySQL 8.0.13 `CHECK` constraints may not enforce status, content type, or size bounds. Job code and validation SQL are mandatory.
- Risk: failed rows could leak details if code writes raw upstream errors. Persist only stable `error_code`.
- Risk: private OSS object cleanup is separate from SQL rollback and needs data/security owner confirmation.
- Risk: if Web read path exposes `object_key` or signed OSS URL, sensitive camera imagery can leak. API must stream server-side bytes only.
- Risk: if Job uses a shared Web DB account with broad write privileges, a bug could mutate `tb_crm_*`. Prefer a dedicated least-privilege Job account.

## 12. Main-Thread Next Steps

1. Review and approve the recommended DDL shape, especially `object_key_hash`, collation, and no foreign key to `tb_crm_iot_device`.
2. Ask operations to execute test DDL and structure validation SQL in test only.
3. Confirm Web code contains no `CREATE TABLE` path for `tb_nvr_camera_snapshots`.
4. Confirm Job account privileges before the single-camera test run.
5. After test DDL is verified, run only the approved single-camera technical gate for tenant `10001`, camera `111`.
