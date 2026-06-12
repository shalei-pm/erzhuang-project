# 技术架构索引

最后更新：2026-06-08

本文是 `erzhuang-project` 的代码地图，用于后续迭代时快速定位改动范围，避免为了一个小需求整体重写。

## 1. 总体分层

```text
浏览器
  ↓
frontend/                 React + TypeScript 前端页面
  ↓ /api/design-plan
internal/app              Go HTTP 总入口和基础接口
  ↓
internal/designplan       设计图标记业务后端
  ↓
Supabase PostgreSQL       设计图门店、区域、操作日志数据
```

主要入口：

- 前端入口：`frontend/src/App.tsx`
- 前端 API adapter：`frontend/src/api.ts`
- 前端样式 token：`frontend/src/styles.css`
- Go 服务入口：`cmd/server/main.go`
- Go HTTP 路由汇总：`internal/app/handler.go`
- 设计图后端路由：`internal/designplan/handler.go`
- 设计图后端业务服务：`internal/designplan/service.go`
- 设计图上传转换：`internal/designplan/uploads.go`
- 设计图 AI 识别：`internal/designplan/recognizer.go`
- 设计图后端存储：`internal/designplan/store.go`
- 设计图数据库 schema：`db/design_plan_schema.sql`

## 2. 业务能力到代码位置

| 业务能力 | 前端入口 | 后端入口 | 数据位置 | 备注 |
| --- | --- | --- | --- | --- |
| 门店列表 | `frontend/src/App.tsx` 的 `loadStores`、列表渲染区 | `internal/designplan/handler.go` 的 `listStores` | `design_plan_stores`、`design_plan_store_areas` | 支持搜索、分页、数量统计 |
| 添加门店 | `openCreateEditor`、`requestPdfUpload`、`handlePdfSelected`、`uploadAndRecognize`、`handleSave` | `uploadPDF`、`recognizeUpload`、`createStore` | `uploads/`、`design_plan_stores`、`design_plan_store_areas` | 前端上传本地 PDF，后端转 PNG 并调用 AI 识别 |
| 编辑门店 | `openEditEditor`、`updateEditor`、`updateArea`、`handleSave` | `updateStore` | 同上 | 后端 `PUT` 采用整批替换区域 |
| 删除门店 | `handleDelete` | `deleteStore` | store 删除级联 area | 删除日志保留在全局日志表 |
| 重复门店检查 | `designPlanApi.checkDuplicate` | `checkDuplicate`、`Service.ensureNoExactDuplicate` | `normalized_name` | 保存时后端会拦截完全同名 |
| 区域校验 | `validateEditor`、`areaDisplayName`、`normalizeAreaForSave` | `ValidateStoreInput` | 后端约束 + 唯一索引 | 前端自动生成区域名称并在保存时自动确认完整区域，后端为最终门禁 |
| 区域框显示/拖动/拉伸/缩放 | `boxStyle`、`clampBox`、`resizeBox`、`dragState`、`planZoom` | 暂无 | `box_x/y/width/height` | 支持移动、四角拉伸和查看缩放 |
| PDF 上传/转换 | `requestPdfUpload`、`handlePdfSelected`、`uploadAndRecognize` | `uploadPDF`、`UploadManager.Save` | `uploads/<upload_id>/original.pdf|preview.png|thumbnail.png` | 依赖服务器 `pdftoppm`，限制 5MB、最多 5 页 |
| AI 识别 | `designPlanApi.recognizeUpload` | `recognizeUpload`、`OpenAIRecognizer` | `recognition_result` | 依赖 `OPENAI_API_KEY`，返回门店名和区域框结构化 JSON |
| 操作日志 | 页面暂不展示 | `insertOperationLog` | `design_plan_operation_logs` | actor 当前固定为 `admin` |

## 3. 前端代码索引

### `frontend/src/App.tsx`

负责页面状态、交互和 UI 渲染。

- 页面级状态：
  - `stores`：门店列表。
  - `query`、`page`、`total`：搜索与分页。
  - `editor`：添加/编辑弹窗状态。
  - `validation`：保存校验结果。
  - `dragState`：图纸区域框移动/拉伸状态。
  - `planZoom`：左侧图纸查看缩放比例。
- 关键函数：
  - `loadStores`：加载门店列表。
  - `openCreateEditor`：打开添加门店弹窗。
  - `openEditEditor`：打开编辑门店弹窗。
  - `requestPdfUpload`：触发浏览器本地 PDF 文件选择器。
  - `handlePdfSelected`：读取用户选择的 PDF 文件名，并进入当前 mock 上传/识别流程。
  - `mockUploadAndRecognize`：模拟上传、转换、AI 识别流程；识别完成后用识别到的门店名称回填顶部门店名称输入框。
  - `updateArea`：更新右侧区域卡片和左侧框。
  - `addArea`：新增手工区域。
  - `handleSave`：保存前端数据。
  - `handleDelete`：删除门店。
  - `validateEditor`：前端保存校验。
  - `areaDisplayName`：按区域类型和编号自动生成区域名称。
  - `normalizeAreaForSave`：保存前自动补全区域名称，并将完整区域置为已确认。
  - `areaBoxPrimaryLabel`、`areaBoxSecondaryLabel`：控制左侧矩形框内标签显示。
  - `clampBox`：限制框不超出图片边界。
  - `resizeBox`：根据四角 handle 计算新的区域框。
  - `APP_VERSION`：读取 `VITE_APP_VERSION`，在首页底部展示当前前端版本。

适合后续拆分的方向：

- `components/StoreList.tsx`：门店列表。
- `components/StoreEditorModal.tsx`：添加/编辑弹窗。
- `components/FloorPlanCanvas.tsx`：左侧图纸和标注框。
- `components/AreaCardList.tsx`：右侧区域卡片。
- `hooks/useDesignPlanStores.ts`：列表和保存数据流。

当前为了快速形成 P0 骨架，页面仍集中在 `App.tsx`；后续复杂迭代前建议先按上述方向拆分。

### `frontend/src/api.ts`

负责前端数据类型、接口路径约定、后端字段映射和混合 adapter。

- 类型：
  - `AreaType`
  - `Confidence`
  - `StoreStatus`
  - `AreaBox`
  - `StoreArea`
  - `StoreSummary`
  - `StoreDetail`
  - `SaveStorePayload`
- API adapter：
  - `designPlanApi.listStores`
  - `designPlanApi.getStore`
  - `designPlanApi.uploadPdf`
  - `designPlanApi.recognizeUpload`
  - `designPlanApi.checkDuplicate`
  - `designPlanApi.saveStore`
  - `designPlanApi.deleteStore`

当前 adapter 规则：

- 默认 API base：`/erzhuang/api/design-plan`。
- 可通过 `VITE_DESIGN_PLAN_API_BASE` 覆盖。
- 可通过 `VITE_DESIGN_PLAN_API_MODE=auto|mock|http` 控制模式。
- `auto`：真实 CRUD 优先，后端不可用、接口未就绪或 5xx 时 fallback 到 mock。
- `mock`：全量使用前端 mock。
- `http`：强制真实后端，不 fallback。
- CRUD 在 `auto` 模式下仍可 fallback 到 mock，方便本地没有后端时预览页面。
- PDF 上传和 AI 识别不做静默 fallback；真实链路失败时必须展示错误，让用户转手动维护，避免样例图纸误导验收。
- `designPlanApi.uploadPdf` 传递真实 `File` 对象，通过 multipart/form-data 上传。

后续 API 行为调整优先改这里，不要先改页面。

### `frontend/src/styles.css`

负责后台 UI 视觉系统和响应式布局。

- 全局 token 位于 `:root`：
  - 品牌色：`--brand`
  - 文本色：`--text-*`
  - 边框/背景：`--border-*`、`--surface-*`
  - 按钮字号：`--button-font-large`、`--button-font-normal`、`--button-font-special`
  - 区域类型色：`--area-treatment`、`--area-consultation`、`--area-beauty`
  - 选中态：沿用区域类型色，通过 `.area-*.is-selected` 增强边框和阴影，不再使用统一黄色。
- 按钮字号规范：
  - 主操作按钮：`--button-font-large`，用于添加门店、保存、上传等最高优先级操作。
  - 普通操作按钮：`--button-font-normal`，用于编辑、删除、上移、下移、新增区域等常规操作。
  - 特殊紧凑按钮：`--button-font-special`，用于图纸查看工具、关闭 toast 等空间很小的辅助操作。
- 图标按钮规范：
  - 使用 `.icon-button` 固定为 32px 正方形，用于关闭、增加、收起等轻操作。
  - 图标按钮必须有 `aria-label`，可补 `title`，页面上不展示解释性长文案。
  - 模态框关闭按钮使用 `.modal-close-button`，不直接用普通文本 `x`，避免不同字体下视觉变形。
- 当前后台风格：
  - 定位为轻量企业后台 / SaaS admin，参考 Ant Design、Arco Design、Semi Design 的克制信息密度和控件层级。
  - 项目没有直接引入这些组件库，而是用 tokenized CSS 自建基础样式，方便快速迭代和后续统一换色。
  - 页面以白色内容面、浅灰背景、细边框、轻阴影为主；卡片半径控制在 8px 内，避免营销化装饰。
- 重点样式区：
  - 列表表格和缩略图。
  - lightbox 弹窗。
  - 左侧图纸区域和查看缩放。
  - 区域框 `.area-box`。
  - 右侧区域卡片 `.area-card`。
  - 首页版本号 `.app-version`。

后续只改配色时，优先改 `:root` token。

## 4. 后端代码索引

### `cmd/server/main.go`

服务启动入口。

- 读取 `ADDR`，默认 `127.0.0.1:18080`。
- 读取 `DATABASE_URL`。
- 有数据库时：
  - 打开 Postgres。
  - 调用 `app.EnsurePostgresSchema` 初始化 tasks 和 design plan schema。
  - 注入 `app.NewPostgresStore` 和 `designplan.NewPostgresStore`。
- 无数据库时：
  - 使用 memory store，便于本地练习。

### `internal/app/handler.go`

HTTP 总入口。

- `GET /health`
- `GET /api/tasks`
- 调用 `designplan.RegisterRoutes` 挂载 `/api/design-plan/*`。

### `internal/designplan/models.go`

业务模型和 JSON 合同。

- 区域类型：`treatment`、`consultation`、`beauty`。
- 状态：`completed`、`needs_review`、`incomplete`。
- `RoomNumber` 支持 JSON 字符串或整数输入，最终统一为字符串输出。
- `Box` 使用 0 到 1 的比例坐标。

### `internal/designplan/handler.go`

设计图 API 路由。

- `GET /api/design-plan/stores`
- `GET /api/design-plan/stores/{id}`
- `POST /api/design-plan/stores`
- `PUT /api/design-plan/stores/{id}`
- `DELETE /api/design-plan/stores/{id}`
- `POST /api/design-plan/stores/check-duplicate`

### `internal/designplan/service.go`

业务服务层。

- 调用 `ValidateStoreInput` 做后端最终校验。
- 调用 `ensureNoExactDuplicate` 拦截完全同名门店。
- 负责把 handler 和 repository 解耦。

新增业务规则时，优先放在这里或 `validation.go`，不要散落到 handler/store。

### `internal/designplan/validation.go`

保存校验。

- 门店名必填。
- 至少 1 个区域。
- 区域名称、类型、框必填。
- 治疗室/面诊室编号必填。
- 编号只能是数字。
- 同门店同类型编号唯一。
- 框必须在图片边界内。

### `internal/designplan/duplicate.go`

门店名标准化和模糊匹配。

- `NormalizeStoreName`
- `IsSimilarStoreName`

后续如果业务要调整“疑似同名”规则，优先改这里。

### `internal/designplan/store.go`

数据访问层。

- `Repository`：统一接口。
- `MemoryStore`：无数据库本地练习。
- `PostgresStore`：Supabase PostgreSQL。
- `insertAreas`：保存区域。
- `insertOperationLog`：保存操作日志。
- `previewURL`、`thumbnailURL`：当前是预留路径，真实图片接口待 Phase 3 实现。

### `internal/designplan/schema.go`

服务启动时自动建表。

### `db/design_plan_schema.sql`

手工查看和未来迁移整理用 SQL。

## 5. 数据库索引

当前表：

- `design_plan_stores`
  - 门店主表。
  - `normalized_name` 唯一。
  - `recognition_result` 存最新识别 JSON。
- `design_plan_store_areas`
  - 区域表。
  - `store_id` 外键，门店删除时级联删除。
  - `box_x`、`box_y`、`box_width`、`box_height` 为比例坐标。
  - 唯一索引：同门店、同类型、同编号唯一，空编号不参与唯一。
- `design_plan_operation_logs`
  - 全局操作日志。
  - 删除门店后仍保留全局删除记录。

## 6. API 合同索引

基础路径：

```text
/api/design-plan
```

当前已实现：

```text
GET    /api/design-plan/stores?q=&page=1&page_size=20
GET    /api/design-plan/stores/{id}
GET    /api/design-plan/stores/{id}/preview
GET    /api/design-plan/stores/{id}/thumbnail
POST   /api/design-plan/stores
PUT    /api/design-plan/stores/{id}
DELETE /api/design-plan/stores/{id}
POST   /api/design-plan/stores/check-duplicate
POST   /api/design-plan/uploads
GET    /api/design-plan/uploads/{upload_id}/preview
GET    /api/design-plan/uploads/{upload_id}/thumbnail
POST   /api/design-plan/uploads/{upload_id}/recognize
```

## 7. 迭代拆分建议

### 接真实 CRUD API

主改：

- `frontend/src/api.ts`

辅助改：

- `frontend/src/App.tsx` 的错误提示和 loading 状态。
- `internal/designplan/handler.go` 如果响应字段需要对齐。

### 真实 PDF 上传和转换

主改：

- `internal/designplan/handler.go`
- `internal/designplan/service.go`
- `internal/designplan/uploads.go`
- `scripts/deploy.sh` 或服务器环境安装 `poppler-utils`

辅助改：

- `frontend/src/api.ts`
- `frontend/src/App.tsx` 的上传状态。
- `docs/deploy-runbook.md`

### AI 识别

主改：

- `internal/designplan/recognizer.go`
- `internal/designplan/handler.go`
- `internal/designplan/models.go`

辅助改：

- systemd 环境变量：`OPENAI_API_KEY`、`OPENAI_MODEL`
- `frontend/src/api.ts`
- `frontend/src/App.tsx`

### 通道截图和监控画面 AI 识别

主改：

- `internal/ezviz/client.go`：萤石云 token、通道列表、抓图 OpenAPI。
- `internal/storespace/ezviz_scanner.go`：把萤石云能力适配到门店空间服务。
- `internal/channelai/recognizer.go`：监控截图视觉识别，读取 `VISION_API_BASE_URL`、`VISION_API_KEY`、`VISION_MODEL`。
- `internal/storespace/channelai_adapter.go`：将通道 AI 结果映射为 store-space 识别结果。
- `internal/storespace/service.go`：串行抓图、AI 预填、耗时统计、已确认通道保护。
- `internal/storespace/store.go`：保存最近截图、识别结果、预填类型和编号。
- `frontend/src/components/VideoChannelTab.tsx`：缩略图预览、识别失败/耗时展示、AI 结果展示。

关键规则：

- AI 只做预填，用户点击“确认”后才锁定为正式通道映射。
- 编号卡片如果明确写了“治疗室 1 / 面诊室 2 / 生美 3”，优先于画面环境判断。
- 已确认通道再次识别时只刷新截图和耗时，不自动覆盖用户确认过的类型和编号。
- `recognition_result` 内记录 `capture_ms`、`recognition_ms`、`total_ms`，用于评估模型速度。

### 图纸框四角拉伸

主改：

- `frontend/src/App.tsx` 的 `DragState`、`resizeBox`、`clampBox`、区域框渲染。
- `frontend/src/styles.css` 的 `.resize-handle` 和四角游标样式。

当前状态：

- 已支持四角拉伸。
- 后续如果需要更丝滑的拖拽手感，可继续优化最小尺寸临界点和固定对边算法。

建议先拆组件：

- `frontend/src/components/FloorPlanCanvas.tsx`

### 前端组件化重构

主改：

- 新增 `frontend/src/components/*`
- 新增 `frontend/src/hooks/*`
- 保持 `frontend/src/api.ts` 合同稳定。

建议在接真实 API 前做，避免 `App.tsx` 继续膨胀。

### 配色和后台风格调整

主改：

- `frontend/src/styles.css` 的 `:root` token。

不要改业务逻辑文件。

## 8. 验证命令

后端：

```sh
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go build ./...
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test -c ./internal/designplan
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test -c ./internal/app
```

说明：当前本机 `go test ./...` 执行阶段有已知 macOS dyld 限制，服务器 Linux 发布前仍必须执行完整 `go test ./...`。

前端：

```sh
cd frontend
PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run build
```

本地预览：

```sh
cd frontend
PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run dev -- --host 127.0.0.1 --port 5173
```

## 9. 协作规则

- 主会话负责架构索引、任务拆解、验收、合并、发布、回滚。
- 前端专项会话只改 `frontend/` 和前端文档。
- 后端专项会话只改 Go 后端、数据库 schema 和后端文档。
- 部署、nginx、systemd、腾讯云操作只由主会话处理。
- 新需求开始前，先在本文件定位“业务能力到代码位置”，再决定是否拆前端/后端专项会话。
- 专项会话需要阶段性向主会话汇报，不只在最后交付时汇报。
- 专项分支的日常技术审批由主会话代管：主会话负责 review、决定合并、决定打回修改。
- 只有涉及产品范围变化、线上发布/回滚、云资源、密钥、外部 AI/API、不可逆数据操作，或需要接受明显风险时，主会话再向用户确认。

专项会话默认上报节点：

- 同步 `main` / 创建分支后：汇报分支、基线 commit、计划改动文件。
- 主要实现草稿完成后：汇报核心设计、已改文件、是否存在范围扩大。
- 验证前后：汇报验证命令、结果、失败原因和风险。
- 提交前：汇报 staged 文件，确认无 `dist`、`node_modules`、`.DS_Store`、密钥、服务器配置。
- 推送后：汇报分支、commit、验证结果、已知风险和建议主会话验收重点。

遇到边界问题时，专项会话应先上报再扩大范围，例如：

- 前端需要后端字段或接口调整。
- 后端需要产品交互判断。
- 任何任务需要 nginx、systemd、腾讯云、数据库密钥、OpenAI key、部署脚本。
- 需要从局部修改升级为组件拆分或架构重构。
