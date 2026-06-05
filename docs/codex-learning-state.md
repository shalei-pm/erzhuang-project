# Codex Learning State

最后更新：2026-06-05

## 当前主题

学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证、回滚流程。

当前新增产品需求讨论：

- 项目方向：设计图标记与诊室区域管理。
- 状态：已形成 PRD 草稿和技术方案草稿。
- 文档：`docs/design-plan-marker-prd.md`。
- 技术方案：`docs/design-plan-marker-tech-plan.md`。
- 已进入代码实现拆分阶段。
- 已安排两个专项会话：
  - 后端 Phase 1：数据模型、schema、CRUD API、校验、重复检查、操作日志。
  - 前端 Phase 2：后台风格 UI、门店列表、编辑弹窗、区域卡片、图纸标注交互骨架。
- 旧前端技术栈会话仅负责 Vite + React + TypeScript 工程初始化，已完成并待命，不再承接当前业务功能。
- 后端专项会话：
  - thread: `019e978c-9e0d-7f53-b48a-75679af9369b`
  - worktree: `/Users/sylar/.codex/worktrees/e6f9/erzhuang-project`
- 前端专项会话：
  - thread: `019e978c-f41f-78d0-a5db-6b940b928c3f`
  - worktree: `/Users/sylar/.codex/worktrees/34e2/erzhuang-project`
- 测试样例：
  - `testdata/design-plans/sample-store-floor-plan.pdf`
  - `testdata/design-plans/generated/sample-store-floor-plan.png`
  - 用途：前端 mock 图纸预览、后续 PDF 转图片和 AI 识别联调。
  - 状态：用户确认该数据不敏感，已提交到 GitHub。
- 技术架构索引：
  - `docs/technical-architecture-index.md`
  - 用途：后续迭代前先定位业务能力对应的前端、后端、数据库和验证入口，避免整体重写。

## 协作模式

当前主会话作为项目架构和交付中枢：

- 需求澄清
- 技术架构
- 任务拆解
- 前后端边界定义
- 验收标准
- 专项 Codex 会话调度
- 合并判断
- 发布、验证、回滚
- 腾讯云 Lighthouse / nginx 操作

专项会话用于聚焦实现：

- 前端会话：`frontend/`、前端工程、页面、构建、本地验证
- 后端会话：Go API、`cmd/server`、`internal/`、后端测试
- 部署专项会话：未来如有需要，再单独拆分

原则：

- 只有主会话操作腾讯云 API/TAT、nginx、systemd、发布和回滚。
- 专项会话只做代码实现和本地验证，不使用云密钥，不直接改服务器。
- 专项会话完成后提交分支，主会话负责验收、合并和发布。
- 详细规则见 `docs/architecture.md`。

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
  - 已推送 `main` 分支到 GitHub：`git@github.com:shalei-pm/erzhuang-project.git`
  - Lighthouse 服务器已通过 GitHub Deploy Key 只读拉取代码
  - Lighthouse 服务器已完成 `go test`、`go build`、systemd 启动、开机自启和 `/health` 验证
  - 已完成从 v1 到 v2 的服务器发布练习
  - 已完成从 v2 回滚到 v1 的服务器回滚练习
  - 已验证 `scripts/deploy.sh` 可以一键发布当前 `main`
  - 已验证 `scripts/rollback.sh <commit-or-tag>` 可以一键回滚
  - 已通过 nginx 暴露公网 HTTPS 路径 `/erzhuang/`
  - 已通过腾讯云 TAT/API 验证可管理韩国 Lighthouse 实例
  - 已确定当前数据库练习方案采用 Supabase PostgreSQL
- 当前待验证：
  - Supabase 项目创建
  - 后端通过环境变量连接 Supabase PostgreSQL
  - 服务器通过 systemd 环境变量连接 Supabase PostgreSQL
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
- 数据库练习优先采用 Supabase PostgreSQL，暂不在 2GB Lighthouse 上安装 MySQL。
- 数据库密钥和连接串只通过环境变量配置，不提交到 GitHub。
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

## 数据库方案

当前决策：

- 使用 Supabase 创建个人练习用 PostgreSQL。
- Go 后端通过 `DATABASE_URL` 读取连接串。
- Lighthouse 上通过 systemd 环境变量注入连接串。
- 不把数据库密码、Supabase Key、连接串写入仓库。
- 暂不安装本机 MySQL，避免把当前学习重点转成数据库运维。

详细计划见 `docs/database-plan.md`。

## 当前进度快照

截至 2026-06-04 下班前，已完成：

1. 本地项目上下文文档：
   - `AGENTS.md`
   - `docs/codex-learning-state.md`
2. 本地 Go 服务骨架：
   - module: `github.com/shalei-pm/erzhuang-project`
   - `GET /health`
   - `GET /api/tasks`
   - `/health` 单元测试
   - `/api/tasks` 单元测试
3. 本地验证：
   - `gofmt` 已通过
   - `go test ./...` 已通过
   - `go build -o bin/erzhuang-project ./cmd/server` 已通过
   - 本地 `curl /health` 已通过
   - 本地 `curl /api/tasks` 已通过
4. Git 和 GitHub：
   - 本地 Git 仓库已初始化，分支 `main`
   - GitHub 仓库已创建：`git@github.com:shalei-pm/erzhuang-project.git`
   - 本地 `main` 已推送到 `origin/main`
   - 本机 GitHub SSH 已配置成功
5. 安全边界：
   - `.tools/`、`.cache/`、`bin/`、`.ssh/` 已加入 `.gitignore`
   - 本机个人 SSH key 只用于开发机向 GitHub push
   - 后续服务器拉取代码计划使用单独的 GitHub Deploy Key，不复用个人 SSH key

明天继续的核心目标：

1. 给 Lighthouse 服务器配置仓库级 read-only Deploy Key。
2. 让服务器从 GitHub clone/pull `erzhuang-project`。
3. 在服务器执行 `go test` 和 `go build`。
4. 准备或更新 `erzhuang-project.service`。
5. 用 `systemctl restart` 发布。
6. 用 `curl /health` 验证。
7. 记录第一次服务器发布。
8. 再讨论如何设计受控部署脚本和回滚策略。

## 2026-06-05 服务器首次发布进度

已完成：

1. GitHub Deploy Key：
   - 在 Lighthouse 服务器生成专用于本仓库的 SSH key。
   - 将公钥添加到 GitHub 仓库 `shalei-pm/erzhuang-project` 的 Deploy keys。
   - 权限策略：read-only，不允许写仓库。
   - 验证结果：服务器可通过 Deploy Key 访问仓库。

2. 服务器拉取代码：
   - 部署目录：`/opt/apps/erzhuang-project`
   - clone 仓库：`git@github.com:shalei-pm/erzhuang-project.git`
   - 服务器当前 commit：`0f1699e Document next deployment steps`

3. 服务器测试和构建：
   - Go 版本：`go1.22.2 linux/amd64`
   - `go test ./...` 通过
   - `go build -o erzhuang-project ./cmd/server` 通过
   - 构建产物：`/opt/apps/erzhuang-project/erzhuang-project`

4. systemd 服务：
   - 服务名：`erzhuang-project.service`
   - service 文件：`/etc/systemd/system/erzhuang-project.service`
   - 运行用户：`lighthouse`
   - 工作目录：`/opt/apps/erzhuang-project`
   - 启动命令：`/opt/apps/erzhuang-project/erzhuang-project`
   - 环境变量：`ADDR=127.0.0.1:18081`
   - 状态：active running
   - 开机自启：enabled
   - cgroup：`/system.slice/erzhuang-project.service`

5. 健康检查：
   - 验证命令：`curl -s http://127.0.0.1:18081/health`
   - 返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要边界：

- 新服务监听 `127.0.0.1:18081`，避免和旧 `codex-demo.service` 的 `127.0.0.1:18080` 冲突。
- 本次没有修改 nginx。
- 本次没有暴露公网端口。
- 本次没有重启 Hermes、Feishu Poll Bot、xray 或旧 `codex-demo.service`。

## 2026-06-05 v2 发布进度

已完成：

1. 本地代码变更：
   - 将 `/health` 返回的 `version` 从 `v1` 改为 `v2`。
   - 将 `/health` 单元测试改为明确期待 `v2`。
   - commit：`bc8d5a3 Release health version v2`

2. 本地验证：
   - `gofmt` 已执行。
   - `go build -o bin/erzhuang-project ./cmd/server` 通过。
   - 本地 `go test ./...` 由于 macOS 26.5 + 项目内 Go 1.22.2 运行测试二进制时出现 `dyld: missing LC_UUID load command`，未通过。
   - 决策：不把本地测试环境问题当作业务通过，改为以服务器 Linux 环境的 `go test ./...` 作为本次发布门禁。

3. GitHub：
   - v2 commit 已推送到 `origin/main`。

4. 服务器发布：
   - 服务器执行 `git pull --ff-only`，从 `0f1699e` 快进到 `bc8d5a3`。
   - 服务器执行 `go test ./...` 通过。
   - 服务器执行 `go build -o erzhuang-project ./cmd/server` 通过。
   - 执行 `sudo systemctl restart erzhuang-project.service`。
   - systemd 状态：active running。
   - cgroup：`/system.slice/erzhuang-project.service`
   - `/health` 返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

重要学习点：

- GitHub 上有新 commit 不等于服务器已经上线，服务器必须显式 pull/build/restart。
- 本地门禁失败时不能装作通过，要记录原因并选择可信的替代门禁。
- 本次服务器 Linux 环境的测试通过后才执行了 systemd restart。
- `git pull --ff-only` 能保证服务器只做快进更新，不产生自动 merge commit。

## 2026-06-05 v2 回滚到 v1 进度

已完成：

1. 回滚目标：
   - 从服务器运行的 v2 commit `bc8d5a3` 回滚到 v1 commit `fbdb249`。
   - `fbdb249` 对应代码中 `/health` 的 `version` 为 `v1`。

2. 执行过程：
   - 服务器尝试执行 `git fetch origin`，但因为没有带 `GIT_SSH_COMMAND='ssh -i ~/.ssh/erzhuang_project_deploy_key -o IdentitiesOnly=yes'`，返回 `git@github.com: Permission denied (publickey)`。
   - 由于服务器本地已经有目标 commit `fbdb249`，继续执行 `git checkout fbdb249` 成功。
   - 服务器进入 detached HEAD 状态，这是本次临时回滚练习可接受的状态。
   - 执行 `go test ./...` 通过。
   - 执行 `go build -o erzhuang-project ./cmd/server` 通过。
   - 执行 `sudo systemctl restart erzhuang-project.service` 成功。
   - systemd 状态：active running。

3. 验证结果：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- 回滚不一定需要改 GitHub 历史，可以先让服务器 checkout 到已知稳定 commit。
- 如果目标 commit 已经存在于服务器本地，`git checkout <commit>` 不依赖网络。
- 访问 GitHub 的服务器命令必须显式使用 Deploy Key，例如：

```sh
GIT_SSH_COMMAND='ssh -i ~/.ssh/erzhuang_project_deploy_key -o IdentitiesOnly=yes' git fetch origin
```

- detached HEAD 适合临时验证和紧急回滚，但长期流程应通过 tag、release 记录或受控脚本管理。
- 本次回滚没有修改 nginx、没有开放公网端口、没有影响 Hermes、Feishu Poll Bot、xray 或旧 `codex-demo.service`。

## 2026-06-05 服务器恢复 main 分支

回滚完成后，服务器曾处于 detached HEAD。已执行：

1. 使用 Deploy Key 执行 `git fetch origin`。
2. `git switch main`。
3. 使用 Deploy Key 执行 `git pull --ff-only`。

结果：

- 服务器 Git 工作区已恢复到 `main`。
- 服务器 `main` 已同步到 `origin/main`。
- 当前服务器工作区 HEAD：`87318ca Record v2 rollback exercise`。
- 运行中的服务仍返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- Git 工作区代码和正在运行的二进制是两回事。
- 服务器工作区已经是最新 `main`，其中代码包含 v2。
- 但没有重新执行 `go build` 和 `systemctl restart`，所以运行中的服务仍是之前回滚构建出的 v1。
- 服务器 `git status` 出现 `?? erzhuang-project`，这是在项目根目录构建出的二进制文件。
- 已在本地 `.gitignore` 增加 `/erzhuang-project`，后续服务器 pull 后该构建产物不应再显示为未跟踪文件。

## 2026-06-05 一键发布脚本验证

已完成：

1. 服务器 pull 最新 `main` 到 `2a40ec9 Add deployment runbook and scripts`。
2. 执行 `./scripts/deploy.sh`。
3. 脚本自动完成：
   - 使用 Deploy Key fetch `origin/main`
   - 将本地 `main` 指向 `origin/main`
   - 输出当前 commit
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
   - `sudo systemctl restart erzhuang-project.service`
   - `curl -fsS http://127.0.0.1:18081/health`

结果：

- 发布 commit：`2a40ec9 Add deployment runbook and scripts`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

重要学习点：

- 现在已经可以用 `./scripts/deploy.sh` 完成标准发布。
- 脚本内部封装了 Deploy Key，不需要每次手写 `GIT_SSH_COMMAND`。
- 脚本会先测试再构建再重启，失败会停止。

## 2026-06-05 一键回滚脚本验证

已完成：

1. 服务器 pull 最新 `main` 到 `70d94db Record deploy script verification`。
2. 执行 `./scripts/rollback.sh fbdb249`。
3. 脚本自动完成：
   - 使用 Deploy Key fetch 远程 refs 和 tags
   - checkout 到目标 commit `fbdb249`
   - 输出当前 commit
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
   - `sudo systemctl restart erzhuang-project.service`
   - `curl -fsS http://127.0.0.1:18081/health`

结果：

- 回滚后运行 commit：`fbdb249 Record first Lighthouse deployment`
- 服务器 Git 状态：detached HEAD
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- 现在可以用 `./scripts/rollback.sh <commit-or-tag>` 完成标准回滚。
- 回滚到 commit 会让服务器进入 detached HEAD，这是脚本文档中已说明的预期行为。
- 后续使用 `./scripts/deploy.sh` 可以恢复到最新 `main` 并重新发布。

## 2026-06-05 公网 HTTPS 入口

已完成：

1. 腾讯云 API/TAT：
   - 使用腾讯云 API 只读查询韩国区 `ap-seoul` Lighthouse 实例。
   - 确认目标实例：
     - InstanceId: `lhins-rjfpwj1u`
     - Public IP: `43.155.237.46`
     - OS: Ubuntu Server 24.04 LTS 64bit
     - State: RUNNING
   - 明确边界：只操作韩国区实例，不操作日本区实例。
   - 使用 TAT 在目标实例执行只读检查和 nginx 配置操作。

2. nginx：
   - 配置文件：`/etc/nginx/sites-enabled/vpn-proxy`
   - 保留原有 `/` 和 `/vless`。
   - 新增：

```nginx
location = /erzhuang {
    return 301 /erzhuang/;
}

location /erzhuang/ {
    proxy_pass http://127.0.0.1:18081/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

3. nginx 备份处理：
   - 曾将备份文件放到 `/etc/nginx/sites-enabled/`，导致 nginx 加载到重复 `server_name 43.155.237.46`，出现 warning。
   - 已将备份移动到 `/etc/nginx/backups/`。
   - `nginx -t` 通过。
   - 已执行 `systemctl reload nginx`。

4. Lighthouse 防火墙：
   - 原规则只放行 TCP 22 和 ICMP。
   - 已只在韩国实例 `lhins-rjfpwj1u` 添加 TCP 443 放行：
     - Protocol: TCP
     - Port: 443
     - CidrBlock: `0.0.0.0/0`
     - Description: `HTTPS for erzhuang nginx`

5. 验证结果：

```sh
curl -k https://43.155.237.46/erzhuang/health
curl -k https://43.155.237.46/erzhuang/api/tasks
```

返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

`/api/tasks` 也成功返回任务列表。

重要备注：

- 当前 HTTPS 证书来自 `/etc/xray/server.crt`，浏览器可能提示证书不受信任。
- 当前服务公网入口是 IP + 路径，不是正式域名。
- Go 服务仍只监听 `127.0.0.1:18081`，公网只通过 nginx 进入。
- 临时腾讯云 API 密钥已在聊天中暴露，完成后应在 CAM 中禁用或删除。

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

1. 创建 GitHub 仓库。已完成。
2. 添加远程仓库 `origin`。已完成。
3. 推送 `main` 分支。已完成。
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

1. 在服务器准备部署目录，例如 `/opt/apps/erzhuang-project`。已完成。
2. 从 GitHub clone 或 pull 代码。已完成。
3. 在服务器运行。已完成：
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
4. 配置或更新 systemd service。已完成。
5. `systemctl start`。已完成。
6. `curl /health` 验证。已完成。
7. `systemctl enable` 开机自启。已完成。
8. 记录发布。已完成。

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

- GitHub 仓库名：`erzhuang-project`
- Go module 名：`github.com/shalei-pm/erzhuang-project`
- 本地服务端口：默认 `127.0.0.1:18080`
- 服务器部署目录：`/opt/apps/erzhuang-project`
- 服务器服务名：`erzhuang-project.service`
- 服务器监听地址：`127.0.0.1:18081`
- GitHub 访问方式：
  - 本机 push：个人 GitHub SSH key，已成功
  - 服务器 pull：GitHub Deploy Key，read-only，已成功
- 是否需要域名：
- 是否需要 nginx 反向代理：

## 发布记录

### 2026-06-05 Supabase PostgreSQL 接入发布

- 发布目标：个人腾讯云 Lighthouse 韩国实例
- 实例：`ap-seoul / lhins-rjfpwj1u`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`186cf5d`
- commit message：`Make deploy health retry robust`
- 数据库：Supabase PostgreSQL Shared Pooler
- 数据库配置方式：
  - systemd `EnvironmentFile=/etc/erzhuang-project.env`
  - 文件权限：`root root 600`
  - 未将连接串写入 GitHub
- 代码变化：
  - 新增 `Store` 接口
  - 新增 memory store
  - 新增 PostgreSQL store
  - `/api/tasks` 支持从 PostgreSQL 读取
  - `/health` 增加 `database` 字段
  - 自动创建 `tasks` 表并写入练习种子数据
  - 部署脚本健康检查增加重试，适配数据库冷启动时间
- 服务器验证：
  - `go test ./...` 通过
  - `go build -o erzhuang-project ./cmd/server` 通过
  - `npm install` 通过，0 个已知漏洞
  - `npm run build` 通过
  - `erzhuang-project.service` active running
- 公网健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

- 公网任务接口：
  - `https://43.155.237.46/erzhuang/api/tasks`
  - 返回 4 条数据库任务
  - 包含 `接入 Supabase PostgreSQL`
- 过程备注：
  - 首次数据库发布时，服务冷启动需要约 2 秒连接数据库和初始化 schema。
  - 原部署脚本重启后立刻 `curl`，导致误判失败。
  - 已修复为最多 15 次健康检查重试。
  - 真实服务已成功启动并连接 PostgreSQL。

### 2026-06-05 前端公网入口发布

- 发布目标：个人腾讯云 Lighthouse 韩国实例
- 实例：`ap-seoul / lhins-rjfpwj1u`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`df7da52`
- commit message：`Prepare frontend deployment path`
- Node 版本：`v22.22.2`
- npm 版本：`10.9.7`
- Go 版本：`go1.22.2 linux/amd64`
- 后端测试：`go test ./...` 通过
- 后端构建：`go build -o erzhuang-project ./cmd/server` 通过
- 前端安装：`npm install` 通过，0 个已知漏洞
- 前端构建：`npm run build` 通过
- 前端构建产物：
  - `frontend/dist/index.html`
  - `frontend/dist/assets/index-DAq-dh7A.css`
  - `frontend/dist/assets/index-DCmH9dBt.js`
- systemd：`erzhuang-project.service` 已重启，active running
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

- nginx 路由：
  - `/erzhuang/` 返回前端静态页面
  - `/erzhuang/health` 反向代理到 Go 后端 `/health`
  - `/erzhuang/api/` 反向代理到 Go 后端 `/api/`
- 公网验证：
  - `https://43.155.237.46/erzhuang/` 返回前端 HTML，HTTP 200
  - `https://43.155.237.46/erzhuang/assets/index-DCmH9dBt.js` 返回 JS，HTTP 200
  - `https://43.155.237.46/erzhuang/health` 返回健康 JSON，HTTP 200
  - `https://43.155.237.46/erzhuang/api/tasks` 返回任务 JSON，HTTP 200
- 备份：
  - nginx 修改前已备份到 `/etc/nginx/backups/`
- 影响范围：
  - 保留原 `/vless` 配置
  - 未改日本 Lighthouse 实例
  - 未接触公司环境、公司代码、公司密钥

### 2026-06-05 首次 Lighthouse 发布

- 发布目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`0f1699e`
- commit message：`Document next deployment steps`
- Go 版本：`go1.22.2 linux/amd64`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 测试命令：`go test ./...`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- 开机自启：enabled
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

- 影响范围：
  - 未修改 nginx
  - 未开放公网端口
  - 未影响 `codex-demo.service`
  - 未影响 `hermes-gateway.service`
  - 未影响 `feishu-poll-bot.service`
  - 未影响 xray

### 2026-06-05 v2 发布

- 发布目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`bc8d5a3`
- commit message：`Release health version v2`
- 测试命令：`go test ./...`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 发布命令：`sudo systemctl restart erzhuang-project.service`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

- 影响范围：
  - 未修改 nginx
  - 未开放公网端口
  - 未影响 `codex-demo.service`
  - 未影响 `hermes-gateway.service`
  - 未影响 `feishu-poll-bot.service`
  - 未影响 xray

### 2026-06-05 v2 回滚到 v1

- 回滚目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 回滚前运行 commit：`bc8d5a3`
- 回滚后运行 commit：`fbdb249`
- 回滚方式：服务器 `git checkout fbdb249`
- 服务器 Git 状态：detached HEAD
- 测试命令：`go test ./...`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 发布命令：`sudo systemctl restart erzhuang-project.service`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

- 过程备注：
  - `git fetch origin` 因未带 Deploy Key 参数失败：`Permission denied (publickey)`。
  - 目标 commit 已在本地存在，因此 checkout 和回滚仍成功。
  - 后续脚本必须统一封装 `GIT_SSH_COMMAND`。

## 本地验证记录

- 2026-06-04：本地 Go 服务 v1 骨架验证通过。
  - `/health` 返回：`{"app":"erzhuang-project","status":"ok","version":"v1"}`
  - `/api/tasks` 返回 3 条练习任务。
- 2026-06-04：完成第一次本地 Git 提交。
  - commit: `245e873 Initial Go service skeleton`
- 2026-06-04：推送本地 `main` 分支到 GitHub。
  - remote: `git@github.com:shalei-pm/erzhuang-project.git`
  - pushed commits:
    - `245e873 Initial Go service skeleton`
    - `cf99612 Document initial local verification`
    - `179f8a9 Ignore local SSH metadata`
- 2026-06-04：记录 GitHub 推送学习状态。
  - commit: `76b0400 Document GitHub push`

## 明日待办

1. 开始前先运行：

```sh
git status -sb
git pull --ff-only
```

2. 后续增强：
   - 在服务器 pull 最新脚本
   - 在服务器 pull 最新 `.gitignore`，确认 `?? erzhuang-project` 消失
   - 给 v1/v2 创建 tag，练习基于 tag 的发布和回滚
   - 删除或禁用本次使用的临时腾讯云 API 密钥
   - 后续如需正式公网访问，配置域名和可信 HTTPS 证书

服务器旧 demo 记录：

- v1：`/health` 返回 version `v1`
- v2：已练习发布
- rollback：已从 v2 回滚到 v1
