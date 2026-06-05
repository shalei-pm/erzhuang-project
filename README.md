# erzhuang-project

个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署流程。

## 本地运行

后端：

```sh
go test ./...
go run ./cmd/server
```

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

## 前端

前端工程位于 `frontend/`，技术栈为 Vite + React + TypeScript。

前端环境、验证记录和后续接入 nginx 或 Go 后端的说明见：

- `docs/frontend-learning-state.md`

## 部署

部署和回滚流程见：

- `docs/deploy-runbook.md`
