# 摄像头空间绑定关系表 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将门店资源详情替换为按摄像头核对空间绑定关系的只读表格。

**Architecture:** 保持既有 `GET /api/store-space-resource-view/stores/{tenantId}` 不变，在前端纯函数中使用 `cameras`、`spaces`、`relations` 回溯每个摄像头的父子空间路径。详情组件只负责展示，路径解析与排序放入 domain 层并由单元测试覆盖。

**Tech Stack:** React、TypeScript、Vite、Vitest、现有 3.0 只读资源查看 API。

---

### Task 1: 绑定关系行模型

**Files:**
- Modify: `frontend/src/domain/resource-view.ts`
- Test: `frontend/src/domain/resource-view.test.ts`

- [ ] 定义 `CameraBindingRow`、`CameraBindingPath` 和 `buildCameraBindingRows(store)`。
- [ ] 从有效 `camera -> space` 关系回溯 `parent_id`，将最多四段路径映射到层级 1、层级 2、层级 3、床位。
- [ ] 覆盖未绑定、重复关系、多条绑定、孤儿空间关系与稳定排序。

### Task 2: 摄像头绑定详情表

**Files:**
- Modify: `frontend/src/components/ResourceStoreDetail.tsx`
- Modify: `frontend/src/styles.css`

- [ ] 移除旧空间树、设备树、异常项 Tab。
- [ ] 新增详情顶部绑定完成度与更新时间信息。
- [ ] 渲染摄像头绑定表，固定列顺序并保留“最近截图”占位列。
- [ ] 对未绑定、已绑定、多绑定路径分别提供清楚的状态与空值呈现。

### Task 3: 验证与测试发布

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] 执行 `cd frontend && npm test -- --run` 和 `npm run build`。
- [ ] 通过 Chrome 验收 mock 与测试环境真实详情页，核对 1440px 表格布局、绑定状态、空截图占位和监控入口。
- [ ] 发布 GitHub 备份与 GitLab 测试分支，等待 Wharf pipeline `752` 自动构建部署，记录版本和验收结果。
