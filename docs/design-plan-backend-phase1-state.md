# 设计图标记后端 Phase 1 状态

最后更新：2026-06-05

## 实现范围

本分支 `codex/design-plan-backend-phase1` 只实现后端 Phase 1：数据模型和 CRUD。

已完成：

- 新增 PostgreSQL schema：`db/design_plan_schema.sql`。
- 新增 Go 包：`internal/designplan/`。
- 新增门店、区域、操作日志领域模型。
- 新增内存 repository，用于本地没有 `DATABASE_URL` 时练习和测试。
- 新增 PostgreSQL repository，复用现有 `DATABASE_URL` 和 `database/sql` + pgx stdlib 连接方式。
- 新增 `/api/design-plan` 路由：
  - `GET /api/design-plan/stores?q=&page=1&page_size=20`
  - `GET /api/design-plan/stores/{id}`
  - `POST /api/design-plan/stores`
  - `PUT /api/design-plan/stores/{id}`
  - `DELETE /api/design-plan/stores/{id}`
  - `POST /api/design-plan/stores/check-duplicate`
- 新增保存校验：
  - 门店名必填。
  - 至少 1 个区域。
  - 区域名称、类型、框必填。
  - 治疗室/面诊室编号必填。
  - 编号只能是数字。
  - 同门店同类型编号唯一。
- 新增门店名称标准化和重复检查：
  - 完全同名。
  - 简单模糊候选。
  - 编辑时支持 `exclude_store_id` 排除自身。
- 新增门店级操作日志写入：
  - `create`
  - `update`
  - `delete`
  - `replace`

未实现，留给后续 Phase：

- PDF 上传。
- PDF 转图片。
- 图片/缩略图读取接口。
- AI 识别。
- OpenAI 调用。
- 服务器配置、nginx、systemd、部署脚本。
- 前端 UI。

## 数据库表

新增表：

- `design_plan_stores`
- `design_plan_store_areas`
- `design_plan_operation_logs`

说明：

- schema 文件用于人工查看和未来迁移整理。
- 当前服务启动时也会通过 `designplan.EnsurePostgresSchema` 自动创建这些表，延续现有 tasks 的初始化方式。
- `design_plan_store_areas` 使用比例坐标，范围为 0 到 1。
- 同门店同类型编号唯一约束由唯一索引兜底。

## 本地降级策略

没有 `DATABASE_URL` 时：

- `/health` 和 `/api/tasks` 继续使用原有 memory store。
- `/api/design-plan/*` 使用新的 designplan memory store。
- 这让前端 Phase 2 可以先在无数据库环境下联调 CRUD。

有 `DATABASE_URL` 时：

- `/api/tasks` 使用原有 PostgreSQL store。
- `/api/design-plan/*` 使用 designplan PostgreSQL store。
- 启动时自动确保 tasks 和 design plan schema 存在。

## 验证结果

已执行：

```sh
GOCACHE=/Users/sylar/.codex/worktrees/e6f9/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/gofmt -w cmd/server/main.go internal/app/handler.go internal/app/postgres_store.go internal/designplan/*.go
GOCACHE=/Users/sylar/.codex/worktrees/e6f9/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
GOCACHE=/Users/sylar/.codex/worktrees/e6f9/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/designplan
GOCACHE=/Users/sylar/.codex/worktrees/e6f9/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/app
```

结果：

- `gofmt` 成功。
- `go build ./...` 成功。
- `go test -c` 编译 `internal/designplan` 和 `internal/app` 测试包成功。

已知本地限制：

- 当前 macOS 环境执行 `go test ./...` 时，测试二进制运行阶段触发 `dyld: missing LC_UUID load command`。
- 这是本项目文档中已记录过的本地 Go 工具链限制；Linux 服务器最终仍应执行 `go test ./...`。

## 风险和后续建议

- 当前 Postgres repository 没有接真实数据库跑集成测试，建议主会话合并前或服务器发布前用 Supabase `DATABASE_URL` 跑一次 `go test ./...` 和手工 CRUD。
- 图片 URL 现在只是预留路径，Phase 3 需要补真实图片读取接口。
- `PUT` 当前采用整批替换区域的方式，简单可靠，适合第一版；如果后续需要区域级审计，可以新增区域级日志。
