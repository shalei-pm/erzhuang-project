# Current Plan

更新时间：2026-08-13

## 当前轮目标

作为二壮项目主会话和技术负责人，推进「门店空间资源查看」3.0 的拆分、实现、验收和项目记忆维护。当前轮不连接真实业务库、不发布公司环境，目标是把 3.0 本地分支收口到可 review、可接入真实只读业务库、可决定是否发布的状态。

## 当前状态摘要

- 当前线上主版本仍为 2.31.x，核心运行时为公司 MySQL + OSS + APISIX SSO + H5 Monitor 萤石云播放。
- 韩国 Lighthouse 链路已彻底废止，不再用于二壮项目发布、回滚、验收或备用环境。
- 旧 Supabase/PostgreSQL 已删除或不再作为运行时/回滚路径。
- 用户确认 3.0 方向：二壮不再维护空间/通道映射主数据，改为读取公司业务库的只读资源查看。
- 3.0 模块名：门店空间资源查看。
- 3.0 不改 H5 Monitor，当前怎么看监控还怎么看。
- 3.0 不做设计图上传/标注、AI 通道识别、人工确认、门店/录像机/通道写入。
- 当前分支：`codex/store-space-resource-view-3`。
- 2.x 完整备份已完成并推送 GitHub：tag `v2.31-stable-before-resource-view-3`，zip `docs/handoffs/archives/erzhuang-project-2.31.8-before-resource-view-3.zip`。
- 3.0 后端领域聚合、只读 repository、API handler 和 `cmd/server` 接线已完成初版。
- 3.0 前端 API 类型、只读列表、只读详情、空间视角、设备视角、异常项和主页面切换已完成初版。
- 主页面已隐藏旧新增、编辑、删除、扫描、识别、确认、设计图上传/标注入口；H5 Monitor 路由和播放页未改。
- 用户已在镜像/公司环境配置 `K8S_SECRET_BUSINESS_MYSQL_DSN`，用于 3.0 业务库只读连接；文档和仓库不记录真实 DSN、账号或密码。

## 技术负责人拆分

### Backend: 业务库只读资源聚合

- 范围：`internal/resourceview`、`cmd/server/main.go`、`internal/app/handler.go`。
- 目标：读取业务库四张表，聚合门店、空间、设备、绑定和异常项。
- 状态：初版完成。
- 已完成：
  - 领域模型：门店摘要、门店详情、空间树、设备树、异常项。
  - 只读 repository：`tb_crm_admin_tenant`、`tb_crm_iot_device`、`tb_crm_consulting_room`、`tb_crm_iot_area_device_relation`。
  - API：`GET /api/store-space-resource-view/stores`、`GET /api/store-space-resource-view/stores/{tenantId}`。
  - 权限：复用 `store:read` 守卫；监控入口只返回 `can_view_monitor` / `monitor_url`，不扩大播放权限。
  - 配置：新增 `BUSINESS_MYSQL_DSN` / `K8S_SECRET_BUSINESS_MYSQL_DSN`，未配置时资源查看 API 返回清晰未配置错误。

### Frontend: 只读资源查看主流程

- 范围：`frontend/src/api.ts`、`frontend/src/domain/resource-view.ts`、`frontend/src/components/ResourceStoreList.tsx`、`frontend/src/components/ResourceStoreDetail.tsx`、`frontend/src/App.tsx`、`frontend/src/styles.css`。
- 目标：把后台主页面切到只读「门店空间资源查看」，保留系统设置和 H5 Monitor。
- 状态：初版完成，本地 mock 浏览器已看过首屏和详情页。
- 已完成：
  - 3.0 API 类型和 snake/camel 映射。
  - 本地 mock adapter，便于无真实业务库时预览。
  - 只读列表：城市筛选、搜索、分页、工控机/NVR/摄像头/空间/绑定/异常统计。
  - 只读详情：空间视角、设备视角、异常项。
  - 查看监控入口按 `canViewMonitor && monitorUrl` 展示。
  - 主页面去掉旧维护入口，系统设置和 H5 路由保留。

### 主会话 Review 与发布控制

- 范围：跨后端、前端、文档、验证、发布。
- 当前状态：正在收口。
- 责任：
  - 确认 3.0 边界不漂移。
  - 防止真实业务库密钥、账号、密码、DSN 进入仓库。
  - 防止旧 PostgreSQL/Supabase 或 OpenClaw 暂缓工作混入。
  - 防止 3.0 新模块出现写 SQL 或副作用接口。
  - 发布前确认业务库只读 Secret、网络白名单和线上验收脚本。

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
- [x] 实施前给当前 2.x 稳定状态打 tag、打包 zip 并写入 handoff。
- [x] 后端 Task 1-4：业务库只读 API 初版完成。
- [x] 前端 Task 5-6：只读资源查看 UI 初版完成。
- [x] 主会话初步 review：主 UI 已切到 3.0，只读边界和 H5 Monitor 边界符合预期。
- [ ] 主会话最终验证：后端、前端、构建、敏感信息扫描。
- [x] 真实业务库接入前配置确认：`K8S_SECRET_BUSINESS_MYSQL_DSN` 已由用户配置到镜像/公司环境。
- [ ] 发布后确认业务库网络白名单、只读权限和字典/枚举口径。
- [ ] 用户本地/线上预览确认。
- [ ] 用户明确要求后，再发布到公司。

## 验证方式

当前轮已执行或待执行：

- 后端：
  - `CGO_ENABLED=0 ... go test ./internal/resourceview`
  - `go test -c ./internal/resourceview`
  - `go test -c ./internal/app`
  - `go test -c ./cmd/server`
  - `go build -o /private/tmp/server-check ./cmd/server`
- 前端：
  - `cd frontend && npm test`
  - `cd frontend && npm run build`
  - mock 浏览器查看列表页和详情页。
- 安全扫描：
  - 新增 3.0 代码不得含真实业务库账号、主机、密码或 DSN。
  - `internal/resourceview` 不得含 `insert/update/delete/replace/drop/truncate` 等写 SQL。
  - 不得混入 `securityVideoUrl`、`content_id`、OpenClaw 暂缓工作、旧 Supabase/PostgreSQL 运行时依赖。

## 当前风险

- 3.0 会引入第二个 MySQL 只读数据源，必须严格区分二壮运行库和公司业务库。
- `K8S_SECRET_BUSINESS_MYSQL_DSN` 已配置，但仍需发布后通过启动日志和 3.0 API 返回确认网络白名单、账号权限和 DSN 参数有效。
- `tb_crm_consulting_room.dict_id`、城市字典、`function_type` 取值和状态枚举仍需业务库维护研发确认。
- 当前城市名第一版按 `city_id` 展示为“城市 N”；如需要真实城市名，需要接入城市字典或业务库同学提供映射。
- 旧 storespace/designplan 代码不会在 3.0 初期删除，避免影响回滚；主 UI 已隐藏旧写入口。
- 真实业务库数据量、长门店名、空间树深度、异常数量需要上线前用真实样本再看一次 UI 信息密度。

## 下一步建议

1. 完成最终本地验证和项目记忆更新。
2. 用户确认后发布到公司 GitLab/K8s，让新镜像读取已配置的 `K8S_SECRET_BUSINESS_MYSQL_DSN`。
3. 发布后先看启动日志是否有 `business resource view enabled`，再用已登录浏览器请求 3.0 API 验证真实业务库数据。
4. 用户确认 3.0 首屏和详情页体验后，再进入正式使用。
