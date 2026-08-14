# MySQL Stage A Readiness Report

最后更新：2026-07-02

本文是 DBA 对 Stage A 可执行交付物的复核结论。当前未连接数据库、未执行 SQL、未提交、未 push、未发布。

## 1. 结论

在主会话已确认目标库是空的 MySQL 测试库 / Stage A 沙箱，且允许重建或 cleanup 的前提下，当前 Stage A 链路可以进入试跑授权评审。

可试跑范围：

- `db/mysql_business_schema_patch_tb.sql`
- `db/mysql_governance_schema_tb.sql`
- `db/mysql_stage_a_seed_sample_tb.sql`
- `docs/mysql-validation-sql.md` 中的只读校验 SQL

重要边界：

- `db/mysql_business_schema_patch_tb.sql` 只适合当前空 14 表 Stage A 首次试跑，不是幂等 migration；重复试跑需先重建 schema，或改成 information_schema guard。
- `db/mysql_governance_schema_tb.sql` 和 Stage A seed 中的 RBAC 表仅用于 MySQL 治理模型预演；当前用户管理第一版已确认采用 `tb_users.role` 单字段，不代表第一版生产权限实现要立刻切到 `tb_roles/tb_user_roles`。
- Stage A 不导入 Supabase/PostgreSQL 历史数据。

## 2. 可执行顺序

1. Preflight：按 `docs/mysql-stage-a-preflight-checklist.md` 确认 10 个问题。
2. 环境只读探针：确认 database、version、`sql_mode`、time_zone、字符集、表清单、行数。
3. 应用 schema patch：执行 `db/mysql_business_schema_patch_tb.sql`。
4. 应用 governance schema：执行 `db/mysql_governance_schema_tb.sql`。
5. 导入 Stage A 样本：执行 `db/mysql_stage_a_seed_sample_tb.sql`。
6. 启动 MySQL 模式后端。
7. API smoke：`/health`、`/api/auth/me`、门店列表/详情、设计图、通道映射、截图代理、H5 Monitor API。
8. 页面 smoke：门店列表、门店详情、设计图标注 Tab、通道映射 Tab、H5 Monitor 页面。
9. 只读校验：执行 `docs/mysql-validation-sql.md` 的相关 SQL。
10. 记录结论：是否允许进入 Stage B 历史数据迁移讨论。

重复试跑：

- 当前推荐：重建空 schema 后按上述顺序重跑。
- 如不重建，必须先执行 `db/mysql_stage_a_cleanup_sample_tb.sql`，再执行 seed；cleanup 禁止在 Stage B 历史数据导入后使用。

## 3. 当前阻断项

当前没有发现阻断 Stage A 空库首次试跑的 SQL 问题。

非阻断风险：

- MySQL 8.0.13 不强制依赖 CHECK，枚举和状态仍需应用层校验。
- 当前 `@@sql_mode=IGNORE_SPACE`，写入脚本已设置严格 session `sql_mode`；执行前要记录实际 session 是否生效。
- `idx_tb_channel_snapshots_channel_id` 与 `idx_tb_channel_snapshots_latest` 均以 `channel_id, created_at` 开头，存在冗余但不会因索引名重复而报错。Stage A 先保留，后续正式 DDL 再收敛。
- governance RBAC 模型与第一版 Postgres 用户管理单字段 role 不同，需在主会话层明确“不作为第一版权限落地模型”。

## 4. 执行前需确认

- 目标库是 `db_pm_erzhuang` 测试库 / Stage A 沙箱，不是正式库。
- 当前库 14 张基础表是可重建或可 cleanup 状态。
- 本轮只执行 Stage A seed，不导历史数据。
- 第一阶段资产读取仍走 Supabase Storage / 当前 AssetStore，不迁图片/PDF/截图二进制。
- `SSO_ENABLED=false` 兼容态先验业务流程；SSO 真实用户、角色、scope、session、audit 对接暂不阻断。
- 若任一步 DDL/DML 报错，停止后续动作，保留现场输出错误。

## 5. 执行后只读校验清单

优先执行：

- 环境探针：version、database、`sql_mode`、time_zone、charset/collation。
- 表清单和索引清单。
- 9 张核心业务表行数、最小/最大 ID。
- 外键孤儿：门店区域、设计图、标注、录像机、通道、截图。
- JSON 合法性：设计图和通道 `recognition_result`。
- `auto_increment > max(id)`。
- 金丝雀：`external_org_id=10030`、`device_code=GN0941203`、通道 1。
- 截图路径：`/api/store-space/channel-snapshots/...`、`snapshot_key`、`snapshot_key_hash`。
- governance seed：角色、权限点、角色权限、Stage A 用户和 scope。
- 日志：`tb_operation_logs`、`tb_audit_logs`、`tb_asset_access_logs` 样本可查。

## 6. 不得执行事项

- 不连接正式库。
- 不导入 Supabase/PostgreSQL 历史数据。
- 不在 Stage B 后执行 cleanup。
- 不把 Stage A RBAC seed 当作第一版线上用户管理模型。
- 不写入真实密码、连接串、access token、app secret、SSO token、设备验证码。
- 不执行注释中的 snapshot backfill/update 草案。
- 不提交、不 push、不发布。

## 7. 待确认清单

| 事项 | 推荐 | 需要确认方 | 确认后下一步 |
| --- | --- | --- | --- |
| 重复试跑策略 | 优先重建空 schema；不重建时 cleanup + seed | 主会话、用户 | 决定执行前是否先 cleanup |
| governance RBAC 是否参与 Stage A 后端验收 | 仅数据库预演，不阻断第一版单字段 role | 主会话 | smoke 只验业务链路，RBAC 只做 SQL 校验 |
| 索引冗余处理 | Stage A 保留；正式 DDL 前收敛 | DBA、主会话 | 根据查询计划决定是否删除旧索引 |
| cleanup 使用窗口 | 仅 Stage A 历史数据导入前 | 主会话、用户 | 导入历史数据后冻结 cleanup |
