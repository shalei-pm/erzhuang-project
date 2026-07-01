# MySQL 只读校验 SQL

最后更新：2026-07-01

本文提供 MySQL 测试库和正式库初始化后的只读校验 SQL。SQL 不包含密码、连接串或真实密钥。执行前先确认目标库、执行账号和阶段 A/B 状态。

除特别标注外，本文件 SQL 应只使用 `SELECT` / `SHOW` 类只读语句。包含 `alter table` 文本的片段只用于生成 DBA 待确认草案，不代表允许直接执行。

## 1. 环境探针

```sql
select version() as mysql_version,
       @@sql_mode as sql_mode,
       @@time_zone as time_zone,
       @@system_time_zone as system_time_zone,
       @@character_set_database as charset_database,
       @@collation_database as collation_database;
```

## 2. 表清单

```sql
select table_name
from information_schema.tables
where table_schema = database()
order by table_name;
```

## 3. 行数和最大 ID

```sql
select 'tb_stores' table_name, count(*) row_count, min(id) min_id, max(id) max_id from tb_stores
union all select 'tb_store_areas', count(*), min(id), max(id) from tb_store_areas
union all select 'tb_store_design_plans', count(*), min(id), max(id) from tb_store_design_plans
union all select 'tb_design_plan_annotations', count(*), min(id), max(id) from tb_design_plan_annotations
union all select 'tb_ezviz_accounts', count(*), min(id), max(id) from tb_ezviz_accounts
union all select 'tb_video_recorders', count(*), min(id), max(id) from tb_video_recorders
union all select 'tb_video_channels', count(*), min(id), max(id) from tb_video_channels
union all select 'tb_channel_snapshots', count(*), min(id), max(id) from tb_channel_snapshots
union all select 'tb_operation_logs', count(*), min(id), max(id) from tb_operation_logs;
```

## 4. 外键孤儿

```sql
select count(*) as orphan_store_areas
from tb_store_areas a
left join tb_stores s on s.id = a.store_id
where s.id is null;

select count(*) as orphan_design_plans
from tb_store_design_plans p
left join tb_stores s on s.id = p.store_id
where s.id is null;

select count(*) as orphan_annotations
from tb_design_plan_annotations a
left join tb_store_design_plans p on p.id = a.design_plan_id
left join tb_store_areas ar on ar.id = a.area_id
where p.id is null or ar.id is null;

select count(*) as orphan_recorders
from tb_video_recorders r
left join tb_stores s on s.id = r.store_id
where s.id is null;

select count(*) as orphan_channels
from tb_video_channels c
left join tb_video_recorders r on r.id = c.recorder_id
where r.id is null;

select count(*) as orphan_channel_areas
from tb_video_channels c
left join tb_store_areas a on a.id = c.area_id
where c.area_id is not null
  and a.id is null;

select count(*) as orphan_snapshots
from tb_channel_snapshots cs
left join tb_video_channels c on c.id = cs.channel_id
where c.id is null;
```

## 5. JSON 合法性

```sql
select count(*) as invalid_design_plan_store_json
from tb_design_plan_stores
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as invalid_store_design_plan_json
from tb_store_design_plans
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as invalid_video_channel_json
from tb_video_channels
where recognition_result is not null
  and json_valid(recognition_result) = 0;
```

## 6. Auto Increment

```sql
select t.table_name,
       t.auto_increment,
       m.max_id,
       case
         when t.auto_increment is null then 'no_auto_increment'
         when t.auto_increment > m.max_id then 'ok'
         else 'needs_fix'
       end as status
from information_schema.tables t
join (
  select 'tb_stores' table_name, coalesce(max(id), 0) max_id from tb_stores
  union all select 'tb_store_areas', coalesce(max(id), 0) from tb_store_areas
  union all select 'tb_store_design_plans', coalesce(max(id), 0) from tb_store_design_plans
  union all select 'tb_design_plan_annotations', coalesce(max(id), 0) from tb_design_plan_annotations
  union all select 'tb_ezviz_accounts', coalesce(max(id), 0) from tb_ezviz_accounts
  union all select 'tb_video_recorders', coalesce(max(id), 0) from tb_video_recorders
  union all select 'tb_video_channels', coalesce(max(id), 0) from tb_video_channels
  union all select 'tb_channel_snapshots', coalesce(max(id), 0) from tb_channel_snapshots
  union all select 'tb_operation_logs', coalesce(max(id), 0) from tb_operation_logs
) m on m.table_name = t.table_name
where t.table_schema = database()
order by t.table_name;
```

生成修复 SQL 草案，仅输出文本，不执行。真正执行 `alter table ... auto_increment` 前必须由主会话确认目标库、备份、阶段状态和执行窗口：

```sql
select concat('alter table ', table_name, ' auto_increment = ', max_id + 1, ';') as ddl
from (
  select 'tb_stores' table_name, coalesce(max(id), 0) max_id from tb_stores
  union all select 'tb_store_areas', coalesce(max(id), 0) from tb_store_areas
  union all select 'tb_store_design_plans', coalesce(max(id), 0) from tb_store_design_plans
  union all select 'tb_design_plan_annotations', coalesce(max(id), 0) from tb_design_plan_annotations
  union all select 'tb_ezviz_accounts', coalesce(max(id), 0) from tb_ezviz_accounts
  union all select 'tb_video_recorders', coalesce(max(id), 0) from tb_video_recorders
  union all select 'tb_video_channels', coalesce(max(id), 0) from tb_video_channels
  union all select 'tb_channel_snapshots', coalesce(max(id), 0) from tb_channel_snapshots
  union all select 'tb_operation_logs', coalesce(max(id), 0) from tb_operation_logs
) x;
```

## 7. 重复或缺失 External Org ID

```sql
select external_org_id, count(*) as cnt
from tb_stores
where trim(external_org_id) <> ''
group by external_org_id
having count(*) > 1;

select count(*) as missing_external_org
from tb_stores
where trim(external_org_id) = '';

select id, city, name, short_name, external_org_id
from tb_stores
where external_org_id = '10030';
```

## 8. 截图路径和资产引用

```sql
select count(*) as api_path_snapshot_count
from tb_channel_snapshots
where thumbnail_path like '/api/%'
   or full_image_path like '/api/%';

select count(*) as logical_key_snapshot_count
from tb_channel_snapshots
where thumbnail_path like 'channel-snapshots/%'
   or full_image_path like 'channel-snapshots/%';

select count(*) as remote_signed_url_count
from tb_channel_snapshots
where thumbnail_path like 'http%'
   or full_image_path like 'http%';

select count(*) as missing_snapshot_path
from tb_channel_snapshots
where trim(thumbnail_path) = ''
  and trim(full_image_path) = '';
```

If `snapshot_key` has been added:

```sql
select count(*) as missing_snapshot_key
from tb_channel_snapshots
where trim(snapshot_key) = '';

select count(*) as missing_snapshot_key_hash
from tb_channel_snapshots
where trim(snapshot_key) <> ''
  and trim(snapshot_key_hash) = '';
```

If `tb_asset_objects` has been applied:

```sql
select storage_provider, sensitivity, migration_status, count(*) as cnt
from tb_asset_objects
group by storage_provider, sensitivity, migration_status
order by storage_provider, sensitivity, migration_status;

select count(*) as missing_asset_hash
from tb_asset_objects
where trim(logical_key) <> ''
  and trim(logical_key_hash) = '';
```

## 9. 权限范围

```sql
select u.email, u.enabled, r.code as role_code, s.scope_type, s.scope_key, s.store_id, s.external_org_id, s.city, s.region
from tb_users u
left join tb_user_roles ur on ur.user_id = u.id
left join tb_roles r on r.id = ur.role_id
left join tb_user_store_scopes s on s.user_id = u.id
order by u.email, r.code, s.scope_type, s.store_id, s.external_org_id;

select u.email, count(s.id) as scope_count
from tb_users u
left join tb_user_store_scopes s on s.user_id = u.id
where u.email in ('admin@example.com', 'viewer.single@example.com', 'viewer.multi@example.com', 'viewer.none@example.com', 'operator.store@example.com')
group by u.email
order by u.email;

select u.email, s.scope_type, s.scope_key, s.store_id, s.external_org_id
from tb_users u
join tb_user_store_scopes s on s.user_id = u.id
where u.email = 'viewer.single@example.com';
```

## 10. H5 Monitor Canary

```sql
select id, city, name, short_name, external_org_id
from tb_stores
where external_org_id = '10030';

select r.id as recorder_id, r.store_id, r.device_code, r.status, r.effective_channel_count
from tb_video_recorders r
join tb_stores s on s.id = r.store_id
where s.external_org_id = '10030'
  and r.device_code = 'GN0941203';

select c.id, c.recorder_id, c.channel_no, c.channel_name, c.status, c.is_active,
       c.scene_type, c.area_type, c.area_number, c.bed_label, c.area_id
from tb_video_channels c
join tb_video_recorders r on r.id = c.recorder_id
join tb_stores s on s.id = r.store_id
where s.external_org_id = '10030'
  and r.device_code = 'GN0941203'
order by c.channel_no;
```

## 11. AI 失败原因和继续识别

```sql
select status, count(*) as cnt
from tb_video_channels
group by status
order by status;

select count(*) as failed_without_reason
from tb_video_channels
where status = 'recognition_failed'
  and (
    recognition_result is null
    or json_extract(recognition_result, '$.error') is null
  );

select recorder_id, count(*) as pending_or_failed_count
from tb_video_channels
where status in ('pending_recognition', 'recognition_failed')
group by recorder_id
having count(*) > 0
order by pending_or_failed_count desc;

select id, recorder_id, channel_no, status, recognition_attempts, recognition_result
from tb_video_channels
where status in ('pending_recognition', 'recognition_failed')
order by recorder_id, channel_no
limit 50;
```

## 12. 业务状态和关键字段

```sql
select design_plan_status, overall_status, count(*) as cnt
from tb_stores
group by design_plan_status, overall_status
order by design_plan_status, overall_status;

select status, is_active, count(*) as cnt
from tb_video_channels
group by status, is_active
order by status, is_active;

select count(*) as confirmed_business_without_area
from tb_video_channels
where status = 'confirmed_business'
  and (area_type is null or area_number is null);

select count(*) as bed_label_count
from tb_video_channels
where trim(bed_label) <> '';
```

## 13. 日志可查性

```sql
select action, entity_type, count(*) as cnt
from tb_operation_logs
group by action, entity_type
order by cnt desc;

select count(*) as logs_without_store_or_entity
from tb_operation_logs
where store_id is null
  and entity_id is null;

select id, action, entity_type, entity_id, store_id, actor, created_at
from tb_operation_logs
order by created_at desc
limit 50;
```

If `tb_audit_logs` has been applied:

```sql
select action, result, count(*) as cnt
from tb_audit_logs
group by action, result
order by action, result;

select id, user_email, action, entity_type, entity_id, store_id, channel_id, result, request_id, created_at
from tb_audit_logs
order by created_at desc
limit 50;
```

If `tb_asset_access_logs` has been applied:

```sql
select action, result, count(*) as cnt
from tb_asset_access_logs
group by action, result
order by action, result;
```

## 14. Index Inventory

```sql
select table_name,
       index_name,
       non_unique,
       seq_in_index,
       column_name
from information_schema.statistics
where table_schema = database()
order by table_name, index_name, seq_in_index;
```

## 15. CHECK Constraints Inventory

```sql
select constraint_name,
       table_name,
       check_clause
from information_schema.check_constraints
where constraint_schema = database()
order by table_name, constraint_name;
```

Note: MySQL 8.0.13 cannot rely on CHECK constraints as the only validation layer.

## 16. Governance Seed Coverage

If governance tables have been applied:

```sql
select r.code as role_code, count(rp.permission_id) as permission_count
from tb_roles r
left join tb_role_permissions rp on rp.role_id = r.id
group by r.code
order by r.code;

select p.code
from tb_permissions p
left join tb_role_permissions rp on rp.permission_id = p.id
where rp.permission_id is null
order by p.code;

select user_id, scope_type, scope_key, count(*) as cnt
from tb_user_store_scopes
group by user_id, scope_type, scope_key
having count(*) > 1;

select count(*) as raw_session_token_risk
from tb_auth_sessions
where length(session_token_hash) <> 64;
```
