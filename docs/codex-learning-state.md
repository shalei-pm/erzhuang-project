# Codex Learning State

最后更新：2026-06-12

## 当前主题

学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证、回滚流程。

## 2026-06-12 通道视觉模型切换 2.7.2 开发记录

- 版本号：`2.7.2`。
- 目标：
  - 给监控截图 AI 识别增加 provider 切换入口，便于对比当前 OpenAI-compatible 模型和 MiniMax/OpenClaw 图像理解脚本的速度。
  - 默认行为保持不变：未配置 `CHANNEL_AI_PROVIDER` 时继续走 `VISION_API_KEY` / `VISION_API_BASE_URL` / `VISION_MODEL`。
  - 新增 `CHANNEL_AI_PROVIDER=minimax-script` 和 `CHANNEL_AI_PROVIDER=external-command`，通过外部命令调用图像理解脚本；MiniMax 脚本默认路径为 `/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py`。
  - `recognition_result` 增加 `provider` 字段，和 `recognition_ms` 一起用于线上速度对比。
- 安全约定：
  - MiniMax key 不写入代码、文档或 Git，只通过服务器环境变量 `MINIMAX_API_KEY` 注入。
  - 当前代码只接 provider 切换和外部脚本适配；真正切到 MiniMax 前，需要先确认服务器上脚本存在并确认脚本参数格式。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - PR：`https://github.com/shalei-pm/erzhuang-project/pull/1`，已合并。
  - Commit：`a786887`。
  - 线上部署：SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 成功。
  - 服务器当前 commit：`a786887`。
  - 服务器当前版本：`2.7.2`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 `active`。
  - 线上前端 JS 已确认包含 `2.7.2 (a786887)`。
- MiniMax 试跑状态：
  - 韩国服务器未发现 `/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py`。
  - 韩国服务器未发现 `/root/.openclaw/config/minimax.json`。
  - 当前线上仍使用默认 `VISION_API_BASE_URL=https://vibe.soyoung.com`、`VISION_MODEL=gpt-5.5`，未切换到 MiniMax。

## 2026-06-12 MiniMax Token Plan 线上识别 2.7.3 发布记录

- 版本号：`2.7.3`。
- Commit：`4ac3fb0`。
- 目标：
  - 确认用户提供的是 MiniMax Token Plan 订阅 Key，不是普通按量计费 API Key。
  - 根据 MiniMax 官方文档，Token Plan 可走 OpenAI-compatible Responses API：`https://api.minimaxi.com/v1`，模型 `MiniMax-M3`。
  - 修复 `VISION_API_BASE_URL` 已包含 `/v1` 时 endpoint 被拼成 `/v1/v1/responses` 的问题。
  - 兼容 MiniMax 可能返回 Markdown fenced JSON 的情况，并收紧 prompt 要求只输出 JSON。
  - `recognition_result.provider` 能正确记录为 `minimax`，避免速度对比时混淆。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 线上配置：
  - `/etc/systemd/system/erzhuang-project.service.d/20-vision-ai.conf`
  - `VISION_API_BASE_URL=https://api.minimaxi.com/v1`
  - `VISION_MODEL=MiniMax-M3`
  - `VISION_API_KEY` 使用 MiniMax Token Plan 订阅 Key，仅保存在服务器 systemd drop-in，不进入 Git。
- 线上验证：
  - 服务器部署脚本执行成功，`erzhuang-project.service` 为 `active`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 直接调用 MiniMax Responses API 返回 HTTP 200，耗时约 `12429ms`。
  - 通道 `131` 真实识别成功，整体接口约 `10s`，后端记录 `capture_ms=1668`、`total_ms=6414`，识别为“弱电机房”。
  - 通道 `132` 真实识别成功，`provider=minimax`，整体接口约 `14s`，后端记录 `capture_ms=1029`、`recognition_ms=10297`、`total_ms=11327`。

## 2026-06-12 视觉模型对比结论

- 用户确认 MiniMax Token Plan 已跑通，但速度相比现有 GPT 链路没有优势。
- 线上视觉识别已切回 GPT：
  - `VISION_API_BASE_URL=https://vibe.soyoung.com`
  - `VISION_MODEL=gpt-5.5`
  - `VISION_API_KEY` 仅保存在服务器 systemd drop-in，不进入 Git。
- 切回后服务健康检查通过，`erzhuang-project.service` 为 `active`。
- 保留 MiniMax 兼容代码和 provider 记录能力，方便后续如果换更快 MiniMax 模型或其他视觉模型时复用。

## 2026-06-12 通道识别反馈弱化 2.7.4 开发记录

- 版本号：`2.7.4`。
- 目标：
  - 将通道缩略图下方的识别反馈数据弱化为灰色小字，避免抢占主要操作注意力。
  - 成功识别时只展示低置信标记和耗时，不重复展示区域类型。
  - 耗时信息压缩为一行，超出后省略。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 删除操作等待态 2.7.5 开发记录

- 版本号：`2.7.5`。
- 目标：
  - 门店删除、录像机删除、通道删除涉及数据库写操作，点击后按钮进入禁用态并显示 spinner + “删除中”。
  - 统一 row action 按钮内容为 inline-flex，保证“确认中 / 识别中 / 删除中”图标和文字对齐。
  - 修复通道区域类型从业务区域切回“其他区域”时，编号/备注输入框仍显示“必填”的问题。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 机构详情默认 Tab 2.7.6 开发记录

- 版本号：`2.7.6`。
- 目标：
  - 进入机构详情时，只要该门店已填写录像机，默认展示“通道映射”。
  - 如果没有录像机但有设计图，则默认展示“设计图标注”。
  - 新建门店后也按同一规则定位默认 tab。
  - 通道映射页增加业务区域类型单选筛选：全部、面诊室、治疗室、生美；默认全部。
  - 添加录像机入口统一把“选择账号”改为“选择区域”；该版本曾误写为华西，已在 `2.9.1` 修正为华中。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 删除等待态并发修复 2.7.7 开发记录

- 版本号：`2.7.7`。
- 目标：
  - 修复连续点击多个删除时，前一个删除按钮动效被后一个删除状态覆盖的问题。
  - 门店、录像机、通道删除等待态均改为多 ID 集合，直到对应接口返回或行消失前保持“删除中”。

## 2026-06-12 录像机识别动效优化 2.8.0 开发记录

- 版本号：`2.8.0`。
- 目标：
  - 录像机级“识别区域”按钮不再在按钮内展示转圈动效，按钮保持稳定。
  - 识别提示文案移动到操作区右侧，以灰色小字展示，并增加类似 Codex 思考态的高光扫过动画。
  - 进度展示改为录像机表格底部 3px 细进度线，从左到右推进，到 100% 后淡出。
- 本地预览：
  - 使用 mock 模式在本地预览，用户确认“还可以，可以发布”。

## 2026-06-12 机构列表城市筛选与汇总 2.9.0 开发记录

- 版本号：`2.9.0`。
- 目标：
  - 机构列表搜索框下方增加城市单选筛选，默认“全部”。
  - 城市选项只展示当前列表中实际存在的城市，避免出现无结果筛选项。
  - 右侧列表汇总增加面诊室、治疗室、生美数量，并随城市筛选联动。
  - 城市筛选后，门店数量和当前展示区间按筛选后的可见列表计算。

## 2026-06-12 萤石云区域选项修复 2.9.1 开发记录

- 版本号：`2.9.1`。
- 问题：
  - 录像机“选择区域”白名单误用了华西，漏掉华中。
  - 这是前端展示白名单问题，不是线上萤石云账号数据被删除。
- 修复：
  - 可选区域调整为华北、华东、华南、华中。
  - mock 账号补充华中，方便本地无后端时也能覆盖该选项。
  - 增加 `scripts/check-region-options.mjs`，用于检查大区白名单必须包含四个区域。

## 2026-06-12 设计图详情图片恢复 2.9.2 修复记录

- 版本号：`2.9.2`。
- 问题：
  - 用户保存设计图标注后，从机构列表重新进入机构详情，设计图图片加载不出来。
- 根因：
  - 保存后的设计图详情接口返回内部存储路径，例如 `uploads/{upload_id}/preview.png`。
  - 前端重新进入详情时没有把该内部路径转换为可访问的图片接口 `/api/design-plan/uploads/{upload_id}/preview`。
  - 服务器文件实际存在，详情接口也返回 200，问题位于前端图片 URL 映射层。
- 修复：
  - `toDisplayImageUrl` 增加对 `uploads/{upload_id}/preview.png` 和 `uploads/{upload_id}/thumbnail.png` 的转换。
  - 新增 `scripts/check-design-plan-image-url.mjs`，用于防止保存后内部图片路径无法恢复为前端可访问 URL。

## 2026-06-12 通道确认等待态 2.9.3 修复记录

- 版本号：`2.9.3`。
- 问题：
  - 通道点击“确认”后会显示“确认中”，但如果再点击其他通道按钮，前一个确认按钮的等待态会消失。
- 根因：
  - 前端用单个 `confirmingChannelId` 记录确认中状态，多个确认请求并发时后一个会覆盖前一个。
- 修复：
  - 通道确认等待态改为 `Set<number>`，按通道 ID 独立管理。
  - 每个通道从点击确认开始保持“确认中”，直到该通道确认请求结束或状态变化。
  - 新增 `scripts/check-channel-confirming-state.mjs`，防止确认等待态退回单 ID 管理。

## 2026-06-12 单通道识别缩略图状态 2.9.4 修复记录

- 版本号：`2.9.4`。
- 问题：
  - 单独点击某个通道识别后，缩略图已经出现；再识别其他通道后，之前通道的缩略图会在页面上消失。
  - 刷新页面后缩略图又恢复，说明数据库和文件没有丢，问题在前端页面状态合并。
- 根因：
  - 单通道识别完成后，前端用发起请求时的旧 `store` 快照合并 `updatedChannel`。
  - 多个识别请求前后完成时，后返回的请求会用旧快照覆盖前一个请求已经写入页面状态的缩略图。
- 修复：
  - `onStoreUpdated` 支持函数式更新，异步结果回写时基于最新门店状态合并。
  - 单通道识别和整台录像机队列识别均改为通过最新状态执行 `replaceChannelInStore`。
  - 录像机局部更新同样改为函数式更新，降低异步操作互相覆盖的风险。
  - 新增 `scripts/check-channel-recognition-merge-state.mjs`，防止单通道识别退回旧 `store` 快照合并。

## 2026-06-12 已确认通道识别锁定 2.9.5 修复记录

- 版本号：`2.9.5`。
- 产品规则：
  - 通道点击确认后，视为人工锁定。
  - 已确认通道的“重新识别”按钮置灰不可点击。
  - 点击录像机“识别区域”时，跳过已确认通道，不再自动重新识别。
  - 已确认通道点击“编辑”后，状态回到待确认，可重新修改区域类型/编号，也可重新识别。
- 修复：
  - 前端整台录像机识别队列过滤已确认通道。
  - 前端单通道“重新识别”按钮在已确认状态下禁用。
  - 后端 `RecognizeRecorderChannels` 跳过已确认通道，避免抓图和 AI 识别覆盖人工确认。
  - 后端 `RecognizeChannel` 对已确认通道返回校验错误，要求先编辑解锁。
  - 新增 `scripts/check-confirmed-channel-recognition-lock.mjs` 和后端测试，防止该锁定规则回退。

## 2026-06-12 删除按钮 hover 文字色 2.9.6 修复记录

- 版本号：`2.9.6`。
- 问题：
  - 各处删除按钮鼠标 hover 时背景变为红色系，但文字仍可能呈现普通操作按钮的蓝色。
- 修复：
  - `.danger-link:hover` 明确设置文字色为 `var(--danger)`。
  - 新增 `scripts/check-danger-link-hover-color.mjs`，防止危险按钮 hover 状态漏掉文字色。

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
- 版本号规范：
  - 采用 `大版本.中版本.小版本`。
  - 大版本：新增完整业务模块、一个及以上新页面、或核心业务流程变化。
  - 中版本：已有模块内交互、样式、信息架构或业务流程小迭代。
  - 小版本：bug 修复、测试补充、技术整理、部署脚本小调整、文档修正。
  - 重要线上验收问题需同时记录版本号和 Git commit。

## 2026-06-08 第一期开口收尾

用户准备 2-3 份真实测试 PDF，用于验证：

- 同名/相似门店重复判断与覆盖流程。
- 不同门店新建流程。
- 如有多页 PDF，用于验证多页上下拼接。

当前继续推进的 P0 收尾：

- 识别完成后触发同名/疑似同名提示。
- 保存前最终重复检查，支持确认覆盖或继续新建。
- 编辑态重新上传 PDF 前二次确认。
- 删除门店后清理对应上传文件目录。

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

## 2026-06-05 设计图管理线上发布

发布目标：

- 腾讯云 Lighthouse 韩国实例：`ap-seoul / lhins-rjfpwj1u`
- 公网入口：`https://43.155.237.46/erzhuang/`
- systemd 服务：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`

本次发布内容：

- 后端 Phase 1：设计图门店和区域 CRUD API。
- 前端 Phase 2：设计图标记后台页面骨架。
- 前端 API adapter：真实后端 CRUD 优先，上传/识别继续 mock。
- 契约修复：
  - `e571d6c Return empty arrays for design plan responses`
  - `6fa5562 Return empty arrays for duplicate checks`

最终线上运行 commit：

```text
6fa5562 Return empty arrays for duplicate checks
```

发布方式：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 600 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

服务器发布脚本完成：

- `git fetch origin`
- `git switch -C main origin/main`
- `go test ./...`
- `go build -o erzhuang-project ./cmd/server`
- `npm install`
- `npm run build`
- `sudo systemctl restart erzhuang-project.service`
- `curl -fsS http://127.0.0.1:18081/health`

服务器验证结果：

- Linux `go test ./...` 通过。
- Go build 通过。
- 前端 Vite build 通过。
- systemd 状态：active running。
- 健康检查返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

公网最终验收：

```sh
curl -k https://43.155.237.46/erzhuang/health
curl -k https://43.155.237.46/erzhuang/
curl -k 'https://43.155.237.46/erzhuang/api/design-plan/stores?page=1&page_size=2'
curl -k -X POST https://43.155.237.46/erzhuang/api/design-plan/stores/check-duplicate \
  -H 'Content-Type: application/json' \
  --data '{"name":"测试门店"}'
```

验收结果：

- `/erzhuang/health`：HTTP 200，数据库 `postgres`。
- `/erzhuang/`：HTTP 200，返回前端 HTML。
- `/erzhuang/api/design-plan/stores?page=1&page_size=2`：HTTP 200。

```json
{"items":[],"page":1,"page_size":2,"total":0}
```

- `/erzhuang/api/design-plan/stores/check-duplicate`：HTTP 200。

```json
{"exact_match":null,"similar_matches":[]}
```

过程问题和学习点：

- 第一次用 TAT 默认 `root` 用户执行发布脚本时失败：
  - Git 报错：`fatal: detected dubious ownership in repository at '/opt/apps/erzhuang-project'`
  - 原因：仓库属于 `lighthouse` 用户，root 操作该仓库会触发 Git safe.directory 检查。
  - 处理：改用 TAT 的 `--username lighthouse` 执行发布脚本。
- 发布后先发现 `/erzhuang/api/design-plan/stores` 从 404 变成 200，但空列表返回 `items:null`。
  - 这会导致前端真实 adapter 对 `items.map` 出错。
  - 已修复为空数组 `items: []`。
- 随后发现重复检查接口空结果返回 `similar_matches:null`。
  - 已修复为空数组 `similar_matches: []`。
- 本地 Mac 的项目内 Go 工具仍有 `dyld missing LC_UUID` 问题，导致本机无法运行 `go test` 和编译出的 Go 二进制。
  - 本机可用 `go build` 做编译验证。
  - 完整 `go test ./...` 以 Lighthouse Linux 发布脚本结果为准。

## 2026-06-08 前端 UI 小迭代发布

发布目标：

- 腾讯云 Lighthouse 韩国实例：`ap-seoul / lhins-rjfpwj1u`
- 公网入口：`https://43.155.237.46/erzhuang/`
- systemd 服务：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`

本次发布内容：

- 前端页面文案产品化：
  - `Design Plan Marker` 调整为 `空间资源管理`。
  - 上传占位说明移除 mock/Phase 文案，改为面向业务用户的说明。
  - `模拟识别失败` 调整为 `手动维护`。
- 前端弹窗状态文案调整：
  - `待上传`
  - `解析图纸中`
  - `识别区域中`
  - `可编辑`
- 区域卡片视觉和信息层级收紧：
  - 卡片标题优先展示区域名称。
  - 增加 `类型 · 编号` 摘要。
  - 缩小间距、表格行高和阴影，整体更接近后台系统风格。

发布 commit：

```text
9673866 Polish design plan frontend UI
```

发布方式：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 600 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

服务器发布脚本完成：

- `git fetch origin`
- `git switch -C main origin/main`
- `go test ./...`
- `go build -o erzhuang-project ./cmd/server`
- `npm install`
- `npm run build`
- `sudo systemctl restart erzhuang-project.service`
- `curl -fsS http://127.0.0.1:18081/health`

服务器验证结果：

- Linux `go test ./...` 通过。
- Go build 通过。
- 前端 Vite build 通过。
- systemd 状态：active running。
- 健康检查返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

公网最终验收：

- `/erzhuang/health`：HTTP 200。
- `/erzhuang/`：HTTP 200，返回前端 HTML。
- `/erzhuang/api/design-plan/stores?page=1&page_size=2`：HTTP 200。

```json
{"items":[],"page":1,"page_size":2,"total":0}
```

给用户的可访问地址：

- 本地预览：`http://127.0.0.1:5173/erzhuang/`
- 公网预览：`https://43.155.237.46/erzhuang/`

注意：

- 公网入口当前使用 IP + 自签证书，浏览器可能提示证书风险；这是当前练习环境的预期现象。
- 本次只改前端文案和样式，没有改数据库、后端接口、nginx 或 systemd 配置。

## 2026-06-08 识别失败可观测性增强

用户反馈：

- 上传一个门店设计图 PDF 后，页面没有识别结果。
- 页面也没有明确成功或失败反馈。
- 需要补充日志能力，方便后续提 bug 时定位问题。

判断：

- 旧代码里后端识别失败会统一返回 `design plan request failed`，没有在 `systemd` 日志中记录具体错误。
- 前端失败提示主要依赖页面 toast，弹窗里没有持久的上传/识别状态提示，用户容易感觉“没有任何提示”。

本次改动：

- 后端 `internal/designplan/handler.go` 增加上传与识别阶段日志：
  - 上传开始、上传完成、上传失败。
  - 识别开始、识别完成、识别失败。
  - 日志包含 `upload_id`、文件名、文件大小、页数、识别区域数量、耗时和错误信息。
- 前端 `frontend/src/App.tsx` 增加弹窗内持久状态提示：
  - PDF 解析中。
  - AI 识别中。
  - AI 识别完成。
  - AI 识别失败，可手动维护，并展示上传编号。
- 前端 `frontend/src/styles.css` 增加状态提示样式。
- 版本号从 `1.1.0` 升级到 `1.1.1`，按规则属于小版本：bug 定位/技术可观测性增强。

后续排查命令：

```sh
sudo journalctl -u erzhuang-project.service --since "30 minutes ago" | grep "designplan:"
```

如果用户再次反馈上传或识别失败，优先按上传时间查 `designplan:` 日志，确认失败阶段是 PDF 转图、上传资产读取、AI 接口调用、AI 返回解析，还是前端状态展示。

## 2026-06-08 上传错误提示增强

用户反馈：

- 上传两页 PDF 时页面直接显示 `HTTP 413`，不清楚业务含义。

判断：

- 两页 PDF 本身不违反第一版“最多 5 页”的规则。
- `HTTP 413` 通常表示请求体过大，可能是 PDF 文件超过 5MB，或 nginx 的请求体限制先于 Go 后端拦截。

本次改动：

- 前端上传前校验文件类型，只允许 PDF。
- 前端上传前校验文件大小，超过 5MB 直接提示“文件过大，请上传 5MB 以内的 PDF。”
- 前端将 `HTTP 413` 映射成文件过大提示。
- 前端将 `HTTP 504` 映射成 AI 识别超时提示。
- 版本号从 `1.1.1` 升级到 `1.1.2`，按规则属于小版本：错误提示和用户体验修复。

## 2026-06-08 门店搜索关键词匹配修复

用户反馈：

- 搜索框输入 `新氧青春` 没有检索出包含该关键词的门店。
- 产品预期：门店名称包含关键词时应该被检索出来。

判断：

- 后端列表搜索只使用 `normalized_name`。
- `normalized_name` 为了重复判断会去掉 `新氧青春`、`门店`、`店` 等品牌和后缀词。
- 这个规则适合“重复门店判断”，但不适合普通列表搜索。

本次改动：

- 后端搜索改为同时匹配：
  - 原始门店名称包含搜索词。
  - 归一化门店名称包含归一化搜索词。
- 重复判断逻辑仍继续使用归一化名称，不受影响。
- 前端 mock 搜索逻辑同步调整，避免本地预览和线上行为不一致。
- 增加后端测试覆盖品牌关键词搜索。
- 版本号从 `1.1.2` 升级到 `1.1.3`，按规则属于小版本：搜索 bug 修复。

## 2026-06-09 前端搜索请求稳定性修复

用户反馈：

- 线上 API 已确认 `q=新氧` 和 `q=新氧青春` 都返回 2 条门店。
- 但页面搜索仍显示没有结果。

判断：

- 后端搜索和公网 nginx API 均正常。
- 前端列表加载逻辑需要显式绑定当前搜索词，并防止旧请求返回后覆盖新请求结果。

本次改动：

- 前端 `loadStores` 显式接收当前 `query` 和 `page`。
- 增加请求序号，旧请求返回时不会覆盖最新列表状态。
- 列表加载失败时给出 toast，而不是静默显示空列表。
- 版本号从 `1.1.3` 升级到 `1.1.4`，按规则属于小版本：前端搜索状态修复。

## 2026-06-09 GitHub CLI 与 PR 流程确认

用户反馈：

- 本机 GitHub CLI 已可用。

确认结果：

- `gh` 路径：`/Users/sylar/.local/bin/gh`
- 版本：`gh version 2.93.0`
- 登录账号：`shalei-pm`
- token scope：`gist`、`read:org`、`repo`、`workflow`
- 仓库访问正常：`shalei-pm/erzhuang-project`
- 默认分支：`main`
- 当前无打开 PR。
- 当前无 GitHub Actions run 记录，说明仓库暂未配置 CI 或没有运行历史。

Codex 使用注意：

- 普通沙箱下 `gh` 可能无法访问 GitHub API 或 macOS keyring。
- 在 Codex 中使用 `gh` 查询仓库、PR、Actions 时，通常需要提升权限。
- 提升权限后已验证 `gh auth status` 和 `gh repo view` 可用。

全局研发流程约定：

1. 常规研发默认从 `main` 创建 `codex/<task-name>` 分支。
2. 专项会话或主会话在分支内实现、测试、提交。
3. 分支推送到 GitHub 后，用 `gh pr create` 创建 PR。
4. 主会话负责 review、决定合并、必要时打回修改。
5. PR 合并后，主会话负责发布到 Lighthouse、验证 `/health`、验证页面版本号，并记录发布结果。
6. 紧急线上修复或用户明确要求快速闭环时，可以直接在 `main` 小步提交并发布，但最终说明必须标注原因。
7. 后续如配置 GitHub Actions，PR 合并前必须检查 CI 结果。

## 2026-06-10 Supabase RLS 安全告警处理

用户收到 Supabase 邮件告警：

- Critical issue: Table publicly accessible
- Issue code: `rls_disabled_in_public`
- Project ref: `alsobcuythtbkldxmbvq`

判断：

- 本项目在 Supabase 的 `public` schema 下创建了 `tasks`、`design_plan_stores`、`design_plan_store_areas`、`design_plan_operation_logs`。
- Supabase 会将 `public` schema 表暴露给 Supabase API 层。
- 即使当前业务只通过 Go 后端连接数据库，也应该对 `public` 表开启 Row-Level Security。

处理方案：

- 在运行时 schema 初始化中增加：
  - `alter table tasks enable row level security`
  - `alter table design_plan_stores enable row level security`
  - `alter table design_plan_store_areas enable row level security`
  - `alter table design_plan_operation_logs enable row level security`
- 同步更新 `db/schema.sql` 和 `db/design_plan_schema.sql`。
- 在 `docs/database-plan.md` 记录 RLS 规则、立即修复 SQL 和验证 SQL。
- 在 `AGENTS.md` 增加数据库安全开发规则：Supabase `public` schema 新表必须开启 RLS。
- 版本号从 `1.1.4` 升级到 `1.1.5`，按规则属于小版本：安全配置修复。

注意：

- 本次不新增 anon/authenticated policy。
- 第一版仍保持浏览器端不直连 Supabase，业务读写统一经过 Go 后端 API。
- Go 后端使用数据库连接串访问 PostgreSQL，不受 Supabase API 层 RLS policy 限制。

发布结果：

- 提交：`38abcf3 Enable Supabase RLS for public tables`
- 线上版本：`1.1.5`
- 服务器部署：成功
- 服务器测试：`go test ./...` 通过
- 前端构建：通过
- systemd 重启：成功
- `/health`：返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`
- RLS 状态验证：通过 TAT 在服务器读取受保护的 `/etc/erzhuang-project.env`，使用只读 SQL 查询 `pg_tables.rowsecurity`。
- 验证结果：
  - `design_plan_operation_logs rowsecurity=true`
  - `design_plan_store_areas rowsecurity=true`
  - `design_plan_stores rowsecurity=true`
  - `tasks rowsecurity=true`
  - `RLS_CHECK=PASS`

## 2026-06-10 Supabase RLS policy 提示处理

用户反馈 Supabase Advisor 继续提示：

- `Detects cases where row level security (RLS) has been enabled on a table but no RLS policies have been created.`

判断：

- 这不是最初“表公开可读写”的问题。
- 当前 RLS 已开启且没有 policy 时，Supabase API 侧默认拒绝访问。
- 但为了让权限意图更明确，并减少 Advisor 提示，项目应增加显式拒绝前端直连的 policy。

处理方案：

- 对 `tasks`、`design_plan_stores`、`design_plan_store_areas`、`design_plan_operation_logs` 增加 `*_no_client_access` policy。
- policy 作用对象：`anon, authenticated`。
- policy 规则：`for all using (false) with check (false)`。
- 结果：浏览器端通过 Supabase anon/authenticated 角色仍不能读写业务表；Go 后端服务端数据库连接不受影响。
- 版本号从 `1.1.5` 升级到 `1.1.6`，按规则属于小版本：数据库安全策略说明和 Advisor 提示修复。

发布结果：

- 提交：`3ee780e Add explicit Supabase deny policies`
- 线上版本：`1.1.6`
- 服务器部署：成功
- 服务器测试：`go test ./...` 通过
- 前端构建：通过
- systemd 重启：成功
- `/health`：返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`

RLS policy 验证：

- `design_plan_operation_logs rowsecurity=true policies=design_plan_operation_logs_no_client_access`
- `design_plan_store_areas rowsecurity=true policies=design_plan_store_areas_no_client_access`
- `design_plan_stores rowsecurity=true policies=design_plan_stores_no_client_access`
- `tasks rowsecurity=true policies=tasks_no_client_access`
- `RLS_POLICY_CHECK=PASS`

## 2026-06-11 门店空间资源后端基础分支

后端专项分支：

- 分支：`codex/store-space-backend-foundation`
- 基线：`f819793`
- 范围：新增 `internal/storespace`，接入 `/api/store-space/*` 基础 API，新增门店空间资源 PostgreSQL schema 和 RLS deny policy。
- 边界：未操作腾讯云、nginx、systemd、Supabase 控制台、部署脚本、云密钥、萤石云密钥或 AI key；未改现有 `internal/designplan` 业务实现。
- 状态文档：`docs/store-space-backend-foundation-state.md`

本地验证：

```sh
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/storespace
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/app
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
```

结果：均通过。`go test ./internal/storespace` 在本机仍命中已知 macOS `missing LC_UUID load command` 问题，最终完整测试需主会话在服务器 Linux 环境执行。

## 2026-06-12 门店空间资源前后端专项合并

主会话完成两条专项分支验收并合并到本地 `main`：

- 后端分支：`codex/store-space-backend-foundation`
- 前端分支：`codex/store-space-frontend-shell`
- 后端合并提交：`cdd1d28 Merge store space backend foundation`
- 前端合并提交：`61ba624 Merge store space frontend shell`

已合入能力：

- 新增 `internal/storespace` 后端基础模型、校验、repository、service、handler。
- 新增 `/api/store-space/*` 基础接口：
  - `GET /api/store-space/ezviz-accounts`
  - `GET /api/store-space/stores`
  - `GET /api/store-space/stores/{id}`
  - `POST /api/store-space/stores`
  - `DELETE /api/store-space/stores/{id}`
  - `POST /api/store-space/stores/check-duplicate`
  - 录像机扫描/识别接口先保留稳定 `501 not implemented` 合同。
- 新增门店空间资源数据库 schema，并对所有新增 public 表启用 RLS + 显式 deny policy。
- 前端从单文件 `App.tsx` 拆分为门店列表、添加门店浮层、门店详情、设计图标注 Tab、通道映射 Tab，以及对应 domain 工具。
- 前端新增 store-space 后端 DTO/mapper；`createStore` 已走新后端 mapper。
- 添加门店浮层不再默认选择第一个萤石云账号；填写录像机设备编码时必须由用户明确选择账号。

合并后本地主线验证：

```sh
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go test -c -o /private/tmp/erzhuang-storespace-merged.test ./internal/storespace
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go test -c -o /private/tmp/erzhuang-app-merged.test ./internal/app
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
cd frontend && PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run build
git diff --check
```

结果：均通过。

当前边界：

- 当前完成的是本地 `main` 合并，尚未推送 GitHub，尚未发布到 Lighthouse。
- 前端 `storeSpaceApi.listStores/getStore` 当前阶段仍暂走旧 `designPlanApi`，用于保护现有设计图列表体验；真正完整切换门店空间资源列表/详情时，需要改为 `storeSpaceHttpAdapter.listStores/getStore`。
- 通道扫描、抓图、识别、确认接口后端尚未接真实萤石云；当前只完成基础合同和前端 UI/mock 壳。
- `ezviz_accounts` 已有只读安全字段列表接口，并补充了仅保存账号名的轻量创建接口；真实 `appKey/appSecret/accessToken` 仍不通过前端表单维护，后续由后端受控配置/加密方案承接。

## 2026-06-12 准备发布门店空间资源 2.0.0

版本号按项目规则从 `1.1.6` 升级到 `2.0.0`：

- 原因：新增“门店空间资源管理/通道映射”完整业务模块，属于大版本升级。
- 线上页脚预期：`2.0.0 (<commit>)`。
- 发布方式：主会话通过腾讯云 TAT 指定韩国实例 `ap-seoul / lhins-rjfpwj1u`，以 `lighthouse` 用户执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
- 风险说明：发布会拉取 GitHub 最新 `main`、执行测试/构建、初始化新增数据库表和 RLS deny policy，并重启 `erzhuang-project.service`。

首次发布结果：

- GitHub 拉取成功。
- 服务器 `go test ./...` 成功。
- 服务器 Go build 成功。
- 服务器前端 build 成功。
- systemd restart 已执行。
- 健康检查失败：`127.0.0.1:18081` 连接失败。

定位结果：

- 服务日志连续出现：`database setup failed: timeout: context deadline exceeded`。
- 根因：本次大版本新增较多 PostgreSQL 表、索引和 RLS policy，启动时 schema 初始化超过原有 10 秒上下文超时。
- 修复：版本号升级到 `2.0.1`，将数据库连接 Ping 超时保留 10 秒，将 schema 初始化超时单独放宽到 90 秒。

第二次发布结果：

- 修复提交：`b79aad1 Extend database schema setup timeout`
- 线上版本：`2.0.1`
- TAT InvocationId：`inv-r4ranigidm`
- TAT 结果：`SUCCESS`
- 服务器 commit：`b79aad1`
- 服务器 `go test ./...`：通过
- 服务器 Go build：通过
- 服务器前端 build：通过
- systemd restart：成功
- 内网健康检查：成功，返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`
- 现象：因为 schema 初始化仍需数秒，健康检查前 11 次连接失败，第 12 次成功；deploy 脚本重试机制生效。

流程复盘：

- 发布链路没有变：本地开发 -> GitHub `main` -> TAT -> 服务器拉取 GitHub -> 测试/构建 -> systemd -> health。
- 本次问题在于主会话一开始没有优先读取既有 runbook 和历史发布记录，导致先撞了一次非交互 `getpass`。
- 已把 TAT 发布方式、必须使用交互式 PTY、失败诊断步骤写入 `AGENTS.md` 和 `docs/deploy-runbook.md`，作为之后本项目的固定发布能力。

## 2026-06-12 创建门店浮层 2.1.0 小迭代

本次版本号从 `2.0.1` 升级到 `2.1.0`：

- 原因：已有“门店空间资源管理”模块内的创建门店浮层交互和样式迭代，并补齐萤石云账号轻量创建入口，让录像机配置链路可继续测试。
- 创建门店浮层默认不再塞入一个删不掉的录像机行；录像机为选填资源，点击加号后再新增设备编码行。
- “增加录像机”改为 32px 图标按钮，符合轻操作定位。
- 右上角关闭按钮改为稳定的 `.modal-close-button`，不再直接使用文本 `x`，避免形状变形。
- 浮层内新增“萤石云账号名称”轻量创建入口；创建后刷新账号列表并自动选中未配置账号的录像机行。
- 后端新增 `POST /api/store-space/ezviz-accounts`，只接收 `account_name`，返回安全字段，不返回也不接收密钥字段。
- 后台风格规范补充到 `docs/technical-architecture-index.md`：当前采用轻量企业后台 / SaaS admin 风格，自建 tokenized CSS，参考 Ant Design、Arco Design、Semi Design 的克制控件层级。

## 2026-06-12 门店详情与通道映射 2.2.0 小迭代

本次版本号从 `2.1.0` 升级到 `2.2.0`：

- 原因：已有“门店空间资源管理”模块内的信息架构和交互迭代。
- 创建门店浮层移除“新增萤石云账号”入口；账号维护不属于创建门店主流程，后续由配置侧或后端受控接口维护。
- 创建门店浮层仍支持选择已有萤石云账号并填写录像机设备编码；如果没有账号，页面展示“暂无可选账号”。
- 门店详情顶部的新氧机构 ID、录像机数、有效通道数、业务区域数改为只读指标陈列，不再呈现类似输入框的样式。
- 门店详情不再展示萤石云账号配置区。
- 通道映射 Tab 的录像机列表改为横向表格：
  - 表头：录像机名称、状态、有效通道数、上次扫描时间、操作。
  - 未扫描录像机只显示“扫描通道”。
  - 已扫描录像机显示“再次扫描”和“识别区域”。
  - 删除入口保留，但后端级联删除接口尚未实现，当前仍提示入口待接。
- 前端验收复盘：
  - 用户指出 2.x 前端细节不如早期 1.x，应提升验收标准。
  - 已将“前端发布前必须实际页面截图/视觉验收”写入 `AGENTS.md`。
  - 本轮本地 mock 视觉验收发现并修复：原生 `Choose File` 露出、详情顶部指标过度卡片化、通道映射操作列挤压换行、Tab 默认焦点框观感差。
  - 2.2.0 已发布上线，线上 commit 为 `eb29e90`。

## 2026-06-12 创建门店 validation failed 2.2.1 修复

本次版本号从 `2.2.0` 升级到 `2.2.1`：

- 用户反馈：创建门店弹窗信息完善后点击“创建门店”不能继续，机构列表出现 `validation failed`。
- 根因：
  - 前端 `storeSpaceApi` 的列表、详情、重复校验、删除仍复用旧 `design-plan` 接口，但创建门店走新的 `store-space` 接口。
  - 因此创建前重复校验可能查旧表，真正创建时新表已存在同名门店，后端返回字段级校验错误。
  - 前端 `ApiError` 没有保留后端返回的 `fields`，导致页面只能显示笼统的 `validation failed`。
- 修复：
  - `storeSpaceApi.listStores/getStore/checkDuplicate/deleteStore` 统一走 `/api/store-space`。
  - `storeSpaceHttpAdapter` 增加新模块重复校验与删除接口。
  - `ApiError` 增加 `fields`，`errorMessage` 优先展示字段级错误文案，例如“已存在同名门店”。
- 验证：
  - 前端 `npm run build` 通过。
  - 后端 `CGO_ENABLED=0 ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-12 通道映射删除录像机 2.3.0 小迭代

本次版本号从 `2.2.1` 升级到 `2.3.0`：

- 原因：通道映射 Tab 中“删除录像机”此前只是占位提示，用户继续测试门店详情时无法清理误填录像机。
- 后端新增：
  - `DELETE /api/store-space/recorders/{recorder_id}`。
  - 删除录像机时依赖数据库外键级联删除其通道。
  - 删除后更新门店 `updated_at`，并写入操作日志。
  - 内存仓储同步支持删除，并释放设备编码，便于本地 mock 和测试复用。
- 前端新增：
  - `storeSpaceApi.deleteRecorder(storeId, recorderId)`。
  - 通道映射 Tab 删除按钮改为真实操作。
  - 删除前二次确认，删除后刷新门店详情和顶部统计。
- 验证：
  - 后端新增 handler/service 测试覆盖删除录像机和设备编码复用。
  - 本地 `CGO_ENABLED=0 ./.tools/go/bin/go test ./...` 通过。
  - 本地前端 `npm run build` 通过。
  - 服务器 `go test ./...` 通过。
  - 服务器 Go build 通过。
  - 服务器前端 build 通过。
  - systemd restart 成功。
  - 内网健康检查成功，返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 公网 `/erzhuang/health` 验证成功。
- 发布结果：
  - 线上 commit：`2563351`
  - 线上版本：`2.3.0`
  - TAT InvocationId：`inv-t4rgda09gn`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-CcEoTbGK.js`
  - 现象：健康检查前 11 次连接失败，第 12 次成功；符合当前数据库 schema 初始化较慢但可恢复的已知模式。

## 2026-06-12 旧设计图门店可见性 2.3.1 修复

本次版本号从 `2.3.0` 升级到 `2.3.1`：

- 用户反馈：门店列表里 1.x 版本创建的机构消失，担心历史数据被测试机构替换。
- 排查结论：
  - 历史数据没有被物理删除。
  - 旧接口 `/erzhuang/api/design-plan/stores` 仍能查到 3 个历史机构。
  - 新接口 `/erzhuang/api/store-space/stores` 只查到 2.x 新门店主数据。
  - 根因是 2.2.1 为修复创建链路把前端列表切到新 `store-space` 表，但旧 `design_plan_*` 数据尚未迁移到新主数据模型，导致页面视图隐藏旧门店。
- 修复：
  - 在 `storespace.EnsurePostgresSchema` 中增加幂等 legacy migration。
  - 将旧 `design_plan_stores` 复制到新 `stores`。
  - 将旧设计图文件信息复制到 `store_design_plans`，使用 `legacy-<old_id>` 标识。
  - 将旧标注区域复制到 `store_areas` 和 `design_plan_annotations`。
  - 使用 `on conflict do nothing`，不覆盖、不删除旧表或新表已有数据。
- UI 小修：
  - 门店详情页移除“门店详情”冗余文案。
  - 录像机列表标题下移除 `1 / 3 台`。
  - 录像机操作改成圆角按钮样式。
- 发布结果：
  - 本地 commit：`191e4ee`
  - 线上 commit：`191e4ee`
  - 线上页面版本：`2.3.1 (191e4ee)`
  - TAT InvocationId：`inv-s4rh2w0iqb`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-DYHlvXR0.js`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/api/store-space/stores?page=1&page_size=50` 返回 `total=8`。
    - 新列表已同时包含 5 个 2.x 测试门店和 3 个 1.x 历史设计图门店：
      - `新氧青春诊所 深圳龙岗坂田万科项目`
      - `新氧青春广州塔门店`
      - `新氧青春诊所 深圳壹方城项目`
    - 抽查 `/erzhuang/api/store-space/stores/6` 可读取 `深圳壹方城项目` 的设计图和 11 个区域。

## 2026-06-12 旧门店标注框坐标 2.3.2 修复

本次版本号从 `2.3.1` 升级到 `2.3.2`：

- 发布后补充验收发现：
  - 旧门店已经恢复到新列表和新详情接口。
  - 但 `store-space` 详情接口只返回区域主数据，没有返回 `design_plan_annotations` 中的矩形框坐标。
  - 前端已经支持读取 `area.box`，因此需要后端补齐该字段，避免历史门店进入设计图 Tab 后缺少左侧标注框。
- 修复：
  - `store-space` 的 `Area` 增加 `box` 返回字段。
  - `PostgresStore.listAreas` 左连接最新一条 `design_plan_annotations`，把 `box_x/y/width/height` 转为前端使用的 `box`。
  - 新增 `parseAreaBox` 单元测试，覆盖坐标解析和缺失坐标不返回 box 的情况。
- 发布结果：
  - 本地 commit：`fac00f7`
  - 线上 commit：`fac00f7`
  - 线上页面版本：`2.3.2 (fac00f7)`
  - TAT InvocationId：`inv-s4rhar0q7h`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-Bn7UK7y3.js`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/api/store-space/stores?page=1&page_size=50` 仍返回 `total=8`。
    - 抽查 `/erzhuang/api/store-space/stores/6`，11 个区域均已返回 `box` 坐标，可供前端恢复旧设计图矩形标注。

## 2026-06-12 门店详情 UI 控件与图纸加载体验 2.3.3 修复

本次版本号从 `2.3.2` 升级到 `2.3.3`：

- 用户反馈：
  - 通道 Tab 下“扫描通道 / 删除”按钮样式不统一。
  - 未上传设计图的门店不应显示默认打底设计图。
  - 上传新 PDF 后应显示加载状态，旧设计图不应继续展示；失败时再恢复旧图。
  - “返回列表”和门店名称距离太近。
  - “新增区域 / 保存标注”也应使用统一圆角按钮。
- 修复：
  - 空设计图路径不再映射到 mock 示例图，只在 `mock/*` 路径时显示示例图。
  - 设计图上传流程增加 `pendingPreviewUrl`：新图加载成功后才切换预览；转换或图片加载失败时恢复旧图和旧区域。
  - 上传/转换/新图加载增加状态文案和转圈提示。
  - 未上传状态的图纸区域改为纯净空状态，不再显示网格底纹，避免误解为已有图纸。
  - 统一按钮圆角、大小和 danger 样式；详情页标题区域增加间距。
- 本地验证：
  - `git diff --check` 通过。
  - `npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 本地浏览器验收：
    - 无设计图门店 `imageCount=0`。
    - 空状态背景 `backgroundImage=none`。
    - `新增区域 / 保存标注` 圆角为 `999px`。
    - 通道 Tab 未扫描录像机仅显示 `扫描通道 / 删除`。
- 发布结果：
  - 本地 commit：`7abcc23`
  - 线上 commit：`7abcc23`
  - 线上页面版本：`2.3.3 (7abcc23)`
  - TAT InvocationId：`inv-r4ri2e0dh9`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-CM-T6EwS.js`
    - `/erzhuang/assets/index-C6Bw6lpq.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.3.3 (7abcc23)`。
  - 备注：部署脚本健康检查前 11 次连接失败，第 12 次成功；服务最终健康，仍符合当前冷启动较慢的已知现象。

## 2026-06-12 门店列表与详情布局 2.3.4 修复

本次版本号从 `2.3.3` 升级到 `2.3.4`：

- 用户反馈：
  - 机构详情页“返回列表”按钮过大。
  - 机构详情页首屏高度偏高，希望默认适配 1080 分辨率。
  - 机构列表页操作按钮出现溢出列表的情况。
- 根因：
  - 2.3.3 统一全局按钮后，`.plain-button` 继承了普通按钮高度、padding，并在详情 header 的 grid 布局中被拉伸为整行宽度。
  - 列表操作按钮统一为 72px 后，最后一列宽度仍为 122px，两个按钮加间距后容易溢出。
  - 设计图画布使用 `calc(100vh - 140px)`，对 1080 高度的后台页面偏高。
- 修复：
  - `.plain-button` 调整为 26px 高轻量文字按钮，详情页返回按钮限制为内容宽度。
  - 列表操作列增宽到 160px，行内操作按钮收敛为 68px。
  - 详情页 header 间距收紧，设计图编辑区域和画布高度改为 `clamp`，适配 1080 首屏。
- 本地验证：
  - `git diff --check` 通过。
  - `npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 1440x1080 浏览器复验：
    - 返回按钮尺寸为 66x26。
    - 详情页无需纵向滚动，主内容底部约 829px。
    - 列表操作按钮组 136px，操作列 160px，表格无横向溢出。
- 发布结果：
  - 本地 commit：`5dd89c6`
  - 线上 commit：`5dd89c6`
  - 线上页面版本：`2.3.4 (5dd89c6)`
  - TAT InvocationId：`inv-t4rigm075g`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-B9o-QAd9.js`
    - `/erzhuang/assets/index-CHPZUwoD.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.3.4 (5dd89c6)`。

## 2026-06-12 城市字段与门店列表 2.4.0 迭代

本次版本号从 `2.3.4` 升级到 `2.4.0`：

- 用户反馈：
  - “添加门店”弹窗没有看到城市字段。
  - 机构列表需要在门店名称前展示城市列，旧数据无城市时展示“未设置”。
  - 机构列表操作按钮虽然已进入列表内，但距离右侧边缘过近。
- 产品规则：
  - 新建门店必须选择城市。
  - 城市先内置一线/新一线城市下拉。
  - 列表列顺序调整为：序号 / 城市 / 门店名称 / 新氧机构 ID / 设计图状态 / 录像机 / 通道 / 面诊室 / 治疗室 / 生美 / 更新时间 / 操作。
- 后端修复：
  - `stores` 表新增 `city text not null default ''`，schema 初始化和迁移都覆盖。
  - 创建门店接口新增 `city` 校验，缺失时返回“城市必填”。
  - MemoryStore 和 PostgresStore 的创建、列表、详情均返回 city。
  - 旧设计图迁移到 stores 时 city 为空，前端统一显示“未设置”。
- 前端修复：
  - 创建门店弹窗新增城市下拉。
  - 创建门店请求体传入 `city`。
  - StoreSummary/StoreDetail 和 store-space API 映射补齐 city。
  - 门店列表新增城市列，空值显示“未设置”。
  - 操作列宽度和右侧 padding 调整，避免按钮贴边。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地浏览器检查：
    - 列表表头顺序包含城市列，且位于门店名称前。
    - 旧数据城市显示“未设置”。
    - 添加门店弹窗展示城市下拉，包含北京、上海、广州、深圳等城市。
    - 操作列按钮右侧留白约 56px。
- 发布状态：
  - 本地功能 commit：`eb6261c`
  - 线上 commit：`eb6261c`
  - 线上页面版本：`2.4.0 (eb6261c)`
  - TAT InvocationId：`inv-t4rjdb0w91`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-PKhj2K0q.js`
    - `/erzhuang/assets/index-DvSqg6-J.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.0 (eb6261c)`、“城市”和“未设置”。

## 2026-06-12 详情页流程体验 2.4.1 修复

本次版本号从 `2.4.0` 升级到 `2.4.1`：

- 用户反馈：
  - 设计图区域较多时，点击左侧矩形框会让页面整体滚动去找右侧卡片，导致左侧图纸跑出视野。
  - 门店只有一台录像机时，删除后没有再次添加录像机的入口。
- 根因：
  - 区域卡片定位直接使用 `scrollIntoView`，浏览器会滚动页面级祖先容器。
  - 右侧 `area-pane` 未固定高度，区域多时会撑高整个详情页，无法形成内部滚动。
  - 通道映射页只支持删除录像机，缺少已有门店补录录像机的接口和表单。
- 修复：
  - 设计图编辑区固定桌面高度，右侧区域面板独立滚动。
  - 点击左侧矩形框或新增区域后，只滚动右侧区域面板，并把对应卡片定位到面板可视区中部。
  - 新增 `POST /api/store-space/stores/{id}/recorders`，支持已有门店补录录像机。
  - 通道映射 Tab 增加“添加录像机”表单，删除到 0 台后仍可补录。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地 mock 浏览器复验：
    - 16 个区域卡片时，右侧面板高度 680、内容高度 2465，形成内部滚动。
    - 点击靠后区域矩形后，`windowScrollY` 保持 0，右侧 `area-pane.scrollTop` 变化到 1785，选中卡片位于右侧面板可视范围内。
    - 删除唯一录像机后，通道映射页仍展示“添加录像机”表单。
    - 填写 `DNEW12345` 后可重新添加录像机，列表恢复为 1 台。
- 发布状态：
  - 线上 commit：`edb5f9c`
  - 线上页面版本：`2.4.1 (edb5f9c)`
  - TAT InvocationId：`inv-t4rk6ggmb6`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-7ssrpJ7q.js`
    - `/erzhuang/assets/index-CXbT8UIV.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.1`、`edb5f9c`、“添加录像机”、“返回列表”、“区域卡片”。

## 2026-06-12 详情页返回按钮 2.4.2 修复

本次版本号从 `2.4.1` 升级到 `2.4.2`：

- 用户反馈：
  - 机构详情页左上角“返回列表”按钮颜色不醒目，几次被忽略。
- 根因：
  - 按钮仍沿用普通弱操作按钮的视觉层级，虽然是浅蓝，但高度只有 26px，缺少方向图标和明确的导航权重。
- 修复：
  - `StoreDetail` 中为返回按钮增加独立 `detail-back-button` 类名和左箭头。
  - 详情页返回按钮改成蓝底白字的专用导航按钮，高度 34px，保留紧凑尺寸但提高第一眼识别度。
  - TAT 工具补充 `TENCENTCLOUD_SECRET_ID` / `TENCENTCLOUD_SECRET_KEY` 环境变量读取能力，避免非交互环境无法发布；密钥仍不写入仓库。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `PYTHONPYCACHEPREFIX=/Users/sylar/erzhuang-project/.cache/pycache python3 -m py_compile tools/tat_run.py tools/tencent_api.py tools/tencent_credentials.py` 通过。
  - 本地 mock 浏览器复验：
    - `.detail-back-button` 位于详情页左上角，尺寸约 101 x 34。
    - 背景色为 `rgb(37, 99, 235)`，文字为白色，标题区域未被异常撑高。
- 发布状态：
  - 按钮修复 commit：`e549029`
  - 部署工具 commit：`3d84d4f`
  - 线上 commit：`3d84d4f`
  - 线上页面版本：`2.4.2 (3d84d4f)`
  - TAT InvocationId：`inv-r4rkfw0f56`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-DBJ1sfb3.js`
    - `/erzhuang/assets/index-Cx20omGX.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.2 (3d84d4f)`、`detail-back-button` 和“返回列表”。
    - 线上 CSS 中已确认包含 `detail-back-button`、`#2563eb` 和 `box-shadow`。

## 明日待办

## 2026-06-12 非业务区域备注与缩略图 2.7.1 发布记录

- 版本号：`2.7.1`。
- Commit：`255d301`。
- 目标：
  - AI 识别到非业务区域时，允许把实体名称放到通道的“编号/备注”字段，例如“机房”“药房”“前台”。
  - 通道列表列名由“编号”改为“编号/备注”：业务区域仍为数字编号，其他区域为文本备注。
  - 缩略图按钮清除全局按钮 padding，固定缩略图尺寸并使用 `object-fit: cover` 铺满，避免挤压变形和异常留白。
- 本地验证：
  - 新增 `TestRecognizeChannelStoresNonBusinessSceneAsNote` 覆盖 `machine_room -> 机房` 备注链路。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - TAT InvocationId：`inv-r4rt590tgw`。
  - TAT 结果：`SUCCESS`。
  - 服务器当前 commit：`255d301`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-CQ5C75RW.js` 和 `/erzhuang/assets/index-CkGYQwCd.css`。
  - 线上 JS 已确认包含 `2.7.1 (255d301)`、`编号/备注`、`area_note`、“机房”“药房”“前台”。
  - 线上 CSS 已确认包含缩略图相关 `padding:0`、`overflow:hidden`、`object-fit:cover`。

## 2026-06-12 通道识别工作流 2.7.0 发布记录

- 版本号：`2.7.0`。
- Commit：`4a94700`。
- 目标：
  - 修复单通道“重新识别”误触发整台录像机识别的问题。
  - 录像机级“识别区域”改为前端按通道队列执行，显示进度百分比，每完成一条立即更新截图和识别结果。
  - “再次扫描”改为增量同步通道有效性，不清空已确认通道的业务区域映射。
  - 通道行增加删除能力；删除后再次扫描如通道仍有效，会作为新的未确认通道出现。
  - 门店列表缩略图改为等比铺满缩略图框，避免挤压变形和两侧留白。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - TAT InvocationId：`inv-s4rsm8giq7`。
  - TAT 结果：`SUCCESS`。
  - 服务器当前 commit：`4a94700`。
  - 服务器发布脚本测试、Go build、前端 build 均通过。
  - `erzhuang-project.service` 重启后为 active running。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-CZDG6jnt.js`。
  - 线上 JS 已确认包含 `2.7.0 (4a94700)`、“重新识别”、“识别进度”、“删除后将移除”。

## 2026-06-12 通道截图与 AI 预识别 2.6.0 开发记录

- 版本号：`2.6.0`。
- 目标：
  - 萤石云通道真实抓图。
  - 通道最近截图保存和前端预览。
  - 接入可选监控画面 AI 识别，按截图预填业务区域类型和编号。
  - 记录单通道抓图、识别、总耗时，便于后续判断是否需要换更快模型。
- 关键产品规则：
  - AI 识别只预填，用户点击确认后才进入锁定确认状态。
  - 编号卡片写明“治疗室 1 / 面诊室 2 / 生美 3”时，以卡片文字为准。
  - 已确认通道再次识别时不覆盖已确认的业务区域类型和编号。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 安全约定：
  - 视觉模型 key 不写入仓库，只通过服务器环境变量 `VISION_API_BASE_URL`、`VISION_API_KEY`、`VISION_MODEL` 配置。
  - 本次密钥扫描未发现真实 key 进入项目文件。

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
