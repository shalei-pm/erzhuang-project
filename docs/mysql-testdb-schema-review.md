# MySQL 测试库 Schema DBA 评审方案

最后更新：2026-07-01

本文用于评审用户提供的 MySQL 测试库结构与仓库内 `db/mysql_schema_tb.sql` 的一致性、合理性，并给出从测试库预演、历史数据导入到正式库镜像初始化的建议流程。本文只做静态审查和只读检查方案，不连接数据库、不写入测试库、不涉及正式环境发布。

## 0. 关键流程前提

MySQL 测试库不是一次性丢弃的沙箱。正确生命周期分两段：

- 阶段 A：schema 折腾期。测试库可以调整、重建、试跑，用于确认表结构、代码适配和小样本迁移。但每次 DDL 变更都要记录来源、目的、影响范围和回滚方式，避免最终无法复盘。
- 阶段 B：历史数据导入后。测试库开始承载从 Supabase/PostgreSQL 迁来的真实历史数据，成为公司测试环境的真实数据环境。此后不应随意清表、重建或手工修数据，schema 变更必须走 migration 脚本，并保证数据可保留、可校验、可回滚。

正式环境初始化不是“重新从 Supabase 直接摸索迁一遍”，而是在测试库结构和历史数据都验证后，由公司运维基于确认后的测试库镜像结构和数据进行初始化/搬运到正式环境。测试库因此既是预演库，也是正式迁移前的准生产数据基线。

凡是涉及以下事项，DBA 专项不得直接实现或执行：清表/重建、历史数据导入、正式 DDL 冻结、权限模型取舍、资产服务切换、敏感数据清理、删除策略、回滚策略、正式库初始化口径。必须先形成待确认清单，写明背景、可选方案、推荐方案、影响范围、风险、需要谁确认、确认后下一步，并交主会话复核，由主会话与用户确认后再推进。

## 1. 评审结论摘要

当前 `db/mysql_schema_tb.sql` 的 14 张表与主会话已知测试库表数量和表名方向一致，适合作为“业务表第一版预演库”的起点，但还不能作为正式环境最终 DDL 直接交给运维初始化。

主要原因：

- 14 张表只覆盖现有业务数据，缺少 SSO 后必须具备的用户、角色、机构范围、权限点、会话和审计表。
- 图片/PDF/截图第一阶段继续 Supabase Storage 可以接受，但未来公司文件服务迁移需要 `tb_asset_objects` 和资产访问审计表，否则敏感截图治理不完整。
- MySQL 8.0.13 且 `sql_mode` 为空、`CHECK` 不可靠，当前 DDL 的枚举和坐标约束不能单靠数据库保证。
- 多个 `text not null` 字段没有默认值，会影响当前 Go 代码常见插入路径，尤其是 `tb_ezviz_accounts` 和 `tb_video_channels.area_note`。
- 索引能覆盖一部分详情查询，但对门店列表、H5 Monitor、最新截图、权限范围过滤还不够稳，需要补索引并用 `EXPLAIN` 验证。
- 在历史数据导入前必须冻结 schema 版本，打 Git tag 或至少记录 commit；导入后测试库进入真实数据阶段，后续只能通过 migration 演进。

建议：测试库阶段 A 继续作为沙箱库折腾和预演；进入阶段 B 前冻结 schema 版本并导入 Supabase/PostgreSQL 历史数据；正式库初始化必须基于阶段 B 验证后的版本化 DDL、migration 和数据镜像，由运维执行。正式 DDL 不建议直接从测试库 `show create table` 反向拷贝，而应以仓库 DDL 为源头，结合测试库结构校验输出确认一致性。

## 2. 当前 14 张表是否足够

### 2.1 当前 14 张表清单

`db/mysql_schema_tb.sql` 包含：

- `tb_tasks`
- `tb_app_settings`
- `tb_design_plan_stores`
- `tb_design_plan_store_areas`
- `tb_design_plan_operation_logs`
- `tb_stores`
- `tb_store_areas`
- `tb_store_design_plans`
- `tb_design_plan_annotations`
- `tb_ezviz_accounts`
- `tb_video_recorders`
- `tb_video_channels`
- `tb_channel_snapshots`
- `tb_operation_logs`

这 14 张表足够支撑“数据库表结构预演 + 小样本业务数据导入 + MySQL repository 适配”的第一阶段，但不足以支撑公司正规环境的身份权限、审计和敏感资产治理。

### 2.2 旧表必须保留

旧 designplan 模块表需要暂时保留：

- `tb_design_plan_stores`
- `tb_design_plan_store_areas`
- `tb_design_plan_operation_logs`

原因：

- 当前代码仍注册旧 `designplan` 路由，`cmd/server/main.go` 仍注入 `designplan.NewPostgresStore(db)`。
- 旧表可能仍承担历史设计图上传、识别、迁移中间态或兼容接口。
- 在 MySQL 适配完成前删除旧表，会让旧路由或迁移脚本缺少依赖。

建议状态：

- 测试库和第一版正式 DDL 中保留。
- 新功能不再往旧表扩展。
- 样本迁移时如源数据仍来自旧 designplan 表，先迁旧表，再转换到新门店模型。
- 全量迁移完成并确认无代码依赖后，再单独做归档/删除 migration。

### 2.3 必须新增的正式治理表

正式环境至少需要补：

- `tb_users`
- `tb_roles`
- `tb_user_roles`
- `tb_user_store_scopes`
- `tb_permissions`
- `tb_role_permissions`
- `tb_auth_sessions`
- `tb_audit_logs`
- `tb_asset_objects`
- `tb_asset_access_logs`

其中 `tb_users`、`tb_user_roles`、`tb_user_store_scopes` 是 SSO 接入后最小权限闭环；`tb_asset_objects` 是未来公司文件服务迁移的关键映射表；`tb_audit_logs` 和 `tb_asset_access_logs` 是 H5 Monitor、实时视频、录像回放和截图读取的安全审计底座。

## 3. 静态 DDL 必须修订项

### 3.1 `text not null` 无默认值会卡插入

MySQL 不允许 `TEXT` 字段设置普通默认值。当前 DDL 中这些字段是 `text not null` 且无默认值：

- `tb_app_settings.value`
- `tb_design_plan_operation_logs.summary`
- `tb_ezviz_accounts.app_secret_ciphertext`
- `tb_ezviz_accounts.access_token_ciphertext`
- `tb_video_channels.area_note`
- `tb_operation_logs.summary`

风险判断：

- `tb_ezviz_accounts` 必须修。当前 Go 代码创建萤石账号/同步账号名时只写 `account_name`、`status`，如果 MySQL 表要求 `app_secret_ciphertext` 和 `access_token_ciphertext` 非空无默认，插入会失败。
- `tb_video_channels.area_note` 必须修。扫描/刷新通道插入时通常不会显式写 `area_note`，非空无默认会导致插入失败。
- 日志表 `summary` 通常业务会传值，可以保留非空，但迁移脚本必须保证不为空。
- `tb_app_settings.value` 由设置写入，通常会传值，可以保留非空。

建议修订：

- `tb_ezviz_accounts.app_secret_ciphertext` 改为 `text null`，或改成 `varchar(2048) not null default ''`。鉴于当前决策是不把密钥写业务库，建议第一版用 `text null`，正式落库前另做 KMS 方案。
- `tb_ezviz_accounts.access_token_ciphertext` 同上。
- `tb_video_channels.area_note` 改为 `varchar(1024) not null default ''`，如果确实可能超长，再改 `text null` 并由应用层把空值当空字符串。
- 日志 `summary` 可保持 `text not null`，但迁移脚本要 `coalesce(summary, '')` 并校验长度/非空。

### 3.2 `CHECK` 在 MySQL 8.0.13 不可靠

主会话探测到测试库版本是 MySQL `8.0.13`。该版本对 `CHECK` 约束不能作为可靠执行保障。当前 DDL 大量使用 `CHECK` 保护：

- 状态枚举：门店状态、识别状态、通道状态、录像机状态等。
- 区域类型枚举：`treatment`、`vip_treatment`、`consultation`、`beauty`。
- 坐标范围：`box_x`、`box_y`、`box_width`、`box_height`。
- 数值范围：`channel_no > 0`、`recognition_attempts >= 0`。

必须兜底：

- Go 层 validation 必须完整保留，不能因为 DDL 有 `CHECK` 就省略。
- 样本迁移脚本必须单独跑非法枚举、非法坐标、非法数值检查。
- 正式库初始化前请运维确认 MySQL 版本；如果仍是 8.0.13，DDL 中 `CHECK` 只当文档提示，不当安全边界。
- 可以考虑用小字典表强化枚举，但第一版不建议过度拆表，避免业务迭代成本过高。

### 3.3 `sql_mode` 为空必须处理

主会话探测到测试库 `sql_mode` 为空，风险很高：

- 字符串截断可能变 warning 而不是失败。
- 非法日期、零日期可能被接受。
- 隐式类型转换可能吞掉脏数据。
- JSON、datetime、数字字段迁移时更难发现问题。

建议：

- 应用 MySQL 连接建立后执行 session 级严格模式：

```sql
set session sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';
```

- 测试库可以先由我们在预演阶段要求连接设置严格模式；正式库建议推动运维设置全局或库级规范。
- 所有样本导入脚本开头也应设置 session `sql_mode`，不要依赖数据库默认值。

### 3.4 `datetime(3)` 与 `timestamptz` 时区要统一

PostgreSQL 当前使用 `timestamptz`，MySQL 草案使用 `datetime(3)`。测试库 `time_zone=+08:00`、`system_time_zone=CST`。

建议口径：

- 迁移时明确源数据转成 UTC 文本写入 MySQL `datetime(3)`，应用层按 Asia/Shanghai 展示；或者明确公司统一使用北京时间存储。两者只能选一个，不能混用。
- 推荐“数据库存 UTC，展示层转北京时间”，因为 Go 代码里已有大量 `time.Now().UTC()` 的习惯。
- 如果公司规范要求北京时间存储，迁移脚本必须显式 `AT TIME ZONE 'Asia/Shanghai'` 或等价转换，并在文档里标明。
- 禁止依赖客户端本机时区或 MySQL session 时区隐式转换。

### 3.5 JSON 迁移必须空值转 NULL

JSON 字段包括：

- `tb_design_plan_stores.recognition_result`
- `tb_store_design_plans.recognition_result`
- `tb_video_channels.recognition_result`

要求：

- PostgreSQL `jsonb null` -> MySQL `json null`。
- 空字符串必须转 `NULL`，不能写入 MySQL JSON 字段。
- 导入前后都用 `json_valid` 做校验。
- 第一阶段只做 JSON 存取，不做复杂 JSON 查询和 JSON 索引设计。

### 3.6 级联删除不适合正式敏感数据

当前 DDL 多处 `on delete cascade`：

- 删除门店会级联区域、设计图、录像机、通道、截图记录。
- 删除通道会级联截图记录。

测试库可以保留，方便预演；正式环境建议谨慎：

- 第一版业务如仍有删除功能，至少要写 `tb_audit_logs`。
- 更稳妥是给 `tb_stores`、`tb_video_recorders`、`tb_video_channels` 增加 `deleted_at/deleted_by` 做软删除。
- 资产对象不要因为业务行删除就立即物理删除，尤其是摄像头截图和审计证据。

## 4. 静态 DDL 建议修订项

### 4.1 补权限和资产治理表

建议新增单独 DDL 文件：

- `db/mysql_governance_schema_tb.sql`

不要把所有治理表一次塞进旧草案里，原因是：

- 业务表迁移和权限治理可以分阶段 review。
- 运维初始化时也可以清楚地区分“业务表”和“治理表”。
- 后续 SSO 字段确认后，只需要修治理 DDL。

治理 DDL 内容可沿用 `docs/mysql-dba-plan.md` 中的表设计，并在主会话确认后固化。

### 4.2 补外部业务区域/床位预留

当前 `area_type + area_number + bed_label` 是临时本地映射。未来要接公司业务系统空间/床位对象，建议预留：

- `tb_store_areas.external_area_id varchar(255) not null default ''`
- `tb_video_channels.external_area_id varchar(255) not null default ''`
- `tb_video_channels.external_bed_id varchar(255) not null default ''`

也可以新增独立映射表，例如 `tb_channel_area_mappings`，但第一版预留字段成本更低。

### 4.3 统一截图路径语义

当前字段：

- `tb_channel_snapshots.thumbnail_path`
- `tb_channel_snapshots.full_image_path`

建议明确只存 logical key，例如 `channel-snapshots/{name}.jpg`，不要混存 `/api/store-space/channel-snapshots/{name}.jpg`。如果历史数据已经有 API 路径，迁移脚本做归一化。

可选新增：

- `snapshot_key varchar(1024) not null default ''`
- 或继续复用 `thumbnail_path/full_image_path`，但在字段注释/文档里明确是 logical key。

### 4.4 保留 `tb_tasks` 但不进入正式主路径

`tb_tasks` 是练习表，正式环境可以不建。如果为了 `/health` 或 demo 兼容暂时保留，应在正式 DDL 注释中标注为非核心表，并且不要参与业务迁移验收。

## 5. 索引评审

### 5.1 已有索引覆盖情况

当前 DDL 已覆盖：

- `tb_stores.normalized_name` 唯一索引：支持重名检查和规范名查重。
- `tb_stores.updated_at`：支持按更新时间倒序的门店列表。
- `tb_store_areas(store_id, area_type, area_number)` 唯一索引：支持区域定位。
- `tb_store_design_plans(store_id)`：支持门店详情加载设计图。
- `tb_design_plan_annotations(design_plan_id, area_id)`：支持标注定位。
- `tb_video_recorders(store_id)`：支持门店详情加载录像机。
- `tb_video_recorders(device_code)` 唯一索引：支持设备编码查重。
- `tb_video_channels(recorder_id, channel_no)` 唯一索引：支持录像机通道列表和 upsert。
- `tb_channel_snapshots(channel_id, created_at)`：支持按通道查截图。

### 5.2 建议新增/调整索引

门店列表和城市筛选：

```sql
create index idx_tb_stores_city_updated_at on tb_stores (city, updated_at, id);
create index idx_tb_stores_external_org_id on tb_stores (external_org_id);
```

H5 Monitor：

```sql
create index idx_tb_video_recorders_store_status on tb_video_recorders (store_id, status, id);
create index idx_tb_video_channels_active_status on tb_video_channels (recorder_id, is_active, status, channel_no);
```

最新截图：

```sql
drop index idx_tb_channel_snapshots_channel_id on tb_channel_snapshots;
create index idx_tb_channel_snapshots_latest on tb_channel_snapshots (channel_id, created_at, id);
```

权限过滤：

```sql
create index idx_tb_user_store_scopes_user_scope on tb_user_store_scopes (user_id, scope_type, store_id);
create index idx_tb_user_store_scopes_external_org on tb_user_store_scopes (user_id, external_org_id);
create index idx_tb_user_roles_user on tb_user_roles (user_id, role_id);
```

审计：

```sql
create index idx_tb_audit_logs_user_time on tb_audit_logs (user_id, created_at);
create index idx_tb_audit_logs_store_time on tb_audit_logs (store_id, created_at);
create index idx_tb_audit_logs_action_time on tb_audit_logs (action, created_at);
```

说明：MySQL 8.0 支持降序索引，但第一版用普通升序组合索引也可服务倒序扫描。最终以测试库 `EXPLAIN` 为准。

## 6. 测试库一致性只读检查清单

如需要连接测试库，只做只读结构检查。不要把密码写入命令、脚本、文档或日志。建议由主会话或用户授权后执行。

### 6.1 命令方式

使用交互式密码输入：

```sh
mysql \
  --host "$MYSQL_HOST" \
  --port "$MYSQL_PORT" \
  --user "$MYSQL_USER" \
  --password \
  --database "$MYSQL_DATABASE"
```

环境变量示例只放非密码项：

```sh
export MYSQL_HOST='polar-dev.rwlb.rds.aliyuncs.com'
export MYSQL_PORT='3306'
export MYSQL_DATABASE='db_pm_erzhuang'
export MYSQL_USER='u_pm_erzhuang_rw'
```

### 6.2 环境探针

```sql
select version() as mysql_version,
       @@sql_mode as sql_mode,
       @@time_zone as time_zone,
       @@system_time_zone as system_time_zone,
       @@character_set_database as charset_database,
       @@collation_database as collation_database;
```

### 6.3 表清单对比

```sql
select table_name
from information_schema.tables
where table_schema = database()
order by table_name;
```

预期 14 张表应与 `db/mysql_schema_tb.sql` 一致。若多表或少表，先确认是否用户手动试验产生，不要直接反向同步进正式 DDL。

### 6.4 字段结构对比

```sql
select table_name,
       ordinal_position,
       column_name,
       column_type,
       is_nullable,
       column_default,
       extra,
       column_key
from information_schema.columns
where table_schema = database()
order by table_name, ordinal_position;
```

重点核对：

- `tb_stores.short_name` 是否存在。
- `tb_video_channels.bed_label` 是否存在。
- `tb_video_channels.area_note` 是否 `not null` 且无默认。
- `tb_ezviz_accounts.app_secret_ciphertext/access_token_ciphertext` 是否 `not null` 且无默认。
- JSON 字段是否是 `json`。
- 时间字段是否是 `datetime(3)`。

### 6.5 索引和外键

```sql
select table_name,
       index_name,
       non_unique,
       seq_in_index,
       column_name
from information_schema.statistics
where table_schema = database()
order by table_name, index_name, seq_in_index;

select table_name,
       constraint_name,
       referenced_table_name,
       delete_rule,
       update_rule
from information_schema.referential_constraints
where constraint_schema = database()
order by table_name, constraint_name;
```

### 6.6 `CHECK` 约束存在性

```sql
select constraint_name,
       table_name,
       check_clause
from information_schema.check_constraints
where constraint_schema = database()
order by table_name, constraint_name;
```

注意：即使查到 `CHECK` 存在，MySQL 8.0.13 也不能把它当强制执行保障。

### 6.7 `SHOW CREATE TABLE` 导出

```sql
show create table tb_stores;
show create table tb_video_channels;
show create table tb_ezviz_accounts;
show create table tb_channel_snapshots;
```

需要完整比对时，可对 14 张表都执行 `show create table`，但输出只用于审查，不建议直接复制成正式 DDL。

## 7. 小样本迁移和校验计划

### 7.1 样本选择

先选 1-2 家门店，必须覆盖：

- `external_org_id`
- 设计图 PDF、预览图、缩略图
- 设计图标注框
- 录像机
- 通道
- 通道截图
- AI `recognition_result`
- `bed_label`
- 操作日志最好有

### 7.2 表顺序

推荐顺序：

1. `tb_stores`
2. `tb_store_areas`
3. `tb_store_design_plans`
4. `tb_design_plan_annotations`
5. `tb_ezviz_accounts`
6. `tb_video_recorders`
7. `tb_video_channels`
8. `tb_channel_snapshots`
9. `tb_operation_logs`

旧 designplan 表如仍需兼容，单独迁：

1. `tb_design_plan_stores`
2. `tb_design_plan_store_areas`
3. `tb_design_plan_operation_logs`

### 7.3 导入规则

- 保留原始 `id`，不要重生成主键。
- 导入脚本开头设置严格 session `sql_mode`。
- JSON 空字符串转 `NULL`。
- 时间字段显式转换，不靠 session 时区。
- 图片路径保持 logical key，不迁二进制。
- 外键尽量保持开启；如必须临时关闭 `foreign_key_checks`，只允许在单次导入会话内使用，并立即恢复和做孤儿检查。

### 7.4 校验 SQL

行数和主键：

```sql
select 'tb_stores' as table_name, count(*) as row_count, max(id) as max_id from tb_stores
union all select 'tb_store_areas', count(*), max(id) from tb_store_areas
union all select 'tb_store_design_plans', count(*), max(id) from tb_store_design_plans
union all select 'tb_design_plan_annotations', count(*), max(id) from tb_design_plan_annotations
union all select 'tb_ezviz_accounts', count(*), max(id) from tb_ezviz_accounts
union all select 'tb_video_recorders', count(*), max(id) from tb_video_recorders
union all select 'tb_video_channels', count(*), max(id) from tb_video_channels
union all select 'tb_channel_snapshots', count(*), max(id) from tb_channel_snapshots;
```

外键孤儿：

```sql
select count(*) as orphan_store_areas
from tb_store_areas a
left join tb_stores s on s.id = a.store_id
where s.id is null;

select count(*) as orphan_recorders
from tb_video_recorders r
left join tb_stores s on s.id = r.store_id
where s.id is null;

select count(*) as orphan_channels
from tb_video_channels c
left join tb_video_recorders r on r.id = c.recorder_id
where r.id is null;

select count(*) as orphan_snapshots
from tb_channel_snapshots cs
left join tb_video_channels c on c.id = cs.channel_id
where c.id is null;
```

JSON：

```sql
select count(*) as invalid_store_plan_json
from tb_store_design_plans
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as invalid_channel_json
from tb_video_channels
where recognition_result is not null
  and json_valid(recognition_result) = 0;
```

`auto_increment` 校准生成：

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
) t;
```

## 8. Supabase/PostgreSQL 到 MySQL 测试库历史数据迁移计划

### 8.1 迁移前验收用例设计

历史数据迁入 MySQL 测试库前，必须先完成一套可重复的迁移验收用例。顺序固定为：

1. 先设计验收用例、测试数据、API smoke test、页面 smoke test、数据对账 SQL。
2. 再用假数据和小样本金丝雀数据跑通 MySQL schema、MySQL repository、资产读取、权限过滤。
3. 全部通过后，才允许把 Supabase/PostgreSQL 历史数据迁入 MySQL 测试库。
4. 历史数据导入后，测试库开始承载真实历史数据，不再随意清表、重建或手工修核心数据。

#### 8.1.1 金丝雀和假数据设计

真实金丝雀：

- 北京保利实验室门店。
- 机构 ID：`10030`。
- 录像机：`GN0941203`。
- 用途：覆盖 H5 Monitor 试点链路、真实录像机/通道、实时视频、录像回放、截图读取。

补充假数据建议：

- 假门店 A：无设计图、无录像机，用于验证空态。
- 假门店 B：有设计图和标注，但无通道，用于验证设计图 Tab 和区域数据。
- 假门店 C：有录像机和通道，但无截图，用于验证缺图诊断。
- 假通道 1：`confirmed_business`，有 `area_type/area_number/area_id`。
- 假通道 2：`confirmed_non_business`，无业务区域。
- 假通道 3：有 `bed_label`，覆盖治疗室/VIP 治疗室/美容室床位拆分。
- 假通道 4：有合法 `recognition_result` JSON。

权限假数据：

- `admin`：全量机构。
- `viewer_single`：只看北京保利实验室门店 `external_org_id=10030`。
- `viewer_multi`：看 2 家门店。
- `viewer_none`：登录成功但没有机构范围，应看不到门店和监控。
- `operator_store`：可维护指定门店，可编辑门店/设计图/通道确认，但不能越权维护其他门店。

#### 8.1.2 Given/When/Then 验收用例

健康和路径：

- Given 应用连接 MySQL 测试库，资产仍使用 Supabase Storage；When 访问 `/erzhuang-project/health`；Then 返回 200，`status=ok`，`database=mysql`，`asset_store=supabase` 或明确的当前资产模式。
- Given 前端部署在 `/erzhuang-project/`；When 打开 `/erzhuang-project/`；Then 后台首页正常渲染，静态资源不 404。
- Given 北京保利实验室门店存在；When 打开 `/erzhuang-project/h5/orgs/10030/monitor`；Then H5 页面正常渲染并请求 `/erzhuang-project/api/h5/orgs/10030/monitor`。

门店列表：

- Given 金丝雀和假门店已导入；When 请求门店列表第一页；Then 返回 `items/page/page_size/total/summary/cities`。
- Given 门店 city 已设置；When 按城市筛选；Then 只返回该城市门店，summary 与结果一致。
- Given 搜索北京保利实验室或 short name；When 搜索；Then 能找到目标门店。

门店详情和设计图：

- Given 假门店 B 有设计图、标注框和区域；When 打开设计图标注 Tab；Then 预览图可读，标注框数量和区域数量与数据库一致。
- Given 修改设计图区域或保存标注；When 保存并刷新详情；Then `tb_store_areas`、`tb_design_plan_annotations` 读取结果保持一致。

通道映射和截图：

- Given 北京保利实验室门店有录像机 `GN0941203`；When 打开通道映射 Tab；Then 录像机、通道、确认状态和最近截图 URL 正常显示。
- Given 假通道 3 有 `bed_label`；When 修改并保存床位拆分；Then 刷新后 `bed_label` 不丢失。
- Given 假通道 C 无截图；When 打开截图诊断；Then 返回可诊断错误，不能只显示泛化 500。
- Given 通道截图 logical key 存在于 Supabase Storage；When 请求 `/api/store-space/channel-snapshots/{name}`；Then 返回图片内容，前端路径不需要知道 Supabase key。

H5 Monitor：

- Given `viewer_single` 只授权 `10030`；When 访问 `/api/h5/orgs/10030/monitor`；Then 返回北京保利实验室通道列表。
- Given `viewer_single` 访问非授权门店；When 请求该门店 H5 API；Then 后端返回 403 或空授权结果，不返回真实通道。
- Given 北京保利实验室通道可播放；When 请求 live-url、record-segments、playback-url；Then 返回播放地址或明确萤石错误码，响应不暴露 app secret/access token。

权限：

- Given `admin` 登录；When 请求门店列表和 H5 Monitor；Then 可见全量授权数据。
- Given `viewer_none` 登录；When 请求门店列表和 H5 Monitor；Then 登录成功但业务数据为空或 403，不能看到门店/监控。
- Given `operator_store` 登录；When 编辑授权门店；Then 成功；When 编辑未授权门店；Then 后端拒绝。

操作日志：

- Given 用户执行敏感操作；When 刷新截图、获取直播地址、获取回放地址、修改通道确认；Then 写入审计或操作日志，至少包含 `actor/email`、`store_id` 或 `channel_id`、`action`、结果、时间。

#### 8.1.3 迁移前 API smoke test

迁移前 smoke test 使用金丝雀和假数据执行，必须可重复。核心清单沿用第 10.5 节，并额外要求：

- 所有 GET 类接口可重复执行，不改变数据。
- POST/PUT/PATCH 类接口只作用于假门店或专用测试通道。
- DELETE 类接口默认不执行；如必须验证，只对假门店执行并在验收后重建假数据。
- H5 live-url/playback-url 如果依赖真实萤石状态，允许返回明确上游错误码，但不能返回数据库错误、空指针或 500 泛化错误。

#### 8.1.4 迁移前页面手工验收

页面手工验收至少覆盖：

1. 后台首页在 `/erzhuang-project/` 打开。
2. 门店列表分页、搜索、城市筛选、统计汇总。
3. 北京保利实验室门店详情基础信息。
4. 设计图标注 Tab：图片显示、标注框显示、区域读取和保存。
5. 通道映射 Tab：录像机、通道、截图路径、区域确认、`bed_label` 保存读取。
6. H5 Monitor：北京保利实验室入口、实时视频列表、录像回放入口、通道详情。
7. 图片代理：存在图可读，缺图有诊断。
8. 权限视角：admin、viewer_single、viewer_none、operator_store 分别登录验证。

#### 8.1.5 迁移前数据对账 SQL

假数据/金丝雀导入后执行：

```sql
select count(*) as store_count from tb_stores;
select count(*) as baoli_count from tb_stores where external_org_id = '10030';
select count(*) as recorder_count from tb_video_recorders where device_code = 'GN0941203';
select count(*) as channel_count
from tb_video_channels c
join tb_video_recorders r on r.id = c.recorder_id
where r.device_code = 'GN0941203';

select count(*) as business_channels
from tb_video_channels
where status = 'confirmed_business';

select count(*) as non_business_channels
from tb_video_channels
where status = 'confirmed_non_business';

select count(*) as bed_label_channels
from tb_video_channels
where trim(bed_label) <> '';

select count(*) as invalid_json
from tb_video_channels
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as orphan_area_id
from tb_video_channels c
left join tb_store_areas a on a.id = c.area_id
where c.area_id is not null
  and a.id is null;
```

权限数据：

```sql
select u.email, r.code as role_code, s.scope_type, s.store_id, s.external_org_id
from tb_users u
left join tb_user_roles ur on ur.user_id = u.id
left join tb_roles r on r.id = ur.role_id
left join tb_user_store_scopes s on s.user_id = u.id
where u.email in (
  'admin@example.com',
  'viewer_single@example.com',
  'viewer_multi@example.com',
  'viewer_none@example.com',
  'operator_store@example.com'
)
order by u.email, r.code, s.scope_type, s.store_id;
```

操作/审计日志：

```sql
select action, entity_type, entity_id, store_id, user_email, result, created_at
from tb_audit_logs
where action in (
  'h5_monitor.play_live',
  'h5_monitor.playback',
  'snapshot.refresh',
  'channel.confirm',
  'asset.sensitive.view'
)
order by created_at desc
limit 20;
```

如果权限表和审计表尚未落地，以上 SQL 标记为“待权限模块实施后启用”，但不取消验收要求。

#### 8.1.6 可重建与必须保留

可以不迁或可重建：

- 通道截图文件二进制，可通过重新抓图或公司文件服务批处理补齐。
- 过期直播/回放 URL。
- 临时诊断状态、播放器状态、进程内并发计数。
- 前端构建产物、缓存。
- 纯练习表 `tb_tasks`。

必须保留：

- 门店、机构 ID、城市、简称、基础状态。
- 设计图 logical key、识别结果、标注框、区域。
- 录像机、通道、通道确认状态、业务/非业务分类。
- `area_id`、`area_type`、`area_number`、`bed_label`。
- AI `recognition_result` JSON。
- 操作日志、审计日志。
- 授权用户、角色、机构范围。
- 图片/PDF logical key，即使文件二进制后续可迁移或重建。

#### 8.1.7 通过/失败判定标准

通过标准：

- API smoke test 全部达到期望状态码和核心字段。
- 页面 smoke test 全部完成，刷新后数据仍在。
- 数据对账 SQL 无外键孤儿、无非法 JSON、关键金丝雀记录存在。
- 图片代理存在图可读，缺图有诊断。
- 权限用例满足 admin 全量、viewer/operator 按范围过滤、后端拒绝越权。
- 敏感操作写入操作日志或审计日志。

失败标准：

- `/health` 不能明确返回 `database=mysql`。
- 任一现有核心路径 404 或因 MySQL repository 报错不可用。
- 门店、设计图、通道、截图、H5 Monitor 任一核心流程不可用且无明确降级说明。
- `id` 断裂导致外键孤儿或前端详情打不开。
- JSON 读取/写入失败。
- 权限绕过：未授权用户能通过直接 API 访问门店、监控或截图。
- 历史数据导入前验收未跑完或结果不可复现。

只有本节门禁通过，才允许执行第 8.2 节之后的历史数据导入。

### 8.2 阶段 A 到阶段 B 的切换门槛

历史数据导入前必须先完成这些动作：

1. DDL 冻结：确认业务表 DDL、治理表 DDL、索引、时区口径、`sql_mode` 口径。
2. 版本冻结：在 Git 中记录 schema commit，建议打 tag，例如 `mysql-schema-baseline-YYYYMMDD`。如果暂时不打 tag，至少在 `docs/codex-learning-state.md` 记录 commit、DDL 文件、测试库状态。
3. 迁移前验收：第 8.1 节的 API、页面、权限、日志和数据 SQL 全部通过。
4. 测试库清洁检查：确认阶段 A 的临时试验表、脏样本、手工插入测试数据已清理或明确标记不迁入正式基线。
5. 备份：导入历史数据前导出测试库当前 schema 和少量样本数据备份，便于回到导入前状态。
6. 停写计划：明确 Supabase/PostgreSQL 的短暂停写窗口，避免导出期间新增数据丢失。

阶段 B 一旦开始，测试库就不再按“随便折腾”处理。任何变更都要通过 migration 脚本执行，禁止直接 `drop table`、`truncate` 或手工改核心数据。

### 8.3 防止折腾期污染历史数据

建议在阶段 A 采用这些隔离办法：

- 所有临时表使用 `tmp_` 或 `zz_` 前缀，正式 DDL 禁止包含这些表。
- 所有手工插入的样本数据记录来源和门店 ID，历史数据导入前统一清理或重建空库。
- 如果测试库已被多轮手工试验污染，进入阶段 B 前优先重建一次测试库 schema，再导入历史数据。
- 所有 DDL 变更先落到 Git 文件，再由测试库执行；不要只在数据库里手工改结构。
- 每次 DDL 变更记录：变更 SQL、执行时间、是否影响数据、回滚 SQL 或恢复方式。

### 8.4 历史数据导入范围

必须迁移和保留：

- 门店：`tb_stores`，包括 `external_org_id`、`city`、`short_name`、状态字段。
- 区域/空间：`tb_store_areas`，包括设计图识别和通道确认形成的区域。
- 设计图：`tb_store_design_plans` 和 `tb_design_plan_annotations`，包括 PDF/预览图/缩略图 logical key、标注框、AI `recognition_result`。
- 录像机和通道：`tb_ezviz_accounts`、`tb_video_recorders`、`tb_video_channels`，包括通道确认状态、`scene_type`、`area_type`、`area_number`、`bed_label`、`area_id`、识别结果。
- 操作日志：`tb_operation_logs` 和旧 `tb_design_plan_operation_logs`，用于追溯历史操作。
- 权限用户：如果 SSO/权限已上线，应迁 `tb_users`、角色、scope 和审计；如果导入时还未上线，至少准备初始化管理员和后续授权导入脚本。

可以不迁或可重建：

- 通道截图文件二进制：第一阶段仍留 Supabase Storage；未来可重新抓图或通过资产迁移批处理补齐。
- 过期的萤石临时播放 URL：不迁。
- 进程内并发状态、临时诊断状态：不迁。
- 前端构建产物和缓存：不迁。
- `tb_tasks` 练习数据：正式主链路可不迁。

注意：虽然截图文件可重建，截图记录中的 logical key、最近截图路径和通道识别上下文仍建议迁移，避免页面历史状态断层。

### 8.5 导入执行顺序

建议分两轮：

第一轮是结构和基础数据：

1. 业务 DDL。
2. 治理 DDL。
3. 角色和权限点 seed。
4. 基础配置 `tb_app_settings`。

第二轮是历史业务数据：

1. 旧 designplan 表，如果仍有依赖。
2. `tb_stores`
3. `tb_store_areas`
4. `tb_store_design_plans`
5. `tb_design_plan_annotations`
6. `tb_ezviz_accounts`
7. `tb_video_recorders`
8. `tb_video_channels`
9. `tb_channel_snapshots`
10. `tb_operation_logs`
11. 权限用户、角色、scope、审计日志，如果当时已经启用。

导入要求：

- 保留原始 `id`。
- 导入后逐表校准 `auto_increment=max(id)+1`。
- JSON 空字符串转 `NULL`，并执行 `json_valid` 校验。
- 时间字段按冻结口径显式转换。
- 图片路径归一化为 logical key。
- 导入脚本设置严格 session `sql_mode`。

### 8.6 历史数据完整性校验

需要证明“之前积累的数据都保留”，不能只看导入脚本成功。

源库导出前生成 baseline：

- 每张表行数。
- 每张核心表 `min(id)`、`max(id)`。
- 按门店抽样的业务摘要：门店数、设计图数、标注数、录像机数、通道数、截图记录数。
- JSON 非空数量。
- 关键门店的 `external_org_id`、通道确认数量、`bed_label` 非空数量。

MySQL 导入后生成同样报告并对比：

```sql
select count(*) as store_count, min(id) as min_id, max(id) as max_id from tb_stores;
select count(*) as area_count, min(id) as min_id, max(id) as max_id from tb_store_areas;
select count(*) as plan_count, min(id) as min_id, max(id) as max_id from tb_store_design_plans;
select count(*) as annotation_count, min(id) as min_id, max(id) as max_id from tb_design_plan_annotations;
select count(*) as recorder_count, min(id) as min_id, max(id) as max_id from tb_video_recorders;
select count(*) as channel_count, min(id) as min_id, max(id) as max_id from tb_video_channels;
select count(*) as snapshot_count, min(id) as min_id, max(id) as max_id from tb_channel_snapshots;
```

业务抽样校验：

- 抽样 5 家有设计图门店，确认预览图 URL、标注框数量、区域数量一致。
- 抽样 5 家有录像机门店，确认录像机、通道、确认状态和床位拆分一致。
- 抽样 5 条 H5 Monitor 通道，确认 `external_org_id -> store -> recorder -> channel -> latest snapshot` 链路可查。
- 抽样敏感截图访问，确认后端仍通过 Supabase Storage 读取，不暴露真实密钥和签名 URL。

若 baseline 与 MySQL 报告不一致，先定位原因，不进入正式镜像初始化。

## 9. MySQL 测试库到正式库镜像初始化流程

### 9.1 Schema 确认

1. 以 `db/mysql_schema_tb.sql` 为业务表源头。
2. 合并本评审必须修订项，形成 `db/mysql_schema_tb_vnext.sql` 或直接修订原 DDL。
3. 新增 `db/mysql_governance_schema_tb.sql`，放权限、审计、资产映射表。
4. 主会话验收表清单、字段、索引、删除策略、时区口径、`sql_mode` 口径。
5. 运维确认正式 MySQL 版本、字符集、collation、时区、`sql_mode` 和账号权限模型。

### 9.2 测试库小样本导入

1. 在测试库应用 vnext DDL。
2. 导入 1-2 家样本门店。
3. 执行行数、外键、JSON、时间、图片路径、`auto_increment` 校验。
4. 后端 MySQL repository 在测试库只读/小范围写入验证。
5. 图片仍读 Supabase Storage，验证设计图和通道截图可打开。

### 9.3 测试库导入完整历史数据

1. 冻结 schema 版本并记录 commit/tag。
2. 清理阶段 A 临时表和试验数据，必要时重建测试库 schema。
3. 从 Supabase/PostgreSQL 导出历史数据 baseline。
4. 将历史数据导入 MySQL 测试库。
5. 执行完整性校验和业务抽样验证。
6. 测试环境应用切到 MySQL 测试库，跑一轮关键页面和接口验收。
7. 将测试库标记为阶段 B 真实数据环境，后续变更只走 migration。

### 9.4 生成正式 DDL 和迁移包

交给运维的包建议包括：

- `001_business_schema.sql`
- `002_governance_schema.sql`
- `003_seed_roles_permissions.sql`
- `004_indexes.sql`，如果索引单独管理。
- `README.md`，说明执行顺序、MySQL 版本要求、字符集、`sql_mode`、时区口径、禁止事项。
- 历史数据搬运说明和 baseline 对比口径。

正式 DDL 包必须来自 Git 仓库，不从本地临时 SQL 或聊天记录复制。

### 9.5 运维初始化正式库

1. 运维在正式环境创建库和账号。
2. 运维按 DDL 包执行初始化。
3. 运维基于已验证的 MySQL 测试库镜像结构和数据进行正式库初始化/搬运。
4. 运维回传只读结构检查输出、行数报告和关键导入日志。
5. 主会话/DBA 专项对比 `information_schema`、baseline 和抽样数据，确认正式库结构与数据一致。

正式库不提供本地直连账号；应用通过 K8s Secret 注入连接信息。

### 9.6 停写窗口和回滚

1. 正式迁移前安排短暂停写窗口，确保 Supabase/PostgreSQL、MySQL 测试库、正式库的数据基线一致。
2. 如果测试库阶段 B 后仍有新增测试环境真实数据，需要先决定这些增量是否进入正式库。
3. 正式库初始化后校验行数、外键、JSON、时间、抽样图片路径、关键页面接口。
4. 应用切换到正式 MySQL。
5. Supabase/PostgreSQL 保留只读备份一段时间作为回滚来源。
6. MySQL 测试库也保留为对照环境，用于排查正式初始化差异。
7. 回滚策略是应用切回 PostgreSQL/Supabase 或已验证的 MySQL 测试环境，正式 MySQL 保留为失败现场，不做立即清空。

## 10. 迁移后接口兼容性验收门禁

数据库迁移不能只验 schema 和数据导入成功，还必须证明现有 Go 接口访问路径、前端页面流程、H5 Monitor 和图片代理在 MySQL 后仍可用。本章作为 MySQL 对接方案的发布前门禁。

### 10.1 路径兼容

必须保持：

- 公司当前线上访问前缀 `/erzhuang-project/` 继续可用。
- 旧个人/韩国路径 `/erzhuang/` 如仍由代码兼容，不应被 MySQL 适配破坏。
- `/erzhuang-project/health` 和 `/health` 返回 HTTP 200，`status=ok`，`database=mysql`，`asset_store` 明确返回当前模式，例如 `supabase` 或未来公司文件服务模式。
- 所有 `/erzhuang-project/api/store-space/*` 经 base path 剥离后继续映射到 `/api/store-space/*`。
- 后台 H5 页面路径 `/erzhuang-project/h5/orgs/{externalOrgId}/monitor` 和 `/erzhuang-project/h5/orgs/{externalOrgId}/monitor/channels/{channelId}` 继续由前端路由打开。
- H5 API 路径 `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor`、`live-url`、`record-segments`、`playback-url`、`disable-url` 继续可用。
- 图片代理 `/erzhuang-project/api/store-space/channel-snapshots/{name}` 继续可用。底层从 Supabase Storage 切到公司文件服务时，前端路径不变化。

### 10.2 后台页面兼容

必须用浏览器实际验收，不只看 API 和构建：

- 门店列表：分页、城市筛选、搜索、统计汇总、空态和有数据态。
- 门店详情：基础信息、设计图标注 Tab、通道映射 Tab 都能打开。
- 新增/编辑门店：`short_name`、`external_org_id`、录像机、设计图字段保存后刷新仍在。
- 设计图：上传、识别、保存、标注读取、预览图/缩略图显示。
- 通道：扫描、AI 识别、单通道刷新截图、确认、解锁编辑、删除。
- 床位拆分：`bed_label` 对治疗室、VIP 治疗室、美容室保存后刷新仍在。
- H5 Monitor：门店入口可打开，首页通道列表按机构显示，实时视频、回放、播放地址失效流程可用。
- Excel 导出：通道映射导出仍能读取截图和通道确认数据。

### 10.3 数据兼容

迁移后必须确认：

- Supabase/PostgreSQL 原有核心 `id` 尽量保留，避免外键、前端缓存、历史引用断裂。
- `recognition_result` JSON 能正常读取和写入；空 JSON 不被写成非法空字符串。
- 图片/PDF 路径字段不丢失，第一阶段仍能从 Supabase Storage 读取。
- 已确认通道状态、区域映射、`area_id`、`area_type`、`area_number`、`bed_label` 保留。
- 操作日志和旧 designplan 操作日志保留，便于追溯。
- `auto_increment` 大于历史最大 `id`，新增数据不会撞历史主键。

### 10.4 权限兼容

权限接入要分开验收“未启用”和“启用后”：

- 未启用 SSO/权限时，当前测试流程不应被阻断，仍可完成门店、设计图、通道、H5 Monitor 验收。
- 启用权限后，`admin` 可看全量机构和全部操作。
- `viewer` 和 `operator` 必须按 `tb_user_store_scopes` 的机构/门店范围过滤门店列表、详情、H5 Monitor。
- 前端可以隐藏按钮，但后端接口必须真实校验权限；直接调用未授权 API 应返回 401/403，而不是只靠页面不显示入口。
- H5 Monitor、实时视频、录像回放、截图读取属于敏感访问，必须写审计日志。

### 10.5 API smoke test 清单

以下 smoke test 使用金丝雀门店执行。具体请求体以当时 API 类型为准，验收关注状态码和核心字段。

路径和健康：

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/erzhuang-project/health` | 200，`status=ok`，`database=mysql`，`asset_store` 非空 |
| GET | `/erzhuang-project/` | 200，返回前端首页 |
| GET | `/erzhuang-project/h5/orgs/{externalOrgId}/monitor` | 200，返回前端 H5 页面 |

Store Space：

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/erzhuang-project/api/store-space/stores?page=1&page_size=20` | 200，含 `items/page/page_size/total/summary/cities` |
| GET | `/erzhuang-project/api/store-space/stores?q={keyword}` | 200，搜索结果稳定 |
| GET | `/erzhuang-project/api/store-space/stores?city={city}` | 200，城市筛选生效 |
| POST | `/erzhuang-project/api/store-space/stores/check-duplicate` | 200，含 exact/similar 判断 |
| GET | `/erzhuang-project/api/store-space/stores/{storeId}` | 200，含基础信息、areas、design_plans、recorders |
| PATCH | `/erzhuang-project/api/store-space/stores/{storeId}` | 200，`short_name/external_org_id` 保存并返回 |
| GET | `/erzhuang-project/api/store-space/stores/{storeId}/design-plan-data` | 200，含设计图和标注区域 |
| GET | `/erzhuang-project/api/store-space/stores/{storeId}/channel-data` | 200，含录像机、通道、截图 URL |
| PUT | `/erzhuang-project/api/store-space/stores/{storeId}/design-plan` | 200，保存后可再次读取 |
| POST | `/erzhuang-project/api/store-space/stores/{storeId}/recorders` | 201，新增录像机成功 |
| POST | `/erzhuang-project/api/store-space/recorders/{recorderId}/scan-channels` | 200，返回通道列表或明确诊断 |
| POST | `/erzhuang-project/api/store-space/recorders/{recorderId}/recognize-channels` | 200，识别结果可写入 |
| POST | `/erzhuang-project/api/store-space/channels/{channelId}/snapshot` | 200，返回 `/api/store-space/channel-snapshots/{name}` |
| GET | `/erzhuang-project/api/store-space/channel-snapshots/{name}` | 200，`Content-Type` 为图片类型 |
| GET | `/erzhuang-project/api/store-space/channel-snapshots/{name}/diagnostics` | 200，含 `asset_store/snapshot_key/exists` |
| POST | `/erzhuang-project/api/store-space/channels/{channelId}/unlock` | 200，状态变为可编辑 |
| PUT | `/erzhuang-project/api/store-space/channels/{channelId}/confirmation` | 200，`bed_label`、区域映射保存 |
| GET | `/erzhuang-project/api/store-space/stores/{storeId}/channel-mappings/export.xlsx` | 200，返回 Excel |

H5 Monitor API：

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor` | 200，含通道列表，不暴露 app secret/access token/device secret |
| POST | `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url` | 200，返回播放地址或明确萤石错误码 |
| GET | `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments?date=YYYY-MM-DD` | 200，返回录像片段数组 |
| POST | `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url` | 200，返回回放地址或明确错误 |
| POST | `/erzhuang-project/api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/disable-url` | 200，`ok=true` 或可解释失败 |

破坏性接口如 `DELETE /stores/{id}`、`DELETE /recorders/{id}`、`DELETE /channels/{id}` 只允许在专门测试门店上验收，不得对历史真实门店执行。

### 10.6 页面验收清单

金丝雀门店页面验收：

1. 打开 `/erzhuang-project/`，进入后台首页。
2. 门店列表搜索金丝雀门店，分页、城市筛选、统计数字正常。
3. 进入门店详情，基础信息展示正确。
4. 切换到设计图标注 Tab，预览图显示，标注框位置和数量正确。
5. 切换到通道映射 Tab，录像机和通道列表显示，最近截图可见。
6. 修改 `short_name` 或非关键备注字段，保存后刷新仍在。
7. 对测试通道执行确认，保存 `bed_label` 后刷新仍在。
8. 打开 H5 Monitor 首页和通道详情，实时视频/回放至少走到可解释状态。
9. 打开图片诊断，确认 `asset_store` 与 `/health` 一致。

### 10.7 数据对账 SQL

迁移后执行：

```sql
select 'tb_stores' table_name, count(*) row_count, max(id) max_id from tb_stores
union all select 'tb_store_areas', count(*), max(id) from tb_store_areas
union all select 'tb_store_design_plans', count(*), max(id) from tb_store_design_plans
union all select 'tb_design_plan_annotations', count(*), max(id) from tb_design_plan_annotations
union all select 'tb_video_recorders', count(*), max(id) from tb_video_recorders
union all select 'tb_video_channels', count(*), max(id) from tb_video_channels
union all select 'tb_channel_snapshots', count(*), max(id) from tb_channel_snapshots
union all select 'tb_operation_logs', count(*), max(id) from tb_operation_logs;

select count(*) as orphan_areas
from tb_store_areas a left join tb_stores s on s.id = a.store_id
where s.id is null;

select count(*) as orphan_channels
from tb_video_channels c left join tb_video_recorders r on r.id = c.recorder_id
where r.id is null;

select count(*) as invalid_channel_json
from tb_video_channels
where recognition_result is not null and json_valid(recognition_result) = 0;

select count(*) as missing_external_org
from tb_stores
where trim(external_org_id) = '';

select count(*) as confirmed_channels_without_mapping
from tb_video_channels
where status = 'confirmed_business'
  and (area_type is null or area_id is null);

select count(*) as bed_label_count
from tb_video_channels
where trim(bed_label) <> '';
```

`auto_increment` 对账：

```sql
select table_name, auto_increment
from information_schema.tables
where table_schema = database()
  and table_name in (
    'tb_stores',
    'tb_store_areas',
    'tb_store_design_plans',
    'tb_design_plan_annotations',
    'tb_ezviz_accounts',
    'tb_video_recorders',
    'tb_video_channels',
    'tb_channel_snapshots',
    'tb_operation_logs'
  )
order by table_name;
```

要求每张表 `auto_increment > max(id)`。

### 10.8 金丝雀策略

迁移验证分三步：

1. 小样本金丝雀：1-2 家门店，覆盖设计图、标注、录像机、通道、截图、AI JSON、`bed_label`、H5 Monitor。
2. 全量导入前复验：金丝雀门店在 MySQL 测试库跑完 API smoke 和页面验收。
3. 全量导入后复验：同一批金丝雀门店再次执行相同清单，确认迁移前后行为一致。

只有金丝雀通过，才能进入正式库镜像初始化。

## 11. 主会话验收点

建议主会话重点拍板：

1. 当前 14 张业务表作为测试库预演起点，但正式 DDL 必须补治理表。
2. 旧 designplan 三表短期保留，不继续扩展。
3. `tb_ezviz_accounts` 密文字段和 `tb_video_channels.area_note` 的 `text not null` 无默认必须修。
4. 测试库和正式库都要求连接/session 严格 `sql_mode`。
5. 统一时间口径，优先建议 MySQL `datetime(3)` 存 UTC。
6. 通道截图和设计图只迁 logical key，不迁二进制；未来通过 `tb_asset_objects` 接公司文件服务。
7. 正式 DDL 由 Git 中版本化 SQL 生成，测试库结构输出用于一致性校验，不作为正式库唯一来源。
8. 测试库分阶段管理：阶段 A 可以折腾，阶段 B 导入历史数据后必须按真实测试环境治理。
9. Supabase/PostgreSQL 历史数据必须保留并导入 MySQL 测试库，再由测试库镜像/搬运到正式库。
10. MySQL 对接完成标准必须包含接口路径、后台页面、H5 Monitor、图片代理、权限和数据对账验收，不以“表建好/数据导入完成”为完成标准。
11. 遇到需要产品/主会话/DBA/运维/安全确认的事项，先输出待确认清单并经主会话确认，不直接执行破坏性或方向性实现。
