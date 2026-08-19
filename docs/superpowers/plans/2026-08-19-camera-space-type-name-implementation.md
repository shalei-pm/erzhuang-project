# 摄像头空间类型与名称展示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 3.0 摄像头详情的空间路径改为业务定义的空间类型和空间名称，并排除诊室区域容器关系。

**Architecture:** 后端服务层在摄像头关系标准化后，根据空间 `parent_id` 过滤直接绑定诊室区域容器的关系。所有绑定派生结果复用过滤后的关系；前端只将 API 返回的有效关系格式化为父级名称与当前名称。

**Tech Stack:** Go、MySQL 只读 repository、React、TypeScript、Vitest。

---

### Task 1: 过滤诊室区域容器关系

**Files:**
- Modify: `internal/resourceview/service.go`
- Test: `internal/resourceview/service_test.go`

- [ ] **Step 1: 写失败的后端回归测试**

设备 70 同时关联 area 2665 与 2667；`2665.parent_id=2387` 必须被忽略，`2667.parent_id=2665` 必须保留为唯一绑定。断言 `BoundCameraCount=1` 且 API 关系只保留 2667。

- [ ] **Step 2: 实现并验证过滤**

新增 `applicableCameraRelations`，仅当关系对应空间存在且 `parent_id=2387` 时排除。用项目内置 Go 工具链运行 `go test ./internal/resourceview`。

### Task 2: 展示空间类型与空间名称

**Files:**
- Modify: `frontend/src/domain/resource-view.ts`
- Modify: `frontend/src/domain/resource-view.test.ts`
- Modify: `frontend/src/components/ResourceStoreDetail.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: 更新 domain 模型与测试**

将 `CameraBindingPath` 从 `level1/level2/level3/bed` 改为 `spaceType/spaceName`。关联空间 `level=3` 时空间类型固定为“治疗室”，其他空间类型读取关联空间父级的 name；空间名称读取关联空间自身 name；父级不存在时类型为空并由 UI 显示 `-`。

- [ ] **Step 2: 更新详情表**

将四列空间层级替换为“空间类型、空间名称”，将空态 `colSpan` 调整为 7，并收窄两列表格的最小宽度。

- [ ] **Step 3: 验证前端**

运行 `cd frontend && npm test -- --run && npm run build`，仅允许既有 chunk-size warning。

### Task 3: 发布测试环境

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: 升版本并记录决策**

将版本从 `3.0.10` 升至 `3.0.11`，记录 `parent_id=2387` 关系排除和双字段展示规则。

- [ ] **Step 2: 全量验证和发布**

运行 `go test ./...`、前端测试和构建；只提交本轮源代码、版本和新增专项文档，同步 GitHub 备份与 GitLab `codex/containerize-single-image` 测试分支。

- [ ] **Step 3: 测试环境验收**

打开 `https://lite.sy.soyoung.com/erzhuang-project/`，确认版本 `3.0.11 (container)`；检查诊室区域容器关系未显示、有效关系显示父空间名称与当前空间名称、列表统计和详情绑定数一致。
