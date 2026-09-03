# AGENTS.md

本项目最初是个人练习项目，用来学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证和回滚流程。当前已经切换为公司 GitLab + K8s 自动发布链路；后续二壮项目发布只走公司 GitLab/K8s，不再走韩国 Lighthouse。

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

公司环境当前目标是：本地 Codex 在同一个仓库目录开发，确认后先按需同步 GitHub 作为代码备份，再进入公司 GitLab 发布链路，Codex 负责版本、分支、验证和文档记录。历史测试环境发布使用 `codex/containerize-single-image`；正式环境发布使用 GitLab `main`，在 `main` 分支提交代码后自动触发正式 Wharf 构建，构建成功后还需要在 Wharf 点部署并走审批。韩国 Lighthouse 上关于二壮项目的库表已经完全删除，不再作为发布、回滚、验证或备用环境。

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
  6. PR 合并后由主会话按需同步 GitHub 备份，再发布到公司 GitLab/K8s，并验证 `/health`、页面版本号和本次改动路径。
- 当前仓库暂未配置 GitHub Actions；后续如增加 CI，PR 合并前必须检查 CI 结果。

## 公司 GitLab 与自动发布流程

公司环境发布链路已经接入公司 GitLab 和 K8s 自动发布。注意：历史上主会话里“发布到公司”实际一直发布到公司测试环境，不等于正式生产环境。

- 公司 GitLab 仓库：`https://gitlab.sy.soyoung.com/pm/shalei-pm/erzhuang-project.git`。
- 本地 remote 名称：`gitlab`。
- 当前测试发布分支：`codex/containerize-single-image`。
- 测试 Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=752`。
- 正式发布分支：`main`。
- 正式 Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=771`。
- 测试入口：`https://lite.sy.soyoung.com/erzhuang-project`，对接测试分支代码、测试实例机器和测试数据库。
- 正式入口：`http://lite.soyoung.com/erzhuang-project`，对接主干分支代码、主干实例机器和线上数据库。
- 后续用户说“发布到公司”如无进一步说明，先确认是测试还是正式；当前项目正在切到公司正式环境，不能再默认把测试分支发布当作最终发布。若用户说“正式环境/线上/生产/主干”，按 GitLab `main` 提交触发构建、Wharf 手动部署、审批通过后上线的正式链路处理，并确认正式实例、线上数据库、回滚点和验收清单。
- 本地当前目录 `/Users/sylar/erzhuang-project` 同时保留 GitHub 与 GitLab remote，不另开目录，避免上下文和代码状态分裂。
- 公司发布分支是受保护分支，不允许 force push；需要使用正常 commit、merge、push。
- GitLab HTTPS 推送凭据已由用户确认保存在本机安全文件：`/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`。发布到公司时，Codex 默认使用临时 `GIT_ASKPASS` 读取该文件推送，用户名使用 `oauth2`；不要把 token 内容打印到终端、写入命令、写入仓库、写入文档或长期 askpass 脚本。临时 askpass 用完必须删除。
- 二壮运行库环境：
  - 测试：host `polar-dev.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码只由 K8s Secret/安全渠道管理，不写入仓库。
  - 正式：host `polar-ops.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码只由 K8s Secret/安全渠道管理，不写入仓库。
  - 运行时通过 `K8S_SECRET_MYSQL_DSN` 或 `MYSQL_DSN` 注入；3.0 业务库只读连接使用独立的 `K8S_SECRET_BUSINESS_MYSQL_DSN` / `BUSINESS_MYSQL_DSN`，不要与二壮运行库混淆。
- 发布测试环境时，从个人 `main` 或其他开发分支同步到公司测试分支，优先：
  1. `git fetch gitlab`
  2. `git switch codex/containerize-single-image`
  3. `git merge main`
  4. 本地验证
  5. `git push gitlab codex/containerize-single-image`
- 发布正式环境时，优先：
  1. `git fetch gitlab`
  2. 确认待发布代码和 `main` 的差异、回滚点和验证清单。
  3. 按公司允许的方式把已验证代码提交到 GitLab `main`。
  4. 观察 Wharf pipeline `771` 构建结果。
  5. 构建成功后在 Wharf 点部署并走审批。
  6. 审批通过且部署成功后，验证 `http://lite.soyoung.com/erzhuang-project`。
- 如果公司分支已有运维调整，例如 Dockerfile、K8s 环境变量、数据库连接方式，不要用本地个人配置覆盖。合并冲突时以公司运行配置为准，再把业务代码和必要文档合进去。
- 公司环境数据库和密钥必须通过运行时环境变量或 K8s Secret 注入。不要把 Supabase、OpenAI、萤石云、MiniMax 等密钥写入仓库、Dockerfile、前端 `VITE_*` 变量或文档。
- 公司环境前端版本号由容器构建时注入 `VITE_APP_VERSION`。Dockerfile 需要从 `VERSION` 和构建参数 `GIT_VERSION` 生成页面底部版本号，避免线上展示 `local-dev`。
- 推送后需要等待自动发布完成，再检查：
  - 页面底部版本号是否为 `VERSION (commit)` 或 `VERSION (container)`。
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 是否健康。
  - 关键页面是否能打开，静态资源和 API 路径是否仍走 `/erzhuang-project/` 前缀。

## 发布术语约定

默认规则：GitHub 的代码备份能力依然保留；除非用户明确说明“不要同步 GitHub”或“只推公司 GitLab”，已确认准备发布的代码仍应同步到 GitHub 作为主代码备份。二壮项目的实际发布只走公司 GitLab/K8s 固定分支。韩国 Lighthouse 发布链路已经终止，且该服务器上关于二壮项目的库表已经完全删除；不得再把韩国服务器作为发布、回滚、验收或备用链路。

用户说“发布到公司”时，必须先确认测试还是正式。历史默认含义曾是公司测试环境；2026-08-14 起项目正在切到公司正式环境，不能再默认把测试发布当作最终发布。

用户说“发布测试环境”时，固定含义是：

1. 将当前已确认代码 merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
2. 推送到 remote `gitlab`。
3. 等待 Wharf pipeline `752` / K8s 自动发布，不操作韩国 Lighthouse。
4. 保留公司环境配置，禁止 force push，禁止用个人服务器配置覆盖公司运行配置。

测试分支的正常链路是“命令推送 -> Wharf 构建 -> K8s 自动部署”。Wharf 页面出现“部署”按钮不代表常规测试发布需要人工点击；构建成功后先等待最多约 5 分钟，再检查测试实例的最近部署 commit 与测试页版本。除非用户明确要求人工补救，或已确认自动部署未触发，禁止在 Wharf 手工提交测试部署，避免重复部署和发布口径混乱。

用户说“发布正式环境/线上/生产/主干”时，正式链路是在 GitLab `main` 提交代码，触发 Wharf pipeline `771` 构建；构建成功后需要在 Wharf 点部署并走审批；审批通过且部署成功后，正式环境入口 `http://lite.soyoung.com/erzhuang-project` 才对接本次主干版本和线上数据库。不能把测试分支自动发布当作正式发布。

用户提到“发布到韩国服务器”时，应提醒该链路已废止：韩国服务器上关于二壮项目的库表已经完全删除，不具备二壮项目发布和回滚条件。历史文档中的韩国 Lighthouse/TAT 内容仅作为学习记录，不作为当前操作手册。

## 前端验收门禁

前端改动不能只以 `npm run build` 通过作为完成标准。涉及页面布局、弹窗、表格、按钮、颜色、交互状态时，发布前必须做实际页面验收：

前端改动开始前必须先读取：

- `docs/ui-standards.md`
- `docs/frontend-review-checklist.md`

1. 本地启动前端预览，必要时使用 mock 数据覆盖列表、详情、弹窗、空态和有数据态。
2. 用浏览器实际打开页面，检查截图或可视页面，不只看 DOM 和类型检查。
   - 浏览器调试优先顺序：先显式检查 Chrome 插件能力，必要时让用户用 `[@chrome](plugin://chrome@openai-bundled)` 唤起；可用时优先用 Chrome 插件或 `node_repl` 配合 Chrome Plugin 读取页面、DOM、日志和网络状态。
   - 只有 Chrome 插件能力未暴露、无法连接或任务确实只需要系统级视觉确认时，才退到 Computer Use 模拟点击/读可访问性树。
   - 如果使用了降级方式，最终说明里要标注原因，避免把“模拟点击”误认为首选验收路径。
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

测试发布标准链路：

1. 本地开发和验证。
2. 提交到 Git。
3. 推送公司 GitLab 固定分支 `codex/containerize-single-image`。
4. 等 Wharf pipeline `752` / K8s 自动构建发布。
5. 主会话或用户在已登录公司浏览器里验证健康检查、页面版本号和本次改动相关路径。
6. 将发布结果、版本、commit、验证结果和故障处理记录到 `docs/codex-learning-state.md`。

正式发布标准链路：

1. 本地开发和验证。
2. 提交到 Git，并按需同步 GitHub 备份。
3. 按公司允许的方式把已验证代码提交到 GitLab `main`。
4. 等 Wharf pipeline `771` 自动构建完成。
5. 构建成功后，在 Wharf 点部署并走审批。
6. 审批通过且部署成功后，在已登录正式环境浏览器验证 `http://lite.soyoung.com/erzhuang-project`、健康检查、页面版本号和本次改动相关路径。
7. 将构建、审批、部署结果、版本、commit、验证结果和故障处理记录到 `docs/codex-learning-state.md`。

韩国 Lighthouse/TAT 链路已经终止，不再用于二壮项目发布、回滚、诊断或线上验证；该服务器上的二壮项目库表已经完全删除。

发布失败处理：

- 不要先猜测或重复部署。
- 先确认公司 GitLab 分支是否已更新、K8s 自动发布是否完成、线上是否仍显示旧版本。
- 如果线上失败，优先恢复公司环境可用状态；回滚只能选择仍兼容 MySQL/OSS 的公司分支提交，不得依赖韩国服务器或旧 PostgreSQL/Supabase。
- 定位到根因后，只做最小修复，升级版本号，重新推送公司 GitLab 固定分支。

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

## DBA 专项协作规则

本项目在迁移到公司正规环境期间，由主会话暂任项目负责人，但数据库治理需要长期专项负责。凡涉及以下事项，主会话默认先交给置顶线程「DBA专项：MySQL迁移、权限模型与资产存储」产出方案，再由主会话验收和决策：

- MySQL schema、表结构、字段类型、索引、约束、`tb_` 表名前缀。
- PostgreSQL/Supabase 到 MySQL 的样本迁移、全量迁移、校验、回滚。
- 用户表、角色、机构范围、页面/Tab/操作权限、SSO 登录态落库。
- 通道截图、设计图、PDF 等资产对象映射表和公司文件服务迁移。
- 数据敏感性、安全审计、操作日志、数据保留与清理策略。

主会话可以做初步判断和风险提问，但不应在未通知 DBA 专项的情况下独立完成数据库方案或大规模 schema 改动。DBA 专项只产出方案、脚本和验证建议；未经用户或主会话明确要求，不直接发布、不改正式数据、不推送公司环境。

如果 DBA 专项遇到需要产品负责人、主会话、运维或安全/SSO 同学双向确认的事项，必须先整理成文档或待确认清单，再交给主会话复核。清单至少写清：背景、可选方案、推荐方案、影响范围、风险、需要谁确认、确认后下一步。不要在未确认的前提下直接实现、清表、改 schema、迁移历史数据或调整权限模型。

当前例外（2026-08-26）：NVR 缩略图初始化没有公司 DBA 支持。主会话必须采用不涉及 schema、DDL 或 MySQL 写入的替代方案：只读候选摄像头、使用既有私有 OSS 的确定性对象 Key、由页面按既有监控权限读取。不得把这一例外扩大到其他数据库治理事项。

---

## 协作红线（pm 组统一规则，2026-09 追加，优先级高于本文件其他约定）

本仓库属于 `gitlab.sy.soyoung.com/pm` 组。组内统一红线如下，与本文件其他条款冲突时以本节为准：

1. **只操作本仓库**。禁止向 pm 组下任何其他人的仓库 push、建分支、打 tag、改设置。
2. **禁止破坏性 git 操作**：`git push --force` / `-f`、删除远端分支（`git push origin :分支名`）、对已推送的共享分支做 rebase、`filter-branch`。
3. **发布分支**：本仓库发布分支为 **codex/containerize-single-image**，推送后自动发布到 `https://lite.sy.soyoung.com（见仓库 README）`。主干为 main。
4. **未经使用者明确确认，不自动 `git push`、不删除文件、不修改 Dockerfile / nginx.conf / .gitlab-ci.yml**。
5. **冲突处理**：push 被拒先 `git pull --rebase`；冲突无法判断时停止并向负责人说明，不强行覆盖。
6. **不提交**密钥、密码、真实用户数据、超过 5MB 的大文件。
