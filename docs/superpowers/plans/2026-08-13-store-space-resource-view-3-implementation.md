# 门店空间资源查看 3.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将二壮门店空间资源主流程升级为基于公司业务库的只读「门店空间资源查看」，展示已部署工控机门店的空间、设备和绑定完整性，同时保留现有 H5 Monitor 播放链路。

**Architecture:** 新增独立只读模块 `internal/resourceview` 读取业务库四张表，并由 `internal/app` 注册新的只读 API。前端新增资源查看类型和组件，替换后台主列表/详情的旧维护入口；H5 Monitor 路由、萤石云播放和 viewer 监控门店范围权限保持原有边界。

**Tech Stack:** Go 1.22 `net/http` + MySQL read-only connection, React + TypeScript + Vite + Ant Design, 公司 GitLab/K8s 自动发布。

---

## Scope

### In Scope

- 公司业务库只读连接和配置解析。
- 只读查询 `tb_crm_admin_tenant`、`tb_crm_iot_device`、`tb_crm_consulting_room`、`tb_crm_iot_area_device_relation`。
- 门店列表只展示有启用工控机的门店。
- 门店详情展示三层业务空间树、设备树、空间-摄像头绑定关系和异常项。
- 后台主模块名称统一为「门店空间资源查看」。
- 3.0 主流程隐藏新增、编辑、删除、扫描、识别、确认、设计图上传和标注入口。
- 普通查看用户的监控门店范围权限继续控制「查看监控」入口。
- 发布前给当前 2.x 状态建立 tag 或备份分支。

### Out Of Scope

- 不改 H5 Monitor 路由、播放器、萤石云取流、Windows H.265 fallback。
- 不接入工控机/NVR 取流。
- 不在二壮侧写业务库或回写映射验收状态。
- 不删除旧 storespace/designplan 代码和数据库表。
- 不把任何业务库账号、密码、DSN 或 token 写入仓库。

## File Structure

- Create `internal/resourceview/models.go`  
  定义 3.0 只读 API 的门店、空间、设备、绑定、异常项和列表统计模型。
- Create `internal/resourceview/repository.go`  
  定义 `Repository` 接口、查询过滤条件和业务库原始记录结构。
- Create `internal/resourceview/mysql_repository.go`  
  使用业务库只读 MySQL 连接查询四张业务表。
- Create `internal/resourceview/service.go`  
  将业务表记录聚合为列表摘要、空间树、设备树和异常项。
- Create `internal/resourceview/handler.go`  
  注册 `GET /api/store-space-resource-view/stores` 和 `GET /api/store-space-resource-view/stores/{tenantId}`。
- Create `internal/resourceview/service_test.go`  
  覆盖聚合、通道号解析、空间树、设备树和异常项。
- Create `internal/resourceview/mysql_repository_test.go`  
  源码守卫测试，防止写 SQL、`securityVideoUrl`、`content_id`、旧识别接口混入 3.0。
- Modify `cmd/server/main.go`  
  增加业务库只读连接配置 `BUSINESS_MYSQL_DSN` / `K8S_SECRET_BUSINESS_MYSQL_DSN`，并接入 `resourceview`。
- Modify `cmd/server/main_test.go`  
  覆盖业务库 DSN parseTime 注入和缺省不启用资源查看 API 的配置。
- Modify `internal/app/handler.go`  
  新增 handler 注入参数，注册 `resourceview` 路由，复用现有 auth 和 monitor visibility resolver。
- Modify `internal/app/handler_test.go`  
  覆盖未配置业务库时 API 返回清晰错误、已配置时只读 API 可访问、viewer 监控入口权限不回退。
- Modify `frontend/src/api.ts`  
  新增 `storeSpaceResourceViewApi` 和 3.0 类型映射。
- Create `frontend/src/domain/resource-view.ts`  
  放置空间树排序、设备状态文案、异常级别、通道号展示等纯函数。
- Create `frontend/src/domain/resource-view.test.ts`  
  覆盖前端展示规则和异常排序。
- Create `frontend/src/components/ResourceStoreList.tsx`  
  新的只读门店列表表格。
- Create `frontend/src/components/ResourceStoreDetail.tsx`  
  新的只读详情页，包含空间视角、设备视角、异常项。
- Modify `frontend/src/App.tsx`  
  后台主流程从旧 store-space API 切到 3.0 resource view API，移除主页面旧写操作入口。
- Modify `frontend/src/styles.css`  
  添加 3.0 列表、详情、树形关系和异常项样式。
- Modify `frontend/src/api.test.ts`  
  覆盖 3.0 API path、snake/camel 映射和 H5 Monitor path 保持不变。
- Modify `README.md`  
  更新当前业务目标、3.0 模块和只读业务库连接说明。
- Modify `docs/codex-learning-state.md`  
  记录 3.0 设计进入实现计划阶段。
- Modify `docs/decisions.md`  
  记录 3.0 只读业务库方案决策。
- Modify `work/current-plan.md`  
  更新当前轮目标、任务状态和下一步。
- Create `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`  
  在正式编码前记录 2.x 稳定备份点、tag/分支、线上状态、回滚方式、已知风险和后续读取顺序。

## Implementation Tasks

### Task 0: 封版当前 2.x 稳定状态

**Files:**
- Read: `VERSION`
- Read: `docs/codex-learning-state.md`
- Create: `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`
- No code modifications in this task.

- [ ] **Step 1: Inspect current branch and changed files**

Run:

```bash
git status --short
git branch --show-current
git rev-parse --short HEAD
cat VERSION
```

Expected:

```text
Current branch is codex/containerize-single-image or a fresh codex/3-resource-view branch.
VERSION is 2.31.x before 3.0 work begins.
Only intentional docs/prototype changes are dirty.
```

- [ ] **Step 2: Create a backup tag for the last stable 2.x commit**

Run after confirming the exact stable commit:

```bash
git tag -a v2.31-stable-before-resource-view-3 -m "backup: stable 2.x before resource view 3.0"
git show --stat v2.31-stable-before-resource-view-3
```

Expected:

```text
Tag points to the reviewed 2.x commit, usually the latest deployed company branch commit.
```

- [ ] **Step 3: Write the 2.x backup handoff document**

Create `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`:

```markdown
# 2.x Stable Backup Before Resource View 3.0

日期：2026-08-13

## 备份目的

在二壮 3.0「门店空间资源查看」大版本开发前，固定当前 2.x 稳定状态，方便后续新会话、发布会话或回滚会话快速判断：3.0 之前线上可用版本在哪里、能回到哪里、哪些能力不应丢失。

## 备份点

- 当前分支：`codex/containerize-single-image`
- 稳定 tag：`v2.31-stable-before-resource-view-3`
- 稳定 commit：执行备份时填写 `git rev-parse --short HEAD` 的结果。
- 版本文件：执行备份时填写 `cat VERSION` 的结果。
- 公司发布分支：`gitlab/codex/containerize-single-image`
- GitHub 备份分支：`origin/codex/containerize-single-image`

## 2.x 稳定能力范围

- 公司运行时：MySQL + OSS。
- 登录：APISIX SSO + `tb_users` 授权。
- 权限：`admin` / `editor` / `viewer`，普通查看用户监控门店范围权限已实现。
- 门店空间资源旧主流程：门店列表、门店详情、设计图标注、通道映射、AI 通道识别、人工确认、门店/录像机/通道写接口。
- H5 Monitor：萤石云直播/回放、区域 Tab 返回保持、Windows H.265 fallback。
- 发布链路：只走公司 GitLab/K8s，不走韩国 Lighthouse。

## 3.0 开发边界

- 3.0 将后台主流程切换为只读「门店空间资源查看」。
- 3.0 不改 H5 Monitor 播放链路。
- 3.0 不删除旧 storespace/designplan 代码和数据；旧能力先作为回滚与历史兼容保留。
- 3.0 新增业务库只读连接时，禁止把业务库 DSN、账号、密码、token 写入仓库。

## 回滚方式

如果 3.0 发布后阻断核心使用，优先回滚到仍兼容 MySQL/OSS 的 2.x 稳定点：

```bash
git switch codex/containerize-single-image
git revert <3.0-merge-commit>
git push gitlab codex/containerize-single-image
```

如需查看 2.x 备份点：

```bash
git show --stat v2.31-stable-before-resource-view-3
git diff v2.31-stable-before-resource-view-3..HEAD --stat
```

## 后续读取顺序

1. `docs/codex-learning-state.md`
2. `docs/decisions.md`
3. `work/current-plan.md`
4. `docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`
5. `docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`
6. 本文件

## 备份后验证清单

- `git show --stat v2.31-stable-before-resource-view-3` 可读取。
- `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md` 已提交。
- 线上 `/erzhuang-project/health` 仍返回 `database=mysql`、`asset_store=oss`。
- H5 Monitor 样本门店仍可打开。
- 用户管理仍可打开，viewer 门店范围权限不回退。
```

Run:

```bash
git add docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md
git commit -m "docs: archive 2x stable backup point"
```

Expected:

```text
The backup tag and its handoff document can be read independently by future sessions.
```

- [ ] **Step 4: Create a feature branch**

Run:

```bash
git switch -c codex/store-space-resource-view-3
```

Expected:

```text
Switched to a new branch 'codex/store-space-resource-view-3'
```

- [ ] **Step 5: Commit only the approved design and plan documents if still uncommitted**

Run:

```bash
git add docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md
git commit -m "docs: design store space resource view 3"
```

Expected:

```text
Commit contains only the 3.0 design and implementation plan.
```

### Task 1: Add Resource View Domain Models

**Files:**
- Create: `internal/resourceview/models.go`
- Create: `internal/resourceview/service_test.go`

- [ ] **Step 1: Write model and aggregation tests**

Create `internal/resourceview/service_test.go` with:

```go
package resourceview

import "testing"

func TestBuildStoreDetailCreatesThreeLevelSpaceTreeAndDeviceTree(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10019, Name: "上海陆家嘴店", HospitalName: "新氧青春诊所(上海陆家嘴店)", Status: 1, CityID: 9},
		Devices: []BusinessDevice{
			{ID: 1, TenantID: 10019, Name: "edge-1", HardwareID: "60beb422a54f", Category: "edge", Status: 1, OnlineStatus: 1},
			{ID: 22, TenantID: 10019, Name: "nvr-1", HardwareID: "NVR001", Category: "nvr", Status: 1, OnlineStatus: 1},
			{ID: 68, TenantID: 10019, ParentID: 22, Name: "治疗室1", HardwareID: "NVRCHANNEL:22-1", Category: "camera", Status: 1, OnlineStatus: 1},
		},
		Spaces: []BusinessSpace{
			{ID: 10, TenantID: 10019, Name: "治疗区域", Level: 1, Status: 1},
			{ID: 11, TenantID: 10019, ParentID: 10, Name: "治疗室1", Level: 2, Status: 1},
			{ID: 12, TenantID: 10019, ParentID: 11, Name: "床位1", Level: 3, Status: 1},
		},
		Relations: []BusinessAreaDeviceRelation{{ID: 99, AreaID: 12, DeviceID: 68, FunctionType: "camera"}},
	}

	detail := BuildStoreDetail(records, MonitorAccess{CanViewMonitor: true})

	if detail.TenantID != 10019 {
		t.Fatalf("tenant id = %d, want 10019", detail.TenantID)
	}
	if len(detail.SpaceTree) != 1 || len(detail.SpaceTree[0].Children) != 1 || len(detail.SpaceTree[0].Children[0].Children) != 1 {
		t.Fatalf("space tree was not three levels: %#v", detail.SpaceTree)
	}
	if len(detail.DeviceTree.Edges) != 1 {
		t.Fatalf("edge count = %d, want 1", len(detail.DeviceTree.Edges))
	}
	if len(detail.DeviceTree.NVRs) != 1 || len(detail.DeviceTree.NVRs[0].Cameras) != 1 {
		t.Fatalf("nvr camera tree = %#v", detail.DeviceTree.NVRs)
	}
	camera := detail.DeviceTree.NVRs[0].Cameras[0]
	if camera.ChannelNo == nil || *camera.ChannelNo != 1 {
		t.Fatalf("channel no = %#v, want 1", camera.ChannelNo)
	}
	if len(camera.SpacePaths) != 1 || camera.SpacePaths[0] != "治疗区域 / 治疗室1 / 床位1" {
		t.Fatalf("space paths = %#v", camera.SpacePaths)
	}
	if len(detail.Issues) != 0 {
		t.Fatalf("issues = %#v, want none", detail.Issues)
	}
}

func TestBuildStoreDetailReportsMappingIssues(t *testing.T) {
	records := StoreRecords{
		Tenant: BusinessTenant{ID: 10030, Name: "北京保利实验室门店", Status: 1, CityID: 1},
		Devices: []BusinessDevice{
			{ID: 22, TenantID: 10030, Name: "nvr-1", HardwareID: "NVR001", Category: "nvr", Status: 1, OnlineStatus: 2},
			{ID: 68, TenantID: 10030, ParentID: 22, Name: "摄像头1", HardwareID: "NVRCHANNEL:22-1", Category: "camera", Status: 1, OnlineStatus: 1},
			{ID: 69, TenantID: 10030, ParentID: 404, Name: "摄像头2", HardwareID: "NVRCHANNEL:404-2", Category: "camera", Status: 1, OnlineStatus: 2},
		},
		Spaces: []BusinessSpace{
			{ID: 11, TenantID: 10030, Name: "治疗室1", Level: 2, Status: 0},
		},
		Relations: []BusinessAreaDeviceRelation{
			{ID: 1, AreaID: 11, DeviceID: 68, FunctionType: "camera"},
			{ID: 2, AreaID: 999, DeviceID: 404, FunctionType: "camera"},
		},
	}

	detail := BuildStoreDetail(records, MonitorAccess{})

	assertIssueTypes(t, detail.Issues, []IssueType{
		IssueInactiveBoundSpace,
		IssueMissingCamera,
		IssueMissingNVR,
		IssueOfflineNVR,
		IssueOfflineCamera,
		IssueUnboundCamera,
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/resourceview -o /private/tmp/resourceview.test
```

Expected:

```text
FAIL because package internal/resourceview does not exist.
```

- [ ] **Step 3: Create the model file**

Create `internal/resourceview/models.go` with these public types:

```go
package resourceview

import "time"

type StoreFilters struct {
	Query    string
	CityID   int64
	Page     int
	PageSize int
}

type BusinessTenant struct {
	ID           int64
	Name         string
	HospitalName string
	Status       int
	ProvinceID   int64
	CityID       int64
}

type BusinessDevice struct {
	ID           int64
	TenantID     int64
	ParentID     int64
	Name         string
	HardwareID   string
	SN           string
	IP           string
	Category     string
	Provider     string
	Status       int
	OnlineStatus int
	ExtParams    string
	HeartbeatAt  *time.Time
	DeletedAt    *time.Time
}

type BusinessSpace struct {
	ID        int64
	TenantID  int64
	ParentID  int64
	Name      string
	Code      string
	Level     int
	Status    int
	DictID    int64
	SortOrder int
}

type BusinessAreaDeviceRelation struct {
	ID           int64
	DeviceID     int64
	AreaID       int64
	FunctionType string
	CreatedAt    time.Time
}

type StoreRecords struct {
	Tenant    BusinessTenant
	Devices   []BusinessDevice
	Spaces    []BusinessSpace
	Relations []BusinessAreaDeviceRelation
}

type MonitorAccess struct {
	CanViewMonitor bool
	MonitorURL     string
}

type StoreListResult struct {
	Items    []StoreListItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
	Summary  StoreSummary    `json:"summary"`
	Cities   []CityOption    `json:"cities"`
}

type CityOption struct {
	CityID int64  `json:"city_id"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
}

type StoreSummary struct {
	StoreCount          int `json:"store_count"`
	EdgeCount           int `json:"edge_count"`
	NVRCount            int `json:"nvr_count"`
	CameraCount         int `json:"camera_count"`
	SpaceCount          int `json:"space_count"`
	BoundCameraCount    int `json:"bound_camera_count"`
	UnboundCameraCount  int `json:"unbound_camera_count"`
	OfflineDeviceCount  int `json:"offline_device_count"`
	WarningCount        int `json:"warning_count"`
}

type StoreListItem struct {
	TenantID           int64  `json:"tenant_id"`
	StoreName          string `json:"store_name"`
	HospitalName       string `json:"hospital_name"`
	CityID             int64  `json:"city_id"`
	CityName           string `json:"city_name"`
	EdgeCount          int    `json:"edge_count"`
	NVRCount           int    `json:"nvr_count"`
	CameraCount        int    `json:"camera_count"`
	SpaceCount         int    `json:"space_count"`
	BoundCameraCount   int    `json:"bound_camera_count"`
	UnboundCameraCount int    `json:"unbound_camera_count"`
	OfflineDeviceCount int    `json:"offline_device_count"`
	WarningCount       int    `json:"warning_count"`
	CanViewMonitor     bool   `json:"can_view_monitor"`
	MonitorURL         string `json:"monitor_url,omitempty"`
}

type StoreDetail struct {
	TenantID     int64                        `json:"tenant_id"`
	StoreName    string                       `json:"store_name"`
	HospitalName string                       `json:"hospital_name"`
	CityID       int64                        `json:"city_id"`
	CityName     string                       `json:"city_name"`
	Summary      StoreSummary                 `json:"summary"`
	Edges        []Device                     `json:"edges"`
	NVRs         []Device                     `json:"nvrs"`
	Cameras      []Camera                     `json:"cameras"`
	Spaces       []Space                      `json:"spaces"`
	Relations    []AreaDeviceRelation         `json:"relations"`
	SpaceTree    []SpaceNode                  `json:"space_tree"`
	DeviceTree   DeviceTree                   `json:"device_tree"`
	Issues       []Issue                      `json:"issues"`
	CanViewMonitor bool                       `json:"can_view_monitor"`
	MonitorURL    string                      `json:"monitor_url,omitempty"`
}

type Device struct {
	ID           int64   `json:"id"`
	ParentID     int64   `json:"parent_id,omitempty"`
	TenantID     int64   `json:"tenant_id"`
	Name         string  `json:"name"`
	HardwareID   string  `json:"hardware_id"`
	SN           string  `json:"sn,omitempty"`
	IP           string  `json:"ip,omitempty"`
	Category     string  `json:"category"`
	Provider     string  `json:"provider,omitempty"`
	Status       int     `json:"status"`
	StatusText   string  `json:"status_text"`
	OnlineStatus int     `json:"online_status"`
	OnlineText   string  `json:"online_text"`
	ExtSummary   string  `json:"ext_summary,omitempty"`
	HeartbeatAt  *string `json:"heartbeat_at,omitempty"`
}

type Camera struct {
	Device
	ChannelNo  *int     `json:"channel_no,omitempty"`
	NVRID      int64    `json:"nvr_id,omitempty"`
	NVRName    string   `json:"nvr_name,omitempty"`
	SpacePaths []string `json:"space_paths"`
}

type Space struct {
	ID               int64    `json:"id"`
	TenantID         int64    `json:"tenant_id"`
	ParentID         int64    `json:"parent_id,omitempty"`
	Name             string   `json:"name"`
	Code             string   `json:"code,omitempty"`
	Level            int      `json:"level"`
	Status           int      `json:"status"`
	StatusText       string   `json:"status_text"`
	DictID           int64    `json:"dict_id,omitempty"`
	SortOrder        int      `json:"sort_order"`
	BoundCameraIDs   []int64  `json:"bound_camera_ids"`
	BoundCameraCount int      `json:"bound_camera_count"`
}

type AreaDeviceRelation struct {
	ID           int64  `json:"id"`
	DeviceID     int64  `json:"device_id"`
	AreaID       int64  `json:"area_id"`
	FunctionType string `json:"function_type"`
}

type SpaceNode struct {
	Space
	BoundCameras []Camera    `json:"bound_cameras"`
	Children     []SpaceNode `json:"children"`
}

type DeviceTree struct {
	Edges []Device  `json:"edges"`
	NVRs  []NVRNode `json:"nvrs"`
}

type NVRNode struct {
	Device
	Cameras []Camera `json:"cameras"`
}

type IssueSeverity string
type IssueType string

const (
	IssueSeverityError IssueSeverity = "error"
	IssueSeverityWarn  IssueSeverity = "warning"
	IssueSeverityInfo  IssueSeverity = "info"

	IssueUnboundCamera          IssueType = "unbound_camera"
	IssueInactiveBoundSpace     IssueType = "inactive_bound_space"
	IssueMissingCamera          IssueType = "missing_camera"
	IssueMissingNVR             IssueType = "missing_nvr"
	IssueOfflineEdge            IssueType = "offline_edge"
	IssueOfflineNVR             IssueType = "offline_nvr"
	IssueOfflineCamera          IssueType = "offline_camera"
	IssueCameraBoundManySpaces  IssueType = "camera_bound_many_spaces"
	IssueSpaceBoundManyCameras  IssueType = "space_bound_many_cameras"
)

type Issue struct {
	Severity   IssueSeverity `json:"severity"`
	Type       IssueType     `json:"type"`
	Message    string        `json:"message"`
	EntityType string        `json:"entity_type"`
	EntityID   int64         `json:"entity_id"`
}
```

- [ ] **Step 4: Commit the model skeleton and tests**

Run:

```bash
git add internal/resourceview/models.go internal/resourceview/service_test.go
git commit -m "feat: model resource view responses"
```

### Task 2: Implement Aggregation Service

**Files:**
- Create: `internal/resourceview/service.go`
- Modify: `internal/resourceview/service_test.go`

- [ ] **Step 1: Implement `BuildStoreDetail` and helper functions**

Create `internal/resourceview/service.go` with:

```go
package resourceview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func BuildStoreDetail(records StoreRecords, access MonitorAccess) StoreDetail {
	spaces := normalizedSpaces(records.Spaces)
	devices := normalizedDevices(records.Devices)
	relations := normalizedRelations(records.Relations)
	cameras := buildCameras(devices, relations, spaces)
	issues := buildIssues(devices, spaces, relations, cameras)
	summary := buildSummary(devices, spaces, relations, issues)

	return StoreDetail{
		TenantID:       records.Tenant.ID,
		StoreName:      records.Tenant.Name,
		HospitalName:   records.Tenant.HospitalName,
		CityID:         records.Tenant.CityID,
		CityName:       cityName(records.Tenant.CityID),
		Summary:        summary,
		Edges:          devicesByCategory(devices, "edge"),
		NVRs:           devicesByCategory(devices, "nvr"),
		Cameras:        cameras,
		Spaces:         spaces,
		Relations:      relations,
		SpaceTree:      buildSpaceTree(spaces, cameras, relations),
		DeviceTree:     buildDeviceTree(devices, cameras),
		Issues:         issues,
		CanViewMonitor: access.CanViewMonitor,
		MonitorURL:     access.MonitorURL,
	}
}

func parseNVRChannelHardwareID(value string) *int {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "NVRCHANNEL:") {
		return nil
	}
	no, err := strconv.Atoi(parts[1])
	if err != nil || no <= 0 {
		return nil
	}
	return &no
}

func assertIssueTypes(t interface{ Fatalf(string, ...any) }, issues []Issue, expected []IssueType) {
	counts := map[IssueType]int{}
	for _, issue := range issues {
		counts[issue.Type]++
	}
	for _, issueType := range expected {
		if counts[issueType] == 0 {
			t.Fatalf("missing issue %s in %#v", issueType, issues)
		}
	}
}
```

Add the following private helpers in the same file:

```go
func normalizedSpaces(input []BusinessSpace) []Space {
	out := make([]Space, 0, len(input))
	for _, space := range input {
		out = append(out, Space{
			ID: space.ID, TenantID: space.TenantID, ParentID: space.ParentID,
			Name: strings.TrimSpace(space.Name), Code: strings.TrimSpace(space.Code),
			Level: space.Level, Status: space.Status, StatusText: enabledText(space.Status),
			DictID: space.DictID, SortOrder: space.SortOrder,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level == out[j].Level {
			if out[i].SortOrder == out[j].SortOrder {
				return out[i].ID < out[j].ID
			}
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Level < out[j].Level
	})
	return out
}

func normalizedDevices(input []BusinessDevice) []Device {
	out := make([]Device, 0, len(input))
	for _, device := range input {
		if device.DeletedAt != nil {
			continue
		}
		out = append(out, Device{
			ID: device.ID, ParentID: device.ParentID, TenantID: device.TenantID,
			Name: strings.TrimSpace(device.Name), HardwareID: strings.TrimSpace(device.HardwareID),
			SN: strings.TrimSpace(device.SN), IP: strings.TrimSpace(device.IP),
			Category: strings.TrimSpace(device.Category), Provider: strings.TrimSpace(device.Provider),
			Status: device.Status, StatusText: enabledText(device.Status),
			OnlineStatus: device.OnlineStatus, OnlineText: onlineText(device.OnlineStatus),
			ExtSummary: extSummary(device.ExtParams), HeartbeatAt: formatOptionalTime(device.HeartbeatAt),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].ID < out[j].ID
		}
		return categoryRank(out[i].Category) < categoryRank(out[j].Category)
	})
	return out
}

func normalizedRelations(input []BusinessAreaDeviceRelation) []AreaDeviceRelation {
	out := make([]AreaDeviceRelation, 0, len(input))
	for _, relation := range input {
		out = append(out, AreaDeviceRelation{
			ID: relation.ID, DeviceID: relation.DeviceID, AreaID: relation.AreaID,
			FunctionType: strings.TrimSpace(relation.FunctionType),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func buildCameras(devices []Device, relations []AreaDeviceRelation, spaces []Space) []Camera {
	nvrs := map[int64]Device{}
	for _, device := range devices {
		if device.Category == "nvr" {
			nvrs[device.ID] = device
		}
	}
	spaceByID := map[int64]Space{}
	for _, space := range spaces {
		spaceByID[space.ID] = space
	}
	pathsByCameraID := map[int64][]string{}
	for _, relation := range relations {
		if _, ok := spaceByID[relation.AreaID]; ok {
			pathsByCameraID[relation.DeviceID] = append(pathsByCameraID[relation.DeviceID], spacePath(spaceByID, relation.AreaID))
		}
	}
	cameras := []Camera{}
	for _, device := range devices {
		if device.Category != "camera" {
			continue
		}
		camera := Camera{Device: device, ChannelNo: parseNVRChannelHardwareID(device.HardwareID), NVRID: device.ParentID, SpacePaths: pathsByCameraID[device.ID]}
		if nvr, ok := nvrs[device.ParentID]; ok {
			camera.NVRName = nvr.Name
		}
		cameras = append(cameras, camera)
	}
	return cameras
}

func buildSummary(devices []Device, spaces []Space, relations []AreaDeviceRelation, issues []Issue) StoreSummary {
	cameraIDs := map[int64]struct{}{}
	boundCameraIDs := map[int64]struct{}{}
	offline := 0
	summary := StoreSummary{StoreCount: 1, SpaceCount: len(spaces), WarningCount: len(issues)}
	for _, device := range devices {
		switch device.Category {
		case "edge":
			summary.EdgeCount++
		case "nvr":
			summary.NVRCount++
		case "camera":
			summary.CameraCount++
			cameraIDs[device.ID] = struct{}{}
		}
		if device.OnlineStatus != 1 {
			offline++
		}
	}
	for _, relation := range relations {
		if _, ok := cameraIDs[relation.DeviceID]; ok {
			boundCameraIDs[relation.DeviceID] = struct{}{}
		}
	}
	summary.BoundCameraCount = len(boundCameraIDs)
	summary.UnboundCameraCount = summary.CameraCount - summary.BoundCameraCount
	summary.OfflineDeviceCount = offline
	return summary
}
```

- [ ] **Step 2: Complete issue and tree helpers**

Add to `internal/resourceview/service.go`:

```go
func buildIssues(devices []Device, spaces []Space, relations []AreaDeviceRelation, cameras []Camera) []Issue {
	deviceByID := map[int64]Device{}
	spaceByID := map[int64]Space{}
	for _, device := range devices { deviceByID[device.ID] = device }
	for _, space := range spaces { spaceByID[space.ID] = space }

	boundsByCamera := map[int64]int{}
	boundsBySpace := map[int64]int{}
	for _, relation := range relations {
		device, hasDevice := deviceByID[relation.DeviceID]
		space, hasSpace := spaceByID[relation.AreaID]
		if !hasDevice || device.Category != "camera" {
			continue
		}
		if !hasSpace {
			continue
		}
		boundsByCamera[relation.DeviceID]++
		boundsBySpace[relation.AreaID]++
		if space.Status != 1 {
			issues := []Issue{{Severity: IssueSeverityWarn, Type: IssueInactiveBoundSpace, Message: fmt.Sprintf("空间 %s 已停用但仍绑定摄像头", space.Name), EntityType: "space", EntityID: space.ID}}
			return append(issues, remainingIssues(devices, spaces, relations, cameras, boundsByCamera, boundsBySpace)...)
		}
	}
	return remainingIssues(devices, spaces, relations, cameras, boundsByCamera, boundsBySpace)
}

func remainingIssues(devices []Device, spaces []Space, relations []AreaDeviceRelation, cameras []Camera, boundsByCamera map[int64]int, boundsBySpace map[int64]int) []Issue {
	issues := []Issue{}
	deviceByID := map[int64]Device{}
	spaceByID := map[int64]Space{}
	for _, device := range devices { deviceByID[device.ID] = device }
	for _, space := range spaces { spaceByID[space.ID] = space }
	for _, relation := range relations {
		device, hasDevice := deviceByID[relation.DeviceID]
		if !hasDevice || device.Category != "camera" {
			issues = append(issues, Issue{Severity: IssueSeverityError, Type: IssueMissingCamera, Message: fmt.Sprintf("绑定关系 %d 指向不存在的摄像头", relation.ID), EntityType: "relation", EntityID: relation.ID})
		}
		if _, hasSpace := spaceByID[relation.AreaID]; !hasSpace {
			issues = append(issues, Issue{Severity: IssueSeverityError, Type: IssueMissingCamera, Message: fmt.Sprintf("绑定关系 %d 指向不存在的空间", relation.ID), EntityType: "relation", EntityID: relation.ID})
		}
	}
	for _, camera := range cameras {
		if camera.NVRID > 0 {
			if nvr, ok := deviceByID[camera.NVRID]; !ok || nvr.Category != "nvr" {
				issues = append(issues, Issue{Severity: IssueSeverityError, Type: IssueMissingNVR, Message: fmt.Sprintf("摄像头 %s 的父级 NVR 不存在", camera.Name), EntityType: "camera", EntityID: camera.ID})
			}
		}
		if len(camera.SpacePaths) == 0 {
			issues = append(issues, Issue{Severity: IssueSeverityWarn, Type: IssueUnboundCamera, Message: fmt.Sprintf("摄像头 %s 未绑定空间", camera.Name), EntityType: "camera", EntityID: camera.ID})
		}
		if camera.OnlineStatus != 1 {
			issues = append(issues, Issue{Severity: IssueSeverityWarn, Type: IssueOfflineCamera, Message: fmt.Sprintf("摄像头 %s 离线", camera.Name), EntityType: "camera", EntityID: camera.ID})
		}
	}
	for _, device := range devices {
		if device.Category == "edge" && device.OnlineStatus != 1 {
			issues = append(issues, Issue{Severity: IssueSeverityWarn, Type: IssueOfflineEdge, Message: fmt.Sprintf("工控机 %s 离线", device.Name), EntityType: "device", EntityID: device.ID})
		}
		if device.Category == "nvr" && device.OnlineStatus != 1 {
			issues = append(issues, Issue{Severity: IssueSeverityWarn, Type: IssueOfflineNVR, Message: fmt.Sprintf("NVR %s 离线", device.Name), EntityType: "device", EntityID: device.ID})
		}
	}
	for cameraID, count := range boundsByCamera {
		if count > 1 {
			issues = append(issues, Issue{Severity: IssueSeverityInfo, Type: IssueCameraBoundManySpaces, Message: "同一摄像头绑定了多个空间", EntityType: "camera", EntityID: cameraID})
		}
	}
	for spaceID, count := range boundsBySpace {
		if count > 1 {
			issues = append(issues, Issue{Severity: IssueSeverityInfo, Type: IssueSpaceBoundManyCameras, Message: "同一空间绑定了多个摄像头", EntityType: "space", EntityID: spaceID})
		}
	}
	sortIssues(issues)
	return issues
}
```

Add the remaining deterministic helpers:

```go
func buildSpaceTree(spaces []Space, cameras []Camera, relations []AreaDeviceRelation) []SpaceNode {
	cameraByID := map[int64]Camera{}
	for _, camera := range cameras {
		cameraByID[camera.ID] = camera
	}
	boundBySpace := map[int64][]Camera{}
	for _, relation := range relations {
		if camera, ok := cameraByID[relation.DeviceID]; ok {
			boundBySpace[relation.AreaID] = append(boundBySpace[relation.AreaID], camera)
		}
	}
	spaceIDs := map[int64]struct{}{}
	for _, space := range spaces {
		spaceIDs[space.ID] = struct{}{}
	}
	childrenByParent := map[int64][]Space{}
	for _, space := range spaces {
		parentID := space.ParentID
		if parentID != 0 {
			if _, ok := spaceIDs[parentID]; !ok {
				parentID = 0
			}
		}
		childrenByParent[parentID] = append(childrenByParent[parentID], space)
	}
	var build func(parentID int64) []SpaceNode
	build = func(parentID int64) []SpaceNode {
		nodes := []SpaceNode{}
		for _, space := range childrenByParent[parentID] {
			boundCameras := append([]Camera{}, boundBySpace[space.ID]...)
			cameraIDs := make([]int64, 0, len(boundCameras))
			for _, camera := range boundCameras {
				cameraIDs = append(cameraIDs, camera.ID)
			}
			space.BoundCameraIDs = cameraIDs
			space.BoundCameraCount = len(cameraIDs)
			nodes = append(nodes, SpaceNode{
				Space:        space,
				BoundCameras: boundCameras,
				Children:     build(space.ID),
			})
		}
		return nodes
	}
	return build(0)
}

func buildDeviceTree(devices []Device, cameras []Camera) DeviceTree {
	nvrNodes := []NVRNode{}
	for _, device := range devices {
		if device.Category != "nvr" {
			continue
		}
		node := NVRNode{Device: device}
		for _, camera := range cameras {
			if camera.NVRID == device.ID {
				node.Cameras = append(node.Cameras, camera)
			}
		}
		nvrNodes = append(nvrNodes, node)
	}
	return DeviceTree{Edges: devicesByCategory(devices, "edge"), NVRs: nvrNodes}
}

func devicesByCategory(devices []Device, category string) []Device { out := []Device{}; for _, d := range devices { if d.Category == category { out = append(out, d) } }; return out }
func spacePath(spaces map[int64]Space, id int64) string { names := []string{}; for id != 0 { s, ok := spaces[id]; if !ok { break }; names = append([]string{s.Name}, names...); id = s.ParentID }; return strings.Join(names, " / ") }
func categoryRank(category string) int { switch category { case "edge": return 1; case "nvr": return 2; case "camera": return 3; default: return 9 } }
func enabledText(status int) string { if status == 1 { return "启用" }; return "停用" }
func onlineText(status int) string { if status == 1 { return "在线" }; return "离线" }
func cityName(cityID int64) string { if cityID == 0 { return "" }; return fmt.Sprintf("城市 %d", cityID) }
func extSummary(value string) string { return strings.TrimSpace(value) }
func formatOptionalTime(value *time.Time) *string { if value == nil { return nil }; s := value.Format(time.RFC3339); return &s }
func sortIssues(issues []Issue) { sort.SliceStable(issues, func(i, j int) bool { return issues[i].Type < issues[j].Type }) }
```

- [ ] **Step 3: Run package compile gate**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/resourceview -o /private/tmp/resourceview.test
```

Expected:

```text
No compile errors.
```

- [ ] **Step 4: Commit aggregation service**

Run:

```bash
git add internal/resourceview/service.go internal/resourceview/service_test.go
git commit -m "feat: aggregate business resource view"
```

### Task 3: Add Read-Only Business Repository

**Files:**
- Create: `internal/resourceview/repository.go`
- Create: `internal/resourceview/mysql_repository.go`
- Create: `internal/resourceview/mysql_repository_test.go`

- [ ] **Step 1: Define repository interface**

Create `internal/resourceview/repository.go`:

```go
package resourceview

import "context"

var ErrNotFound = errNotFound()

type Repository interface {
	ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error)
	GetStoreRecords(ctx context.Context, tenantID int64) (StoreRecords, error)
}

func errNotFound() error {
	return resourceViewError("resource view record not found")
}

type resourceViewError string

func (e resourceViewError) Error() string { return string(e) }
```

- [ ] **Step 2: Add repository source guard tests**

Create `internal/resourceview/mysql_repository_test.go`:

```go
package resourceview

import (
	"os"
	"strings"
	"testing"
)

func TestMySQLRepositoryIsReadOnlyAndBusinessTableScoped(t *testing.T) {
	content, err := os.ReadFile("mysql_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(content))
	for _, banned := range []string{
		" insert ", " update ", " delete ", " replace ",
		"securityvideourl", "content_id",
		"recognize", "design_plan",
	} {
		if strings.Contains(source, banned) {
			t.Fatalf("mysql repository contains banned token %q", banned)
		}
	}
	for _, required := range []string{
		"tb_crm_admin_tenant",
		"tb_crm_iot_device",
		"tb_crm_consulting_room",
		"tb_crm_iot_area_device_relation",
		"category = 'edge'",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("mysql repository missing required token %q", required)
		}
	}
}
```

- [ ] **Step 3: Implement MySQL read-only queries**

Create `internal/resourceview/mysql_repository.go` with a repository that only uses `QueryContext` and `QueryRowContext`. Use `exists` subqueries instead of `join` in SQL text to avoid company hook issues that previously blocked `join`.

Key query shape:

```go
const listStoreBaseSQL = `
select t.id, t.name, t.hospital_name, t.status, t.province_id, t.city_id
from tb_crm_admin_tenant t
where t.status = 1
  and exists (
    select 1
    from tb_crm_iot_device d
    where d.tenant_id = t.id
      and d.category = 'edge'
      and d.status = 1
      and d.deleted_at is null
  )
`
```

Use these detail queries:

```go
select id, name, hospital_name, status, province_id, city_id
from tb_crm_admin_tenant
where id = ? and status = 1

select id, tenant_id, coalesce(parent_id, 0), name, hardware_id, sn, ip, category, provider,
       status, online_status, ext_params, heartbeat_at, deleted_at
from tb_crm_iot_device
where tenant_id = ?
  and deleted_at is null
  and category in ('edge', 'nvr', 'camera')

select id, tenant_id, coalesce(parent_id, 0), name, code, level, status, dict_id, sort_order
from tb_crm_consulting_room
where tenant_id = ?

select id, device_id, area_id, function_type, created_at
from tb_crm_iot_area_device_relation
where area_id in (
  select id from tb_crm_consulting_room where tenant_id = ?
)
```

The implementation must scan nullable strings with `sql.NullString`, nullable ints with `sql.NullInt64`, and nullable times with `sql.NullTime`, then convert them into the business record structs.

- [ ] **Step 4: Compile backend package**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/resourceview -o /private/tmp/resourceview.test
```

Expected:

```text
No compile errors. Source guard passes when tests can run in CI.
```

- [ ] **Step 5: Commit repository**

Run:

```bash
git add internal/resourceview/repository.go internal/resourceview/mysql_repository.go internal/resourceview/mysql_repository_test.go
git commit -m "feat: read business resource tables"
```

### Task 4: Wire Backend API and Business DB Config

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `cmd/server/main_test.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/app/handler_test.go`
- Create: `internal/resourceview/handler.go`

- [ ] **Step 1: Add config tests**

Modify `cmd/server/main_test.go` with tests that require:

```go
func TestBusinessDatabaseConfigFromEnvUsesK8SSecret(t *testing.T) {
	t.Setenv("K8S_SECRET_BUSINESS_MYSQL_DSN", "readonly:pass@tcp(mysql:3306)/db_groupbuy")
	config := businessDatabaseConfigFromEnv()
	if config.DSN != "readonly:pass@tcp(mysql:3306)/db_groupbuy?parseTime=true" {
		t.Fatalf("dsn = %q", config.DSN)
	}
}

func TestBusinessDatabaseConfigFromEnvDisabledWhenEmpty(t *testing.T) {
	config := businessDatabaseConfigFromEnv()
	if config.DSN != "" {
		t.Fatalf("dsn = %q, want empty", config.DSN)
	}
}
```

- [ ] **Step 2: Add backend route tests**

Modify `internal/app/handler_test.go` to cover:

```go
func TestResourceViewRoutesReturnNotConfiguredWhenServiceMissing(t *testing.T) {
	handler := NewHandler()
	request := httptest.NewRequest(http.MethodGet, "/api/store-space-resource-view/stores", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "resource_view_not_configured") {
		t.Fatalf("body = %s", response.Body.String())
	}
}
```

- [ ] **Step 3: Implement `resourceview` HTTP handler**

Create `internal/resourceview/handler.go`:

```go
package resourceview

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type MonitorAccessResolver func(r *http.Request, tenantID int64) (MonitorAccess, error)

type Handler struct {
	service *Service
	monitorAccess MonitorAccessResolver
}

func RegisterRoutes(mux *http.ServeMux, service *Service, resolver MonitorAccessResolver) {
	handler := &Handler{service: service, monitorAccess: resolver}
	mux.HandleFunc("GET /api/store-space-resource-view/stores", handler.listStores)
	mux.HandleFunc("GET /api/store-space-resource-view/stores/{tenantId}", handler.getStore)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeResourceJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "resource_view_not_configured", "error": "store space resource view is not configured"})
		return
	}
	filters := StoreFilters{
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
		Page: parsePositiveInt(r.URL.Query().Get("page"), 1),
		PageSize: parsePositiveInt(r.URL.Query().Get("page_size"), 20),
		CityID: parseOptionalInt64(r.URL.Query().Get("city_id")),
	}
	result, err := h.service.ListStores(r.Context(), filters, func(tenantID int64) MonitorAccess {
		if h.monitorAccess == nil {
			return MonitorAccess{}
		}
		access, err := h.monitorAccess(r, tenantID)
		if err != nil {
			return MonitorAccess{}
		}
		return access
	})
	if err != nil {
		writeResourceJSON(w, http.StatusInternalServerError, map[string]string{"code": "list_resource_stores_failed", "error": err.Error()})
		return
	}
	writeResourceJSON(w, http.StatusOK, result)
}

func (h *Handler) getStore(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeResourceJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "resource_view_not_configured", "error": "store space resource view is not configured"})
		return
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("tenantId")), 10, 64)
	if err != nil || tenantID <= 0 {
		writeResourceJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_tenant_id", "error": "invalid tenant id"})
		return
	}
	access := MonitorAccess{}
	if h.monitorAccess != nil {
		var accessErr error
		access, accessErr = h.monitorAccess(r, tenantID)
		if accessErr != nil {
			writeResourceJSON(w, http.StatusForbidden, map[string]string{"code": "resource_view_forbidden", "error": "forbidden"})
			return
		}
	}
	detail, err := h.service.GetStore(r.Context(), tenantID, access)
	if err != nil {
		writeResourceJSON(w, http.StatusInternalServerError, map[string]string{"code": "get_resource_store_failed", "error": err.Error()})
		return
	}
	writeResourceJSON(w, http.StatusOK, detail)
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseOptionalInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func writeResourceJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
```

- [ ] **Step 4: Add service methods**

Add to `internal/resourceview/service.go`:

```go
func (s *Service) ListStores(ctx context.Context, filters StoreFilters, access func(int64) MonitorAccess) (StoreListResult, error)
func (s *Service) GetStore(ctx context.Context, tenantID int64, access MonitorAccess) (StoreDetail, error)
```

`ListStores` must:

- normalize page/pageSize.
- ask repo for business list records.
- compute summary over the filtered dataset, not the current page.
- apply monitor access only to `can_view_monitor` and `monitor_url`; do not filter admin/editor data here unless product later confirms resource-view itself must be limited for viewer.

- [ ] **Step 5: Register resource view in app handler**

Modify `internal/app/handler.go`:

```go
func NewHandlerWithServicesAndH5MonitorAndResourceView(
	store Store,
	designPlanService *designplan.Service,
	storeSpaceService *storespace.Service,
	h5MonitorService *h5monitor.Service,
	resourceViewService *resourceview.Service,
) http.Handler
```

Inside `newHandlerWithServices`, call:

```go
resourceview.RegisterRoutes(mux, resourceViewService, handler.resourceViewMonitorAccess)
```

Add:

```go
func (h *Handler) resourceViewMonitorAccess(r *http.Request, tenantID int64) (resourceview.MonitorAccess, error) {
	externalOrgID := strconv.FormatInt(tenantID, 10)
	user, err := h.currentAuthUser(r)
	if err != nil {
		return resourceview.MonitorAccess{}, err
	}
	ok, err := h.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
	if err != nil {
		return resourceview.MonitorAccess{}, err
	}
	if !ok {
		return resourceview.MonitorAccess{CanViewMonitor: false}, nil
	}
	return resourceview.MonitorAccess{
		CanViewMonitor: true,
		MonitorURL: "/erzhuang-project/h5/orgs/" + url.PathEscape(externalOrgID) + "/monitor",
	}, nil
}
```

- [ ] **Step 6: Open business DB only when configured**

Modify `cmd/server/main.go`:

```go
businessConfig := businessDatabaseConfigFromEnv()
if businessConfig.DSN != "" {
	businessDB, err := openDatabase(businessConfig)
	if err != nil {
		log.Fatalf("business database setup failed: %v", err)
	}
	defer businessDB.Close()
	resourceViewService = resourceview.NewService(resourceview.NewMySQLRepository(businessDB))
	log.Print("business resource view enabled")
}
```

Use:

```go
func businessDatabaseConfigFromEnv() databaseConfig {
	dsn := envValue("BUSINESS_MYSQL_DSN", "K8S_SECRET_BUSINESS_MYSQL_DSN")
	if dsn == "" {
		return databaseConfig{}
	}
	return databaseConfig{Driver: "mysql", DSN: mysqlDSNWithParseTime(dsn)}
}
```

- [ ] **Step 7: Compile backend**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/resourceview -o /private/tmp/resourceview.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server
```

Expected:

```text
All compile gates pass.
```

- [ ] **Step 8: Commit backend API wiring**

Run:

```bash
git add cmd/server/main.go cmd/server/main_test.go internal/app/handler.go internal/app/handler_test.go internal/resourceview/handler.go internal/resourceview/service.go
git commit -m "feat: expose read only resource view api"
```

### Task 5: Add Frontend API and Domain Helpers

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/api.test.ts`
- Create: `frontend/src/domain/resource-view.ts`
- Create: `frontend/src/domain/resource-view.test.ts`

- [ ] **Step 1: Add frontend API tests**

Modify `frontend/src/api.test.ts` with:

```ts
describe("store space resource view api", () => {
  it("uses the 3.0 read-only resource view paths", () => {
    expect(__testing.resourceViewStoresPath()).toBe("/erzhuang-project/api/store-space-resource-view/stores");
    expect(__testing.resourceViewStorePath(10019)).toBe("/erzhuang-project/api/store-space-resource-view/stores/10019");
  });
});
```

- [ ] **Step 2: Add TypeScript types and API functions**

Modify `frontend/src/api.ts` by adding:

```ts
export type ResourceIssueSeverity = "error" | "warning" | "info";
export type ResourceIssueType =
  | "unbound_camera"
  | "inactive_bound_space"
  | "missing_camera"
  | "missing_nvr"
  | "offline_edge"
  | "offline_nvr"
  | "offline_camera"
  | "camera_bound_many_spaces"
  | "space_bound_many_cameras";

export type ResourceStoreSummary = {
  tenantId: number;
  storeName: string;
  hospitalName: string;
  cityId: number;
  cityName: string;
  edgeCount: number;
  nvrCount: number;
  cameraCount: number;
  spaceCount: number;
  boundCameraCount: number;
  unboundCameraCount: number;
  offlineDeviceCount: number;
  warningCount: number;
  canViewMonitor: boolean;
  monitorUrl?: string;
};

export type ResourceStoreListSummary = {
  storeCount: number;
  edgeCount: number;
  nvrCount: number;
  cameraCount: number;
  spaceCount: number;
  boundCameraCount: number;
  unboundCameraCount: number;
  offlineDeviceCount: number;
  warningCount: number;
};

export type ResourceStoreListResponse = {
  items: ResourceStoreSummary[];
  page: number;
  pageSize: number;
  total: number;
  summary: ResourceStoreListSummary;
  cities: Array<{ cityId: number; name: string; count: number }>;
};
```

Add `ResourceDevice`, `ResourceCamera`, `ResourceSpace`, `ResourceSpaceNode`, `ResourceDeviceTree`, `ResourceIssue`, and `ResourceStoreDetail` with camelCase fields matching backend JSON.

Add:

```ts
export const storeSpaceResourceViewApi = {
  listStores(query = "", page = 1, pageSize = 20, cityId?: number | "all") {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
    if (query.trim()) params.set("q", query.trim());
    if (cityId && cityId !== "all") params.set("city_id", String(cityId));
    return httpJson<ResourceStoreListResponse>(`${resourceViewStoresPath()}?${params.toString()}`).then(mapResourceStoreListResponse);
  },
  getStore(tenantId: number) {
    return httpJson<ResourceStoreDetail>(resourceViewStorePath(tenantId)).then(mapResourceStoreDetail);
  },
};
```

- [ ] **Step 3: Add domain helper tests**

Create `frontend/src/domain/resource-view.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { issueSeverityRank, resourceDeviceOnlineLabel, sortedResourceIssues } from "./resource-view";

describe("resource view domain", () => {
  it("labels online state from business db values", () => {
    expect(resourceDeviceOnlineLabel(1)).toBe("在线");
    expect(resourceDeviceOnlineLabel(2)).toBe("离线");
    expect(resourceDeviceOnlineLabel(0)).toBe("未知");
  });

  it("sorts error issues before warnings and info", () => {
    const issues = sortedResourceIssues([
      { severity: "info", type: "space_bound_many_cameras", message: "info", entityType: "space", entityId: 1 },
      { severity: "error", type: "missing_camera", message: "error", entityType: "relation", entityId: 2 },
      { severity: "warning", type: "unbound_camera", message: "warning", entityType: "camera", entityId: 3 },
    ]);
    expect(issues.map((issue) => issue.severity)).toEqual(["error", "warning", "info"]);
    expect(issueSeverityRank("error")).toBeLessThan(issueSeverityRank("warning"));
  });
});
```

- [ ] **Step 4: Implement helpers**

Create `frontend/src/domain/resource-view.ts`:

```ts
import type { ResourceIssue, ResourceIssueSeverity } from "../api";

export function resourceDeviceOnlineLabel(value: number | null | undefined) {
  if (value === 1) return "在线";
  if (value === 2) return "离线";
  return "未知";
}

export function issueSeverityRank(severity: ResourceIssueSeverity) {
  if (severity === "error") return 1;
  if (severity === "warning") return 2;
  return 3;
}

export function sortedResourceIssues(issues: ResourceIssue[]) {
  return [...issues].sort((left, right) => {
    const severityDiff = issueSeverityRank(left.severity) - issueSeverityRank(right.severity);
    if (severityDiff !== 0) return severityDiff;
    return left.type.localeCompare(right.type);
  });
}
```

- [ ] **Step 5: Run frontend tests**

Run:

```bash
cd frontend
npm test
```

Expected:

```text
All existing tests and new resource-view tests pass.
```

- [ ] **Step 6: Commit frontend API/domain**

Run:

```bash
git add frontend/src/api.ts frontend/src/api.test.ts frontend/src/domain/resource-view.ts frontend/src/domain/resource-view.test.ts
git commit -m "feat: add resource view frontend api"
```

### Task 6: Replace Admin Main UI With Read-Only Resource View

**Files:**
- Create: `frontend/src/components/ResourceStoreList.tsx`
- Create: `frontend/src/components/ResourceStoreDetail.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Create list component**

Create `frontend/src/components/ResourceStoreList.tsx`:

```tsx
import type { ResourceStoreSummary } from "../api";

type Props = {
  stores: ResourceStoreSummary[];
  loading: boolean;
  page: number;
  pageSize: number;
  openingStoreIds: Set<number>;
  onOpenStore: (tenantId: number) => void;
  onOpenMonitor: (store: ResourceStoreSummary) => void;
};

export function ResourceStoreList({ stores, loading, page, pageSize, openingStoreIds, onOpenStore, onOpenMonitor }: Props) {
  return (
    <section className="table-frame" aria-label="门店空间资源查看列表">
      <table className="store-table resource-store-table">
        <thead>
          <tr>
            <th>序号</th>
            <th>城市</th>
            <th>门店名称</th>
            <th>机构 ID</th>
            <th>工控机</th>
            <th>NVR</th>
            <th>摄像头</th>
            <th>空间</th>
            <th>已绑定</th>
            <th>未绑定</th>
            <th>异常</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr><td colSpan={12} className="empty-cell">正在加载门店空间资源</td></tr>
          ) : stores.length === 0 ? (
            <tr><td colSpan={12} className="empty-cell">暂无已部署工控机的门店</td></tr>
          ) : stores.map((store, index) => (
            <tr key={store.tenantId}>
              <td>{(page - 1) * pageSize + index + 1}</td>
              <td>{store.cityName || `城市 ${store.cityId}`}</td>
              <td className="store-name">{store.storeName}</td>
              <td>{store.tenantId}</td>
              <td>{store.edgeCount}</td>
              <td>{store.nvrCount}</td>
              <td>{store.cameraCount}</td>
              <td>{store.spaceCount}</td>
              <td>{store.boundCameraCount}</td>
              <td>{store.unboundCameraCount}</td>
              <td>{store.warningCount}</td>
              <td>
                <div className="row-actions">
                  <button disabled={openingStoreIds.has(store.tenantId)} onClick={() => onOpenStore(store.tenantId)}>
                    {openingStoreIds.has(store.tenantId) ? "进入中" : "详情"}
                  </button>
                  {store.canViewMonitor && store.monitorUrl ? (
                    <button className="secondary-action-button" onClick={() => onOpenMonitor(store)}>查看监控</button>
                  ) : null}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
```

- [ ] **Step 2: Create detail component**

Create `frontend/src/components/ResourceStoreDetail.tsx` with three tabs:

```tsx
import { useState } from "react";
import type { ResourceStoreDetail } from "../api";
import { sortedResourceIssues } from "../domain/resource-view";

type TabKey = "spaces" | "devices" | "issues";

type Props = {
  store: ResourceStoreDetail;
  onBack: () => void;
  onOpenMonitor: (url: string) => void;
};

export function ResourceStoreDetail({ store, onBack, onOpenMonitor }: Props) {
  const [tab, setTab] = useState<TabKey>("spaces");
  const issues = sortedResourceIssues(store.issues);

  return (
    <section className="detail-page resource-detail-page">
      <header className="detail-header">
        <div className="detail-header-main">
          <button className="secondary-action-button" onClick={onBack}>返回列表</button>
          <h1>{store.storeName} / 空间资源映射</h1>
          <div className="detail-metrics" aria-label="门店资源概览">
            <div><span>机构 ID</span><strong>{store.tenantId}</strong></div>
            <div><span>工控机</span><strong>{store.summary.edgeCount}</strong></div>
            <div><span>NVR</span><strong>{store.summary.nvrCount}</strong></div>
            <div><span>摄像头</span><strong>{store.summary.cameraCount}</strong></div>
            <div><span>异常</span><strong>{store.summary.warningCount}</strong></div>
          </div>
        </div>
        {store.canViewMonitor && store.monitorUrl ? (
          <div className="detail-header-side">
            <button className="secondary-action-button" onClick={() => onOpenMonitor(store.monitorUrl!)}>查看监控</button>
          </div>
        ) : null}
      </header>
      <nav className="tabs" aria-label="资源视角">
        <button className={tab === "spaces" ? "is-active" : ""} onClick={() => setTab("spaces")}>空间视角</button>
        <button className={tab === "devices" ? "is-active" : ""} onClick={() => setTab("devices")}>设备视角</button>
        <button className={tab === "issues" ? "is-active" : ""} onClick={() => setTab("issues")}>异常项</button>
      </nav>
      {tab === "spaces" ? <SpaceTree nodes={store.spaceTree} /> : null}
      {tab === "devices" ? <DeviceTree store={store} /> : null}
      {tab === "issues" ? <IssueList issues={issues} /> : null}
    </section>
  );
}
```

Implement `SpaceTree`, `DeviceTree`, and `IssueList` in the same file. Keep them read-only and do not import `DesignPlanTab`, `VideoChannelTab`, `CreateStoreModal`, or `EditStoreModal`.

- [ ] **Step 3: Switch `App.tsx` to resource view state**

Modify `frontend/src/App.tsx`:

- Replace `StoreSummary` list state with `ResourceStoreSummary`.
- Replace `StoreDetailType` active detail with `ResourceStoreDetail`.
- Replace `storeSpaceApi.listStores` with `storeSpaceResourceViewApi.listStores`.
- Replace old `StoreList` component with `ResourceStoreList`.
- Replace old `StoreDetail` component with `ResourceStoreDetail`.
- Remove main-page create/edit/delete buttons from the 3.0 screen.
- Keep `SystemTopBar`, `UserManagement`, auth loading, logout, H5 routes, and H5 Monitor pages unchanged.

- [ ] **Step 4: Update summary cards**

Change top-right/list summary labels to:

```text
共 N 家门店
工控机 N
录像机 N
摄像头 N
已绑定 N
异常 N
```

Remove old visible labels in the 3.0 main view:

```text
面诊室
治疗室
美容室
设计图状态
识别模型
```

- [ ] **Step 5: Add compact resource styles**

Modify `frontend/src/styles.css` to add:

```css
.resource-store-table th,
.resource-store-table td {
  white-space: nowrap;
}

.resource-detail-page .tabs {
  margin-top: 16px;
}

.resource-tree {
  display: grid;
  gap: 8px;
}

.resource-node {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 10px;
  background: #fff;
}

.resource-node-children {
  margin-top: 8px;
  margin-left: 16px;
  display: grid;
  gap: 8px;
}

.resource-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.resource-issue-error {
  border-left: 3px solid #dc2626;
}

.resource-issue-warning {
  border-left: 3px solid #d97706;
}

.resource-issue-info {
  border-left: 3px solid #2563eb;
}
```

- [ ] **Step 6: Build frontend**

Run:

```bash
cd frontend
npm test
npm run build
```

Expected:

```text
Tests pass. Build passes. Existing Vite chunk warning may remain.
```

- [ ] **Step 7: Commit frontend view switch**

Run:

```bash
git add frontend/src/App.tsx frontend/src/components/ResourceStoreList.tsx frontend/src/components/ResourceStoreDetail.tsx frontend/src/styles.css
git commit -m "feat: show read only store resource view"
```

### Task 7: Documentation, Version, and Migration Notes

**Files:**
- Modify: `VERSION`
- Modify: `README.md`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: Bump version to 3.0.0**

Modify `VERSION`:

```text
3.0.0
```

- [ ] **Step 2: Update README current status**

In `README.md`, change the current business goal section to:

```markdown
当前主业务目标：二壮 3.0「门店空间资源查看」基于公司业务库只读展示已部署工控机门店的业务空间、工控机、NVR、摄像头和空间-设备绑定完整性；现有 H5 Monitor 监控查看方式保持不变。
```

Add runtime env:

```markdown
业务库只读连接：

- `BUSINESS_MYSQL_DSN` 或 `K8S_SECRET_BUSINESS_MYSQL_DSN`
```

- [ ] **Step 3: Update project memory**

Add a new dated entry to `docs/codex-learning-state.md`:

```markdown
## 2026-08-13 门店空间资源查看 3.0 实施记录

- 目标：将后台主流程切换为基于公司业务库的只读资源查看。
- 边界：不改 H5 Monitor，不做 AI 识别、设计图标注、门店/通道/空间写入。
- 数据源：二壮运行库仍用于登录、权限、系统设置和 H5 Monitor；业务库只读连接用于 3.0 资源查看。
- 发布前备份：已准备 `v2.31-stable-before-resource-view-3` 或等价备份分支。
- 待线上验证：资源查看列表、详情三视角、异常项、viewer 监控入口权限、H5 Monitor 原链路。
```

- [ ] **Step 4: Add decision**

Add to `docs/decisions.md`:

```markdown
## 2026-08-13 二壮 3.0 采用业务库只读资源查看

- 背景：公司业务库已维护工控机、NVR、摄像头、空间和空间-设备绑定关系，二壮继续维护独立映射会造成重复维护和口径分裂。
- 结论：3.0 主流程改为「门店空间资源查看」，只读展示业务库事实；不在二壮侧新增、编辑、删除或确认门店空间映射。
- 原因：业务库是设备与空间映射的主数据源，二壮更适合作为资源完整性查看和运营验收入口。
- 影响：旧设计图标注、AI 识别和人工确认入口在 3.0 主流程隐藏；旧代码和数据先保留作为回滚与历史兼容。H5 Monitor 仍沿用现有萤石云链路，工控机/NVR 取流作为后续单独版本讨论。
```

- [ ] **Step 5: Commit docs and version**

Run:

```bash
git add VERSION README.md docs/codex-learning-state.md docs/decisions.md work/current-plan.md
git commit -m "docs: record resource view 3 rollout"
```

### Task 8: Local Verification

**Files:**
- No source modifications unless verification finds a defect.

- [ ] **Step 1: Backend compile gates**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/resourceview -o /private/tmp/resourceview.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server
```

Expected:

```text
All commands exit 0.
```

- [ ] **Step 2: Frontend tests and build**

Run:

```bash
cd frontend
npm test
npm run build
```

Expected:

```text
All tests pass and Vite build exits 0.
```

- [ ] **Step 3: Source safety scans**

Run:

```bash
rg -n -i "securityVideoUrl|content_id|supabase|postgres|OPENAI_API_KEY|MINIMAX|INSERT|UPDATE|DELETE|REPLACE" internal/resourceview frontend/src/components/ResourceStoreList.tsx frontend/src/components/ResourceStoreDetail.tsx
rg -n -i "password|secret|token|dsn" internal/resourceview frontend/src/components/ResourceStoreList.tsx frontend/src/components/ResourceStoreDetail.tsx
```

Expected:

```text
No business DB credentials. No write SQL in internal/resourceview. No old Supabase/Postgres/runtime AI credential names in new resource view code.
```

- [ ] **Step 4: Browser verification**

Before browser verification, read:

```bash
sed -n '1,220p' docs/ui-standards.md
sed -n '1,220p' docs/frontend-review-checklist.md
```

Then start local frontend:

```bash
cd frontend
npm run dev -- --host 127.0.0.1
```

Use Chrome plugin first when available. Verify:

- 主导航显示「门店空间资源查看」。
- 列表为空态文案是“暂无已部署工控机的门店”。
- 有 mock 数据时列表字段是工控机/NVR/摄像头/空间/已绑定/未绑定/异常。
- 详情页有「空间视角」「设备视角」「异常项」三个 Tab。
- 页面没有新增门店、编辑门店、删除、上传设计图、识别、确认、扫描通道入口。
- 「查看监控」按钮仍打开 `/erzhuang-project/h5/orgs/{externalOrgId}/monitor`。
- H5 Monitor 页仍可进入，区域 Tab 返回逻辑不回退。

### Task 9: Company Release and Rollback

**Files:**
- Read: `docs/deploy-runbook.md`
- Read: `docs/codex-learning-state.md`

- [ ] **Step 1: Pre-release checks**

Run:

```bash
git status --short
git log --oneline -5
rg -n -i "join" internal/resourceview cmd/server internal/app frontend/src
```

Expected:

```text
Only intentional files are dirty or the branch is clean.
No risky SQL join token in changed backend files unless company hook policy has changed.
```

- [ ] **Step 2: Merge to company branch**

Run:

```bash
git fetch gitlab
git switch codex/containerize-single-image
git merge codex/store-space-resource-view-3
```

Expected:

```text
Merge succeeds without overwriting company runtime configuration.
```

- [ ] **Step 3: Push to GitHub backup and company GitLab**

Run GitHub backup if the user has not asked to skip it:

```bash
git push origin codex/containerize-single-image
```

Run company push with temporary `GIT_ASKPASS` reading `/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`. Do not print the token.

Expected:

```text
GitLab remote branch receives the new 3.0 commit and company K8s auto-build starts.
```

- [ ] **Step 4: Online verification by logged-in browser**

After company deployment completes, verify:

```text
GET /erzhuang-project/health returns database=mysql and asset_store=oss.
Page footer shows 3.0.0 with the deployed commit or container suffix.
门店空间资源查看列表 loads from business DB.
Store detail opens and shows space/device/issues.
Viewer user cannot see monitor entry for unauthorized stores.
Existing H5 Monitor live/playback behavior is unchanged.
```

- [ ] **Step 5: Rollback plan**

If 3.0 blocks core usage, roll back only to a MySQL/OSS-compatible 2.x commit:

```bash
git switch codex/containerize-single-image
git revert <3.0-merge-commit>
git push gitlab codex/containerize-single-image
```

Expected:

```text
Company K8s redeploys the reverted code. H5 Monitor and old 2.x store-space UI recover.
```

Do not use Korean Lighthouse and do not depend on PostgreSQL/Supabase for rollback.

## Self-Review

- Spec coverage:
  - 已覆盖业务库只读、四张业务表、工控机门店列表、三层空间树、设备树、异常项、旧入口隐藏、H5 Monitor 不变、viewer 监控权限、备份、验证、发布和回滚。
- Placeholder scan:
  - 本计划不包含真实密钥、token、数据库密码或业务库 DSN。
  - 计划中未要求调用 `securityVideoUrl` 或 `content_id`。
- Type consistency:
  - 后端模型使用 snake_case JSON。
  - 前端类型使用 camelCase。
  - API 路径统一为 `/api/store-space-resource-view/*`。

## Execution Choice

Plan complete and saved to `docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`.

Two execution options:

1. **Subagent-Driven (recommended)** - 主会话按任务拆分派发，逐段验收，适合 3.0 这种跨后端/前端/发布的大版本。
2. **Inline Execution** - 当前主会话直接按计划执行，节奏更连续，但上下文压力更大。

推荐选择 Subagent-Driven：先让后端子会话做 Task 1-4，主会话 review；再让前端子会话做 Task 5-6；最后主会话执行 Task 7-9。
