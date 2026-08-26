# erzhuang-project

二壮项目最初是个人练习项目，用来学习 Codex 开发、Go 后端、GitHub、腾讯云 Lighthouse 部署和回滚。当前已经演进为新氧青春门店空间资源管理系统，并接入公司 GitLab + K8s 自动发布链路；二壮项目后续发布不再走韩国 Lighthouse。

当前主业务目标：以门店为主体，统一维护设计图、业务区域、录像机、通道截图、AI 识别和 H5 监控查看能力，替代原多维表格维护门店/录像机/通道/区域关系的核心流程。

## 当前状态

- 当前版本：`2.30.23`
- 公司运行时：MySQL + OSS
- 测试入口：`https://lite.sy.soyoung.com/erzhuang-project/`
- 正式入口：`http://lite.soyoung.com/erzhuang-project`
- 健康检查：`/erzhuang-project/health`
- 当前健康检查应返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"mysql","asset_store":"oss"}
```

重要口径：

- 当前有效门店数按 54 家有 `external_org_id` 的门店计算。
- 第 55 家空 `external_org_id` 门店“新氧青春诊所(长沙北辰荟店)”不迁移。
- PostgreSQL/Supabase 已不是公司运行时回滚路径。旧数据源归档/删除需要产品、安全、运维和相关研发共同确认。
- 历史上“发布到公司”一直发布到测试环境；2026-08-14 起项目准备切到公司正式环境，正式发布走 GitLab `main` 和正式 Wharf pipeline。`main` 提交会自动触发构建，构建成功后还需要在 Wharf 点部署并走审批。

## 项目记忆

长期主会话、新会话、专项会话或发布会话应优先读取：

- `docs/codex-learning-state.md`：长期状态、当前进度、发布记录、关键上下文。
- `docs/decisions.md`：产品/技术关键决策。
- `work/current-plan.md`：当前轮工作目标、进度、验证方式和下一步。
- `docs/technical-architecture-index.md`：当前代码地图和业务能力定位。

## 技术栈

- 后端：Go 1.22，`net/http`
- 前端：Vite + React + TypeScript + Ant Design
- 数据库：公司 MySQL，业务表以 `tb_` 前缀为主
- 资产：公司 OSS，后端代理访问设计图、预览图、通道截图
- 登录：公司 APISIX SSO，后端按 `tb_users` 做授权
- 监控：萤石云 OpenAPI + H5 播放组件
- AI：OpenAI-compatible provider，当前支持 GPT/OpenAI 类接口和 MiniMax

## 本地运行

后端：

```sh
go run ./cmd/server
```

服务默认监听：

```text
127.0.0.1:18080
```

前端：

```sh
cd frontend
npm install
npm run dev
```

生产构建：

```sh
cd frontend
npm run build
```

## 关键环境变量

公司运行时由 K8s Secret 或运行时环境变量注入，不能提交到仓库、Dockerfile、文档或前端 `VITE_*`。

数据库：

- `APP_DB_DRIVER=mysql`
- `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN`
- 测试二壮运行库：host `polar-dev.rwlb.rds.aliyuncs.com`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret/安全渠道管理。
- 正式二壮运行库：host `polar-ops.rwlb.rds.aliyuncs.com`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret/安全渠道管理。
- 3.0「门店空间资源查看」复用上述二壮主库连接，只读查询已同步的 `tb_crm_*` 资源表；不需要独立业务库 DSN。

资产：

- `ASSET_STORE=oss`
- `OSS_BUCKET`
- `OSS_ENDPOINT`
- `OSS_ACCESS_KEY_ID`
- `OSS_ACCESS_KEY_SECRET`

AI 与通道识别：

- `OPENAI_API_KEY`、`OPENAI_MODEL`、`OPENAI_BASE_URL`
- `VISION_API_KEY`、`VISION_MODEL`、`VISION_API_BASE_URL`
- `CHANNEL_AI_PROVIDER`

设计图 PDF 上传识别依赖：

- `pdftoppm`：用于 PDF 转 PNG，Ubuntu 可通过 `sudo apt install poppler-utils` 安装。
- `UPLOAD_DIR`：PDF 转图临时工作目录；在 OSS 模式下不是最终持久化目录。

## 主要接口

健康与认证：

- `GET /health`
- `GET /api/auth/me`
- `POST /api/auth/logout`

门店空间资源：

- `GET /api/store-space/stores`
- `POST /api/store-space/stores`
- `GET /api/store-space/stores/{id}`
- `PATCH /api/store-space/stores/{id}`
- `DELETE /api/store-space/stores/{id}`
- `GET /api/store-space/stores/{id}/design-plan-data`
- `PUT /api/store-space/stores/{id}/design-plan`
- `GET /api/store-space/stores/{id}/channel-data`
- `POST /api/store-space/stores/{id}/recorders`
- `POST /api/store-space/recorders/{recorder_id}/scan-channels`
- `POST /api/store-space/recorders/{recorder_id}/recognize-channels`
- `POST /api/store-space/channels/{channel_id}/recognize`
- `POST /api/store-space/channels/{channel_id}/snapshot`
- `PUT /api/store-space/channels/{channel_id}/confirmation`

H5 Monitor：

- `GET /api/h5/orgs/{external_org_id}/monitor`

历史兼容：

- `/api/design-plan/*` 仍保留为兼容路径。后续是否下线旧 `designplan` 路由和兼容表，需要单独确认。

## 验证

本机 macOS 直接 `go test ./...` 可能触发测试二进制 `missing LC_UUID` 问题。当前主会话常用编译门禁：

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

涉及页面布局、弹窗、表格、按钮、颜色和交互状态时，不能只看 build，必须按 `docs/ui-standards.md` 和 `docs/frontend-review-checklist.md` 做浏览器验收。

## 部署

公司环境默认走 GitLab + K8s 自动发布：

- remote：`gitlab`
- 测试分支：`codex/containerize-single-image`
- 测试 pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=752`
- 测试入口：`https://lite.sy.soyoung.com/erzhuang-project`
- 正式分支：`main`
- 正式 pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=771`
- 正式入口：`http://lite.soyoung.com/erzhuang-project`
- 文档：`docs/deploy-runbook.md`

GitHub 仍保留为代码备份和历史留存，不再承担二壮项目线上发布职责。

韩国 Lighthouse 发布链路已终止，且该服务器上关于二壮项目的库表已经完全删除；后续二壮项目发布、回滚和验收均不再使用韩国服务器。历史链路仅保留在文档中作为学习记录。

## 重要文档

- `docs/deploy-runbook.md`：发布、验证、回滚流程。
- `docs/technical-architecture-index.md`：当前代码地图。
- `docs/post-cutover-regression-checklist.md`：MySQL/OSS 切换后回归清单。
- `docs/legacy-postgres-supabase-shutdown-checklist.md`：旧 PostgreSQL/Supabase 下线确认清单。
- `docs/store-space-resource-prd.md`：门店空间资源产品需求。
- `docs/store-space-resource-tech-plan.md`：门店空间资源技术方案。
- `docs/model-provider-switching.md`：AI provider 切换说明。
