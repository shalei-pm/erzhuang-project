# Current Plan

更新时间：2026-07-08

## 当前轮目标

继续作为二壮项目主会话维护项目记忆，并推进“普通查看用户查看监控门店范围权限”的生产实现。本轮目标是完成后端 scope 权限、H5 Monitor 强校验、用户管理勾选交互、门店监控入口隐藏和验证收口。

## 已阅读文件

- `AGENTS.md`
- `README.md`
- `docs/deploy-runbook.md`
- `docs/architecture.md`
- `docs/technical-architecture-index.md`
- `docs/codex-learning-state.md`
- `docs/current-version-requirements.md`
- `docs/store-space-resource-prd.md`
- `docs/store-space-resource-tech-plan.md`
- `docs/mysql-dba-plan.md`
- `docs/mysql-migration-handoff.md`
- `docs/mysql-migration-acceptance-cases.md`
- `docs/oss-asset-stage-a-runbook.md`
- `docs/ui-standards.md`
- `docs/frontend-review-checklist.md`
- `docs/model-provider-switching.md`
- `docs/frontend-learning-state.md`
- `docs/h5-monitor-dev-task.md`
- `cmd/server/main.go`
- `internal/app/handler.go`
- `internal/app/auth.go`
- `internal/app/authz.go`
- `internal/app/auth_users.go`
- `internal/app/mysql_store.go`
- `Dockerfile`
- `scripts/deploy.sh`
- `scripts/rollback.sh`
- `frontend/package.json`
- `db/mysql_schema_tb.sql`
- `db/mysql_governance_schema_tb.sql`
- `VERSION`
- `internal/storespace/handler.go`
- `internal/storespace/service.go`
- `internal/storespace/mysql_store.go`
- `internal/channelai/recognizer.go`
- `internal/h5monitor/handler.go`

## 当前状态摘要

- 公司线上已确认：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"mysql","asset_store":"oss"}
```

- 当前代码版本：`VERSION=2.31.0`。
- 最新提交：`02a6623 fix: clear local auth before sso logout`。
- 当前分支：`codex/containerize-single-image`。
- 发布目标：公司 GitLab/K8s 固定分支 `gitlab/codex/containerize-single-image`。
- GitHub 定位：保留为代码备份和历史留存；不再代表线上发布完成。
- 当前核心运行时：Go + MySQL + OSS + APISIX SSO + React/Vite + 萤石云 + MiniMax/OpenAI。
- PostgreSQL 运行时联系已删除，不再作为回滚路径。
- 韩国 Lighthouse 发布链路已终止；该服务器上关于二壮项目的库表已经完全删除，不再作为发布、回滚、验收或备用环境。

## 任务拆分与进度

- [x] 盘点项目结构、docs、work 目录。
- [x] 阅读项目规则、README、部署、架构、MySQL/OSS、前端规范、当前代码入口。
- [x] 在 `docs/codex-learning-state.md` 顶部增加当前项目记忆快照。
- [x] 新建 `docs/decisions.md`，记录关键产品/技术决策。
- [x] 新建 `work/current-plan.md`，记录当前轮工作目标、进度、验证方式和下一步。
- [x] 验证本轮改动范围仅为项目记忆文档，未改业务代码。
- [x] 检查新增记忆文件未记录真实密钥、token、数据库密码或公司敏感连接串。
- [x] 更新 `README.md`，追平当前 MySQL/OSS、公司发布和 54 家门店口径。
- [x] 更新 `docs/technical-architecture-index.md`，改为当前 store-space 主路径代码地图。
- [x] 新建 `docs/post-cutover-regression-checklist.md`，沉淀 MySQL/OSS 切换后回归清单。
- [x] 新建 `docs/legacy-postgres-supabase-shutdown-checklist.md`，沉淀旧 PostgreSQL/Supabase 下线确认清单。
- [x] 复查新增/更新文档标题、过期口径和敏感信息风险。
- [x] 记录线上只读回归结果：health、auth、54 家门店、H5 Monitor 样本门店均通过。
- [x] 执行临时门店写接口回归，并清理临时数据。
- [x] 抽验资产链路和单通道识别链路：截图刷新接口 200，识别接口 200 但通道 `900065` 业务结果为 `recognition_failed`。
- [x] 定位通道 `900065` 识别失败原因：萤石抓图成功，AI provider upstream 502。
- [x] 用另一个样本区分单次 AI provider 波动还是识别链路整体问题：通道 `900076` 识别成功。
- [x] 创建独立调研会话“OpenClaw 摄像头截图技能调研”，交接 OpenClaw 基于本系统门店/区域/通道数据截图的调研任务。
- [x] 向 OpenClaw 调研会话补充安全边界：接口只提供读权限，不授予写权限；实时刷新截图如需支持需单独受控设计。
- [x] 初步排查 Supabase 控制台 Last 60 minutes 仍显示请求：主服务运行时 DB 只接受 MySQL，资产运行时当前应由 `K8S_SECRET_ASSET_STORE=oss` 固定到 OSS；仍需通过 Supabase Logs / `pg_stat_activity` 区分控制台自访问、旧脚本/旧服务器、迁移工具或其他调用方。
- [x] 解读 `pg_stat_activity` 截图：连接均为 Supabase 内部组件或本地 loopback，未看到公司业务服务连接旧库的证据。
- [ ] 后续单独安排历史迁移文档状态标记，避免阶段性旧事实误导。
- [x] 用户确认已删除旧 Supabase 数据库。
- [x] 删除旧 Supabase/PostgreSQL 后，执行线上健康、只读和 H5 Monitor 核心回归：全部 200，`failed=[]`。
- [ ] 删除旧 Supabase/PostgreSQL 后，如有必要再抽验真实门店资产/识别链路。
- [x] 定位用户管理编辑角色不生效问题：MySQL `tb_roles` 漏种 `editor`，保存角色关系插入 0 行后列表回退为 `viewer`。
- [x] 修复用户角色保存：写角色关系前幂等 seed `admin/editor/viewer`，并更新治理 schema 的 `editor` seed。
- [x] 创建本地发布提交 `f732228 fix: persist editor user role`。
- [x] 推送公司 GitLab 发布分支：`338fd1f -> f732228`，触发公司 K8s 自动发布。
- [x] 沉淀 GitLab HTTPS token 获取规则：本机安全文件 `/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`，通过临时 `GIT_ASKPASS` 使用，禁止输出或写入仓库。
- [x] 沉淀韩国 Lighthouse 链路终止事实：二壮项目不再发布韩国服务器，且该服务器上的项目库表已完全删除。
- [x] 补充 GitHub 口径：GitHub 代码备份能力依然保留，但不作为二壮项目线上发布链路。
- [x] 修复退出登录不生效：前端先调用本系统 `/api/auth/logout` 清理本地 cookie，再跳 APISIX 统一退出；后端补充父域 cookie 清理。
- [x] 退出登录修复提交 `02a6623` 已推送 GitHub 备份分支和公司 GitLab 发布分支，触发公司 K8s 自动发布。
- [ ] 等待公司自动发布完成，并做线上退出登录回归。
- [x] 需求澄清“普通查看用户查看监控门店范围权限”：仅限制普通查看用户的监控入口和 H5 Monitor；不限制普通页面浏览；管理员和编辑运维保持全量。
- [x] 确认授权口径：门店级别授权，候选门店为所有有 `external_org_id` 的门店，不按是否已有录像机/H5 Monitor 过滤。
- [x] 确认交互口径：新增/编辑普通查看用户时自动展开“查看监控门店范围”，默认空范围，支持按城市全选。
- [x] 确认技术方向：第一版采用前后端共同限制；数据模型先按 scope 化设计，后续可扩展为更通用的资源范围授权。
- [x] 创建静态交互 demo：`work/prototypes/viewer-store-scope-demo.html`，用于本地评审后再进入生产实现。
- [x] 根据用户反馈新增 D 版推荐交互：城市筛选 + checkbox 门店列表 + 当前筛选批量全选/取消，默认打开 D。
- [x] 根据原型确认结果写入正式设计文档：`docs/superpowers/specs/2026-07-07-viewer-monitor-store-scope-design.md`。
- [x] 提交设计文档：`3f68cca docs: design viewer monitor store scopes`。
- [x] 用户确认可以继续下一步。
- [x] 写入实现计划：`docs/superpowers/plans/2026-07-08-viewer-monitor-store-scope-implementation.md`。
- [x] 按 Inline Execution 执行实现计划，未新开子会话，避免和 OpenClaw 专项混线。
- [x] 新增 `tb_user_resource_scopes` 通用 scope 表和 `AuthUserResourceScope` 模型。
- [x] MySQL / Memory auth user store 支持候选门店、viewer scope 持久化、scope count 和 `CanUserViewMonitorStore`。
- [x] 用户管理 API 返回 `monitor_store_scope_count`、`monitor_store_scopes`，并新增候选门店接口。
- [x] H5 Monitor 后端接入 authorizer：门店列表过滤，直接访问未授权门店返回 403。
- [x] 前端 API / mock 支持 monitor store scope。
- [x] 用户管理弹窗实现 D 版城市筛选 + compact checkbox 门店列表；列表显示 viewer 监控范围数量。
- [x] 门店列表/详情新增 `can_view_monitor` 提示字段，前端按该字段隐藏监控入口。
- [x] 版本号提升到 `2.31.0`。
- [x] 本地浏览器验收用户管理弹窗。
- [x] 根据验收反馈优化用户弹窗体验：弹窗加宽到适合真实门店列表的尺寸，登录状态行改为横向阅读结构，开关宽度放宽避免中文挤压。
- [x] 修复门店列表右上角统计口径：MySQL 列表接口不再用当前页 `items` 汇总，而是按当前搜索/城市筛选条件汇总全量 filtered dataset，保证翻页不改变统计；全部 Tab 统计全部门店，城市 Tab 统计该城市全部门店。
- [ ] 发布到公司 GitLab/K8s 并做线上回归。

## 验证方式

本轮包含文档整理、用户角色修复和普通查看门店范围权限实现。验证方式：

- `git status --short` 确认改动范围。
- 阅读新增/更新文件，确认没有记录密钥、token、数据库密码或公司敏感连接串。
- `rg` 检查新入口文档中不再出现“当前计划使用 Supabase”“只支持 PostgreSQL”等误导性旧口径。
- 用户角色修复验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test`
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server`
  - 本机直接运行 Go 测试仍会触发已知 macOS `missing LC_UUID load command`，以编译门禁为准。
- 权限实现验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test`
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor -o /private/tmp/h5monitor.test`
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test`
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server`
  - `cd frontend && npm run build`
- 原型/浏览器验证：
  - 直接用浏览器打开 `work/prototypes/viewer-store-scope-demo.html`。
  - 优先查看 D 版“城市筛选列表”，再按需对比 A/B/C。
  - 已启动本地 mock 前端 `http://127.0.0.1:5173/erzhuang-project/` 验收用户管理弹窗。
  - 已确认普通查看新增用户默认展示“查看监控门店范围”，城市筛选生效，上海筛选只显示上海门店。
  - 已确认在城市筛选下勾选门店后，切回“全部”时已选门店前置。
  - 已确认 `全选` / `清空` 文案和入口存在，弹窗宽度、登录状态横向展示、开关文字压缩问题已优化。
  - 已补 `TestMySQLListStoresSummaryUsesFilteredDataset` 源码守卫测试，防止 MySQL 门店统计重新退回“当前页汇总”。

## 当前风险

- README 和 `docs/technical-architecture-index.md` 已追平当前架构；仍需后续标记历史迁移文档的阶段性状态。
- `docs/mysql-dba-plan.md`、`docs/mysql-migration-handoff.md` 等迁移文档包含阶段性旧事实，需要新会话按日期和当前快照判断是否仍有效。
- PostgreSQL 运行时回滚已删除，后续发布回滚需要谨慎选择仍兼容 MySQL/OSS 的提交。
- 韩国服务器上的二壮项目库表已经完全删除，不再具备发布、回滚、验收或备用验证能力。
- 用户已删除旧 Supabase 数据库；删除后线上核心回归通过，旧库不再是可用数据源或回滚路径。
- Supabase Dashboard 本身可能产生 Postgres / Storage / Realtime / API Gateway 请求；仅凭 Dashboard “Total Requests” 不能证明公司线上服务仍在访问旧库。
- `internal/assets/store.go` 仍保留 Supabase asset provider 兼容代码；只要公司运行时明确配置 `K8S_SECRET_ASSET_STORE=oss`，不会选择 Supabase。删除旧密钥后可进一步降低误触发风险。
- 用户角色修复发布后需要线上验证保存“编辑运维”后列表和编辑弹窗均保持 `editor`，并确认相关用户重新登录后具备写权限。
- 公司 GitLab HTTPS token 已确认在本机安全文件 `/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`。后续公司发布默认用临时 `GIT_ASKPASS` 读取该文件推送，用户名 `oauth2`，用完删除临时脚本，不再让用户重复手输 token。
- “普通查看用户查看监控门店范围权限”已进入生产实现阶段；发布前必须做浏览器实际验收。
- 该权限已后端强校验 H5 Monitor/API 访问；前端隐藏入口只是体验提示，不能替代后端 403。

## 下一步建议

1. 本地启动前端，浏览器验收用户管理普通查看门店范围弹窗。
2. 复查 `git status --short`，只提交本需求相关文件，避免混入 OpenClaw 并行改动。
3. 发布到公司 GitLab/K8s 后，线上验证：
   - 管理员编辑 viewer 用户，保存空范围/部分门店范围。
   - viewer 登录后普通后台页面可浏览。
   - viewer 仅在授权门店看到监控入口。
   - viewer H5 Monitor 门店切换只出现授权门店。
   - viewer 直接访问未授权机构 H5 URL 返回无权限。
4. 等待独立会话“OpenClaw 摄像头截图技能调研”产出 `work/openclaw-camera-skill-research.md`，主会话再评审是否进入实现。
5. 给历史迁移文档加状态标记，避免阶段性旧事实误导新会话。
