# AGENTS.md

本项目是个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证和回滚流程。

## 用户背景

- 用户是新氧青春的产品负责人，正在学习使用 Codex。
- 因安全原因，当前阶段不接入公司开发环境。
- 先在个人项目和个人腾讯云 Lighthouse 服务器上练熟开发、测试、部署、回滚流程，再考虑向研发申请公司环境权限。
- 公司后端技术方向是 Go，因此本练习项目优先使用 Go 后端。

## 协作偏好

- 默认用中文沟通。
- 逐步教学，不只给命令，也解释每一步在真实研发流程中对应什么。
- 重要操作先解释风险，再给命令。
- 可以动手实操，但每次操作后尽量给出验证方式。
- 当前阶段不用过度保守，因为腾讯云轻量服务器可以专门供练习使用。
- 不接触公司环境、公司密钥、公司代码。
- 不直接建议使用云厂商主账号或高权限长期密钥。
- 不把重要部署动作隐藏在“魔法脚本”里，除非用户已经理解脚本里的关键步骤。

## 项目目标

形成一条可重复的真实研发练习链路：

1. 本地 Codex 开发 Go 项目。
2. 使用 Git 管理代码版本。
3. 推送到 GitHub。
4. 服务器从 GitHub 拉取代码。
5. 在服务器执行 `go test` 和 `go build`。
6. 通过 `systemctl restart` 发布服务。
7. 通过 `curl /health` 验证服务。
8. 保留发布记录。
9. 支持回滚。

长期目标是：用户可以对 Codex 说“开发并发版”，Codex 能通过 GitHub + SSH 或受控部署脚本完成发布，而不需要用户手动转述每一步给 Hermes。

## 当前服务器背景

个人腾讯云 Lighthouse：

- OS: Ubuntu 24.04.4 LTS
- 机器名：`VM-0-12-ubuntu`
- 资源：约 2GB 内存，50GB 磁盘
- Git: `git 2.43.0`
- Go: `go1.22.2 linux/amd64`
- Docker: 未安装
- UFW: inactive，安全主要依赖腾讯云控制台安全组/防火墙

已有服务：

- `nginx`: 443，active
- `hermes-gateway.service`: Hermes Gateway，监听 `0.0.0.0:8644`
- `feishu-poll-bot.service`: Feishu Poll Bot
- `xray`: 本地代理 `127.0.0.1:10086`

已有 Go demo 服务：

- 路径：`/opt/apps/codex-demo`
- systemd 服务：`codex-demo.service`
- 监听：`127.0.0.1:18080`
- 状态：active running
- 开机自启：enabled
- cgroup：`/system.slice/codex-demo.service`
- `/health` 当前返回：`{"app":"codex-demo","status":"ok","version":"v1"}`

重要学习点：

- 第一次临时启动时，`codex-demo` 挂在 `hermes-gateway.service` 的 cgroup 下。
- 改成 systemd 独立服务后，变成 `/system.slice/codex-demo.service`。
- 结论：Hermes 适合做临时执行通道，正式服务应该交给 systemd 管理。

## 开发原则

- 本地优先：先在 `/Users/sylar/erzhuang-project` 里开发和测试。
- 常规研发默认走分支和 PR：从 `main` 拉出 `codex/...` 分支开发，验证后推送到 GitHub，并用 GitHub CLI 创建 PR。
- 小步提交：每个可验证的小功能都适合形成一次 Git 提交；提交信息应描述业务或技术目标。
- 主会话负责 PR review、合并判断、发布和回滚；专项前端/后端会话负责各自分支内实现，并向主会话汇报结果。
- 紧急线上修复或用户明确要求快速闭环时，可以直接在 `main` 小步提交并发布，但需要在最终说明中标注原因。
- 测试先行或同步补测试：尤其是 `/health` 这类发布验证接口。
- 发布前必须跑：
  - `go test ./...`
  - `go build`
- 发布后必须验证：
  - `systemctl status`
  - `curl http://127.0.0.1:<port>/health`
- 每次发布和回滚，都在文档或发布记录中记录版本、时间、操作和验证结果。

## GitHub CLI 与 PR 流程

- 本机 GitHub CLI 路径：`/Users/sylar/.local/bin/gh`。
- `gh` 已登录 GitHub 账号：`shalei-pm`。
- Codex 在沙箱中调用 `gh` 访问 GitHub API 时，通常需要提升权限以访问网络和系统 keyring。
- 只读检查常用命令：
  - `/Users/sylar/.local/bin/gh auth status`
  - `/Users/sylar/.local/bin/gh repo view shalei-pm/erzhuang-project`
  - `/Users/sylar/.local/bin/gh pr list --repo shalei-pm/erzhuang-project`
- 标准研发流程：
  1. 从干净 `main` 创建 `codex/<task-name>` 分支。
  2. 实现代码和文档，按风险补测试。
  3. 本地验证通过后提交并推送分支。
  4. 用 `gh pr create` 创建 PR。
  5. 主会话 review PR，必要时打回专项会话修改。
  6. PR 合并后由主会话发布到 Lighthouse，并验证 `/health` 和页面版本号。
- 当前仓库暂未配置 GitHub Actions；后续如增加 CI，PR 合并前必须检查 CI 结果。

## 版本号规则

项目采用三段式版本号：`大版本.中版本.小版本`，例如 `1.2.3`。

- 大版本：从 `1` 开始。新增一个完整业务模块、一个及以上新页面、或改变核心业务流程时递增，例如新增“设备管理”模块或新增完整“门店详情页”。
- 中版本：从 `1` 开始。已有模块内的交互、样式、信息架构或业务流程小迭代时递增，例如优化设计图标注交互、调整颜色体系、补充重复门店确认流程。
- 小版本：从 `1` 开始。修复 bug、补测试、技术性整理、部署脚本小调整、文档修正时递增，例如修复输入框焦点残留、修复健康检查重试。

递增规则：

- 大版本递增时，中版本和小版本归零，例如 `1.4.7` -> `2.0.0`。
- 中版本递增时，小版本归零，例如 `1.4.7` -> `1.5.0`。
- 小版本递增时，只增加最后一段，例如 `1.4.7` -> `1.4.8`。
- 页面底部展示的版本号应尽量和本次发布版本一致；重要线上验收问题需要同时记录版本号和 Git commit。

## 文件约定

- `docs/codex-learning-state.md`: 记录学习进度、服务器状态、下一步计划、关键命令和发布记录。
- 后续可新增：
  - `README.md`: 项目说明和本地运行方式。
  - `cmd/server/main.go`: Go 服务入口。
  - `internal/...`: Go 内部业务代码。
  - `scripts/deploy.sh`: 受控部署脚本，等用户理解手动流程后再引入。
  - `docs/deploy-runbook.md`: 部署操作手册。

## Codex 工作方式

Codex 在本项目中应当：

1. 先检查目录结构和现有文档。
2. 说明将要做什么以及风险点。
3. 修改代码或文档。
4. 运行验证命令。
5. 把重要学习状态同步到 `docs/codex-learning-state.md`。
6. 最后用中文总结结果和下一步建议。
