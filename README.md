# erzhuang-project

个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署流程。

## 本地运行

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

## 接口

- `GET /health`
- `GET /api/tasks`

## 数据库

当前计划使用 Supabase PostgreSQL 作为练习数据库。

数据库连接串通过环境变量配置，不提交到 GitHub。详细方案见：

- `docs/database-plan.md`

## 部署

部署和回滚流程见：

- `docs/deploy-runbook.md`
