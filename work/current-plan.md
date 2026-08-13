# Current Plan

更新时间：2026-08-13

## 当前轮目标

作为二壮项目主会话，完成「门店空间资源查看」3.0 的方案沉淀和实施准备。当前轮只整理设计、计划和项目记忆，不改业务代码、不连接业务库、不发布。

## 当前状态摘要

- 当前线上主版本仍为 2.31.x，核心运行时为公司 MySQL + OSS + APISIX SSO + H5 Monitor 萤石云播放。
- 韩国 Lighthouse 链路已彻底废止，不再用于二壮项目发布、回滚、验收或备用环境。
- 旧 Supabase/PostgreSQL 已删除或不再作为运行时/回滚路径。
- 用户确认 3.0 方向：二壮不再维护空间/通道映射主数据，改为读取公司业务库的只读资源查看。
- 3.0 模块名：门店空间资源查看。
- 3.0 不改 H5 Monitor，当前怎么看监控还怎么看。
- 3.0 不做设计图上传/标注、AI 通道识别、人工确认、门店/录像机/通道写入。

## 已阅读文件

- `AGENTS.md`
- `README.md`
- `docs/deploy-runbook.md`
- `docs/codex-learning-state.md`
- `docs/decisions.md`
- `docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`
- `cmd/server/main.go`
- `internal/app/handler.go`
- `internal/app/authz.go`
- `internal/app/auth_users.go`
- `internal/storespace/models.go`
- `internal/storespace/store.go`
- `internal/storespace/service.go`
- `frontend/src/App.tsx`
- `frontend/src/api.ts`
- `frontend/src/components/StoreList.tsx`
- `frontend/src/components/StoreDetail.tsx`

## 业务库 3.0 关键关系

- `tb_crm_admin_tenant.id = tenant_id = 二壮 external_org_id`
- `tb_crm_iot_device.category='edge'` 表示工控机/边缘设备。
- `tb_crm_iot_device.category='nvr'` 表示录像机。
- `tb_crm_iot_device.category='camera'` 表示摄像头通道。
- `tb_crm_consulting_room` 表示业务空间，展示三层：
  - `level=1`：大类。
  - `level=2`：房间/区域。
  - `level=3`：床位/子区域。
- `tb_crm_iot_area_device_relation.area_id -> tb_crm_consulting_room.id`
- `tb_crm_iot_area_device_relation.device_id -> tb_crm_iot_device.id where category='camera'`
- `camera.parent_id -> tb_crm_iot_device.id where category='nvr'`

## 任务拆分与进度

- [x] 与用户讨论 3.0 产品方向和边界。
- [x] 确认采用 A 方案：业务库只读展示，不在二壮侧维护映射主数据。
- [x] 确认空间类型使用业务库自己的三层分类，不沿用旧面诊室/治疗室/美容室固定统计口径。
- [x] 确认 H5 Monitor 不在 3.0 改造，后续工控机/NVR 取流单独作为 3.1 或专项研究。
- [x] 创建 3.0 设计文档：`docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`。
- [x] 创建 3.0 实施计划：`docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`。
- [x] 更新项目长期记忆：`docs/codex-learning-state.md`。
- [x] 更新决策台账：`docs/decisions.md`。
- [x] 更新当前计划：`work/current-plan.md`。
- [ ] 等用户确认是否按计划进入实施。
- [x] 实施前给当前 2.x 稳定状态打 tag 或备份分支，并写入 `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`，说明备份点、能力范围、回滚方式和后续读取顺序。
- [ ] 后端子会话实施业务库只读 API。
- [ ] 前端子会话实施只读资源查看 UI。
- [ ] 主会话 review、浏览器验收、发布到公司。

## 验证方式

当前轮只做文档验证：

- `rg` 检查实施计划不含 `TODO/TBD/stub` 等模糊占位。
- `git status --short` 确认未改业务代码。
- 复查文档不包含真实业务库账号、密码、token 或 DSN。

后续代码实施验证见：

- `docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`

## 当前风险

- 3.0 会引入第二个 MySQL 只读数据源，必须严格区分二壮运行库和公司业务库。
- 业务库只读账号、网络白名单、K8s Secret 注入方式尚需运维/研发确认。
- `tb_crm_consulting_room.dict_id`、城市字典、`function_type` 取值和状态枚举仍需业务库维护研发确认。
- 旧 storespace/designplan 代码不会在 3.0 初期删除，避免影响回滚；但主 UI 切换后需要防止旧写入口继续暴露。
- 前端主页面切换涉及信息架构变化，必须做浏览器实际验收，不能只跑 build。

## 下一步建议

1. 用户 review 设计和实施计划，确认是否进入开发。
2. 使用子会话分工实施：后端只读 API、前端只读 UI；主会话负责 review 和发布。
3. 找业务库维护研发确认字典/枚举/function_type，并找运维确认业务库只读 Secret 注入。
