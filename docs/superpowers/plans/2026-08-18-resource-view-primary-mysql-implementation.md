# Resource View Primary MySQL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 3.0 门店空间资源查看复用二壮主 MySQL，并以真实业务关系正确识别摄像头绑定与异常。

**Architecture:** `cmd/server/main.go` 只创建项目主数据库连接；当主库可用时，将同一 `*sql.DB` 注入 `resourceview.NewMySQLRepository`。repository 只加载摄像头关系或摄像头用途的孤儿关系，service 沿用现有空间树、设备树和异常聚合。

**Tech Stack:** Go 1.22、`database/sql`、MySQL 8、React/TypeScript 既有 3.0 前端。

---

### Task 1: 锁定主库接线与摄像头关系规则

**Files:**
- Modify: `cmd/server/main_test.go`
- Modify: `internal/resourceview/mysql_repository_test.go`
- Modify: `internal/resourceview/service_test.go`

- [x] **Step 1: 先写失败测试，防止独立业务 DSN 回归**

在 `cmd/server/main_test.go` 加入源码守卫：读取 `main.go`，断言不包含 `BUSINESS_MYSQL_DSN`、`K8S_SECRET_BUSINESS_MYSQL_DSN`、`businessDatabaseConfigFromEnv`，并包含 `resourceview.NewMySQLRepository(db)`。

运行：

```bash
go test ./cmd/server -run TestResourceViewUsesPrimaryMySQLOnly
```

预期：改代码前失败。

- [x] **Step 2: 写摄像头用途关系的行为测试**

在 `internal/resourceview/service_test.go` 建立一个空间，附带 camera、pad、tv、gateway 四类设备关系，并加入一条 `security_camera` 的缺失设备关系。断言：

```go
// 只有已存在的 camera 形成绑定；pad/tv/gateway 不产生 missing_camera。
assertIssueCount(t, detail.Issues, IssueMissingCamera, 1)
if detail.Summary.BoundCameraCount != 1 { t.Fatalf("...") }
```

运行：

```bash
go test ./internal/resourceview -run TestBuildStoreDetailIgnoresNonCameraRelations
```

预期：现有实现失败，因为它把非 camera 关系当作缺失摄像头。

### Task 2: 移除第二数据库连接并过滤非摄像头关系

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/resourceview/mysql_repository.go`

- [x] **Step 1: 在主库初始化分支中创建资源查看服务**

删除 `main` 开头的独立业务数据库打开逻辑和 `businessDatabaseConfigFromEnv`。主库 `db` 成功建立后加入：

```go
resourceViewService = resourceview.NewService(resourceview.NewMySQLRepository(db))
log.Print("resource view enabled with primary mysql")
```

保持内存模式的 `resourceViewService=nil`，使未配置主库时资源查看 API 仍返回既有未配置错误。

- [x] **Step 2: 将关系读取限定为摄像头语义**

将 `listRelations` 改成关联 `tb_crm_iot_device`，只返回：

```sql
(d.category = 'camera' or (d.id is null and r.function_type like '%camera'))
```

其中设备已缺失时仍以 `function_type like '%camera'` 留下摄像头用途孤儿关系；已存在的 pad、电视、网关关系不会传入摄像头聚合。

- [x] **Step 3: 运行聚焦测试**

```bash
go test ./cmd/server -run TestResourceViewUsesPrimaryMySQLOnly
go test ./internal/resourceview -run 'TestBuildStoreDetail(IgnoresNonCameraRelations|ReportsMappingIssues)'
```

预期：全部通过。

### Task 3: 回归、文档与测试环境发布准备

**Files:**
- Modify: `README.md`
- Modify: `docs/deploy-runbook.md`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [x] **Step 1: 更新运行说明**

移除 README 和发布手册中“3.0 使用独立业务库 DSN”的表述，明确 3.0 读取项目主库中的同步表，且不需要保留 `K8S_SECRET_BUSINESS_MYSQL_DSN`。

- [x] **Step 2: 运行完整门禁**

```bash
go test ./internal/resourceview
go test ./internal/app
go test ./cmd/server
go build -o /private/tmp/erzhuang-server-check ./cmd/server
cd frontend && npm test && npm run build
git diff --check
```

预期：测试和构建通过；前端仅允许既有 Vite chunk-size warning。

- [ ] **Step 3: 发布测试环境并验收**

提交后同步 GitHub 备份，再推送 GitLab `codex/containerize-single-image`，等待 Wharf pipeline `752` 自动发布。使用已登录浏览器验证：

```text
GET /erzhuang-project/health
GET /erzhuang-project/api/store-space-resource-view/stores?page=1&page_size=20
GET /erzhuang-project/api/store-space-resource-view/stores/{tenant_id}
```

验收列表真实数量、详情空间树、摄像头绑定、4 条缺失设备异常，以及 H5 Monitor 入口未回退。测试验收通过后，才将同一已验证提交发布到 GitLab `main` 并走 pipeline `771`、部署审批和正式验收。
