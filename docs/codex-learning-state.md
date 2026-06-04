# Codex Learning State

最后更新：2026-06-04

## 当前主题

学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证、回滚流程。

## 用户背景

- 用户是新氧青春的产品负责人，正在学习使用 Codex。
- 出于安全原因，当前阶段不接入公司开发环境。
- 用户希望先在个人项目和个人腾讯云 Lighthouse 服务器上练熟 Codex 的开发、测试、部署、回滚流程。
- 等个人练习链路成熟后，再考虑向研发申请公司环境权限。

## 沟通和操作偏好

- 默认中文沟通。
- 逐步教学，不只给命令。
- 重要操作先解释风险，再给命令。
- 解释每一步在真实研发流程里的对应含义。
- 可以动手实操，但每一步尽量可验证。
- 不接触公司环境、公司密钥、公司代码。
- 不建议使用云厂商主账号或高权限长期密钥。

## 本地项目状态

- 本地路径：`/Users/sylar/erzhuang-project`
- 当前已创建：
  - `AGENTS.md`
  - `docs/codex-learning-state.md`
- 当前已初始化：
  - Git 仓库，默认分支 `main`
  - Go module: `github.com/shalei-pm/erzhuang-project`
  - 最小 Go HTTP 服务骨架
- 当前已验证：
  - 项目内临时 Go 工具链：`.tools/go/bin/go`，版本 `go1.22.2 darwin/arm64`
  - `gofmt`
  - `go test ./...`
  - `go build -o bin/erzhuang-project ./cmd/server`
  - 本地启动服务并验证 `/health`
  - 本地启动服务并验证 `/api/tasks`
- 当前尚未创建：
  - GitHub 远程仓库
  - 部署脚本
- 当前本地限制：
  - 系统 PATH 暂时找不到全局 `go` 命令。
  - 已通过项目内 `.tools/go` 临时解决本项目的 Go 测试和构建问题。
  - 当前 Codex 沙箱下，Go 构建缓存需要显式设置到项目内绝对路径：`GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build`。

当前目录结构：

```text
.
├── .gitignore
├── AGENTS.md
├── README.md
├── bin
│   └── erzhuang-project
├── cmd
│   └── server
│       └── main.go
├── docs
│   └── codex-learning-state.md
├── go.mod
└── internal
    └── app
        ├── handler.go
        └── handler_test.go
```

## 技术方向

- 公司后端开发环境是 Go。
- 本项目优先练习 Go 后端。
- 目标链路：
  1. Codex 本地开发
  2. Git 管理代码
  3. 推送到 GitHub
  4. 腾讯云服务器拉取代码
  5. `go test`
  6. `go build`
  7. `systemctl restart`
  8. `curl /health` 验证
  9. 保留发布记录
  10. 支持回滚

## 个人腾讯云 Lighthouse 状态

- 系统：Ubuntu 24.04.4 LTS
- 机器名：`VM-0-12-ubuntu`
- 资源：约 2GB 内存，50GB 磁盘
- Git：`git 2.43.0`
- Go：`go1.22.2 linux/amd64`
- Docker：未安装
- UFW：inactive
- 安全主要依赖：腾讯云控制台安全组/防火墙

已有服务：

- `nginx`: 443，active
- Hermes Gateway:
  - 监听：`0.0.0.0:8644`
  - systemd 服务：`hermes-gateway.service`
- Feishu Poll Bot:
  - systemd 服务：`feishu-poll-bot.service`
- xray:
  - 本地代理：`127.0.0.1:10086`

## 已完成的服务器 Go Demo 练习

服务器上已有 Go demo 服务：

- 路径：`/opt/apps/codex-demo`
- 接口：
  - `GET /health`
  - `GET /api/tasks`
- systemd 服务：`codex-demo.service`
- 监听：`127.0.0.1:18080`
- 状态：active running
- 开机自启：enabled
- cgroup：`/system.slice/codex-demo.service`
- 当前 `/health` 返回：

```json
{"app":"codex-demo","status":"ok","version":"v1"}
```

已经练习过：

- 创建 Go 项目
- `gofmt`
- `go test ./...`
- `go build -o codex-demo .`
- `nohup` 临时启动
- `curl` 本机验证
- 创建 systemd service
- `systemctl start`
- `systemctl restart`
- `systemctl status`
- `journalctl` 查看日志
- `systemctl enable` 开机自启
- 从 v1 发版到 v2
- 从 v2 回滚到 v1
- 给 `/health` 写单元测试
- 故意改错 version，确认 `go test` 可以拦截问题，避免部署

## 重要学习点

第一次临时启动时，`codex-demo` 挂在 `hermes-gateway.service` 的 cgroup 下；后来改成 systemd 独立服务后，变成 `/system.slice/codex-demo.service`。

这说明：

- Hermes 适合做临时执行通道。
- 正式服务应该交给 systemd 管理。
- 真实生产环境中，服务归属、进程生命周期、日志和重启策略都应该清晰可控。

## 下一步目标

把服务器上的练习迁移到本地项目 `erzhuang-project` 中，建立更真实的链路：

1. 本地 Codex 开发 Go 项目。
2. Git 管理代码。
3. 推送到 GitHub。
4. 服务器拉取代码。
5. 服务器执行 `go test` 和 `go build`。
6. `systemctl restart`。
7. `curl /health` 验证。
8. 保留发布记录。
9. 支持回滚。

后续希望做到：用户对 Codex 说“开发并发版”，Codex 能通过 GitHub + SSH 或受控部署脚本完成发布，不再每一步都让用户转述给 Hermes。

## 建议的本地 Go 项目初始化路线

第一阶段：本地最小服务

1. 初始化 Git 仓库。已完成。
2. 初始化 Go module。已完成。
3. 创建最小 HTTP 服务。已完成。
4. 实现。已完成：
   - `GET /health`
   - `GET /api/tasks`
5. 添加单元测试。已完成。
6. 本地验证。已完成：
   - `.tools/go/bin/gofmt -w cmd/server/main.go internal/app/handler.go internal/app/handler_test.go`
   - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build .tools/go/bin/go test ./...`
   - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build .tools/go/bin/go build -o bin/erzhuang-project ./cmd/server`
   - `ADDR=127.0.0.1:18080 ./bin/erzhuang-project`
   - `curl http://127.0.0.1:18080/health`
   - `curl http://127.0.0.1:18080/api/tasks`

第二阶段：GitHub

1. 创建 GitHub 仓库。
2. 添加远程仓库 `origin`。
3. 推送 `main` 分支。
4. 学习常用 Git 流程：
   - `git status`
   - `git add`
   - `git commit`
   - `git log`
   - `git diff`
   - `git push`
   - `git pull`
   - tag 或 release 标记版本

第三阶段：服务器拉取和发布

1. 在服务器准备部署目录，例如 `/opt/apps/erzhuang-project`。
2. 从 GitHub clone 或 pull 代码。
3. 在服务器运行：
   - `go test ./...`
   - `go build -o erzhuang-project .`
4. 配置或更新 systemd service。
5. `systemctl restart`。
6. `curl /health` 验证。
7. 记录发布。

第四阶段：受控部署脚本

等手动流程熟悉后，再把固定步骤整理为脚本，例如：

```text
git pull
go test ./...
go build
systemctl restart
curl /health
record release
```

脚本要避免保存高权限长期密钥。

## 风险提醒模板

后续执行重要操作前，先说明风险：

- `git init`: 会在当前目录创建版本管理元数据，通常安全；如果目录里已有复杂文件，需要确认不要误提交敏感信息。
- `git push`: 会把本地代码上传到 GitHub；推送前要确认没有密钥、公司代码、个人隐私文件。
- SSH 到服务器：可能修改线上运行状态；执行前要说明会影响哪个服务、端口和目录。
- `systemctl restart`: 会重启服务；如果服务配置或构建产物有问题，可能造成服务短暂不可用。
- 修改 nginx：可能影响已有 HTTPS 服务；需要备份配置并执行 `nginx -t`。
- 安全组/防火墙放行端口：会增加公网暴露面；优先只开放必要端口。

## 待补充信息

- GitHub 仓库名：建议 `erzhuang-project`
- Go module 名：`github.com/shalei-pm/erzhuang-project`
- 本地服务端口：默认 `127.0.0.1:18080`
- 服务器部署目录：
- 服务器服务名：
- GitHub 访问方式：SSH key 或 HTTPS token
- 是否需要域名：
- 是否需要 nginx 反向代理：

## 发布记录

当前本地项目还没有发布记录。

## 本地验证记录

- 2026-06-04：本地 Go 服务 v1 骨架验证通过。
  - `/health` 返回：`{"app":"erzhuang-project","status":"ok","version":"v1"}`
  - `/api/tasks` 返回 3 条练习任务。

服务器旧 demo 记录：

- v1：`/health` 返回 version `v1`
- v2：已练习发布
- rollback：已从 v2 回滚到 v1
