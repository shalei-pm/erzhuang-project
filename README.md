# erzhuang-project

个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署流程。

## 本地运行

后端：

```sh
go test ./...
go run ./cmd/server
```

设计图 PDF 上传识别依赖：

- `pdftoppm`：用于 PDF 转 PNG，Ubuntu 可通过 `sudo apt install poppler-utils` 安装。
- `OPENAI_API_KEY`：用于 AI 识别图纸，放在服务器环境变量或 systemd EnvironmentFile，不提交到 Git。
- `OPENAI_MODEL`：可选，默认 `gpt-4o`。
- `OPENAI_BASE_URL`：可选，默认 `https://api.openai.com`；自定义兼容网关可覆盖。
- `OPENAI_API_STYLE`：可选，默认 `responses`；兼容网关可设置为 `openai-completions`。
- `ASSET_STORE`：可选，默认 `local`。公司 K8s 可设置为 `supabase`。
- `UPLOAD_DIR`：可选，默认 `uploads/design-plan`。
- `SUPABASE_URL`、`SUPABASE_SERVICE_ROLE_KEY`、`SUPABASE_STORAGE_BUCKET`：当 `ASSET_STORE=supabase` 时必填。service role key 只放服务端环境变量，不提交到仓库，也不放前端 `VITE_*` 配置。

监控截图区域识别依赖：

- `CHANNEL_AI_PROVIDER`：可选，默认 `openai`。可设置为 `minimax-script` / `external-command` 来调用外部图像理解脚本。
- `VISION_API_KEY`：监控截图识别的 OpenAI-compatible key；未设置时回退 `OPENAI_API_KEY`。
- `VISION_API_BASE_URL`：可选，监控截图识别网关；未设置时回退 `OPENAI_BASE_URL`。
- `VISION_MODEL`：可选，默认 `gpt-5.5`。
- MiniMax Token Plan 可直接使用 OpenAI-compatible Responses API：`VISION_API_BASE_URL=https://api.minimaxi.com/v1`、`VISION_MODEL=MiniMax-M3`、`VISION_API_KEY=<Token Plan 订阅 Key>`。
- `MINIMAX_UNDERSTAND_IMAGE_SCRIPT`：当 `CHANNEL_AI_PROVIDER=minimax-script` 时使用，默认 `/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py`。
- `CHANNEL_AI_COMMAND_ARGS`：外部脚本参数模板，可用 `{image_url}` 和 `{prompt}` 占位；未设置时默认 `--image-url {image_url}`。

服务默认监听：

```text
127.0.0.1:18080
```

验证：

```sh
curl http://127.0.0.1:18080/health
curl http://127.0.0.1:18080/api/tasks
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

## 接口

- `GET /health`
- `GET /api/tasks`
- `GET /api/design-plan/stores?q=&page=1&page_size=20`
- `GET /api/design-plan/stores/{id}`
- `GET /api/design-plan/stores/{id}/preview`
- `GET /api/design-plan/stores/{id}/thumbnail`
- `POST /api/design-plan/stores`
- `PUT /api/design-plan/stores/{id}`
- `DELETE /api/design-plan/stores/{id}`
- `POST /api/design-plan/stores/check-duplicate`
- `POST /api/design-plan/uploads`
- `GET /api/design-plan/uploads/{upload_id}/preview`
- `GET /api/design-plan/uploads/{upload_id}/thumbnail`
- `POST /api/design-plan/uploads/{upload_id}/recognize`

## 前端

前端工程位于 `frontend/`，技术栈为 Vite + React + TypeScript。

前端环境、验证记录和后续接入 nginx 或 Go 后端的说明见：

- `docs/frontend-learning-state.md`

## 技术架构索引

后续迭代前，优先查看代码地图，按业务能力定位前端、后端、数据库和验证命令：

- `docs/technical-architecture-index.md`

## 数据库

当前计划使用 Supabase PostgreSQL 作为练习数据库。

数据库连接串通过环境变量配置，不提交到 GitHub。详细方案见：

- `docs/database-plan.md`

## 部署

部署和回滚流程见：

- `docs/deploy-runbook.md`


## 测试CI
