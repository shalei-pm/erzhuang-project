# MySQL Stage A Preflight Checklist

最后更新：2026-07-01

用途：主会话/用户决定是否授权连接 MySQL 测试库试跑 Stage A。本文只包含确认清单和只读 SQL 草案，不包含 host、user、password、连接串，不代表已经允许执行。

## 1. 执行前 10 个确认问题

执行前必须全部有明确答案：

- [ ] 目标库已确认是 MySQL 测试库 / Stage A 沙箱，不是正式库。
- [ ] 已确认是否允许重建测试库，或是否允许执行 `db/mysql_stage_a_cleanup_sample_tb.sql`。
- [ ] 已确认当前测试库已有备份，或明确处于可重建、可丢弃状态。
- [ ] 已确认使用当前代码 commit 和当前 DDL 草案：`db/mysql_schema_tb.sql`、`db/mysql_business_schema_patch_tb.sql`、`db/mysql_governance_schema_tb.sql`。
- [ ] 已确认本轮只导入 Stage A seed：`db/mysql_stage_a_seed_sample_tb.sql`，不导入 Supabase/PostgreSQL 历史数据。
- [ ] 已确认第一阶段资产读取仍使用当前 `asset_store=supabase` 模式；图片/PDF/截图二进制不迁入 MySQL。
- [ ] 已确认 SSO 暂不作为 Stage A 首轮阻断项；先以 `SSO_ENABLED=false` 本地 admin 兼容态验业务流程。
- [ ] 已确认失败停止条件：P0 阻断出现即停止后续 DDL/DML，保留现场并回报主会话。
- [ ] 已确认执行记录保存位置，建议复用 `docs/mysql-stage-a-execution-plan.md` 中的“Stage A 执行记录模板”。
- [ ] 已确认只有 Stage A 通过后，才进入历史数据迁移和阶段 B 冻结/备份/回滚讨论。

## 2. 只读环境探针 SQL 草案

执行前只允许用只读账号或只读语句确认环境。不要把连接信息写入文档、脚本或日志。

```sql
select version() as mysql_version,
       database() as current_database,
       @@sql_mode as sql_mode,
       @@time_zone as time_zone,
       @@system_time_zone as system_time_zone,
       @@character_set_database as charset_database,
       @@collation_database as collation_database;
```

```sql
select table_name
from information_schema.tables
where table_schema = database()
order by table_name;
```

```sql
select table_name, table_rows
from information_schema.tables
where table_schema = database()
order by table_name;
```

如业务表已存在，可补充行数探针：

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

## 3. 一页版执行 Checklist

- [ ] 环境探针：确认 version、database、`sql_mode`、time_zone、字符集、表清单、已有行数。
- [ ] 应用业务 schema：执行 `db/mysql_schema_tb.sql`。
- [ ] 应用 business patch：执行 `db/mysql_business_schema_patch_tb.sql`。
- [ ] 应用 governance schema：执行 `db/mysql_governance_schema_tb.sql`。
- [ ] 执行 seed 策略：首次空库执行 seed；重复试跑先 cleanup + seed，或改幂等 migration，二选一需主会话确认。
- [ ] 启动 MySQL 模式后端：确认 `/health` 返回 `database=mysql`，`asset_store` 明确。
- [ ] API smoke：覆盖 `/health`、`/api/auth/me`、门店列表/详情、设计图、通道、截图代理、H5 Monitor API。
- [ ] 页面 smoke：覆盖门店列表、门店详情、设计图标注 Tab、通道映射 Tab、H5 Monitor 页面。
- [ ] validation SQL：执行 `docs/mysql-validation-sql.md` 的只读校验，记录摘要。
- [ ] 记录结论：是否通过 Stage A、阻断项、待确认项、是否允许进入阶段 B 讨论。

## 4. P0 阻断条件

出现任一项即停止后续动作：

- 目标库不确定，或存在误连正式库风险。
- 非空库但未确认可 cleanup / 重建。
- 任一 DDL 报错且无法解释。
- seed 出现 FK / 唯一键冲突且无法解释。
- `/health` 不是 `database=mysql`。
- 门店列表、门店详情、H5 Monitor 核心接口不可用。
- 权限/SSO 被误启用，导致现有业务流程被登录或权限拦截。
- 图片路径全量不可读，且 diagnostics 无法说明 `asset_store`、`snapshot_key`、错误原因。
- 发生可能污染历史数据的操作，例如误清真实门店、误导入历史数据、写入真实密钥或真实 token。

## 5. SSO 待补

当前只记录待补，不作为 Stage A 首轮阻断项。Stage A 首轮按 `SSO_ENABLED=false` 本地 admin 兼容态验证业务流程。

等安全/运维把 SSO 链路稳定后，补充：

- 真实企业邮箱用户 seed。
- disabled 用户拒绝登录。
- viewer 单机构 / 多机构 scope。
- operator 单机构 scope。
- 登录成功、登录拒绝、权限变更审计。
- 后端接口真实权限过滤验收。
