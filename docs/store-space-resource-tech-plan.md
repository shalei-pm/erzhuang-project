# 门店空间资源管理系统技术方案

最后更新：2026-06-11

版本：v0.1 正式草案

## 1. 方案结论

在现有 `erzhuang-project` 内扩展，不另建仓库。

项目从“设计图标记与诊室区域管理”升级为“门店空间资源管理系统”。核心变化是抽象独立的区域主数据，让设计图和视频通道都围绕同一套门店与业务区域工作。

技术栈沿用：

- 前端：Vite + React + TypeScript。
- 后端：Go HTTP 服务。
- 数据库：Supabase PostgreSQL。
- 文件存储：Lighthouse 本地磁盘，后续可迁移对象存储。
- PDF 转图片：服务器端处理。
- AI 识别：后端统一调用模型。
- 萤石云 API：后端统一调用。
- 部署：GitHub + Lighthouse + systemd + nginx `/erzhuang/`。

## 2. 架构原则

- 不重写现有设计图能力，优先适配和复用。
- 新增区域主数据层，逐步把现有 `design_plan_store_areas` 的业务含义迁移为通用 `store_areas`。
- 前端不接触数据库连接串、AI key、萤石云 appSecret/accessToken。
- 后端统一管理第三方 API、文件、AI、数据库。
- 第一版接真实萤石云 API 和真实 AI 识别，不以纯 mock 作为交付目标。
- 第三方接口接入要可降级、可重试、可记录失败原因。
- Supabase `public` 表必须开启 RLS，并增加显式拒绝前端直连的 policy。

## 3. 目标模块

```text
frontend/
  门店列表
  添加门店浮层
  门店详情
    设计图标注 Tab
    通道映射 Tab
    萤石云账号配置入口

internal/app
  HTTP 总入口
  路由挂载
  health

internal/designplan
  现有设计图上传、转换、识别、图纸标注能力

internal/storespace
  门店主数据
  区域主数据
  萤石云账号
  录像机
  通道
  截图
  通道识别
  图纸和通道校验

internal/ezviz
  萤石云 API client

internal/vision
  视觉识别 client 或 adapter
```

说明：

- `internal/designplan` 短期保留，减少重构风险。
- 新增 `internal/storespace` 作为门店空间资源聚合层。
- 设计图数据可先兼容旧表，再逐步迁移到通用表。

## 4. 数据模型草案

### 4.1 stores

通用门店表。

```sql
create table stores (
  id bigserial primary key,
  name text not null,
  normalized_name text not null unique,
  external_org_id text not null default '',
  design_plan_status text not null default 'not_uploaded',
  overall_status text not null default 'partial',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint stores_design_plan_status_check
    check (design_plan_status in ('not_uploaded', 'pending_recognition', 'pending_annotation', 'completed')),
  constraint stores_overall_status_check
    check (overall_status in ('incomplete', 'partial', 'completed', 'exception'))
);
```

### 4.2 store_areas

业务区域主数据。

```sql
create table store_areas (
  id bigserial primary key,
  store_id bigint not null references stores(id) on delete cascade,
  area_type text not null,
  area_number integer not null,
  display_name text not null,
  source text not null default 'manual',
  status text not null default 'confirmed',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint store_areas_type_check
    check (area_type in ('treatment', 'consultation', 'beauty')),
  constraint store_areas_source_check
    check (source in ('manual', 'design_plan', 'video_channel', 'multiple')),
  constraint store_areas_status_check
    check (status in ('candidate', 'confirmed'))
);
```

唯一索引：

```sql
create unique index store_areas_unique_number_per_type
on store_areas (store_id, area_type, area_number);
```

### 4.3 store_design_plans

门店设计图文件表。

```sql
create table store_design_plans (
  id bigserial primary key,
  store_id bigint not null references stores(id) on delete cascade,
  pdf_file_name text not null default '',
  original_pdf_path text not null default '',
  preview_image_path text not null default '',
  thumbnail_path text not null default '',
  page_count integer not null default 0,
  recognition_status text not null default 'not_started',
  recognition_result jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint store_design_plans_recognition_status_check
    check (recognition_status in ('not_started', 'running', 'failed', 'completed'))
);
```

### 4.4 design_plan_annotations

业务区域在设计图上的标注。

```sql
create table design_plan_annotations (
  id bigserial primary key,
  design_plan_id bigint not null references store_design_plans(id) on delete cascade,
  area_id bigint not null references store_areas(id) on delete cascade,
  box_x numeric not null,
  box_y numeric not null,
  box_width numeric not null,
  box_height numeric not null,
  status text not null default 'pending',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint design_plan_annotations_status_check
    check (status in ('pending', 'confirmed')),
  constraint design_plan_annotations_box_check check (
    box_x >= 0 and box_x <= 1 and
    box_y >= 0 and box_y <= 1 and
    box_width > 0 and box_width <= 1 and
    box_height > 0 and box_height <= 1 and
    box_x + box_width <= 1 and
    box_y + box_height <= 1
  )
);
```

唯一索引：

```sql
create unique index design_plan_annotations_unique_area
on design_plan_annotations (design_plan_id, area_id);
```

### 4.5 ezviz_accounts

萤石云账号配置。

```sql
create table ezviz_accounts (
  id bigserial primary key,
  account_name text not null unique,
  app_key text not null,
  app_secret_ciphertext text not null,
  access_token_ciphertext text not null,
  status text not null default 'unverified',
  last_verified_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint ezviz_accounts_status_check
    check (status in ('unverified', 'available', 'unavailable'))
);
```

密钥说明：

- 第一版若尚未引入加密组件，可先放在服务器环境文件或受控数据库字段，但不得回显到前端。
- 推荐尽快增加服务端加密密钥 `APP_SECRET_KEY`，对 appSecret/accessToken 加密存储。

### 4.6 video_recorders

录像机表。

```sql
create table video_recorders (
  id bigserial primary key,
  store_id bigint not null references stores(id) on delete cascade,
  ezviz_account_id bigint not null references ezviz_accounts(id),
  device_code text not null unique,
  status text not null default 'offline',
  effective_channel_count integer not null default 0,
  last_scanned_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint video_recorders_status_check
    check (status in ('online', 'offline'))
);
```

规则：

- 单门店最多 3 台录像机，由后端校验。
- 设备编码全系统唯一。

### 4.7 video_channels

有效通道表。

```sql
create table video_channels (
  id bigserial primary key,
  recorder_id bigint not null references video_recorders(id) on delete cascade,
  channel_no integer not null,
  channel_name text not null default '',
  status text not null default 'pending_recognition',
  is_active boolean not null default true,
  scene_type text not null default 'unknown',
  area_type text,
  area_number integer,
  area_id bigint references store_areas(id),
  recognition_attempts integer not null default 0,
  recognition_result jsonb,
  confirmed_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint video_channels_status_check
    check (status in ('pending_recognition', 'pending_confirmation', 'confirmed_business', 'confirmed_non_business', 'recognition_failed', 'inactive')),
  constraint video_channels_scene_type_check
    check (scene_type in ('treatment', 'consultation', 'beauty', 'front_desk', 'corridor', 'passage', 'waiting_area', 'hall', 'entrance', 'storage', 'pharmacy', 'machine_room', 'unknown'))
);
```

唯一索引：

```sql
create unique index video_channels_unique_channel
on video_channels (recorder_id, channel_no);
```

### 4.8 channel_snapshots

通道截图表。

```sql
create table channel_snapshots (
  id bigserial primary key,
  channel_id bigint not null references video_channels(id) on delete cascade,
  thumbnail_path text not null default '',
  full_image_path text not null default '',
  full_image_expires_at timestamptz,
  created_at timestamptz not null default now()
);
```

规则：

- 每个通道仅展示最近一次截图。
- 大图一周后删除文件。
- 缩略图和识别结果保留。

### 4.9 operation_logs

可继续复用现有操作日志思路，后续统一为：

```sql
create table operation_logs (
  id bigserial primary key,
  action text not null,
  entity_type text not null,
  entity_id bigint,
  store_id bigint,
  actor text not null default 'admin',
  summary text not null,
  created_at timestamptz not null default now()
);
```

## 5. API 草案

### 5.1 门店

```text
GET    /api/store-space/stores?q=&page=&page_size=
GET    /api/store-space/stores/{id}
POST   /api/store-space/stores
DELETE /api/store-space/stores/{id}
POST   /api/store-space/stores/check-duplicate
```

创建门店请求：

```json
{
  "name": "深圳壹方城",
  "external_org_id": "12345",
  "design_plan_upload_id": "tmp_xxx",
  "recorders": [
    {
      "ezviz_account_id": 1,
      "device_code": "D12345678"
    }
  ]
}
```

后端校验：

- name 必填。
- design_plan_upload_id 和 recorders 至少一个非空。
- recorders 最多 3 个。
- device_code 全系统唯一。

### 5.2 设计图

```text
POST /api/store-space/design-plans/upload
POST /api/store-space/stores/{store_id}/design-plan/recognize
PUT  /api/store-space/stores/{store_id}/design-plan/annotations
```

说明：

- 可复用现有 `/api/design-plan/uploads`、`recognize`、保存区域的能力。
- 新接口需要改为保存通用 `store_areas` 和 `design_plan_annotations`。

### 5.3 萤石云账号

```text
GET    /api/store-space/ezviz-accounts
POST   /api/store-space/ezviz-accounts
PUT    /api/store-space/ezviz-accounts/{id}
DELETE /api/store-space/ezviz-accounts/{id}
POST   /api/store-space/ezviz-accounts/{id}/verify
```

前端只展示：

- id
- account_name
- status
- last_verified_at

不返回完整 appSecret/accessToken。

### 5.4 录像机

```text
POST   /api/store-space/stores/{store_id}/recorders
DELETE /api/store-space/recorders/{recorder_id}
POST   /api/store-space/recorders/{recorder_id}/scan-channels
```

扫描通道行为：

- 调用萤石云 API。
- 更新录像机在线/离线。
- 只保存有效通道。
- 失效通道标记 inactive 并隐藏。
- 保留仍有效通道的确认结果。

### 5.5 通道识别和确认

```text
POST /api/store-space/recorders/{recorder_id}/recognize-channels
POST /api/store-space/channels/{channel_id}/recognize
PUT  /api/store-space/channels/{channel_id}/confirmation
```

识别全部通道：

- 只处理单台录像机。
- 后端队列执行。
- 第一版可以用同步请求 + 流程状态轮询，若耗时过长则升级异步 job。

建议异步化接口：

```text
POST /api/store-space/recorders/{recorder_id}/recognition-jobs
GET  /api/store-space/recognition-jobs/{job_id}
```

通道确认请求：

```json
{
  "kind": "business",
  "area_type": "consultation",
  "area_number": 2
}
```

或：

```json
{
  "kind": "non_business",
  "scene_type": "front_desk"
}
```

后端行为：

- business：按 store_id + area_type + area_number 查找区域；存在则绑定，不存在则创建正式区域并绑定。
- non_business：不生成区域，标记为已确认非业务画面。

## 6. 萤石云集成

详细接口和工程约束见：

- `docs/ezviz-openapi-notes.md`

### 6.1 Client 边界

建议新增：

```text
internal/ezviz/client.go
```

能力：

- 验证账号。
- 查询设备在线状态。
- 查询设备通道列表。
- 判断有效通道。
- 抓取通道截图。

已确认的抓图链路接口：

- 获取 token：`POST https://open.ys7.com/api/lapp/token/get`
- 设备抓图：`POST https://open.ys7.com/api/lapp/device/capture`

请求格式必须是：

```text
application/x-www-form-urlencoded
```

禁止使用 JSON body。

### 6.2 多账号

录像机必须关联一个萤石云账号。

扫描和抓图时：

- 根据录像机关联的 `ezviz_account_id` 取密钥。
- 调用对应账号的 API。
- 如果设备不存在或无权限，录像机置为离线。
- token 必须按 `ezviz_account_id` 缓存，禁止使用全局单 token。
- 映射缺少账号 ID 时必须返回结构化错误，禁止默认使用任意账号。

### 6.3 token 刷新

萤石 `accessToken` 有有效期，服务不能假设长期有效。

如果萤石接口返回以下错误码：

- `10002`
- `10014`

后端必须：

1. 丢弃该账号缓存 token。
2. 使用该账号 appKey/appSecret 重新获取 accessToken。
3. 使用新 token 重试当前请求一次。
4. 如果仍失败，再返回错误。

不要只依赖定时刷新。

### 6.4 失败处理

- 账号不可用：提示账号不可用。
- 设备不存在/无权限：录像机离线。
- 通道抓图失败：单通道重试，最多 3 次。
- AI 识别失败：单通道重试，最多 3 次。
- 3 次失败后，通道状态为 recognition_failed。
- 所有萤石 HTTP 请求必须设置 timeout。
- token 获取失败最多重试 2 次。
- 抓图普通失败最多重试 1 次。
- 设备超时最多重试 1 次，不要长时间阻塞。

常见错误码：

| 错误码 | 处理建议 |
| --- | --- |
| 200 | 成功 |
| 10002 | token 过期/无效，刷新 token 后重试一次 |
| 10014 | token 过期/无效，刷新 token 后重试一次 |
| 60012 | 可能是通道不存在、设备未响应或账号无权限 |
| 20008 | 设备响应超时，可能设备离线、网络异常或通道无响应 |
| 9001 | 抓图场景可作为无有效画面或设备异常处理 |

### 6.5 图片处理

萤石抓图成功后返回 `data.picUrl`。

本项目不直接把萤石 `picUrl` 透传给前端作为长期地址。后端应：

1. 获取 `picUrl`。
2. 服务端下载图片。
3. 保存最近一次缩略图和大图。
4. 前端展示本系统图片地址。
5. 大图一周后删除，保留缩略图和识别结果。

### 6.6 审计日志

每次抓图请求必须记录审计日志，不记录 appSecret/accessToken。

建议字段：

- request_id
- request_user，第一版可固定 `admin`
- store_id
- store_name
- device_serial
- channel_no
- area_name
- ezviz_account_id
- result
- error_code
- error_message
- captured_at
- cost_ms

## 7. AI 识别

### 7.1 输入

通道截图大图或压缩后的识别图。

### 7.2 输出 JSON

```json
{
  "scene_type": "consultation",
  "is_business_area": true,
  "area_type": "consultation",
  "area_number": 2,
  "confidence": "medium",
  "raw_text": "面诊室 2",
  "reason": "画面中可见编号纸"
}
```

非业务画面：

```json
{
  "scene_type": "front_desk",
  "is_business_area": false,
  "area_type": null,
  "area_number": null,
  "confidence": "high",
  "raw_text": "",
  "reason": "画面中可见前台台面和接待区"
}
```

### 7.3 Prompt 要点

- 只把治疗室、面诊室、生美识别为业务区域。
- 前台、走廊、通道、候诊区、大厅、门口、库房、药房、机房、未知识别为非业务画面。
- 优先读取白底黑字编号纸。
- 如果识别不到编号，编号返回 null。
- 不要编造编号。
- 返回严格 JSON。

## 8. 前端改造

### 8.1 复用现有能力

保留现有：

- 门店列表样式。
- 大弹窗/图纸预览体验。
- PDF 上传。
- 图纸缩放。
- 矩形框拖动/缩放。
- 区域卡片编辑。
- 版本号展示。

### 8.2 需要拆分的组件

建议本次改造前先拆分 `frontend/src/App.tsx`：

```text
components/StoreList.tsx
components/CreateStoreModal.tsx
components/StoreDetail.tsx
components/DesignPlanTab.tsx
components/VideoChannelTab.tsx
components/EzvizAccountModal.tsx
components/FloorPlanCanvas.tsx
components/AreaCardList.tsx
```

不要在一个巨大 `App.tsx` 中继续叠加录像机和通道逻辑。

### 8.3 状态管理

第一版不强制引入全局状态库。可使用 React state + hooks：

```text
hooks/useStores.ts
hooks/useStoreDetail.ts
hooks/useDesignPlan.ts
hooks/useVideoChannels.ts
```

## 9. 迁移策略

当前已有表：

- `design_plan_stores`
- `design_plan_store_areas`
- `design_plan_operation_logs`

建议分两阶段：

### 阶段 A：兼容扩展

- 新增 `stores`、`store_areas`、视频相关表。
- 现有设计图能力先通过适配层读写新表。
- 保留旧表，避免线上立即迁移风险。

### 阶段 B：数据迁移

- 将旧 `design_plan_stores` 迁移到 `stores` + `store_design_plans`。
- 将旧 `design_plan_store_areas` 迁移到 `store_areas` + `design_plan_annotations`。
- 验证完成后再考虑停用旧表。

第一版开发建议先做阶段 A，降低风险。

## 10. 验证和发布

本地验证：

- Go 编译检查。
- Go 单元测试或服务器 `go test ./...`。
- 前端 `tsc -b`。
- 前端 `vite build`。

服务器发布：

- 沿用 `scripts/deploy.sh`。
- 发布后验证：
  - `/health`
  - 页面版本号
  - 创建测试门店
  - 扫描测试录像机
  - 识别测试通道
  - 确认业务区域
  - 设计图反向待标注

回滚：

- 保留发布 commit。
- 可用现有 rollback 脚本回滚代码。
- 数据库迁移需设计为向前兼容，避免代码回滚后无法读取旧数据。

## 11. 风险

- 萤石云 API 字段和限制尚未确认。
- accessToken 刷新机制未确认。
- 视频截图可能包含隐私内容，需要后续加入权限和留存策略。
- 多账号密钥存储需要安全设计。
- AI 识别编号纸准确率依赖业务执行标准。
- 现有设计图模块改造范围较大，需要避免破坏现有体验。

## 12. 待确认

- 萤石云接口文档和测试账号。
- 测试录像机设备编码。
- AI 识别接口 key 和模型能力。
- OpenClaw 未来查询 API 合同。
