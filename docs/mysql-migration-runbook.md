# MySQL 迁移 Runbook

最后更新：2026-07-01

本文是 MySQL 第一阶段交付包的执行说明草案。它只描述流程和门禁，不包含密码、连接串或真实密钥。任何连接测试库、修改 schema、导入历史数据、发布或推送都必须由主会话确认后执行。

## 1. 适用范围

覆盖：

- 阶段 A：MySQL 测试库 schema 折腾期。
- 阶段 B：Supabase/PostgreSQL 历史数据导入 MySQL 测试库后。
- MySQL 测试库到公司正式库的结构和数据初始化/镜像搬运。

不覆盖：

- 具体 MySQL 密码。
- 公司正式库直连操作。
- 生产发布执行。

## 2. 前置输入

必须基于这些文件：

- `db/mysql_schema_tb.sql`
- `db/mysql_business_schema_patch_tb.sql`
- `db/mysql_governance_schema_tb.sql`
- `docs/mysql-migration-acceptance-cases.md`
- `docs/mysql-validation-sql.md`
- `docs/mysql-dba-gap-checklist.md`

## 3. 阶段 A：Schema 折腾期

### 3.1 目标

- 修订并验证业务表 DDL。
- 验证治理表 DDL。
- 准备假数据、金丝雀数据、API smoke、页面 smoke 和数据对账 SQL。
- 跑通 MySQL repository 和关键前端流程。

### 3.2 允许动作

- 修改、重建测试库表结构。
- 插入假门店、假录像机、假通道、假权限用户。
- 反复执行 schema patch 和清理脚本。

### 3.3 禁止动作

- 导入 Supabase/PostgreSQL 全量历史数据。
- 把阶段 A 假数据当作正式数据来源。
- 把测试库当前结构直接交给运维作为正式 DDL。
- 写入真实密钥。

### 3.4 阶段 A 执行顺序

1. 主会话确认阶段 A 可以开始。
2. 记录当前代码 commit、DDL 文件版本和测试库状态。
3. 应用业务 schema 和 patch 草案到测试库。
4. 应用治理 schema 草案到测试库。
5. 准备假数据和金丝雀数据。
6. 启动 MySQL 模式后端。
7. 运行 P0 API smoke。
8. 完成页面 smoke。
9. 执行 `docs/mysql-validation-sql.md` 中的只读校验 SQL。
10. 记录失败项和待确认项。

### 3.5 阶段 A 出口门禁

必须满足：

- `/health` 返回 `database=mysql`。
- 金丝雀 `externalOrgId=10030`、录像机 `GN0941203` 的 H5 Monitor 链路可用或有可解释萤石错误。
- 门店列表、详情、设计图、通道、截图代理可用。
- P0 权限用例通过，或主会话明确确认权限模块暂未启用且不阻断阶段 A。
- 数据对账无外键孤儿、非法 JSON、自增风险。
- 备份和恢复演练完成。

未满足以上门禁时，不进入历史数据导入。发现 schema、权限、资产、H5 Monitor 或 API 兼容性方向性问题时，先输出待确认清单，不直接重建、清表或修订历史数据。

## 4. 阶段 B：历史数据导入前冻结

### 4.1 必须冻结

- schema 文件版本。
- migration/patch 文件版本。
- 代码 commit。
- 测试库当前结构。
- 历史数据导入脚本版本。

建议打 tag，例如：

```text
mysql-schema-baseline-YYYYMMDD
```

如不打 tag，必须在 `docs/codex-learning-state.md` 记录 commit 和 DDL 文件。

### 4.2 清理或重建测试库

可选方案：

- 方案 A：重建测试库 schema 后导入历史数据。
- 方案 B：用清理脚本删除/隔离阶段 A 假数据。

推荐：

- 如果阶段 A 已多轮手工试验，优先重建 schema。
- 如果不能重建，假数据必须有 `canary_` 标识并有清理脚本。

需要主会话/用户确认后才能执行。

### 4.3 导入前备份

历史数据导入前必须备份：

- 测试库当前 schema。
- 测试库阶段 A 可复盘数据。
- Supabase/PostgreSQL 源库 baseline 报告。

备份记录至少包含：

- 生成时间。
- schema 版本。
- 代码 commit。
- 执行人。
- 恢复命令或恢复说明。

## 5. Supabase/PostgreSQL 到 MySQL 测试库

### 5.1 停写窗口

导入历史数据前，由主会话与用户确认：

- 停写开始时间。
- 停写结束条件。
- 受影响页面和接口。
- 失败回滚方式。

### 5.2 导入顺序

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
11. 权限、session、审计、资产对象相关表，如果当时已启用。

### 5.3 导入规则

- 保留原始 `id`。
- JSON 空字符串转 `NULL`。
- 时间字段按冻结口径显式转换。
- 图片/PDF/截图只迁 logical key，不迁二进制。
- 导入脚本设置严格 session `sql_mode`。
- 导入后校准 `auto_increment`。

时间字段推荐口径：

- PostgreSQL `timestamptz` 导出时统一转为北京时间 `Asia/Shanghai` 的无时区字符串，再写入 MySQL `datetime(3)`。
- 导出报告记录源库时区、目标库 `@@time_zone`、样本 `created_at/updated_at/confirmed_at` 对账结果。
- 若主会话决定改用 UTC 口径，必须同步修改 Go repository、页面展示和验收 SQL，不允许混用。

可不迁或可重建的数据：

- 可重建：通道截图二进制、短期播放地址、临时 AI 诊断日志、阶段 A 假数据。
- 必须保留：门店、`external_org_id`、设计图路径、标注、录像机、通道确认状态、`bed_label`、`recognition_result`、操作日志、授权用户和机构范围。

### 5.4 导入后校验

执行 `docs/mysql-validation-sql.md`：

- 行数和最大 ID。
- 外键孤儿。
- JSON 合法性。
- `auto_increment`。
- 重复 `external_org_id`。
- 截图路径。
- 权限范围。
- AI 失败原因。
- 日志可查性。

### 5.5 失败处理

任何 P0 校验失败：

1. 停止后续导入。
2. 保留失败现场。
3. 记录失败原因。
4. 由主会话决定回滚到导入前备份，或修复脚本后继续。

禁止直接手工改历史数据。

## 6. MySQL 测试库到公司正式库

### 6.1 交接前确认

必须由主会话确认：

- 测试库已进入阶段 B 并通过验收。
- schema 版本和数据快照时间已记录。
- 正式库初始化方式已由运维确认。
- 正式库 MySQL 版本、字符集、时区、`sql_mode` 已确认。
- 回滚方案已确认。

### 6.2 运维交接包

交接包应包含：

- 业务 DDL。
- 治理 DDL。
- patch/migration 顺序。
- seed roles/permissions。
- 数据快照时间。
- 导入步骤。
- 校验 SQL。
- 回滚方案。
- 环境变量清单。
- K8s Secret 注入清单。
- 已知风险和待确认项。
- 联系人。

交接包禁止包含真实密钥。

### 6.3 正式初始化后校验

运维初始化后回传：

- 表结构检查结果。
- 行数报告。
- 关键导入日志。
- 错误/跳过记录。

DBA 专项对照：

- `information_schema` 结构。
- baseline 行数。
- 金丝雀门店。
- P0 API smoke。
- 页面 smoke。

## 7. 需要确认后才能执行的事项

| 事项 | 需要确认方 | 未确认前禁止 |
| --- | --- | --- |
| 阶段 A 是否可重建测试库 | 主会话、用户 | 重建、清表 |
| 历史数据导入时间 | 主会话、用户 | 导入历史数据 |
| SSO payload 字段 | 安全/SSO、主会话 | 固化登录映射 |
| 正式库初始化方式 | 运维、主会话 | 输出最终交接包 |
| 公司文件服务 key 能力 | 运维/文件服务负责人、主会话 | 固化资产迁移实现 |
| H5 多 Pod 并发方案 | 研发/运维、主会话 | 固化并发租约设计 |
| 软删除是否落地 | 产品负责人、主会话 | 替换删除语义 |
| 时间字段采用北京时间还是 UTC | 主会话、研发、运维 | 写历史导入脚本 |
| `tb_asset_objects` 第一阶段是否落库 | 主会话、运维/文件服务负责人 | 固化资产映射实现 |
| `tb_user_store_scopes.scope_key` 生成规则 | 主会话、研发 | 导入权限样本和写后端校验 |

## 8. 当前禁止事项

- 不连接正式库。
- 不导入历史数据。
- 不发布公司环境。
- 不把测试库阶段 A 结构直接当正式结构。
- 不把密码或密钥写入仓库。
