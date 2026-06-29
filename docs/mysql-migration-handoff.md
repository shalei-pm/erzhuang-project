# MySQL 迁移交接说明

最后更新：2026-06-29

本文用于交接给后续负责 `erzhuang-project` MySQL 适配和数据迁移的同事/Codex 会话。

## 当前状态

- 测试 MySQL 库已创建 14 张 `tb_` 前缀表。
- MySQL DDL 文件保存在 `db/mysql_schema_tb.sql`。
- 当前 Go 后端代码仍只支持 PostgreSQL/Supabase，不支持直接切 MySQL。
- 当前图片、PDF、通道截图仍在 Supabase Storage，暂不迁入 MySQL。
- MySQL 测试库只建了表结构，尚未导入 Supabase 业务数据。

## 已创建的 MySQL 表

```text
tb_app_settings
tb_channel_snapshots
tb_design_plan_annotations
tb_design_plan_operation_logs
tb_design_plan_store_areas
tb_design_plan_stores
tb_ezviz_accounts
tb_operation_logs
tb_store_areas
tb_store_design_plans
tb_stores
tb_tasks
tb_video_channels
tb_video_recorders
```

表结构来源：

- `db/mysql_schema_tb.sql`

注意：仓库内不保存数据库密码。连接信息和密码应由运维通过安全方式提供，并在公司环境通过 K8s Secret 注入。

## 推荐迁移策略

本次迁移建议拆成两个阶段，不要同时迁数据库和图片。

### 第一阶段：数据库迁 MySQL，图片继续 Supabase

目标：

- 少量样本数据先导入测试 MySQL。
- 改造后端支持 MySQL 读写。
- 图片/PDF/截图继续从 Supabase Storage 读取。
- 跑通页面和关键业务流程后，再做全量数据库迁移。

优势：

- 先解决数据库兼容问题。
- 不把图片迁移和数据库迁移耦合在一起。
- 如果 MySQL 版本有问题，可以快速切回 PostgreSQL/Supabase。

### 第二阶段：图片迁公司文件服务或 Go RPC 上传服务

目标：

- 等 MySQL 版本稳定后，再迁移 Supabase Storage 里的文件。
- 优先保留现有 logical key，减少数据库字段改动。

当前图片 key 主要有两类：

```text
uploads/{upload_id}/original.pdf
uploads/{upload_id}/preview.png
uploads/{upload_id}/thumbnail.png
channel-snapshots/{snapshot_name}.jpg
```

前端通道截图访问形式：

```text
/api/store-space/channel-snapshots/{snapshot_name}.jpg
```

这个 URL 可以保持不变，由 Go 后端内部切换读取来源。

## 后端代码改造范围

当前代码是 PostgreSQL 实现，不能只换连接串。

需要重点改造：

- 新增 MySQL 驱动，例如 `github.com/go-sql-driver/mysql`。
- 增加 MySQL 连接配置，不要复用 PostgreSQL DSN 语义。
- 新增 MySQL schema 初始化逻辑，表名使用 `tb_` 前缀。
- 新增或抽象 MySQL repository。
- 全量替换 PostgreSQL 专属 SQL 写法：
  - `$1` 占位符改为 `?`。
  - `bigserial` 改为 `bigint auto_increment`。
  - `jsonb` 改为 `json`。
  - `timestamptz` 改为 `datetime(3)` 或公司规范时间类型。
  - `on conflict` 改为 `insert ignore` 或 `on duplicate key update`。
  - `returning` 改为 `last_insert_id()` 或插入后查询。
  - `::jsonb`、`::text` 等 PostgreSQL cast 需要删除或改写。
  - CTE 写入、`returning` 组合语句需要单独重写。
- 所有业务表名从无前缀改为 `tb_` 前缀，例如：
  - `stores` -> `tb_stores`
  - `video_channels` -> `tb_video_channels`
  - `channel_snapshots` -> `tb_channel_snapshots`

关键代码位置：

```text
cmd/server/main.go
internal/app/postgres_store.go
internal/designplan/store.go
internal/designplan/schema.go
internal/storespace/store.go
internal/storespace/schema.go
internal/storespace/h5_monitor_repository.go
internal/assets/store.go
internal/assets/supabase.go
```

## 图片暂留 Supabase 的要求

如果第一阶段图片继续读 Supabase，公司测试/线上环境必须继续配置：

```text
ASSET_STORE=supabase
SUPABASE_URL
SUPABASE_SERVICE_ROLE_KEY
SUPABASE_STORAGE_BUCKET
```

并确认公司 K8s 环境能访问 Supabase Storage。

如果公司环境无法访问 Supabase，则必须先完成图片迁移或搭建图片代理，否则页面设计图和通道截图会挂。

## 样本数据迁移建议

先选 1-2 家门店导入 MySQL，样本必须覆盖：

- 有设计图的门店。
- 有设计图标注框。
- 有录像机。
- 有摄像头通道。
- 有通道截图。
- 有 AI 识别结果。
- 有操作日志更好。

样本导入至少涉及这些表：

```text
tb_stores
tb_store_areas
tb_store_design_plans
tb_design_plan_annotations
tb_ezviz_accounts
tb_video_recorders
tb_video_channels
tb_channel_snapshots
tb_operation_logs
```

旧设计图表如仍有历史接口或迁移逻辑依赖，也需要导入：

```text
tb_design_plan_stores
tb_design_plan_store_areas
tb_design_plan_operation_logs
```

基础配置表：

```text
tb_tasks
tb_app_settings
```

## 数据迁移注意事项

- 迁移时尽量保留原始 `id`，不要重生成主键。
- 外键字段必须保持一致。
- 导入后检查每张自增表的 `auto_increment` 大于当前最大 `id`。
- `recognition_result` 等 JSON 字段必须是合法 JSON。
- 时间字段统一处理时区，避免比原数据差 8 小时。
- 图片路径字段先不要改，继续保留原 logical key 或现有 API URL。
- 正式迁移前需要短暂停写窗口，避免 Supabase 和 MySQL 数据不一致。
- 切换后 Supabase 保留只读备份一段时间，作为回滚来源。

## 功能验证清单

小样本跑通后至少验证：

- `/health` 正常返回。
- 门店列表能打开。
- 门店详情能打开。
- 设计图预览能显示。
- 设计图标注框能显示。
- 通道列表能打开。
- 通道截图能显示。
- 新增门店能写入 MySQL。
- 编辑门店基础信息能写入 MySQL。
- 保存设计图信息能写入 MySQL。
- 添加录像机能写入 MySQL。
- 扫描/刷新通道能写入 MySQL。
- 通道识别结果能写入 MySQL。
- 删除门店/录像机/通道的级联关系符合预期。
- Excel 导出能正常读取截图。

数据校验至少包括：

- 14 张表行数。
- 核心表最大 `id`。
- 外键孤儿数据检查。
- 抽样 5 家门店。
- 抽样 5 张设计图。
- 抽样 5 张通道截图。

## 后续图片迁移方案

如果公司 Go RPC 上传服务支持自定义 key，优先保持原 key：

```text
channel-snapshots/{snapshot_name}.jpg
uploads/{upload_id}/preview.png
```

这样数据库字段几乎不用改。

如果 Go RPC 上传服务只返回 `file_id`，建议新增映射表：

```text
tb_asset_objects
- id
- logical_key
- file_id
- content_type
- size_bytes
- created_at
- updated_at
```

后端读取图片时：

1. 通过 logical key 查 `tb_asset_objects.file_id`。
2. 调公司 RPC 下载。
3. 如果没有映射，回退 Supabase Storage。
4. 回退成功后可顺手上传到公司 RPC，并写入映射。

更稳的过渡方式是双读：

```text
先读公司 RPC
读不到再读 Supabase Storage
读到后异步或顺手回填公司 RPC
```

这样不用一次性迁完所有图片，用户打开过的图片会逐步迁过去，后续再跑批处理补齐冷数据。

## 当前结论

推荐下一步：

1. 保留 `db/mysql_schema_tb.sql`。
2. 让 Codex 开始做 MySQL 代码适配。
3. 写样本数据迁移脚本，只导 1-2 家门店。
4. 图片继续读 Supabase。
5. 测试环境跑通完整流程后，再安排 Supabase PostgreSQL 到线上 MySQL 的全量迁移。
6. 图片迁移单独排后续阶段。
