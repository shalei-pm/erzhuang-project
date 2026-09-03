# 技术架构索引

最后更新：2026-07-04

本文是 `erzhuang-project` 的当前代码地图。后续迭代、评审、发布或新开专项会话时，先读 `docs/codex-learning-state.md` 的当前快照，再读本文定位改动范围。

## 1. 当前主路径

```text
浏览器
  ↓
frontend/                         Vite + React + TypeScript + Ant Design
  ↓ /erzhuang-project/api/*
internal/app                       Go HTTP 总入口、认证、权限、ops
  ↓
internal/storespace                门店空间资源主业务
  ↓
MySQL tb_*                         门店、设计图、录像机、通道、截图台账、用户权限
  ↓
internal/assets                    统一资产存储抽象
  ↓
OSS                                设计图文件、预览图、通道截图
```

当前公司运行时只应是 MySQL + OSS。PostgreSQL/Supabase 不再是公司运行时回滚路径。

## 2. 重要入口

- 后端入口：`cmd/server/main.go`
- HTTP 路由汇总：`internal/app/handler.go`
- 认证与权限：`internal/app/auth.go`、`internal/app/authz.go`、`internal/app/auth_users.go`
- 门店空间业务：`internal/storespace/service.go`
- 门店空间路由：`internal/storespace/handler.go`
- 门店空间 MySQL 仓储：`internal/storespace/mysql_store.go`
- H5 Monitor：`internal/h5monitor/*`
- 萤石能力：`internal/ezviz/*`、`internal/storespace/ezviz_scanner.go`
- 通道截图识别：`internal/channelai/*`、`internal/storespace/channelai_adapter.go`
- 资产存储：`internal/assets/store.go`、`internal/assets/oss.go`
- 前端入口：`frontend/src/App.tsx`、`frontend/src/main.tsx`
- 前端 API：`frontend/src/api.ts`、`frontend/src/api-h5.ts`
- 前端页面：`frontend/src/components/*`、`frontend/src/pages/*`
- MySQL schema：`db/mysql_schema_tb.sql`、`db/mysql_governance_schema_tb.sql`

## 3. 启动与运行时配置

`cmd/server/main.go` 当前只接受 MySQL：

- `APP_DB_DRIVER=mysql`
- `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN`

资产存储通过 `internal/assets.NewStoreFromEnv` 创建。公司环境应使用：

- `ASSET_STORE=oss`
- `OSS_BUCKET`
- `OSS_ENDPOINT`
- `OSS_ACCESS_KEY_ID`
- `OSS_ACCESS_KEY_SECRET`

本地练习仍可用 `ASSET_STORE=local` 和 `UPLOAD_DIR`，但这不是公司运行时口径。

## 4. 业务能力到代码位置

| 业务能力 | 前端入口 | 后端入口 | 数据/资产 |
| --- | --- | --- | --- |
| 门店列表/筛选 | `StoreList.tsx`、`api.ts` | `storespace.handler.listStores` | `tb_stores` |
| 门店创建/编辑/删除 | `CreateStoreModal.tsx`、`EditStoreModal.tsx`、`StoreDetail.tsx` | `POST/PATCH/DELETE /api/store-space/stores` | `tb_stores` 及关联表 |
| 设计图上传/标注/保存 | `DesignPlanTab.tsx`、`FloorPlanCanvas.tsx`、`AreaCardList.tsx` | `PUT /api/store-space/stores/{id}/design-plan` | `tb_store_design_plans`、`tb_design_plan_annotations`、OSS |
| 录像机维护 | `VideoChannelTab.tsx`、`StoreDetail.tsx` | `POST /stores/{id}/recorders`、`DELETE /recorders/{id}` | `tb_video_recorders` |
| 扫描有效通道 | `VideoChannelTab.tsx` | `POST /recorders/{id}/scan-channels` | 萤石 OpenAPI、`tb_video_channels` |
| 刷新单通道截图 | `VideoChannelTab.tsx` | `POST /channels/{id}/snapshot` | 萤石截图、OSS、`tb_channel_snapshots` |
| AI 识别通道 | `VideoChannelTab.tsx` | `POST /channels/{id}/recognize` | `internal/channelai`、`tb_video_channels` |
| 批量识别 | `VideoChannelTab.tsx` | `POST /recorders/{id}/recognize-channels` | 每次最多推进少量通道，当前按节流策略设计 |
| 通道人工确认 | `VideoChannelTab.tsx` | `PUT /channels/{id}/confirmation` | `tb_video_channels` |
| H5 Monitor | `pages/H5Monitor*.tsx`、`H5FlvPlayer.tsx` | `internal/h5monitor` | MySQL + 萤石播放地址 |
| 用户与权限 | `UserManagement.tsx`、`SystemTopBar.tsx` | `internal/app/users_handler.go`、`authz.go` | `tb_users`、角色权限表 |
| AI 设置 | `SystemTopBar.tsx`、`api.ts` | `/api/ai-settings` | 后端 provider reader |

## 5. API 合同索引

健康与认证：

```text
GET  /health
GET  /api/auth/me
POST /api/auth/logout
```

用户与设置：

```text
GET  /api/users
POST /api/users
PUT  /api/users/{id}
GET  /api/ai-settings
POST /api/ai-settings/toggle
```

门店空间资源：

```text
GET    /api/store-space/ezviz-accounts
POST   /api/store-space/ezviz-accounts
GET    /api/store-space/stores
POST   /api/store-space/stores
POST   /api/store-space/stores/check-duplicate
GET    /api/store-space/stores/{id}
PATCH  /api/store-space/stores/{id}
DELETE /api/store-space/stores/{id}
GET    /api/store-space/stores/{id}/design-plan-data
PUT    /api/store-space/stores/{id}/design-plan
GET    /api/store-space/stores/{id}/channel-data
GET    /api/store-space/stores/{id}/channel-mappings/export.xlsx
POST   /api/store-space/stores/{id}/recorders
DELETE /api/store-space/recorders/{recorder_id}
POST   /api/store-space/recorders/{recorder_id}/scan-channels
POST   /api/store-space/recorders/{recorder_id}/recognize-channels
GET    /api/store-space/channel-snapshots/{name}
DELETE /api/store-space/channels/{channel_id}
POST   /api/store-space/channels/{channel_id}/recognize
POST   /api/store-space/channels/{channel_id}/snapshot
POST   /api/store-space/channels/{channel_id}/unlock
PUT    /api/store-space/channels/{channel_id}/confirmation
```

H5 Monitor：

```text
GET /api/h5/orgs/{external_org_id}/monitor
```

受控 ops 入口仍存在，但公司运行时已删除 pg-mysql 导出类入口。使用 ops 前必须先确认权限、目标环境和风险。

## 6. 数据与资产模型

核心 MySQL 表：

- `tb_stores`：门店主表，当前有效业务口径依赖 `external_org_id`。
- `tb_store_areas`：门店业务区域。
- `tb_store_design_plans`：设计图文件和识别结果。
- `tb_design_plan_annotations`：设计图标注框。
- `tb_ezviz_accounts`：萤石账号。
- `tb_video_recorders`：录像机。
- `tb_video_channels`：通道、识别结果、人工确认状态。
- `tb_channel_snapshots`：通道截图台账。
- `tb_users` 及角色权限表：后台用户、角色、权限。
- `tb_asset_objects` / 资产访问相关表：OSS 对象映射与治理。

资产原则：

- 数据库只保存 logical key、路径、状态和元数据。
- PDF、预览图、缩略图、通道截图走 `internal/assets.Store`，公司环境落 OSS。
- 前端不直连 OSS；统一通过 Go 后端代理路径访问。

## 7. AI 与萤石集成规则

通道识别链路：

1. 后端调用萤石抓取通道截图。
2. 截图写入 OSS，并记录到 MySQL。
3. 后端把可访问图片交给 `internal/channelai` 识别。
4. 识别结果只做预填；用户确认后才成为正式通道映射。

重要规则：

- 已确认通道再次识别时，不应自动覆盖人工确认的类型和编号。
- 批量识别必须节流，避免 APISIX 504、萤石限流或 AI 超时。
- MiniMax/GPT 都可能返回非 JSON 或解释文本，后端需要兜底，但低置信结果仍需要人工复核。
- 日志不得记录 app secret、access token、完整签名播放 URL、OSS 密钥或完整敏感图片 URL。

## 8. 旧 designplan 兼容路径

`internal/designplan/*` 和 `/api/design-plan/*` 仍作为历史兼容模块存在，当前主业务应优先走 `store-space`。

待决策：

- 是否彻底下线旧 `designplan` 独立路由。
- 是否清理旧 `tb_design_plan_*` 兼容表。
- 下线前必须确认当前前端、H5、历史数据、用户流程和迁移脚本不再依赖。

## 9. 验证命令

本机 macOS 直接 `go test ./...` 可能触发测试二进制 `missing LC_UUID`，当前可靠门禁是编译关键测试包和服务：

```sh
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server
```

前端：

```sh
cd frontend
npm run build
```

涉及 UI 时，还必须读取：

- `docs/ui-standards.md`
- `docs/frontend-review-checklist.md`

并做实际浏览器验收。

## 10. 发布与回滚

公司发布：

- remote：`gitlab`
- 分支：`codex/containerize-single-image`
- 自动发布：推送后由公司 GitLab/K8s 构建发布。
- 发布前读：`docs/deploy-runbook.md`、`docs/codex-learning-state.md`。
- 发布后验：`/erzhuang-project/health` 返回 `database=mysql`、`asset_store=oss`。

韩国 Lighthouse 发布仍保留在 `docs/deploy-runbook.md`，用于个人练习和备用验证。

回滚注意：

- 不要回滚到依赖 PostgreSQL/Supabase 运行时的旧提交。
- 公司运行时旧库连接已删除，回滚只能选择仍兼容 MySQL/OSS 的提交。
- 数据删除、旧数据源清理、schema 破坏性调整必须先形成确认清单。

## 11. 协作规则

- 主会话负责架构索引、任务拆解、验收、合并、发布、回滚和项目记忆维护。
- 其他专项会话只承担具体阶段任务，完成后必须回写文档或向主会话交接。
- 新需求开始前，先用本文定位业务能力和代码位置，再决定是否拆前端、后端、DBA 或发布专项。
- 需要新开开发/评审/发布会话时，先生成 handoff 文档，写明目标、当前状态、相关文件、风险、验证方式和下一步。
