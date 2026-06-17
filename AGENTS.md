# AGENTS.md

本项目最初是个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证和回滚流程。当前已经新增公司 GitLab + K8s 自动发布链路；后续涉及公司环境发布时，以公司 GitLab 分支流程为准。

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

形成一条可重复的真实研发链路：

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

公司环境当前目标是：本地 Codex 在同一个仓库目录开发，确认后推送到公司 GitLab 固定分支，由公司流水线自动构建和发布，Codex 负责版本、分支、验证和文档记录。

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
- Supabase `public` schema 内的新表必须开启 Row-Level Security；除非明确设计前端直连 Supabase，否则不要给 anon/authenticated 角色添加业务表开放读写 policy，并应增加显式拒绝 policy：`using (false) with check (false)`。
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

## 公司 GitLab 与自动发布流程

公司环境发布链路已经接入公司 GitLab 和 K8s 自动发布。本项目后续涉及公司线上环境时，默认使用该流程：

- 公司 GitLab 仓库：`https://gitlab.sy.soyoung.com/pm/shalei-pm/erzhuang-project.git`。
- 本地 remote 名称：`gitlab`。
- 固定公司发布分支：`codex/containerize-single-image`。
- 公司 GitLab 分支约每 5 分钟自动发布一次。
- 本地当前目录 `/Users/sylar/erzhuang-project` 同时保留 GitHub 与 GitLab remote，不另开目录，避免上下文和代码状态分裂。
- 公司发布分支是受保护分支，不允许 force push；需要使用正常 commit、merge、push。
- 从个人 `main` 或其他开发分支同步到公司分支时，优先：
  1. `git fetch gitlab`
  2. `git switch codex/containerize-single-image`
  3. `git merge main`
  4. 本地验证
  5. `git push gitlab codex/containerize-single-image`
- 如果公司分支已有运维调整，例如 Dockerfile、K8s 环境变量、数据库连接方式，不要用本地个人配置覆盖。合并冲突时以公司运行配置为准，再把业务代码和必要文档合进去。
- 公司环境数据库和密钥必须通过运行时环境变量或 K8s Secret 注入。不要把 Supabase、OpenAI、萤石云、MiniMax 等密钥写入仓库、Dockerfile、前端 `VITE_*` 变量或文档。
- 公司环境前端版本号由容器构建时注入 `VITE_APP_VERSION`。Dockerfile 需要从 `VERSION` 和构建参数 `GIT_VERSION` 生成页面底部版本号，避免线上展示 `local-dev`。
- 推送后需要等待自动发布完成，再检查：
  - 页面底部版本号是否为 `VERSION (commit)` 或 `VERSION (container)`。
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 是否健康。
  - 关键页面是否能打开，静态资源和 API 路径是否仍走 `/erzhuang-project/` 前缀。

## 发布术语约定

用户说“发布到公司”时，固定含义是：

1. 将当前已确认代码 merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
2. 推送到 remote `gitlab`。
3. 等待公司 GitLab / K8s 自动发布，不操作韩国 Lighthouse。
4. 保留公司环境配置，禁止 force push，禁止用个人服务器配置覆盖公司运行配置。

用户说“发布到韩国服务器”时，固定含义是：

1. 将当前已确认代码推送到 GitHub `origin/main`。
2. 通过腾讯云 TAT 让韩国 Lighthouse 服务器拉取 GitHub 最新 `main`。
3. 服务器执行 `scripts/deploy.sh` 自动测试、构建、重启 `erzhuang-project.service`。
4. 验证 `http://127.0.0.1:18081/health` 和公网 `/erzhuang/` 入口。

如果用户同时要求两个环境，先确认代码已在同一 commit 或明确记录两个环境对应的 commit，避免线上版本对不齐。

## 前端验收门禁

前端改动不能只以 `npm run build` 通过作为完成标准。涉及页面布局、弹窗、表格、按钮、颜色、交互状态时，发布前必须做实际页面验收：

前端改动开始前必须先读取：

- `docs/ui-standards.md`
- `docs/frontend-review-checklist.md`

1. 本地启动前端预览，必要时使用 mock 数据覆盖列表、详情、弹窗、空态和有数据态。
2. 用浏览器实际打开页面，检查截图或可视页面，不只看 DOM 和类型检查。
3. 至少检查桌面常用宽度下的：
   - 页面顶部信息密度和对齐。
   - 弹窗尺寸、关闭按钮、主次按钮层级。
   - 表格列宽、操作按钮是否换行挤压。
   - 表单控件是否只出现在可编辑信息上，只读信息应陈列展示。
   - 焦点态、hover 态、空态、加载态、错误提示。
4. 如果视觉质量明显不如早期已认可版本，主会话应先打回或本地返工，不进入发布。
5. 前端验收结果应在最终回复里说明，重要页面可附截图结论。

## Codex 发布操作能力

本项目发布链路是固定基础能力，不应临场重新摸索。任何会话在执行“发布、上线、部署、回滚、线上验证”前必须先读取：

- `docs/deploy-runbook.md`
- `docs/codex-learning-state.md` 最近一次发布记录
- `scripts/deploy.sh`
- `scripts/rollback.sh`，仅在需要回滚时读取

标准链路：

1. 本地开发和验证。
2. 提交到 Git。
3. 推送 GitHub `main`。
4. 通过腾讯云 TAT 指定韩国 Lighthouse 实例 `ap-seoul / lhins-rjfpwj1u`。
5. 以 `lighthouse` 用户在服务器执行：
   `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`
6. 服务器从 GitHub 拉取最新 `main`。
7. 服务器执行 `go test ./...`。
8. 服务器执行 Go build。
9. 服务器执行前端 `npm install` 和 `VITE_APP_VERSION="<VERSION> (<commit>)" npm run build`。
10. 服务器重启 `erzhuang-project.service`。
11. 服务器验证 `curl http://127.0.0.1:18081/health`。
12. 主会话验证公网入口和页面版本号。
13. 将发布结果、版本、commit、验证结果和故障处理记录到 `docs/codex-learning-state.md`。

TAT 执行注意：

- `tools/tat_run.py` 使用 `getpass` 读取腾讯云 `SecretId` / `SecretKey`。
- 在 Codex 中执行时必须使用交互式 PTY：`tty=true`。
- 不要把 SecretId / SecretKey 直接拼进 shell 命令、脚本、文档或 Git。
- 只允许指定韩国实例 `lhins-rjfpwj1u`；不要操作日本实例。
- TAT 命令示例：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 900 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

发布失败处理：

- 不要先猜测或重复部署。
- 先通过 TAT 只读检查：
  - 当前服务器 commit 和 `VERSION`
  - `systemctl status erzhuang-project.service --no-pager`
  - `journalctl -u erzhuang-project.service -n 80 --no-pager`
  - `ss -ltnp | grep -E '18081|18080'`
  - `curl -sv http://127.0.0.1:18081/health`
- 如果服务不可用，先恢复可用状态；必要时使用 `scripts/rollback.sh <commit-or-tag>`。
- 定位到根因后，只做最小修复，升级版本号，推送 GitHub，再重新发布。

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
