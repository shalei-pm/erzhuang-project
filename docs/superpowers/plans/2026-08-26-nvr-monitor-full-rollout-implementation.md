# 工控机监控全量替换 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将日常 H5 监控入口切换为工控机 NVR 取流，保持原有入口路径、角色授权和可快速恢复的萤石回滚能力。

**Architecture:** 新建通用 `internal/nvrmonitor` 模块，复用 `resourceview` 的只读同步表数据和已验证的 NVR 鉴权客户端。`MONITOR_PLAYBACK_MODE` 只在服务端决定正常 H5 路由采用 NVR 或 legacy 萤石实现；NVR 模式下将门店、摄像头和流会话三个接口逐层做准入与授权校验，前端继续使用既有 H5 页面信息架构和已验收的 NVR 播放器体验。

**Tech Stack:** Go 1.22、`net/http`、MySQL、React、TypeScript、Vite、NVRPlayer SDK、Vitest。

---

## Files and Responsibilities

- Create: `internal/nvrmonitor/models.go` - NVR 监控 API 契约、模式、稳定错误定义。
- Create: `internal/nvrmonitor/service.go` - 有效摄像头过滤、门店分组、空间显示、会话签发前校验。
- Create: `internal/nvrmonitor/handler.go` - 通用 API 路由、请求校验、`Cache-Control: no-store` 与脱敏错误映射。
- Create: `internal/nvrmonitor/*_test.go` - 服务、路由、越权和敏感数据回归测试。
- Create: `internal/app/monitor_mode.go` - 运行时 `MONITOR_PLAYBACK_MODE` 解析和默认安全行为。
- Modify: `cmd/server/main.go` - 用 `K8S_SECRET_NVR_STREAM_AUTHORIZATION` 创建通用 NVR 服务，注入模式配置。
- Modify: `internal/app/handler.go` - 注册当前模式下的正常 H5 路由、资源详情入口口径、旧频道深链接保护。
- Modify: `internal/app/authz.go` - 将现有 SSO 用户和 `monitor:view` scope 适配为 NVR 服务端授权器。
- Modify: `internal/resourceview/repository.go` and `internal/resourceview/mysql_repository.go` - 提供仅查有效 NVR 摄像头门店的仓储读取，保留 3.0 门店空间资源列表原有 edge 口径。
- Modify: `frontend/src/domain/nvr-lab.ts` and tests - 替换为通用 NVR 监控领域模型和路径工具，保留固定小时窗口逻辑。
- Modify: `frontend/src/api-nvr-lab.ts` - 替换为通用 NVR API client，不暴露 Authorization 或 token。
- Modify: `frontend/src/pages/NVRLabMonitor.tsx`, `frontend/src/pages/NVRLabCamera.tsx`, `frontend/src/components/NVRLabPlayer.tsx` - 复用已验收 UI，移除 10001/实验表述并接入正常机构路由。
- Modify: `frontend/src/components/H5StoreSwitcher.tsx`, `frontend/src/App.tsx`, `frontend/src/domain/store-detail-navigation.ts` - 正常入口、门店导航和返回路径使用当前模式的 API；旧频道深链接显示可恢复提示。
- Modify: `docs/codex-learning-state.md`, `docs/decisions.md`, `work/current-plan.md` - 记录实现、测试发布、模式切换和回滚结果。

### Task 1: 固化运行时模式与默认回滚行为

**Files:**
- Create: `internal/app/monitor_mode.go`
- Create: `internal/app/monitor_mode_test.go`
- Modify: `cmd/server/main.go`
- Test: `cmd/server/main_test.go`

- [ ] **Step 1: 写出模式解析失败测试**

```go
func TestMonitorPlaybackModeFromEnv(t *testing.T) {
    t.Setenv("MONITOR_PLAYBACK_MODE", "nvr")
    if got := app.MonitorPlaybackModeFromEnv(); got != app.MonitorPlaybackModeNVR {
        t.Fatalf("mode = %q", got)
    }
    t.Setenv("MONITOR_PLAYBACK_MODE", "unexpected")
    if got := app.MonitorPlaybackModeFromEnv(); got != app.MonitorPlaybackModeLegacy {
        t.Fatalf("mode = %q, want legacy", got)
    }
}
```

- [ ] **Step 2: 运行测试确认缺失**

Run: `go test ./internal/app ./cmd/server`

Expected: FAIL，因为 `MonitorPlaybackModeFromEnv` 尚不存在。

- [ ] **Step 3: 实现模式值与安全默认值**

```go
type MonitorPlaybackMode string

const (
    MonitorPlaybackModeLegacy MonitorPlaybackMode = "legacy"
    MonitorPlaybackModeNVR    MonitorPlaybackMode = "nvr"
)

func MonitorPlaybackModeFromEnv() MonitorPlaybackMode {
    if strings.EqualFold(strings.TrimSpace(os.Getenv("MONITOR_PLAYBACK_MODE")), string(MonitorPlaybackModeNVR)) {
        return MonitorPlaybackModeNVR
    }
    return MonitorPlaybackModeLegacy
}
```

`cmd/server/main.go` 必须把该模式传入应用构造器；未设置、拼写错误或 NVR Secret 缺失时一律保留 `legacy`，不让生产意外进入不可播放空页。

- [ ] **Step 4: 运行后端相关测试**

Run: `go test ./internal/app ./cmd/server`

Expected: PASS。

- [ ] **Step 5: 提交独立回滚开关基线**

```bash
git add internal/app/monitor_mode.go internal/app/monitor_mode_test.go cmd/server/main.go cmd/server/main_test.go
git commit -m "feat: add monitor playback mode switch"
```

### Task 2: 提供只读 NVR 准入数据读取

**Files:**
- Modify: `internal/resourceview/repository.go`
- Modify: `internal/resourceview/mysql_repository.go`
- Modify: `internal/resourceview/mysql_repository_test.go`

- [ ] **Step 1: 为准入查询写 SQL 回归测试**

测试需断言新查询同时含有如下条件，且不修改原有 `listStoreBaseSQL` 的 edge 口径：

```sql
d.category = 'camera'
and d.provider = 'HikVisionNvrChannel'
and d.status = 1
and d.deleted_at is null
```

另断言门店状态仍要求 `t.status = 1`，查询结果按 `t.city_id, t.id` 稳定排序。

- [ ] **Step 2: 运行仓储测试确认先失败**

Run: `go test ./internal/resourceview -run 'NVR|Monitor'`

Expected: FAIL，因为 NVR 准入仓储方法与 SQL 常量未定义。

- [ ] **Step 3: 扩展 Repository，不改变资源查看原入口**

```go
type Repository interface {
    ListStores(ctx context.Context, filters StoreFilters) ([]StoreRecords, error)
    ListNVRMonitorStores(ctx context.Context) ([]StoreRecords, error)
    GetStoreRecords(ctx context.Context, tenantID int64) (StoreRecords, error)
}
```

`ListNVRMonitorStores` 使用独立基础 SQL 找出具备有效工控机摄像头的启用门店，再复用 `getStoreRecordsForTenant` 补齐摄像头、空间、关系和旧缩略图。不得向 `tb_crm_*` 任何表写数据，也不得把旧萤石通道作为新准入条件。

- [ ] **Step 4: 运行仓储与资源视图全量测试**

Run: `go test ./internal/resourceview`

Expected: PASS。

- [ ] **Step 5: 提交只读准入读取**

```bash
git add internal/resourceview/repository.go internal/resourceview/mysql_repository.go internal/resourceview/mysql_repository_test.go
git commit -m "feat: add nvr monitor store eligibility query"
```

### Task 3: 把 10001 实验服务演进为通用 NVR 监控服务

**Files:**
- Create: `internal/nvrmonitor/models.go`
- Create: `internal/nvrmonitor/service.go`
- Create: `internal/nvrmonitor/service_test.go`
- Modify: `internal/nvrlab/authorization_client.go`
- Modify: `internal/nvrlab/authorization_client_test.go`

- [ ] **Step 1: 写通用服务的行为测试**

至少覆盖：

```go
func TestListStoresOnlyReturnsEligibleCameras(t *testing.T)
func TestGetCamerasRejectsCameraOutsideTenant(t *testing.T)
func TestCreateSessionValidatesLiveAndOneHourPlayback(t *testing.T)
func TestCreateSessionNeverReturnsAuthorizationCredential(t *testing.T)
func TestCamerasKeepHourlyPlaybackWindowSeparateFromSeekStart(t *testing.T)
```

测试数据包含：同门店有效摄像头、`provider` 不匹配摄像头、`status=0` 摄像头、未绑定空间摄像头以及 `area_id` 父级为 `2387` 的忽略关系。断言服务只输出有效摄像头，并维持已确认空间类型展示规则：`level=3` 显示为“治疗室”，其他空间显示父级名称和本级名称。

- [ ] **Step 2: 运行服务测试确认先失败**

Run: `go test ./internal/nvrmonitor`

Expected: FAIL，因为包尚不存在。

- [ ] **Step 3: 定义 API 模型和服务边界**

```go
type Authorizer interface {
    CanViewStore(ctx context.Context, externalOrgID string) (bool, error)
    FilterStores(ctx context.Context, stores []StoreInfo) ([]StoreInfo, error)
}

type Service struct {
    repository    resourceview.Repository
    authorization AuthorizationClient
}

func (s *Service) ListStores(ctx context.Context) (MonitorStoresResponse, error)
func (s *Service) GetCameras(ctx context.Context, externalOrgID string) (CameraListResponse, error)
func (s *Service) CreateSession(ctx context.Context, externalOrgID string, cameraID int64, request StreamSessionRequest) (StreamSessionResponse, error)
```

`CreateSession` 必须先取本门店有效摄像头集合，再调用鉴权服务；camera ID 不匹配、门店无准入或上游无 token 时不调用播放器也不暴露 token。可复用 `nvrlab.HTTPAuthorizationClient`，但其对外接口与包名应迁移到 `nvrmonitor` 或抽到无实验语义的小包，避免新模块依赖实验名称。

- [ ] **Step 4: 运行通用服务与鉴权客户端测试**

Run: `go test ./internal/nvrmonitor ./internal/nvrlab`

Expected: PASS，且上游失败测试只允许稳定分类码，例如 `nvr_stream_authorization_upstream_http_401`。

- [ ] **Step 5: 提交通用 NVR 服务**

```bash
git add internal/nvrmonitor internal/nvrlab/authorization_client.go internal/nvrlab/authorization_client_test.go
git commit -m "feat: add reusable nvr monitor service"
```

### Task 4: 注册 NVR 路由并强制执行门店范围授权

**Files:**
- Create: `internal/nvrmonitor/handler.go`
- Create: `internal/nvrmonitor/handler_test.go`
- Modify: `internal/app/authz.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/app/handler_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 写路由与越权测试**

```go
GET  /api/h5/nvr-monitor/stores
GET  /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras
POST /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras/{cameraId}/stream-session
```

断言：`viewer` 只能得到 `monitor:view` scope 中的门店，直接请求无授权门店和跨门店 camera ID 都为 `403` 或 `404` 的稳定中文错误；`admin`、`editor` 可见全部准入门店；流会话响应带 `Cache-Control: no-store`；响应和日志替身均不含 Authorization、JWT 或 `wss://` URL 以外的凭据字段。

- [ ] **Step 2: 运行测试确认路由缺失**

Run: `go test ./internal/nvrmonitor ./internal/app -run 'NVRMonitor|MonitorAccess'`

Expected: FAIL，因为新路由和授权适配尚未注册。

- [ ] **Step 3: 用现有 scope 规则实现授权适配**

在 `internal/app/authz.go` 中以当前 `currentAuthUser`、`normalizeRole` 和 `CanUserViewMonitorStore` 实现 NVR authorizer：

```go
func (a nvrMonitorAuthorizer) CanViewStore(ctx context.Context, externalOrgID string) (bool, error) {
    return a.handler.store.CanUserViewMonitorStore(ctx, a.user, externalOrgID)
}
```

实际实现需从 `http.Request` 取得用户，避免把用户从前端请求体传入。`admin` 和 `editor` 依赖已有 `CanUserViewMonitorStore` 的全量语义，`viewer` 仍使用数据库 scope。通用服务无权直接读取 HTTP cookie。

- [ ] **Step 4: 在 NVR 模式注册新路由并保护旧深链接**

在 `internal/app/handler.go`：

```go
if monitorMode == MonitorPlaybackModeNVR && nvrMonitorService != nil {
    nvrmonitor.RegisterRoutesWithAuthorizer(mux, nvrMonitorService, nvrMonitorAuthorizer{handler: handler})
} else if h5MonitorService != nil {
    h5monitor.RegisterRoutesWithAuthorizer(mux, h5MonitorService, h5MonitorAuthorizer{handler: handler})
}
```

不要同时让同一路径由两个 handler 注册。旧 `/channels/{channelId}` API 和前端深链接在 NVR 模式必须返回/展示“监控地址已更新”，并回到对应机构新摄像头列表；不得用旧 channel ID 猜 camera ID。

- [ ] **Step 5: 运行应用集成测试**

Run: `go test ./internal/app ./internal/nvrmonitor ./cmd/server`

Expected: PASS。

- [ ] **Step 6: 提交通用路由与权限控制**

```bash
git add internal/nvrmonitor internal/app/authz.go internal/app/handler.go internal/app/handler_test.go cmd/server/main.go
git commit -m "feat: route h5 monitor through authorized nvr sessions"
```

### Task 5: 让资源查看入口遵循当前模式和 NVR 准入口径

**Files:**
- Modify: `internal/app/handler.go`
- Modify: `internal/app/handler_test.go`
- Modify: `internal/resourceview/service_test.go`
- Modify: `frontend/src/domain/store-detail-navigation.ts`
- Modify: `frontend/src/domain/store-detail-navigation.test.ts`

- [ ] **Step 1: 写 NVR 模式资源入口测试**

测试三个结果：有效 NVR 摄像头门店的 `can_view_monitor=true` 且 URL 为 `/h5/orgs/{tenant}/monitor`；无有效 NVR 摄像头门店为 `false` 且无 URL；普通查看无 scope 即使有摄像头也为 `false`。legacy 模式必须保持既有萤石判定，保证环境开关能真实回滚。

- [ ] **Step 2: 运行测试确认先失败**

Run: `go test ./internal/app ./internal/resourceview -run 'MonitorURL|CanViewMonitor'`

Expected: FAIL，因为资源入口目前只看旧监控范围，不知道播放模式与 NVR 准入。

- [ ] **Step 3: 在 `resourceViewMonitorAccess` 组合权限与准入**

实现顺序固定为：认证用户 -> `CanUserViewMonitorStore` -> 当前模式可用性 -> URL。NVR 模式使用同一份 `nvrmonitor` 有效摄像头判断；legacy 模式保持旧逻辑。前端仍只消费 `canViewMonitor` 和 `monitorUrl`，不推断 NVR 资格。

- [ ] **Step 4: 运行相关测试**

Run: `go test ./internal/app ./internal/resourceview`

Expected: PASS。

- [ ] **Step 5: 提交资源入口兼容改造**

```bash
git add internal/app/handler.go internal/app/handler_test.go internal/resourceview/service_test.go frontend/src/domain/store-detail-navigation.ts frontend/src/domain/store-detail-navigation.test.ts
git commit -m "feat: gate monitor entry by playback mode"
```

### Task 6: 将前端实验页复用为正常 H5 NVR 监控页

**Files:**
- Modify: `frontend/src/domain/nvr-lab.ts`
- Modify: `frontend/src/domain/nvr-lab.test.ts`
- Modify: `frontend/src/api-nvr-lab.ts`
- Modify: `frontend/src/pages/NVRLabMonitor.tsx`
- Modify: `frontend/src/pages/NVRLabCamera.tsx`
- Modify: `frontend/src/components/NVRLabPlayer.tsx`
- Modify: `frontend/src/components/NVRLabPlayer.test.tsx`
- Modify: `frontend/src/components/NVRLabHourlyPlaybackPicker.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: 为通用路径与 API client 写前端失败测试**

```ts
expect(parseNVRMonitorRoute("/erzhuang-project/h5/orgs/10019/monitor"))
  .toEqual({ name: "home", externalOrgId: "10019" });
expect(parseNVRMonitorRoute("/erzhuang-project/h5/orgs/10019/monitor/cameras/111"))
  .toEqual({ name: "camera", externalOrgId: "10019", cameraId: 111 });
expect(nvrMonitorCameraPath("10019", 111)).toBe("/erzhuang-project/h5/orgs/10019/monitor/cameras/111");
```

保留当前回放窗口与 seek 会话分离测试，确保拖到中段或末段时滑块仍按原一小时比例定位。

- [ ] **Step 2: 运行前端测试确认先失败**

Run: `cd frontend && npm test -- --run src/domain/nvr-lab.test.ts src/components/NVRLabPlayer.test.tsx`

Expected: FAIL，因为正常机构路由与通用 API 尚未实现。

- [ ] **Step 3: 将接口和页面改为按机构请求**

```ts
listCameras(externalOrgId: string) {
  return requestJSON(`${API_BASE}/h5/nvr-monitor/orgs/${encodeURIComponent(externalOrgId)}/cameras`);
}

createStreamSession(externalOrgId: string, cameraId: number, mode: NVRMonitorMode, startTime?: number, endTime?: number) {
  return requestJSON(`${API_BASE}/h5/nvr-monitor/orgs/${encodeURIComponent(externalOrgId)}/cameras/${encodeURIComponent(String(cameraId))}/stream-session`, {
    method: "POST",
    body: JSON.stringify({ mode, start_time: startTime, end_time: endTime }),
  });
}
```

`NVRLabMonitor`、`NVRLabCamera` 与 CSS class 可以在本批保留文件名以控制改动范围，但页面可见文案不得再出现“实验”或“10001”。H5 页面的顶部、门店切换、Tab、摄像头卡片、返回路径和移动端信息密度应继续复用已验收的 2.x 结构。播放器静音、暂停/恢复、全屏和回放进度控制不得回退。

- [ ] **Step 4: 处理旧频道深链接**

`App.tsx` 在 NVR 模式解析 `/h5/orgs/{org}/monitor/channels/{oldId}` 时，渲染明确提示和“返回摄像头列表”命令；不调用旧萤石 API，不把 `oldId` 放进 NVR API。

- [ ] **Step 5: 运行前端单测与构建**

Run: `cd frontend && npm test -- --run`

Expected: PASS。

Run: `cd frontend && npm run build`

Expected: PASS，允许既有 chunk-size warning，不允许类型错误或新控制台警告。

- [ ] **Step 6: 用 Chrome 插件做视觉验收**

在测试环境以已登录管理员完成：门店切换、至少三家城市门店直播、至少一组有录像样本的日期/小时选择、中段/末段进度拖动、声音、暂停、全屏、缺缩略图、无准入入口、旧深链接提示。再用普通查看账号验证导航和直接 URL/API 越权。

- [ ] **Step 7: 提交前端正常路由替换**

```bash
git add frontend/src/domain/nvr-lab.ts frontend/src/domain/nvr-lab.test.ts frontend/src/api-nvr-lab.ts frontend/src/pages/NVRLabMonitor.tsx frontend/src/pages/NVRLabCamera.tsx frontend/src/components/NVRLabPlayer.tsx frontend/src/components/NVRLabPlayer.test.tsx frontend/src/components/NVRLabHourlyPlaybackPicker.test.tsx frontend/src/App.tsx frontend/src/styles.css
git commit -m "feat: use nvr player for h5 monitor routes"
```

### Task 7: 完成测试发布、回滚演练和项目记忆

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: 执行最终检查**

Run: `git diff --check`

Expected: 无输出。

Run: `rg -n -i "authorization|token|wss://" internal/nvrmonitor internal/app frontend/src/api-nvr-lab.ts`

Expected: 只允许固定字段名、脱敏测试或后端构造器；不得有 Secret 值、日志输出或前端长期 token。

Run: `go test ./internal/app ./internal/resourceview ./internal/nvrmonitor ./internal/nvrlab ./cmd/server`

Expected: PASS；若本机 Go 受 macOS `LC_UUID` 限制，运行 `go test -c` 记录为编译门禁，Wharf 构建补全验证。

Run: `cd frontend && npm test -- --run && npm run build`

Expected: PASS。

- [ ] **Step 2: 发布至公司测试环境**

按 `docs/deploy-runbook.md`：将已验证提交同步 GitHub 备份，再将公司测试分支合入并正常推送 `gitlab/codex/containerize-single-image`。禁止 force push，禁止手工重复部署。等待 pipeline `752` 自动部署。

- [ ] **Step 3: 完成 NVR 与回滚演练**

测试 K8s 设置 `MONITOR_PLAYBACK_MODE=nvr` 并验证 Task 6 清单；随后切为 `legacy`、滚动重启并验证萤石 H5 入口、门店导航和一个可播放样本恢复。两次模式值、部署 commit、健康检查和验收账号角色都记录到长期状态文档。

- [ ] **Step 4: 形成正式发布前置清单**

确认正式环境存在 `K8S_SECRET_NVR_STREAM_AUTHORIZATION`，业务同步表已就绪，`MONITOR_PLAYBACK_MODE=nvr` 通过测试，且上一个 MySQL/OSS 兼容稳定 commit 可回退。只有用户明确要求正式发布时才合入 GitLab `main`、等待 pipeline `771`、在 Wharf 部署并走审批。

- [ ] **Step 5: 提交版本、验证和交接记录**

```bash
git add VERSION docs/codex-learning-state.md docs/decisions.md work/current-plan.md
git commit -m "docs: record nvr monitor rollout verification"
```

## Plan Self-Review

- 设计覆盖：门店准入对应 Task 2，服务端会话与脱敏对应 Task 3/4，角色与 scope 对应 Task 4/5，正常路由及既有播放器体验对应 Task 6，运行时和 Git 回滚对应 Task 1/7。
- 非目标：计划不删除萤石代码、账号或历史数据；不新增 DDL、不写 `tb_crm_*`；不增加抓图或并发限制。
- 一致性：NVR API 固定使用 `/api/h5/nvr-monitor`，正常前端路由固定使用 `/h5/orgs/{externalOrgId}/monitor`，所有会话请求统一携带 `mode` 与可选 Unix 秒级 `start_time`/`end_time`。
- 发布门禁：先测试分支，完成 `nvr -> legacy` 演练后才具备正式发布条件。
