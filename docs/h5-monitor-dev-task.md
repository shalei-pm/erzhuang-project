# H5 监控查看端 - 新人开发任务

## 你是谁

你是这个项目的新开发者。项目所有者（沙磊）和 Codex（四喜）已完成技术验证和选型，你来负责实现 H5 监控查看端模块。这个模块相对独立，你开发完后合并回主分支即可。

## 项目背景

这是一个门店空间资源管理系统，后端 Go，前端 React + TypeScript。已有萤石云 OpenAPI 集成（通道扫描、抓图、AI 识别），现在要新增一个 H5 移动端监控查看页面。

技术验证已完成，所有结论记录在 `docs/h5-monitor-technical-spike.md` 第 10-11 节，你务必先读完。

## 你的交付物

### 后端（Go）

新增文件，不要修改现有文件（除非标注"允许修改"）：

1. `internal/ezviz/aac_transfer.go` - 萤石 AAC 转码接口调用
2. `internal/ezviz/aac_transfer_test.go` - 测试
3. `internal/ezviz/playback.go` - 回放地址获取（扩展现有 LiveAddress，支持 type=2 回放参数）
4. `internal/ezviz/playback_test.go` - 测试
5. `internal/ezviz/record_segments.go` - 录像片段查询
6. `internal/ezviz/record_segments_test.go` - 测试
7. `internal/h5monitor/` - H5 监控服务层（新 package）
   - `handler.go` - HTTP handler
   - `service.go` - 业务逻辑
   - `models.go` - 数据模型
8. `internal/h5monitor/handler_test.go` - 测试

允许修改的文件（只追加，不改动现有逻辑）：

- `internal/storespace/handler.go` - 只在 `RegisterRoutes` 末尾追加 H5 路由注册
- `internal/storespace/service.go` - 只新增 H5 相关方法，不改现有方法
- `internal/storespace/models.go` - 只追加 H5 相关类型
- `internal/storespace/schema.go` - 只追加 H5 相关建表语句

### 前端（React + TypeScript）

1. `frontend/src/pages/H5Monitor.tsx` - H5 监控首页
2. `frontend/src/pages/H5MonitorChannel.tsx` - 摄像头详情页
3. `frontend/src/components/H5FlvPlayer.tsx` - ezuikit-flv 播放器封装
4. `frontend/src/api-h5.ts` - H5 专用 API 调用
5. `frontend/src/domain/h5-types.ts` - H5 类型定义

## 技术规格

### 萤石 AAC 转码接口

```text
POST https://open.ys7.com/api/service/media/aac/transfer?enable=1
Header: accessToken / deviceSerial / localIndex
```

关键：参数必须在 Header，不能放 body。返回 `meta.code=200` 表示成功。token 过期（10002）需刷新重试。

### 播放地址接口（已验证可用）

```text
POST https://open.ys7.com/api/lapp/v2/live/address/get
Content-Type: application/x-www-form-urlencoded

直播: protocol=4, type=1, supportH265=1, mute=0, quality=2
回放: protocol=4, type=2, supportH265=1, mute=0, quality=2, startTime, stopTime
```

现有 `internal/ezviz/client.go` 的 `LiveAddress` 方法只支持直播（type=1），你需要扩展支持回放参数。

### 录像片段查询接口

```text
GET https://open.ys7.com/api/v3/device/local/video/unify/query
Header: accessToken / deviceSerial / localIndex
Query: startTime / endTime (秒级时间戳) / pageSize
```

返回 `data.records[]`，每条有 `startTime`、`endTime`、`type`。

### 播放器

使用 `ezuikit-flv@2.1.1`（npm 包）。需要自行部署 `decoder.js` 和 `decoder.wasm` 到前端静态资源目录。

播放器初始化：

```js
const player = new EzuikitFlv({
  id: 'player-container',
  url: playbackUrl, // 后端返回的 FLV URL
  decoder: '/assets/ezuikit-flv/decoder.js',
  autoPlay: true,
  isLive: false, // 回放时 false，直播时 true
  muted: true, // 默认静音，用户点击后取消
  autoWasm: true,
  useMSE: true,
  useWCS: true,
});
```

### API 路由

```text
GET  /api/h5/orgs/{externalOrgId}/monitor
  -> 返回门店信息 + 已确认有效通道列表（按区域分组）

POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url
  -> 确保AAC转码 + 获取直播FLV地址
  -> 返回 { url, expireTime }

GET  /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments?date=YYYY-MM-DD
  -> 返回当天录像片段列表

POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url
  -> body: { startTime, stopTime }
  -> 返回 { url, expireTime }
```

### 前端路由

```text
/h5/orgs/{externalOrgId}/monitor          -> 首页（通道列表）
/h5/orgs/{externalOrgId}/monitor/channels/{channelId} -> 详情（播放器）
```

### 通道分组复用规则

后台通道映射页已经沉淀一套可复用的通道筛选/排序规则。H5 monitor 首页不要重新发明分组口径，应复用同样的领域规则：

- 共享规则位置：`frontend/src/domain/channel-filters.ts`。
- 分组：全部、面诊室、治疗室、生美、前台/候诊区、通道/其他。
- `VIP治疗室` 属于治疗室大类，筛选“治疗室”时必须包含。
- `前台/候诊区` 只按可见/可维护文本包含 `前台`、`候诊`、`等候` 判断，不根据 AI 原始长文本猜测。
- `通道/其他` 是非业务且非前台候诊的兜底组，包含通道、走廊、药房、机房、办公室、未知等。
- 排序：全部按录像机编号 + 通道号；业务区域按编号/备注里的数字；前台/候诊区和通道/其他按展示文本，相同文本再按录像机编号 + 通道号。

#### 复用实现方案

前端已新增 `frontend/src/domain/channel-filters.ts` 作为通道列表的领域规则模块。H5 monitor 首页实现分组时，应直接复用这个模块：

```ts
import { channelListFilters, filterAndSortChannels } from "../domain/channel-filters";
```

该模块不绑定后台通道映射页组件，只要求通道对象具备最小字段：`id`、`recorderCode`、`channelNo`、`areaType`、`areaNumber`、`areaNote`、`sceneType`、`channelName`、`status`。如果 H5 API 返回的字段名不同，应在 `frontend/src/api-h5.ts` 或 H5 页面入口处做一次轻量适配，不要复制筛选和排序逻辑。

后台类型口径：

- `internal/storespace/models.go` 中 `AreaTypeVIPTreatment = "vip_treatment"`。
- `VIP治疗室` 计入治疗室总数和治疗室筛选。
- `VIP治疗室` 的 `area_number` 允许为 `0`，表示未填写编号/备注；同一门店最多有一个未编号 VIP 治疗室。
- 普通 `治疗室`、`面诊室`、`生美` 仍要求正整数编号。

H5 列表建议不要新增自己的分组枚举。若需要在 H5 页面隐藏“全部”筛选，仍应从 `channelListFilters` 过滤展示项，而不是手写新数组。

### 交互要求

- 首页：通道按共享筛选口径分组或筛选（全部、面诊室、治疗室、生美、前台/候诊区、通道/其他），显示最近截图，点击进入详情。
- 详情页：默认静音播放，显著显示"点击开启声音"按钮，用户点击后取消静音。
- 回放：选择日期 -> 查询片段 -> 点击片段播放。
- 离开详情页时主动失效播放地址（调用 `/api/lapp/v2/live/address/disable`）。

### 并发限制

- 普通用户：最多 15 路同时播放。
- 管理员：额外 5 路。
- 后端按用户计数活跃路数，离开时释放。

## 代码规范

- Go: 遵循现有项目风格，用 `gofmt`，测试用标准 `testing`。
- 前端: 遵循现有 `frontend/src/` 风格，TypeScript 严格模式。
- 不要把萤石 appSecret/accessToken 写入代码、配置或文档。
- 不要修改 `.tools/`、`scripts/`、`docs/` 中现有内容。
- 不要碰 `Dockerfile`、`VERSION`、部署相关文件。

## 验收标准

1. `go test ./...` 通过。
2. `cd frontend && npm run build` 通过。
3. 用 `AZ3988334`（华北账号）通道 1 能在 H5 页面播放直播。
4. 能查询回放片段并播放回放。
5. 点击"开启声音"后能听到声音（AAC 转码已确保）。
6. 离开详情页后播放地址被失效。
7. 代码不包含任何密钥、token、secret。

## 开发分支

从 `main` 拉出 `codex/h5-monitor` 分支开发，完成后创建 PR。

## 需要帮助时

- 技术验证细节：`docs/h5-monitor-technical-spike.md` 第 10-11 节。
- 萤石接口规则：`docs/ezviz-openapi-notes.md`。
- 现有萤石 client：`internal/ezviz/client.go`。
- 现有后端结构：`internal/storespace/`。
- 前端结构：`frontend/src/`。

## 交接流程

### 1. 代码仓库

新人 clone 项目仓库：

```bash
git clone https://github.com/shalei-pm/erzhuang-project.git
cd erzhuang-project
git switch -c codex/h5-monitor
```

开发期间所有改动都在 `codex/h5-monitor` 分支上提交，不要动 `main`。

### 2. 必读文档（按顺序）

开始写代码前必须读完：

1. `docs/h5-monitor-dev-task.md`（本文件，你的任务书）
2. `docs/h5-monitor-technical-spike.md` 第 1-11 节（完整技术验证结论）
3. `docs/ezviz-openapi-notes.md`（萤石接口规则和错误码）
4. `internal/ezviz/client.go`（现有萤石 client，你的新代码要和它风格一致）
5. `internal/storespace/handler.go` 前 50 行（看路由注册模式）
6. `internal/storespace/models.go`（看数据模型风格）

### 3. 开发顺序

建议按这个顺序，每步可独立验证：

**第一周：后端萤石能力扩展**

1. `internal/ezviz/aac_transfer.go` + 测试
2. `internal/ezviz/playback.go`（回放地址）+ 测试
3. `internal/ezviz/record_segments.go`（录像片段）+ 测试
4. 运行 `go test ./internal/ezviz/...` 验证

**第二周：H5 后端 API**

5. `internal/h5monitor/` package（handler + service + models）
6. 在 `internal/storespace/handler.go` 的 `RegisterRoutes` 末尾追加 H5 路由
7. 运行 `go test ./...` 验证

**第三周：前端页面**

8. 播放器封装 `H5FlvPlayer.tsx`
9. H5 首页 `H5Monitor.tsx`
10. H5 详情页 `H5MonitorChannel.tsx`
11. 前端路由注册
12. 运行 `cd frontend && npm run build` 验证

**第四周：联调 + 产出物**

13. 用 `AZ3988334` 华北测试录像机端到端联调
14. 写产出物文档（见下方）
15. 推送分支，通知验收

### 4. 产出物文档

开发完成后，在 `docs/h5-monitor-handoff.md` 写一份交接文档，包含：

```markdown
# H5 监控模块交接文档

## 已完成功能清单
- [ ] 直播播放
- [ ] 回放播放
- [ ] 声音（AAC 转码 + 用户点击开声）
- [ ] 录像片段查询
- [ ] 通道列表分组展示
- [ ] 播放地址失效
- [ ] 并发计数（如未完成标注原因）

## 新增文件清单
列出所有新增文件路径。

## 修改的现有文件清单
列出所有修改的现有文件，说明改了什么。

## API 接口文档
每个 H5 API 的请求参数、响应结构、错误码。

## 数据库变更
新增了哪些表或字段，建表语句。

## 测试结果
- go test ./... 结果
- npm run build 结果
- 端到端验证结果（用哪个设备哪个通道验证的）

## 已知问题
列出未解决的问题、待确认的点。

## 环境依赖
- ezuikit-flv 版本
- 需要部署的静态资源（decoder.js / decoder.wasm 路径）
- 需要的环境变量
```

### 5. 提交和验收

开发完成后：

1. 推送 `codex/h5-monitor` 分支到 GitHub。
2. 把 `docs/h5-monitor-handoff.md` 发给沙磊。
3. 沙磊把交接文档和代码交给 Codex（四喜）做技术验收：
   - 代码质量、测试覆盖、安全性（无密钥泄露）
   - API 设计合理性
   - 萤石接口调用正确性
4. 沙磊验收前端呈现和交互：
   - 页面在移动端/飞书浏览器的展示效果
   - 直播/回放/声音交互
   - 通道列表和回放片段的体验
5. 验收通过后，由 Codex 合并到 `main` 并发布。

### 6. 沟通规则

- 技术问题（萤石接口、代码结构、测试）：直接看文档和代码，文档里没有的先查 `docs/ezviz-openapi-notes.md` 和 `docs/h5-monitor-technical-spike.md`。
- 文档里确实没有的技术问题：记录在 `docs/h5-monitor-handoff.md` 的"待确认问题"里，验收时一起讨论。
- 不要通过口头传话解决技术问题，所有结论落到文档里。
