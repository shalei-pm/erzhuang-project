# 门店空间资源后端基础阶段状态

最后更新：2026-06-11

分支：`codex/store-space-backend-foundation`

## 实现范围

- 新增 `internal/storespace` 基础包：
  - 门店、区域主数据、设计图、萤石云账号、录像机、通道、截图的后端模型。
  - `handler -> service -> repository` 分层。
  - memory repository，便于本地无数据库练习。
  - postgres repository，覆盖门店列表、详情、创建、删除、重复检查、录像机编码唯一检查、区域查找或创建。
- 新增 `/api/store-space/*` 基础 API：
  - `GET /api/store-space/stores`
  - `GET /api/store-space/stores/{id}`
  - `POST /api/store-space/stores`
  - `DELETE /api/store-space/stores/{id}`
  - `POST /api/store-space/stores/check-duplicate`
  - `POST /api/store-space/recorders/{recorder_id}/scan-channels`
  - `POST /api/store-space/recorders/{recorder_id}/recognize-channels`
- 录像机扫描和识别暂不接真实萤石云/API，返回稳定 `501 not implemented` 合同。
- 新增 PostgreSQL schema 初始化：
  - `stores`
  - `store_areas`
  - `store_design_plans`
  - `design_plan_annotations`
  - `ezviz_accounts`
  - `video_recorders`
  - `video_channels`
  - `channel_snapshots`
  - `operation_logs`
- 所有新增 public 表均开启 RLS，并创建 `*_no_client_access` policy：
  - `for all to anon, authenticated`
  - `using (false)`
  - `with check (false)`
- 总入口接入：
  - `cmd/server/main.go`
  - `internal/app/handler.go`
  - `internal/app/postgres_store.go`

## 业务规则覆盖

- 添加门店：
  - 门店名称必填。
  - `design_plan_upload_id` 和有效录像机设备编码至少提供一个。
  - 录像机最多 3 台。
  - 同一创建请求内设备编码不能重复。
  - 全系统设备编码唯一。
  - 同名门店创建被拦截。
- 区域主数据：
  - 按 `store_id + area_type + area_number` 查找或创建。
  - 三类区域编号均必填。
  - 编号必须为正整数。
  - 同门店、同类型、同编号唯一。
  - 来源不同时，已存在区域来源升级为 `multiple`。

## 改动文件

- `cmd/server/main.go`
- `internal/app/handler.go`
- `internal/app/postgres_store.go`
- `internal/storespace/*`
- `db/store_space_schema.sql`
- `docs/store-space-backend-foundation-state.md`

## 验证结果

本地可用 Go 路径：

```sh
/Users/sylar/erzhuang-project/.tools/go/bin/go
```

已执行：

```sh
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/storespace
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/app
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
```

结果：均通过。

尝试执行：

```sh
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test ./internal/storespace
```

结果：命中文档已记录的 macOS `missing LC_UUID load command` 问题，测试二进制编译成功但本机运行阶段被 dyld 拦截。最终完整 `go test ./...` 仍需主会话在服务器 Linux 环境执行。

## 风险和限制

- 本阶段只完成基础模型/API，不接真实萤石云，也不做 AI 识别。
- `design_plan_upload_id` 目前仅作为新 `store_design_plans.upload_id` 记录，尚未和现有 `internal/designplan` 上传文件元数据做适配迁移。
- `ezviz_accounts` schema 已预留密文字段，但本阶段没有账号管理 API，也没有加密组件。
- 旧 `design_plan_*` 表未迁移，新表与旧设计图模块并行存在；后续需要专门做设计图适配阶段。
- Postgres schema 使用 `create table if not exists` 和增量式索引/policy，尚未引入正式迁移工具。

## 给主会话的 review 重点

- API 路径和 JSON 字段是否满足前端 Phase 2 使用。
- `store_design_plans.upload_id` 是否足够支撑后续复用现有上传链路，或是否应在本阶段同步更多文件字段。
- 录像机创建是否允许 `ezviz_account_id` 暂为空；当前为支持“先录设备编码，后配置账号”的练习链路，数据库允许为空。
- `operation_logs` 是否应长期复用旧 `design_plan_operation_logs`，还是保持新系统独立日志表。
