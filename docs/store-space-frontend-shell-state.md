# Store Space Frontend Shell State

更新时间：2026-06-11

分支：

```text
codex/store-space-frontend-shell
```

基线提交：

```text
f819793 Document store space resource expansion
```

## 实现范围

- 将 `frontend/src/App.tsx` 从单文件页面改为轻量编排层，负责列表、创建浮层、详情页状态切换和 toast。
- 新增前端组件：
  - `StoreList`：门店空间资源列表，新字段包含新氧机构 ID、设计图状态、录像机数量、通道数量、三类业务区域数量。
  - `CreateStoreModal`：添加门店浮层，支持门店名称、新氧机构 ID、设计图 PDF、最多 3 台录像机设备编码和萤石云账号选择。
  - `StoreDetail`：门店详情页，包含“设计图标注”和“通道映射”两个 Tab。
  - `DesignPlanTab`：复用现有图纸预览、区域卡片、矩形框拖动、四角缩放和保存标注体验；识别改为手动触发。
  - `VideoChannelTab`：通道映射 UI 壳，支持录像机列表、扫描按钮、识别按钮、有效通道列表、截图缩略图占位、区域类型/编号编辑、确认/编辑状态和识别进度占位。
  - `FloorPlanCanvas`、`AreaCardList`：从原 `App.tsx` 拆出的图纸与区域卡片组件。
- 新增前端 domain 工具：
  - `areas.ts`：区域标签、展示名、保存前规范化、区域计数。
  - `designPlan.ts`：图纸上传状态、矩形框 clamp/resize、文件名生成。
  - `format.ts`：时间和错误消息格式化。
- 扩展 `frontend/src/api.ts`：
  - 增加门店空间资源相关类型：设计图状态、萤石云账号、录像机、通道、非业务画面类型。
  - 增加 `storeSpaceApi`，新 store-space 能力只在 `VITE_DESIGN_PLAN_API_MODE=mock` 时使用前端 mock。
  - 保持现有 `designPlanApi` 兼容，旧设计图能力仍保留原有 fallback 逻辑。

## 交互说明

- 创建门店时，设计图 PDF 与录像机设备编码至少填写一个。
- 创建后默认进入：
  - 有设计图：设计图标注 Tab。
  - 只有录像机：通道映射 Tab。
- 设计图 Tab 保留原有比例坐标矩形框交互，支持拖动和四角缩放。
- 设计图识别不会在上传后自动触发，需要点击“识别图纸区域”。
- 通道映射 Tab 在显式 mock 模式下使用前端演示数据：
  - 扫描录像机会补齐有效通道。
  - 识别本录像机会预填业务区域或非业务画面。
  - 人工确认业务通道后，会生成或绑定业务区域。
- 萤石云账号入口仅展示账号名和配置入口，不展示、不保存、不请求真实密钥。

## Review 修复记录

更新时间：2026-06-11

主会话 review 后已修复：

- 添加门店浮层不再在账号未加载时默认使用 `ezvizAccountId = 1`。
  - `RecorderDraft.ezvizAccountId` 支持空值。
  - 没有账号时，账号下拉显示“暂无可选账号”且不可选。
  - 只要填写录像机设备编码，就必须选择真实 `accounts` 里的账号，否则提示“请选择萤石云账号”。
- 设计图识别结果不再直接覆盖现有区域。
  - 新增 `mergeRecognizedAreas`，按 `区域类型 + 编号` 匹配业务区域。
  - 匹配成功时保留已有区域 id 和业务身份，只更新/补充图纸 box、confidence、needsReview。
  - 未识别到的已有区域会保留，并在没有 box 时生成待标注占位框，支持“先通道映射，后补设计图”。
  - 新识别到的业务区域会追加。
- `storeSpaceApi` 新能力改为显式安全边界。
  - `createStore`、`listEzvizAccounts`、`scanRecorder`、`recognizeRecorder`、`confirmChannel` 只有在 `VITE_DESIGN_PLAN_API_MODE=mock` 时使用前端 mock。
  - `auto` 和 `http` 模式都会请求 `/erzhuang/api/store-space` 或 `VITE_STORE_SPACE_API_BASE` 指定的真实接口，不再自动 fallback 到前端 mock。
  - 旧 `designPlanApi` 保持原 fallback 行为，避免影响现有设计图功能。
- 去掉用户可见文案里的“mock”字样。

## 第二轮 Review 修复记录

更新时间：2026-06-12

主会话第二轮 review 后已修复：

- 新增 store-space 后端独立 DTO 和 mapper：
  - `BackendStoreSpaceSummary`
  - `BackendStoreSpaceDetail`
  - `BackendStoreSpaceArea`
  - `BackendStoreSpaceRecorder`
  - `mapStoreSpaceSummary`
  - `mapStoreSpaceDetail`
- `storeSpaceHttpAdapter.createStore()` 已改用 `mapStoreSpaceDetail()`，不再复用旧 design-plan 的 `mapBackendDetail()`。
- `storeSpaceHttpAdapter.confirmChannel()` 的返回类型已改为 `BackendStoreSpaceDetail`，并使用 `mapStoreSpaceDetail()` 解析。
- store-space mapper 会读取新后端字段：
  - `external_org_id`
  - `design_plan_status`
  - `overall_status`
  - `design_plans`
  - `recorders`
  - `areas[].area_type`
  - `areas[].display_name`
  - `areas[].area_number`
- `CreateStoreModal` 仍保持“填写录像机设备编码必须选择萤石云账号”的前端校验；提交 payload 时只传有效数字 `ezviz_account_id`，不会把空值传成 `""`。
- 注意：`storeSpaceApi.listStores()` 和 `storeSpaceApi.getStore()` 在当前 shell 阶段仍复用旧 `designPlanApi`，用于保护现有设计图列表体验。真正切换到门店空间资源列表/详情时，需要改为 `storeSpaceHttpAdapter.listStores()` / `storeSpaceHttpAdapter.getStore()` 并使用本次新增的 store-space list/get mapper。

## 第三轮 Review 修复记录

更新时间：2026-06-12

主会话第三轮 review 后已修复：

- 添加门店浮层的初始录像机行不再默认选中第一个萤石云账号，`ezvizAccountId` 初始值固定为空字符串。
- 点击“增加录像机”新增的录像机行同样不再默认选中第一个萤石云账号。
- 删除 `defaultAccountId()`，避免后续误用导致静默默认账号。
- 保持现有校验：只要填写录像机设备编码但未选择账号，就提示“请选择萤石云账号”。

## 改动文件

```text
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/styles.css
frontend/src/components/AreaCardList.tsx
frontend/src/components/CreateStoreModal.tsx
frontend/src/components/DesignPlanTab.tsx
frontend/src/components/FloorPlanCanvas.tsx
frontend/src/components/StoreDetail.tsx
frontend/src/components/StoreList.tsx
frontend/src/components/VideoChannelTab.tsx
frontend/src/domain/areas.ts
frontend/src/domain/designPlan.ts
frontend/src/domain/format.ts
docs/store-space-frontend-shell-state.md
```

## 验证结果

已执行：

```sh
PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm install
PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run build
```

结果：

- `npm install` 通过，审计 0 个漏洞。
- `npm run build` 通过，包含 `tsc -b` 和 `vite build`。

## 风险和限制

- `storeSpaceApi` 在 `auto/http` 模式下会尝试真实 `/api/store-space` 接口；如果后端接口未就绪，会直接暴露错误，不会静默使用前端 mock。
- 前端 mock 仅用于 `VITE_DESIGN_PLAN_API_MODE=mock` 的本地演示。
- 当前列表和详情读取仍暂走旧 design-plan adapter；这不是完整接入 store-space 后端列表/详情。
- 详情页新增录像机、删除录像机、萤石云账号配置只预留入口，等待后端接口和安全方案。
- 当前设计图保存仍复用旧 `designPlanApi.saveStore` 合同；后端新表和 API 合并后，需要继续对齐 `storeSpaceApi` 的真实 HTTP adapter。
- 视觉回归只通过构建检查，尚未在浏览器中逐屏截图验收。

## 给主会话的 Review 重点

- 是否接受 `App.tsx` 的拆分边界：页面编排在 App，图纸交互在 `DesignPlanTab`/`FloorPlanCanvas`，通道映射在 `VideoChannelTab`。
- 门店列表字段是否符合 PRD 第一阶段口径。
- 添加门店浮层是否满足“设计图或录像机至少一个”的产品规则。
- 通道映射交互状态是否足够支撑后续真实 API 接入。
- 样式是否保持现有简洁后台风格，没有偏离设计图标注体验。
