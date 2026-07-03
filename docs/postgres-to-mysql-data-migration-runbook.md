# Postgres 到 MySQL 真实数据迁移 Runbook

最后更新：2026-07-02

本文纠正一个关键边界：OSS Stage A 只证明了对象迁移链路可用，并没有把当前 Supabase/PostgreSQL 的真实业务数据迁入 MySQL。公司 MySQL 测试库后续要作为测试环境数据基座，因此必须先完成真实业务数据迁移和校验，再基于 MySQL 里的真实资产引用迁移对象到 OSS。

## 总体顺序

1. 冻结迁移窗口：确认运营暂时停写，记录代码 commit、Postgres 源库时间点、MySQL schema 版本。
2. Postgres 只读导出：用 `cmd/pg-to-mysql-export` 生成 MySQL 导入 SQL、auto increment 修复 SQL 和报告。
3. MySQL 小样本导入：优先用 `external_org_id=10030` 做金丝雀导入。
4. MySQL 只读校验：跑 `docs/mysql-validation-sql.md`，对行数、外键、JSON、自增、H5 金丝雀做检查。
5. MySQL 真实资产清单：在业务行导入后，再跑 `db/oss_asset_inventory_sql_tb.sql` 生成设计图和通道截图迁移清单。
6. OSS 对象迁移：按清单从 Supabase Storage 复制到 OSS，先 dry-run，再 apply。
7. 数据库资产状态回写：审核 `asset-migrate` 生成的 `result_sql` 后写入 MySQL。
8. 应用 MySQL runtime 适配：单独实现 Go MySQL repository，不能只换连接串。
9. 页面/API 验收：门店列表、详情、设计图、通道、H5 Monitor、截图代理全部验收。

## 导出工具

工具路径：

```sh
cmd/pg-to-mysql-export
```

本工具只读 Postgres，不写 MySQL。它输出：

```text
01-import.sql
02-auto-increment.sql
report.json
```

小样本导出示例，连接串必须放在环境变量里，不要写进命令或文件：

```sh
SOURCE_DATABASE_URL='postgres://...' ./.tools/go/bin/go run ./cmd/pg-to-mysql-export \
  --source-dsn-env SOURCE_DATABASE_URL \
  --external-org-id 10030 \
  --include-users=true \
  --batch-id canary-10030-YYYYMMDD \
  --out-dir /private/tmp/erzhuang-pg-mysql-canary-10030
```

全量导出时去掉 `--external-org-id`：

```sh
SOURCE_DATABASE_URL='postgres://...' ./.tools/go/bin/go run ./cmd/pg-to-mysql-export \
  --source-dsn-env SOURCE_DATABASE_URL \
  --include-users=true \
  --batch-id full-YYYYMMDD \
  --out-dir /private/tmp/erzhuang-pg-mysql-full
```

## 当前表映射

| Postgres 源表 | MySQL 目标表 | 说明 |
| --- | --- | --- |
| `tasks` | `tb_tasks` | 示例任务表 |
| `app_settings` | `tb_app_settings` | AI provider 等设置 |
| `design_plan_stores` | `tb_design_plan_stores` | 旧设计图识别表，保留兼容 |
| `design_plan_store_areas` | `tb_design_plan_store_areas` | 旧设计图区域表 |
| `design_plan_operation_logs` | `tb_design_plan_operation_logs` | 旧设计图日志 |
| `stores` | `tb_stores` | 新门店主表 |
| `store_areas` | `tb_store_areas` | 业务区域表 |
| `store_design_plans` | `tb_store_design_plans` | 新设计图表 |
| `design_plan_annotations` | `tb_design_plan_annotations` | 新设计图标注 |
| `ezviz_accounts` | `tb_ezviz_accounts` | 萤石账号配置 |
| `video_recorders` | `tb_video_recorders` | 录像机 |
| `video_channels` | `tb_video_channels` | 通道，包含 `bed_label` |
| `channel_snapshots` | `tb_channel_snapshots` | 通道截图路径，不含二进制 |
| `operation_logs` | `tb_operation_logs` | 操作日志 |
| `tb_users` | `tb_users` + `tb_user_roles` | 用户主表按当前公司测试库字段导入；Postgres `phone` 写入 MySQL `mobile`，Postgres `role` 转入 MySQL 角色关系 |

## 关键校验

导入前：

- Postgres 源表是否存在。
- `stores.external_org_id` 是否包含金丝雀 `10030`。
- Postgres 当前 `video_channels` 是否已经包含 `bed_label`。
- Postgres `tb_users` 是否包含 `role`，以便同步转入 MySQL `tb_user_roles`。
- MySQL `tb_users` 是否包含 `mobile`、`department`、`sso_subject` 等当前测试库字段；当前迁移不再要求 MySQL `tb_users.phone` 或 `tb_users.role` 必须存在。

## 受控金丝雀导入入口

公司环境提供受控 ops 入口：

```text
POST /erzhuang-project/api/admin/ops/mysql-canary-import
```

请求体：

```json
{
  "external_org_id": "10030",
  "import_sql": "...",
  "apply": false,
  "batch_id": "canary-10030-YYYYMMDD"
}
```

约束：

- 仅在 `OPS_ENABLED` / `K8S_SECRET_OPS_ENABLED` 开启时可用。
- 仅管理员可调用。
- 仅允许 `external_org_id=10030`。
- `import_sql` 必须包含 `-- Scope external_org_id: 10030`。
- `apply=false` 只连接 MySQL、检查必要表、返回当前校验摘要，不执行导入 SQL。
- `apply=true` 才在事务中执行导入 SQL，并返回校验摘要。
- 入口读取 `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN`，不要把 MySQL 密码放入仓库、文档或前端变量。

响应摘要包含：

- 门店数、录像机数、通道数、截图数、操作日志数、用户数。
- 外键孤儿数量。
- JSON 非法数量。

敏感数据注意：

- 导入 SQL 可能包含手机号、飞书 ID、通道截图 proxy path、模型识别原始文本。
- 不要把完整 SQL 粘贴到聊天、文档或 Git。
- 仅在本机临时目录、浏览器下载文件、受控 ops 请求体中短期使用。

导入后：

- 执行 `docs/mysql-validation-sql.md`。
- 所有外键孤儿数量必须为 0。
- JSON 字段 `json_valid` 必须为 1 或 NULL。
- 各表 `auto_increment` 必须大于当前 `max(id)`。
- 金丝雀 `external_org_id=10030`、录像机 `GN0941203` 可查。
- `tb_asset_objects` 清单必须基于真实业务行生成，不能再使用 Stage A 假数据代表历史资产。

## OSS 迁移顺序

OSS 对象迁移必须在 MySQL 业务行导入后进行，理由：

- 资产归属依赖 `tb_stores`、`tb_store_design_plans`、`tb_video_channels` 等真实主键。
- 目标 OSS key 需要 `external_org_id`、`recorder_id`、`channel_id` 等真实关系。
- 需要先知道哪些截图路径已经在业务数据中存在，哪些被用户手工删除或历史临时 URL 过期。
- `tb_asset_objects` 的 `owner_entity_type` / `owner_entity_id` 需要引用已导入的业务对象。

第一批建议迁设计图文件，第二批再迁通道截图。通道截图属于敏感信息，源对象不存在时应标记 failed/skipped，不要用假图替代真实历史图。

## 未完成事项

- Go 后端 MySQL repository 尚未实现，当前生产代码仍是 PostgreSQL 方言。
- DBA 专项已复核并指出：第一版应用仍可保留 Postgres `tb_users.role` 单字段逻辑；本次 MySQL 测试库迁移按当前治理表结构写入 `tb_user_roles`，后续 MySQL repository 适配时再统一权限读取口径。
- 需要确认真实 Postgres 源连接是在本地、公司 Pod，还是只能通过运行时环境变量读取。
- MySQL 小样本导入前，需要确认是否允许清理 Stage A 假数据，或重建测试库 schema。
