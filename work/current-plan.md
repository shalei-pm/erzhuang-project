# Current Plan

更新时间：2026-08-26

## 当前轮目标

作为二壮项目主会话和技术负责人，推进「门店空间资源查看」3.0 的拆分、实现、验收和项目记忆维护，并准备从过去一直使用的公司测试环境切换到公司正式环境。当前重点不是继续把测试分支当作线上，而是确认正式环境主干发布链路、正式实例、正式数据库、正式 SSO/网关和回滚方案。

## 2026-08-26 增量：截图复用与 NVR 播放控制

- [x] 旧截图仅在旧系统恰有一台录像机时以 `external_org_id + channel_no` 映射；多录像机或不确定情形保留占位。
- [x] 资源详情仅对具备门店监控权限的用户返回受控图片 URL；图片请求再次校验该权限，不暴露 OSS 原始路径。
- [x] 前端展示映射截图，加载失败与未命中时使用灰色默认图。
- [x] NVR 实验页增加标题与播放器间距；声音在按钮手势中初始化 Web Audio；暂停改为本地冻结画面，恢复后继续渲染新帧。
- [x] 前端 `npm test -- --run` 通过：8 个文件、54 项测试；`npm run build` 通过。
- [ ] 当前机器没有 `go`/`gofmt`，后端 `go test ./...` 与 `go build ./cmd/server` 由 Wharf 构建日志补验。
- [x] 已推送 GitLab 测试分支与 GitHub 备份：`7355395 fix: reuse legacy snapshots and stabilize nvr controls`。
- [x] Wharf `752` 已自动部署。已登录测试页显示 `3.1.7 (container)`；10001 / 摄像头 111 直播成功渲染，标题与播放器间距已校准。
- [ ] 仍需在真实终端点验：单录像机截图命中/多录像机占位，以及“开启声音”和暂停/恢复的实际控制结果。当前浏览器会话仅开放页面导航与截图，无法安全模拟这些点击。

## 2026-08-26 已确认：NVR 回放按小时定位

- [x] 产品确认：NVR 实验页的录像模式参考 2.x 的日期和时间选择，不参考其他页面的时间段样式。
- [x] 产品确认：使用固定 24 个一小时时段作为快捷的起止时间填写方式；不依赖录像片段查询接口。
- [x] 产品确认：单次 NVR 回放时长从 30 分钟扩展到 1 小时。
- [x] 设计文档：`docs/superpowers/specs/2026-08-26-nvr-lab-hourly-playback-design.md`。
- [x] 验收约束：发布前必须通过 Chrome 插件实际验证回放选择和播放，不能只看构建、DOM 或模拟点击。
- [x] 抽取 2.x 回放日期时间选择器，旧 H5 Monitor 与 NVR 实验页共用；旧萤石回放会话逻辑未改。
- [x] NVR 录像模式改为“单一回放起点 + 派生结束时间”，增加 24 个固定小时段快捷选择；最长窗口前后端统一为 1 小时。
- [x] 前端本地验证：Vitest 10 个文件 / 58 项测试通过，生产构建通过；Chrome 插件实际打开本地预览，验证桌面布局、24 个小时段、未来时段提示及“昨天 11:00 - 12:00”范围。
- [ ] 本机无 `go`/`gofmt`；Go 单测和编译由 Wharf 构建日志补验。
- [x] `VERSION=3.1.8` 已提交：`c76b560 feat: add hourly nvr playback locator`；已快进合并到测试分支，并推送 GitLab 与 GitHub 备份。
- [x] Wharf `752` 自动构建/部署后，Chrome 插件在测试环境实际验收：新控件已生效，无原生 `datetime-local`；未来时段被阻止；“昨天 11:00”派生为 `11:00 - 12:00`；点击定位后 `camera_id=111` 回放首帧成功，媒体包、WASM、解码与 Canvas 帧均递增，页面和控制台未见 token、WSS URL 或 Authorization。
- [ ] 移动端/iPhone 与微信内的工控机直播仍是独立硬门槛，未因本轮桌面回放验收而关闭。

## 2026-08-26 增量：3.1.9 直接按小时回放与播放器稳定性

- [x] 产品确认：日期仅作为小时段所属日期；点击有效小时段即发起回放，不保留“定位回放”二次操作。
- [x] 产品确认：今天未来小时不可选，直接置灰禁用，不通过点击后错误提示处理。
- [x] 根因确认：回放诊断作为播放器横向 flex 子元素，且每 250ms 更新，导致 Canvas 宽度反复参与布局计算并引发画面抖动。
- [x] 实现：NVR 页日期控件只显示日期/日历；未来小时 disabled；点击小时直接创建会话；移除播放页诊断 UI 与高频诊断状态订阅。
- [x] Chrome 本地预览：无定位按钮、无诊断面板；今天未来小时 disabled；点击小时立即请求回放。
- [x] 前端回归：Vitest 10 个文件 / 58 项测试与生产构建通过；`b9f027c fix: simplify nvr playback controls` 已推送 GitLab 测试分支和 GitHub 备份。
- [x] Wharf `752` 自动部署后，Chrome 插件测试：无定位按钮、无诊断面板、今天后续 11 个小时 disabled；点击“昨天 11:00 - 12:00”直接回放并首帧成功。5 秒间隔两次采样的 Canvas CSS 尺寸均为 `1358 x 763.875`，控制台无 error。

## 2026-08-26 增量：3.1.10 NVR 播放器内模式切换

- [x] 产品确认：参考 2.x 的播放模式语义，NVR 实验页的“实时视频 / 录像”不再作为播放器下方的独立页面 Tab，而在播放器底部控制栏中切换。
- [x] 实现：`NVRLabCamera` 持有的模式状态和切换行为不变；`NVRLabPlayer` 在共享 `H5PlayerControls` 的中央渲染双态控件。切至实时视频立即创建直播会话；切至录像仅显示日期和小时段，仍由点击小时发起回放。
- [x] 移除页面外 `.nvr-lab-mode-tabs`，避免出现两个模式入口；控制栏中的切换具有 `aria-pressed` 状态。
- [x] 前端回归：Vitest 10 个文件 / 59 项测试与生产构建通过。
- [x] Chrome 本地页面验收：桌面底栏中央显示模式双态控件，左右控制按钮不挤压；切至录像显示 24 个小时段且页面外无残留 Tab，切回实时视频后小时段隐藏。
- [ ] 待提交、同步 GitHub 备份和发布 GitLab 测试分支。

## 当前状态摘要

- 当前线上主版本仍为 2.31.x，核心运行时为公司 MySQL + OSS + APISIX SSO + H5 Monitor 萤石云播放。
- 过去主会话里“发布到公司”实际一直发布到测试环境。
- 测试入口：`https://lite.sy.soyoung.com/erzhuang-project`，对接测试分支 `codex/containerize-single-image`、测试实例机器和测试数据库，Wharf pipeline `752`。
- 正式入口：`http://lite.soyoung.com/erzhuang-project`，对接 GitLab `main`、主干实例机器和线上数据库，Wharf pipeline `771`。
- 当前目标已调整为公司正式环境切换；正式发布是在 `main` 分支提交代码后自动触发 pipeline `771` 构建，构建成功后还需要在 Wharf 点部署并走审批，不能继续套用 `codex/containerize-single-image` 测试分支自动发布假设。
- 二壮运行库测试环境：`polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 Secret 管理。
- 二壮运行库正式环境：`polar-ops.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 Secret 管理。
- 本机 TablePro 只连接测试库做开发和只读验收；线上库不通过客户端直连，只由线上代码或公司批准链路访问。涉及线上表结构变更时，Codex 只产出 SQL、影响说明、验证 SQL 和回滚建议，由运维执行。
- 韩国 Lighthouse 链路已彻底废止，不再用于二壮项目发布、回滚、验收或备用环境。
- 旧 Supabase/PostgreSQL 已删除或不再作为运行时/回滚路径。
- 用户确认 3.0 方向：二壮不再维护空间/通道映射主数据，改为读取公司业务库的只读资源查看。
- 3.0 模块名：门店空间资源查看。
- 3.0 不改 H5 Monitor，当前怎么看监控还怎么看。
- 3.0 不做设计图上传/标注、AI 通道识别、人工确认、门店/录像机/通道写入。
- 当前分支：`codex/containerize-single-image`。
- 2.x 完整备份已完成并推送 GitHub：tag `v2.31-stable-before-resource-view-3`，zip `docs/handoffs/archives/erzhuang-project-2.31.8-before-resource-view-3.zip`。
- 3.0 后端领域聚合、只读 repository、API handler 和 `cmd/server` 接线已完成初版。
- 3.0 前端 API 类型、只读列表、只读详情、空间视角、设备视角、异常项和主页面切换已完成初版。
- 主页面已隐藏旧新增、编辑、删除、扫描、识别、确认、设计图上传/标注入口；H5 Monitor 路由和播放页未改。
- 3.0.3 已移除独立业务库 DSN 接线，资源查看复用二壮主 MySQL；首次测试验收发现同步表字段与初版查询不一致，3.0.4 已改为读取 `ip_addr`、`last_heartbeat_time`、`order_num` 并通过测试验证。3.0.5 已按产品确认移除首页“异常”展示，测试验收通过后可清理不再使用的 `K8S_SECRET_BUSINESS_MYSQL_DSN`，文档和仓库不记录真实 DSN、账号或密码。
- 2026-08-18 已通过 TablePro 只读确认：原 `db_groupbuy` 的四张资源查看表均已同步至测试 `db_pm_erzhuang`；用户确认线上二壮库也已同步。3.0 可从独立业务库 DSN 收敛为读取项目主库连接，后续以代码和正式环境 API 验证线上同步结果。

## 技术负责人拆分

### Backend: 业务库只读资源聚合

- 范围：`internal/resourceview`、`cmd/server/main.go`、`internal/app/handler.go`。
- 目标：读取业务库四张表，聚合门店、空间、设备、绑定和异常项。
- 状态：初版完成。
- 已完成：
  - 领域模型：门店摘要、门店详情、空间树、设备树、异常项。
  - 只读 repository：`tb_crm_admin_tenant`、`tb_crm_iot_device`、`tb_crm_consulting_room`、`tb_crm_iot_area_device_relation`。
  - API：`GET /api/store-space-resource-view/stores`、`GET /api/store-space-resource-view/stores/{tenantId}`。
  - 权限：复用 `store:read` 守卫；监控入口只返回 `can_view_monitor` / `monitor_url`，不扩大播放权限。
  - 配置：复用二壮主 MySQL 连接；未配置主库时资源查看 API 沿用清晰未配置错误。

### Frontend: 只读资源查看主流程

- 范围：`frontend/src/api.ts`、`frontend/src/domain/resource-view.ts`、`frontend/src/components/ResourceStoreList.tsx`、`frontend/src/components/ResourceStoreDetail.tsx`、`frontend/src/App.tsx`、`frontend/src/styles.css`。
- 目标：把后台主页面切到只读「门店空间资源查看」，保留系统设置和 H5 Monitor。
- 状态：初版完成，本地 mock 浏览器已看过首屏和详情页。
- 已完成：
  - 3.0 API 类型和 snake/camel 映射。
  - 本地 mock adapter，便于无真实业务库时预览。
  - 只读列表：城市筛选、搜索、分页、工控机/NVR/摄像头/空间/绑定统计；异常仅在详情页展示。
  - 只读详情：空间视角、设备视角、异常项。
  - 查看监控入口按 `canViewMonitor && monitorUrl` 展示。
  - 主页面去掉旧维护入口，系统设置和 H5 路由保留。

### 主会话 Review 与发布控制

- 范围：跨后端、前端、文档、验证、发布。
- 当前状态：正在收口。
- 责任：
  - 确认 3.0 边界不漂移。
  - 防止真实业务库密钥、账号、密码、DSN 进入仓库。
  - 防止旧 PostgreSQL/Supabase 或 OpenClaw 暂缓工作混入。
  - 防止 3.0 新模块出现写 SQL 或副作用接口。
  - 发布前确认业务库只读 Secret、网络白名单和线上验收脚本。

## 业务库 3.0 关键关系

- `tb_crm_admin_tenant.id = tenant_id = 二壮 external_org_id`
- `tb_crm_iot_device.category='edge'` 表示工控机/边缘设备。
- `tb_crm_iot_device.category='nvr'` 表示录像机。
- `tb_crm_iot_device.category='camera'` 表示摄像头通道。
- `tb_crm_consulting_room` 表示业务空间，展示三层：
  - `level=1`：大类。
  - `level=2`：房间/区域。
  - `level=3`：床位/子区域。
- `tb_crm_iot_area_device_relation.area_id -> tb_crm_consulting_room.id`
- `tb_crm_iot_area_device_relation.device_id -> tb_crm_iot_device.id`；展示摄像头绑定时再筛 `category='camera'`，不得假定关系表每一行都是摄像头。
- `tb_crm_iot_area_device_relation` 是房间/床位与设备的唯一权威绑定表，也是房间/床位与摄像头绑定的权威来源；`area_id` 即诊室/床位 ID，不需要另找中间表。
- `function_type` 是同一设备与同一空间绑定的用途维度；在业务方确认具体枚举前，3.0 不按它过滤关系，查询和异常统计需保留其原始值。
- 表的唯一约束为 `(device_id, area_id, function_type)`，因此表结构允许同一设备与同一空间在不同用途下保留多条关系；在获得真实数据和业务枚举前，前端展示与统计不能只以 `device_id + area_id` 假定唯一。
- `camera.parent_id -> tb_crm_iot_device.id where category='nvr'`

## 任务拆分与进度

- [x] 与用户讨论 3.0 产品方向和边界。
- [x] 确认采用 A 方案：业务库只读展示，不在二壮侧维护映射主数据。
- [x] 确认空间类型使用业务库自己的三层分类，不沿用旧面诊室/治疗室/美容室固定统计口径。
- [x] 确认 H5 Monitor 不在 3.0 改造，后续工控机/NVR 取流单独作为 3.1 或专项研究。
- [x] 创建 3.0 设计文档：`docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`。
- [x] 创建 3.0 实施计划：`docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`。
- [x] 实施前给当前 2.x 稳定状态打 tag、打包 zip 并写入 handoff。
- [x] 后端 Task 1-4：业务库只读 API 初版完成。
- [x] 前端 Task 5-6：只读资源查看 UI 初版完成。
- [x] 主会话初步 review：主 UI 已切到 3.0，只读边界和 H5 Monitor 边界符合预期。
- [x] 主会话最终验证：后端、前端、构建、敏感信息扫描。
- [x] 真实业务库接入前配置确认：`K8S_SECRET_BUSINESS_MYSQL_DSN` 已由用户配置到镜像/公司环境。
- [ ] 发布后确认业务库 MySQL 协议连通、只读权限和字典/枚举口径。
- [ ] 用户本地/线上预览确认 3.0 真实业务库资源列表。
- [x] 使用 TablePro 连接测试库 `db_pm_erzhuang`，只读确认 `tb_crm_admin_tenant`、`tb_crm_iot_device`、`tb_crm_consulting_room`、`tb_crm_iot_area_device_relation` 是否已同步且有有效数据。
- [x] 使用 TablePro 确认 `tb_crm_iot_area_device_relation` 已同步至测试二壮库；用户确认线上二壮库也已同步。
- [x] 调整 3.0 数据源代码：复用项目主库连接，移除独立业务库 DSN；补回归测试，修正非摄像头关系误报，完成本地构建验证。
- [x] 自动化测试验收 3.0.5：门店列表、空间树、设备树、摄像头绑定、异常项和长数据 UI 均已从测试同步数据加载；首页已移除异常统计/列，等待用户确认业务呈现后再进入正式发布。
- [x] 用户明确要求后，发布到公司。
- [x] 3.0.1 热修复已推公司：修复 3.0 发布后 Pod BackOff/首页 404 风险。
- [x] 3.0.2 已部署公司：前端显示 `3.0.2 (container)`，当前实例镜像为 `ded1832e`。
- [x] 3.0.2 测试环境排查：`resource_view_not_configured` 根因收敛为测试环境无法访问必须在正式环境访问的业务库表，不是没发布或没注入 DSN，也不是资源查看 SQL bug。
- [x] 环境口径纠偏：确认此前使用的是测试环境，正式入口/主干实例/线上数据库是隔离链路。
- [x] 确认正式环境 GitLab 主干发布方式：`main` 分支提交代码后触发 Wharf pipeline `771` 构建；构建成功后手动点部署并走审批。
- [ ] 确认正式实例名、命名空间、Pod/容器名、构建/部署完成信号。
- [ ] 确认正式环境运行时变量：主库 MySQL、OSS、SSO、萤石、AI provider、3.0 业务库只读 DSN。
- [ ] 确认正式环境网关/SSO：`lite.soyoung.com`、`/erzhuang-project`、`/_/auth/callback`、`/logout`、Cookie 域和 `SSO_EXPECTED_SUB`。
- [ ] 代码侧补正式域名 `lite.soyoung.com` 的前端 SSO 域名判断。
- [ ] 正式发布前形成生产 cutover checklist、回滚点、构建/部署审批检查项和验收脚本。
- [x] 为验证正式环境配置，将 2.x 最后稳定版 `2.31.8` 恢复提交到 GitLab `main`，commit `c95545a`，触发 Wharf pipeline `771` 构建。
- [x] Wharf pipeline `771` 构建成功。
- [x] 构建成功后已在 Wharf 点部署并走审批。
- [x] 审批通过且部署成功后，正式入口 `http://lite.soyoung.com/erzhuang-project` 显示 `2.31.8 (container)`。
- [x] 正式环境核心 API 冒烟：health、SSO 登录态、门店列表、H5 Monitor 样本门店均 200；正式门店总数 `71`。
- [x] 正式环境退出登录根因定位并修复，GitLab `main` commit `960ade2 fix: logout from production sso domain`。
- [x] `960ade2` 正式构建失败根因定位：新增后端测试暴露 `lite.soyoung.com` 父域 cookie 清理逻辑错误。
- [x] 已推构建修复到 GitLab `main`：`b097c3d fix: clear production sso parent cookie`。
- [ ] `b097c3d` 等待 Wharf pipeline `771` 构建、部署审批和线上复验。
- [ ] 正式环境页面级补充冒烟：门店详情、H5 Monitor 实际播放页首帧、系统设置/用户管理。

## 验证方式

当前轮已执行或待执行：

- 后端：
  - `CGO_ENABLED=0 ... go test ./internal/resourceview`
  - `go test -c ./internal/resourceview`
  - `go test -c ./internal/app`
  - `go test -c ./cmd/server`
  - `go build -o /private/tmp/server-check ./cmd/server`
- 前端：
  - `cd frontend && npm test`
  - `cd frontend && npm run build`
  - mock 浏览器查看列表页和详情页。
- 安全扫描：
  - 新增 3.0 代码不得含真实业务库账号、主机、密码或 DSN。
  - `internal/resourceview` 不得含 `insert/update/delete/replace/drop/truncate` 等写 SQL。
  - 不得混入 `securityVideoUrl`、`content_id`、OpenClaw 暂缓工作、旧 Supabase/PostgreSQL 运行时依赖。

## 当前风险

- 3.0 已收敛到二壮主 MySQL，不再引入第二个 MySQL 数据源；同步表仍只读，禁止任何写 SQL。
- 二壮运行库也已分成测试/正式两套 PolarDB 地址：测试 `polar-dev`，正式 `polar-ops`。任何验收结果必须标注来自哪个域名、分支、pipeline 和数据库。
- 测试与线上都需持续保证同步表完整性；当前测试核查有 4 条摄像头用途关系引用缺失设备，3.0 应展示为异常项。
- 正式环境与测试环境完全隔离，主干实例和线上数据库配置不能从测试环境推断；任何线上发布必须先确认线上变量、实例、数据库和回滚方式。
- 前端公司域名判断当前只识别 `lite.sy.soyoung.com`，正式域名 `lite.soyoung.com` 上 SSO 自动进入、退出登录和显示退出入口可能需要代码适配。
- 若正式环境配置了 `SSO_EXPECTED_SUB=lite.sy.soyoung.com`，切换到 `lite.soyoung.com` 后后端 SSO token 校验可能失败，需要与网关/SSO 配置一起确认。
- 当前容器日志还出现二壮运行库登录资料同步 collation mismatch：`auth: profile sync failed ... Illegal mix of collations ... for operation '<>'`，需要作为独立小 bug 修复。
- `tb_crm_consulting_room.dict_id`、城市字典、`function_type` 取值和状态枚举仍需业务库维护研发确认。
- 测试库只读核查已发现 4 条关系引用不存在的设备，空间侧无孤儿关系；3.0 应将其作为数据异常项展示，不能在聚合时静默丢弃。
- 当前城市名第一版按 `city_id` 展示为“城市 N”；如需要真实城市名，需要接入城市字典或业务库同学提供映射。
- 旧 storespace/designplan 代码不会在 3.0 初期删除，避免影响回滚；主 UI 已隐藏旧写入口。
- 真实业务库数据量、长门店名、空间树深度、异常数量需要上线前用真实样本再看一次 UI 信息密度。

## 下一步建议

1. 用户确认完整测试验收后，冻结 commit `ae63860` 进入正式 `main` 发布流程；不夹带其他未验证改动。
3. 正式验收通过后，清理测试环境不再使用的独立业务库 Secret。
4. 正式发布后通过线上 API 验证资源查看，不用本机客户端直连线上库。
5. 正式发布前形成 cutover checklist：发布 commit、回滚 commit/tag、pipeline `771` 构建状态、Wharf 部署审批、健康检查、登录、用户管理、门店列表、H5 Monitor、3.0 资源查看、退出登录。
6. 正式发布操作：按公司允许的方式把已验证代码提交到 GitLab `main`，观察 Wharf pipeline `771` 构建；构建成功后在 Wharf 点部署并走审批。
7. 审批通过且部署成功后验证入口：`http://lite.soyoung.com/erzhuang-project`，并确认它读取的是线上数据库而不是测试数据库。

## 2026-08-18 当前热修复：3.0.6 列宽回归

- [x] 定位 3.0.5 首页表格错位根因：移除“异常”列后，CSS 未将操作列从第 12 列迁移到第 11 列。
- [x] 修复资源列表第 11 列操作区宽度为 168px，避免“详情 / 查看监控”溢出。
- [x] 前端单测通过：5 files / 41 tests。
- [x] 前端生产构建通过。
- [x] Chrome 插件以本地 mock 数据实际验收：含双操作按钮的首行不溢出，表头和数据列均为 11 列。
- [x] 发布到公司测试环境 GitLab `codex/containerize-single-image`：commit `238dd0a`，Wharf pipeline `752` 构建 `167902` 成功。
- [x] 测试环境自动部署并确认 `3.0.6 (container)`；真实同步数据首行“详情 / 查看监控”均在操作列内，未溢出。
- [x] 流程记忆纠正：测试发布以 GitLab 推送触发的 K8s 自动部署为准，Wharf 手动部署不是常规步骤；构建成功后先等待最多约 5 分钟并检查实例最近部署 commit 和页面版本。
- [ ] 用户确认本次测试验收后，再决定 3.0 正式发布范围；本次不发布 GitLab `main`。

## 2026-08-19 当前开发：3.0.7 列表映射完成度

- [x] 恢复资源列表“更新时间”，数据源确定为业务库摄像头-空间关联关系的最近维护时间。
- [x] 将“已确认”改为全门店摄像头绑定完成度：有摄像头且未绑定数为 0 才显示。
- [x] 恢复旧版序号格已确认标识，新增第 11 列“更新时间”，并将操作列调整为第 12 列。
- [x] 通过前端单测和生产构建；Chrome mock 验收 1440px 桌面表格无横向溢出。
- [x] 编译 `internal/resourceview` 测试包；本机 macOS 无法执行测试二进制是既有 LC_UUID 限制。
- [x] 提交本次源码、测试与 `VERSION=3.0.7`：`cbda79e feat: show resource mapping completion`，已推送 GitHub 备份与 GitLab 测试分支。
- [x] Wharf pipeline `752` 构建 `167904` 成功并自动部署；测试实例当前 commit `cbda79ec`、状态运行中。
- [x] 测试页面显示 `3.0.7 (container)`，真实资源列表已出现“更新时间”列且无横向溢出；首屏全部为未完全绑定门店，未显示“已确认”符合口径。
- [ ] 后续选择一家具备全量绑定摄像头的真实门店，确认其列表序号格显示“已确认”；同时与业务库关系表维护时间抽样比对。

## 2026-08-19 当前开发：3.0.8 摄像头空间绑定详情表

- [x] 以 `cameras + nvrs + spaces + relations` 构建前端摄像头绑定行模型，使用 `parent_id` 回溯空间链并稳定排序。
- [x] 增加 domain 单测：未绑定摄像头、重复关系、父链回溯、多条绑定与第四段床位展示均已覆盖。
- [x] 将详情从空间/设备/异常三 Tab 改为摄像头绑定表，保留查看监控入口；最近截图明确显示 `-`，本版不渲染操作列。
- [x] 本地验证：`frontend npm test -- --run`（5 files / 41 tests）及 `frontend npm run build` 通过。
- [x] Chrome 本地 mock 验收：1440px 内容宽度下表格保持 9 列对齐，未发生文字重叠，横向滚动容器有效。
- [ ] 提交 `VERSION=3.0.8`、源码、单测与本次 spec/plan；不得混入已有未提交文档。
- [ ] 同步 GitHub 备份与 GitLab 测试分支，等待 Wharf pipeline `752` 自动部署。
- [ ] 测试环境以真实门店 `10001` 验收：无旧 Tab、摄像头行数/绑定状态/空间路径正确、页面为 `3.0.8 (container)`；用户确认后再讨论区域筛选与截图/重新截图。

- [x] 提交 `VERSION=3.0.8`、源码、单测与本次 spec/plan：`7db9670 feat: show camera space bindings`；未混入既有未提交文档。
- [x] 已同步 GitHub 备份和 GitLab 测试分支；Wharf 测试环境自动部署完成。
- [x] 测试环境真实验收 `10001`：页面为 `3.0.8 (container)`，无旧 Tab，55 行摄像头与首页 39 已绑定/16 未绑定一致，绑定表的 9 列均存在且不重叠。
- [ ] 与用户确认真实数据的字段口径：缺少 NVR/通道的摄像头是否只展示 `-`，以及父子同名空间是否需要额外显示空间 ID/路径消歧；在此之前不做区域筛选、截图或重新截图。

## 2026-08-19 当前开发：3.0.9 NVRCHANNEL 录像机通道解析

- [x] 明确真实字段规则：`NVRCHANNEL:<nvr设备ID>-<通道号>`，示例 `NVRCHANNEL:22-10` 对应录像机 `22`、通道 `10`。
- [x] 前端优先按该规则派生录像机编号和通道号，解析失败才回退现有字段；不改变后端或业务表。
- [x] 补充 domain 回归测试。
- [ ] 完成前端测试、构建和 Chrome 本地/测试环境验收。
- [ ] 提交 `VERSION=3.0.9`、源码与本次设计说明，发布 GitHub 备份和 GitLab 测试分支。

- [x] 前端测试（5 files / 41 tests）和生产构建通过。
- [x] 已提交 `d135c55 fix: parse NVR camera channel identifiers`，同步 GitHub 备份与 GitLab 测试分支，Wharf 自动部署完成。
- [x] 测试环境显示 `3.0.9 (container)`；真实 `NVRCHANNEL:22-10` 记录已展示为录像机 `22`、通道 `10`。
- [ ] 后续可讨论多绑定路径的视觉分隔/空间 ID 消歧，不与本次字段解析修复混合发布。

## 2026-08-19 当前开发：3.0.10 有效摄像头范围收敛

- [x] 确认资源查看有效摄像头口径：`category='camera' AND provider='HikVisionNvrChannel' AND status=1 AND deleted_at IS NULL`。
- [x] 在 MySQL repository 与 service 聚合层统一该条件，覆盖门店摄像头数、已绑定/未绑定、已确认、详情摄像头行与关系筛选。
- [x] 保留缺失设备但 `function_type` 为 camera 的关系作为数据异常线索，不将其作为有效摄像头计数。
- [x] 新增有效海康、其他 provider、停用海康三类回归用例；`go test ./internal/resourceview`、后端构建、前端 41 项测试和生产构建均通过。
- [x] 提交 `36d2b03 feat: limit resource view to active hikvision channels`，已同步 GitHub 备份与 GitLab 测试分支；已核对 GitLab 分支指向该 commit。
- [ ] 等待 Wharf pipeline `752` 自动部署后，以真实门店核验摄像头、已绑定、未绑定、已确认和详情行均按新口径收敛；确认版本为 `3.0.10 (container)`。

## 2026-08-19 当前开发：3.0.11 摄像头空间类型与名称

- [x] 明确关系口径：摄像头关系的 `area_id` 对应空间若 `parent_id=2387`，该直接关联是诊室区域容器，必须忽略。
- [x] 后端在统一关系集合中执行过滤，因此详情、绑定状态、已绑定/未绑定和已确认全部使用相同结果。
- [x] 详情表由“空间层级 1/2/3、床位”调整为“空间类型、空间名称”；分别显示关联空间父级名称和关联空间自身名称。
- [x] 补充展示规则：关联空间 `level=3` 时空间类型固定显示“治疗室”，不写回业务库。
- [x] 新增 `70 -> 2665, 2667` 的回归用例，覆盖忽略 `2665.parent_id=2387`、保留 `2667.parent_id=2665` 的规则。
- [x] 将既有资源查看 domain 测试纳入默认 Vitest 清单；前端测试由 41 项增至 47 项。
- [x] 升版 `3.0.11` 并提交 `3c96f0b feat: show camera space type and name`；已同步 GitHub 与 GitLab 测试分支，远端 SHA 已核对。
- [x] 测试环境已自动部署，用户确认页面更新为 `3.0.11`；本次存在发布传播延迟。
- [ ] 待业务验收：检查表头为 7 列、诊室区域容器关系不展示、`2667` 类关系显示父空间/当前空间名称，且首页统计与详情绑定状态一致。

## 2026-08-19 当前开发：3.0.12 资源详情表紧凑展示

- [x] 修复自动列宽导致的中间留白：使用固定列轨道，空间名称承接剩余宽度。
- [x] 增加固定尺寸的“暂无截图”缩略占位。
- [x] 空间名称列改为完整换行显示，不再省略号截断。
- [x] 前端 47 项测试与生产构建通过。
- [x] 提交 `f51b48f fix: compact resource binding table` 并同步 GitHub 与 GitLab 测试分支。
- [ ] 等待自动部署后进行真实页面视觉验收：确认版本 `3.0.12`、固定列无中间留白、截图占位显示、空间名称完整换行。

## 2026-08-19 当前开发：3.0.13 摄像头列表信息结构

- [x] 增加“摄像头列表”标题，详情表固定为 8 列：摄像头 ID、录像机编号、通道号、最近截图、空间类型、空间名称、绑定状态、操作。
- [x] 无截图占位改为纯灰色图片图标；“刷新截图”按钮保留为禁用态，明确不接入旧萤石截图写链路。
- [x] `listSpaces` 补载直接父空间，确保 `parent_id -> name` 可稳定生成空间类型。
- [x] 升级 `VERSION` 至 `3.0.13`；Go 全量测试、Go 构建、前端 47 项测试与生产构建通过。
- [x] 仅提交本轮源码、测试与版本文件：`d550dc8 feat: refine camera binding list`；已同步 GitHub 备份与 GitLab 测试分支。
- [ ] 等 Wharf `752` 自动部署，验证测试页为 `3.0.13 (container)`：表头字段顺序、灰色图标、空间类型父名称和禁用“刷新截图”按钮均正确。

## 2026-08-19 当前开发：3.0.14 摄像头列表空间类型筛选

- [x] 默认“全部”列表按已绑定优先排序。
- [x] 按当前门店动态生成空间类型筛选 Tab，避免前端硬编码业务分类。
- [x] 筛选后仅展示命中空间类型的关系，空间名称正序排列；切换门店后过期筛选自动回退全部。
- [x] 新增 2 项 domain 回归测试，前端测试增至 49 项；Go 全量测试、Go 构建和前端生产构建通过。
- [x] 升级 `VERSION` 至 `3.0.14`，仅提交本轮源码、测试与版本文件：`6eda789 feat: filter camera bindings by space type`，已同步 GitHub 备份、GitLab 测试分支。
- [ ] 等 Wharf `752` 自动部署，验证 `3.0.14 (container)`：前三列紧凑、后五列均分；默认已绑定在上、筛选 Tab 类型完整、选择类型后空间名称正序且无跨类型关系。

## 2026-08-19 当前开发：3.0.15 门店类型摄像头统计与未绑定筛选

- [x] 门店列表以“面诊室 / 治疗室”摄像头数量替换“空间”数量。
- [x] 后端摘要新增两类摄像头数，按展示空间类型和摄像头 ID 去重；列表、分页摘要与详情共用口径。
- [x] 详情筛选 Tab 显示全部总摄像头数、各类型摄像头数和“未绑定”数，支持直接筛选未绑定行。
- [x] Go 全量测试、Go 构建、前端 49 项测试和生产构建通过。
- [x] 升级 `VERSION` 至 `3.0.15`，仅提交本轮源码、测试与版本文件：`55184fa feat: show camera counts by space type`，已同步 GitHub 备份与 GitLab 测试分支。
- [ ] 等 Wharf `752` 自动部署，验证 `3.0.15 (container)`：列表两类数字准确；详情筛选数量与表格行数对应，未绑定可直接筛选。

## 2026-08-19 当前开发：3.0.16 摄像头操作列对齐

- [x] 操作列表头和刷新截图按钮右对齐，并保留 20px 右侧间距。
- [x] 前端 49 项测试和生产构建通过。
- [x] 升级 `VERSION` 至 `3.0.16`，仅提交样式与版本文件：`60376fb fix: align camera actions`，已同步 GitHub 备份和 GitLab 测试分支。
- [ ] 等 Wharf `752` 自动部署，验证 `3.0.16 (container)`：按钮与操作表头右对齐，距右边框留白自然。

## 2026-08-20 预研：10001 新取流灰度改造

- [x] 盘点现有 H5 Monitor：页面、权限、直播、录像片段、回放、URL 释放、并发限制、播放器与诊断均为现有萤石云链路的一部分。
- [x] 确认产品边界：只灰度 `10001`，其他门店完全保持旧方案；页面布局和已有功能优先保持。
- [x] 确认首期兼容性硬门槛：iPhone 与微信内必须可直播。私有 WSS/H.265/WebCodecs 播放器仅可做桌面调试，不得作为正式唯一播放路径。
- [ ] 向新取流接口提供方取得直播/回放/片段/释放 API 文档、鉴权与样例响应，并确认 URL 协议和浏览器兼容性。
- [x] 确认新链路映射：空间 `area_id` 经关系表得到摄像头/设备 `device_id`，不使用录像机/NVR 标识；不传时间为直播，传起止时间为回放。
- [x] 完成最小鉴权/WSS 协议探针：鉴权接口可签发 token；按外推规则连接 WSS 返回 token 无效，说明需取得业务后端实际下发的完整 `wsUrl` 合同，不能继续猜测 token 拼接方式。凭据未写入仓库或项目文档。
- [x] 按接口方确认的 `camera_id=111/stream_type=2/start_time=123/end_time=456` 示例复核：鉴权 `200/code=0`，本机签发 token 连接 WSS 为 `400 token invalid`，排除先前使用 `77` 或日期参数造成的误判。
- [x] 用接口方提供的新签发 WSS 地址复核：标准 Upgrade 返回 `101 Switching Protocols`。确认 WSS 流服务与 URL 结构有效，不读取媒体内容。
- [x] 确认可用签发条件：回放 `start_time/end_time` 使用 Unix 秒级时间戳。运维提供的真实时间戳请求已验证鉴权 `200/code=0` 与 WSS `101 Switching Protocols`。
- [ ] 确认 `stream_type` 枚举、直播不传时间的请求形态、token 生命周期及 URL 过期后的刷新策略；SDK README 中的 `securityVideoUrl` 历史调用与当前 `auth/camera` 说明仍需在实现时隔离，避免混用。
- [x] 完成 `NVRPlayer-SDK` 代码阅读与官方替代调研：当前 WSS 直播为 RTP/H.265/WebCodecs、回放为海康私有封装/WASM；萤石官方播放器不直接兼容该输入协议。SDK 接入前必须处理 token 日志、重连刷新、受控静态资源和 iOS/Android 真机兼容性。
- [ ] 确认新接口是否有录像分段/存在性查询；没有则先决策是否接受 `10001` 回放流程简化，或要求接口方补齐能力。
- [ ] 确认 `10001` 的业务空间到新接口 `camera_id` 的唯一映射，包含缺失/多映射处理方式。
- [ ] 基于接口能力确定播放器适配层和灰度/回滚开关后，再写设计方案和实施计划；未经确认不修改监控业务代码。
- [x] 确认工控机/NVR 当前不能提供 HLS、FLV 或 WebRTC 等标准输出；不再以硬件改造作为实验页前置条件。
- [ ] 设计独立实验页的两条 iOS 验证路径：浏览器端 RTP/H.265 + WASM 软解，或独立软件网关从既有 WSS 转为兼容流；实验页验收需覆盖 iPhone Safari 和微信内播放。

### 2026-08-21 鉴权前置确认

- [x] 确认直播与回放均使用 `stream_type=2`；直播不传 `start_time/end_time`，回放传 Unix 秒级时间戳。
- [x] 确认鉴权接口 `Authorization` 是长期服务端凭据，测试/正式当前值相同且不会过期。
- [ ] 测试环境由运维在 K8s Secret 中配置长期凭据，二壮后端只在运行时读取，不进入前端或仓库。
- [ ] 实验页实现前补齐设计/实施文档，明确短时 token 的脱敏、刷新和失效处理；不改旧 H5 Monitor 主流程。

### 2026-08-21 10001 NVR 实验页设计确认

- [x] 确认视觉与交互必须复用测试环境 2.x H5 Monitor，不做独立调试台。
- [x] 查询测试同步库，确认 `10001` 有 55 个符合当前口径的海康通道；默认实验样本为 `camera_id=111`（治疗室4，NVR 22 / 通道 56）。
- [x] 形成并提交设计规范：`docs/superpowers/specs/2026-08-21-nvr-lab-10001-design.md`，commit `4ade1ea`。
- [ ] 用户审阅设计文档后，创建实施计划并开始第一阶段：后端短期会话、实验路由、2.x 风格页面、NVRPlayer 安全适配、桌面首帧验证。
- [ ] 测试 K8s Secret 注入长期服务端 Authorization 后，以 `camera_id=111` 验证直播与回放；确认 iPhone Safari/微信实机验收前不扩大灰度或替换旧 H5 Monitor。

## 2026-08-14 正式 main 2.31.8 冒烟状态

- 目标：先把 3.0 前最后稳定 2.x 版本放到正式 `main`，验证正式环境数据库、SSO、网关、部署审批链路是否配置正确；3.x 暂时保留在测试环境。
- 已推送：GitLab `main` commit `c95545a release: restore 2.31.8 on main`。
- 版本：`2.31.8`。
- 当前状态：已提交到 `main` 并通过 GitLab hook，应触发 Wharf pipeline `771` 构建。
- 更新：2026-08-17 用户确认部署审批通过后，正式入口显示 `2.31.8 (container)`。
- 待用户/主会话跟进：
  - 退出登录修复构建部署后是否真正退出。
  - 门店详情、H5 Monitor 实际播放页首帧、系统设置/用户管理是否正常。
  - 已确认：health 返回 MySQL/OSS，SSO 登录态 200，门店列表 200 且正式库总数 71，H5 Monitor 样本门店 10030 返回 200。

## 2026-08-17 正式退出登录修复状态

- 根因：正式域名 `lite.soyoung.com` 没加入前端公司 SSO 域名识别，退出只清本地 cookie，没有跳公司 `logouttogether` 清上游 SSO session。
- 已修复：GitLab `main` commit `960ade2 fix: logout from production sso domain`。
- 构建修复：`960ade2` 构建失败后，已推 `b097c3d fix: clear production sso parent cookie`，修正正式域名父域 cookie 清理为 `soyoung.com`。
- 已验证：
  - `frontend npm test` 通过。
  - `frontend npm run build` 通过。
  - `go test -c ./internal/app` 通过。
  - `go build ./cmd/server` 通过。
- 待完成：
  - Wharf pipeline `771` 使用 `b097c3d` 构建成功。
  - 点部署并走审批。
  - 线上点击退出登录复验，不应再自动恢复登录态。

## 2026-08-13 3.0.1 热修复记录

- 现象：3.0 发布后用户访问公司地址看到 `404 page not found`，随后收到公司告警：`Pod异常-erzhuang-project-erzhuang-project-7fb4d69c77-gbz92-BackOff`。
- 判断：故障不是普通前端路由问题，而是线上 Pod 启动失败/重启退避；3.0 新增业务库连接原本启动时 `fatal`，如果业务库 DSN、白名单或网络异常，会拖垮整站。
- 修复：
  - `Dockerfile` 固化 `APP_BASE_PATH=/erzhuang-project`、`FRONTEND_DIR=/app/frontend/dist`，避免 K8s 未注入前端目录时首页 404。
  - `cmd/server/main.go` 将业务库资源视图连接失败从 `fatal` 改为降级日志，主系统和旧 H5 Monitor 不被新只读数据源拖垮。
  - 版本升至 `3.0.1`。
- 发布：
  - Commit：`06e67bf fix: restore company static startup defaults`
  - 已推公司 GitLab 固定分支 `codex/containerize-single-image`。
  - 已同步 GitHub 备份分支 `codex/containerize-single-image`。
- 已验证：
  - `go test -c ./cmd/server` 通过。
  - `go build -o /private/tmp/server-check ./cmd/server` 通过。
  - `frontend npm run build` 通过。
  - 未登录访问 `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 SSO `302`，说明网关入口仍存在；最终业务可用性需用户用已登录浏览器确认。
- 待用户验证：
  - 已登录访问 `https://lite.sy.soyoung.com/erzhuang-project/` 不再出现 404。
  - 浏览器控制台请求 `/erzhuang-project/health`、`/erzhuang-project/api/auth/me`、`/erzhuang-project/api/store-space-resource-view/stores?page=1&page_size=20`。

## 2026-08-21 当前工作：10001 工控机 NVR 实验页

- [x] 形成并提交设计规范与实施计划，明确不修改旧萤石 H5 Monitor。
- [x] 新增后端只读摄像头列表、管理员守卫、实时鉴权会话和回放时间校验。
- [x] 新增隔离实验路由、2.x 风格摄像头墙/播放页、回放时间输入和受控重连。
- [x] 固定 NVRPlayer/WASM 静态资源，移除预发 CDN、敏感 URL 日志和旧地址自动重连。
- [x] 前端 `npm test`（8 files / 52 tests）与 `npm run build` 通过。
- [ ] 在可用 Go 工具链执行 `go test ./...` 与 `go build ./cmd/server`。
- [ ] 确认测试 K8s Secret `K8S_SECRET_NVR_STREAM_AUTHORIZATION` 已注入，发布测试环境。
- [x] 已推送 GitLab 测试分支与 GitHub 备份：`647b5a6`；等待 Wharf `752` 自动构建/部署传播。
- [ ] 管理员用 `camera_id=111` 验收桌面直播、回放、重新连接；iPhone Safari/微信直播通过前不替换旧方案。

### 2026-08-21 发布与验收增量

- [x] 定位并修复 NVR 实验页首次构建失败：`647b5a6` 的后端测试返回类型断言过期，最小修复为 `ab3f4d5`。
- [x] Wharf 测试构建 `168473` 成功，测试实例自动部署为 `ab3f4d5` 且运行中。
- [x] Chrome 插件确认隐藏实验路由可访问：`/erzhuang-project/h5/nvr-lab/10001`。
- [ ] 测试实例需绑定 K8s Secret 变量 `K8S_SECRET_NVR_STREAM_AUTHORIZATION`；未配置时实验服务拒绝创建播放会话，不进行旧萤石回退。
- [ ] Secret 生效后，管理员验收 `camera_id=111` 的摄像头列表、直播、回放、受控重新连接及控制台脱敏；随后进行 iPhone Safari 与微信内直播真机验收。

### 2026-08-21 Secret 生效后二次验收

- [x] 测试 Secret 已生效：10001 实验首页显示 44 路有效摄像头及空间类型筛选。
- [x] 默认 `camera_id=111` 可进入播放详情，摄像头业务映射正确；浏览器控制台未发现敏感会话信息泄露。
- [ ] 直播会话签发失败：接口返回 `nvr_stream_authorization_failed`，未建立 WSS/首帧。先核对 Secret 的长期 Authorization 值格式；若无误，增加安全的上游状态分类后继续定位。
- [ ] 回放、受控重新连接、iPhone Safari/微信真机验收均依赖直播会话签发恢复，暂不执行。

### 2026-08-21 鉴权根因定位完成

- [x] 发布 `3.1.1`、`3.1.2`、`3.1.3` 的安全诊断链路；完整 Wharf Go 测试和前端 `52` 项测试、生产构建均通过。
- [x] 测试实例已运行 `2933125`；默认直播请求稳定返回 `nvr_stream_authorization_upstream_http_422`。
- [x] 排除 Secret 未读取、前端路由、资源查询、WSS 播放器与首帧阶段；当前失败发生在公司鉴权服务对请求参数进行校验时。
- [ ] 向接口方核对 10001 的真实 `camera_id` 与直播参数合同，重点询问 HTTP 422 的字段级原因；确认前不绕过摄像头白名单、不猜测其他参数、不尝试旧萤石回退。

### 2026-08-21 鉴权参数对照

- [x] 排除测试 Secret 格式/读取问题：同一凭据下，`camera_id=111` 与 `584` 带 Unix 秒时间范围均成功签发 token。
- [x] 确认 HTTP 422 与摄像头 ID 无关：两个摄像头在只传 `stream_type=2`、不带时间时均返回 HTTP 422。
- [ ] 向接口方确认直播的真实参数合同；在明确前不以伪造回放时间冒充直播，不猜测 `stream_type`，不改旧链路。

### 2026-08-21 直播 `stream_type=1` 直接请求验证

- [x] 以 `camera_id=584`、`stream_type=1`、不带时间参数直接请求鉴权接口；仅记录 HTTP 状态，不保留响应体或短期 token。
- 结果：HTTP `422`。
- 结论：直播失败不能通过将 `stream_type` 从 `2` 改为 `1` 解决；代码改动已撤回，未提交、未发布。后续应要求接口方提供 HTTP 422 对应的字段级校验规则或可工作的直播请求样例。

### 2026-08-21 回放鉴权复验

- [x] 以 `camera_id=584`、`stream_type=2` 和 Unix 秒级起止时间直接请求鉴权接口；仅记录 HTTP 状态。
- 结果：HTTP `200`。
- 结论：长期鉴权、摄像头 ID、回放时间格式与测试环境到鉴权服务的网络链路均正常。后续播放器验收需使用已确认存在录像的时间窗，单独验证 WSS 建连和首帧渲染。

### 2026-08-21 测试环境回放播放器验收

- [x] Chrome 测试环境打开 10001 NVR 实验页，确认有 44 路可访问摄像头。
- [x] 使用白名单内 `camera_id=82`（走廊）和 `camera_id=111`（治疗室4），分别请求 2025-08-20 12:25 至 12:38 的回放。
- [x] 两次均完成后端鉴权与 WSS 建连，页面状态为“视频流已连接，等待画面”。
- [x] 两次等待超过 30 秒均未触发首帧；Chrome 未记录 WASM worker、WebSocket 或静态资源错误。
- [x] `camera_id=584` 的鉴权可返回 200，但不在当前 10001 实验页白名单中，不能作为页面验收样本。
- [ ] 向接口方索取 10001 白名单摄像头中“确认存在录像”的 `camera_id + Unix 起止时间` 样本；或在实验页补不含媒体内容的安全帧计数诊断，区分“无录像数据”与“转封装/解码失败”。
- [x] 按用户指定精确 Unix 时间窗复验 `camera_id=111`：`1755663940` 至 `1755664704`；页面确实接受并传入秒级时间，但连续约 33 秒仍只有 WSS 已连接、无首帧。
- 结论：此前页面分钟级输入不是原因；当前故障边界已收敛到回放媒体数据是否下发，或已下发数据的转封装/解码阶段。

### 2026-08-21 回放媒体链路诊断（进行中）

- [x] 在隔离 NVR 实验播放器中增加脱敏诊断计数：WSS 媒体包、WASM runtime/转封装状态、WASM 输出帧、解码输入帧、Canvas 渲染帧和 WSS 关闭码。
- [x] 确认诊断不渲染长期 Authorization、短时 token、WSS 地址、媒体字节或上游响应正文，并补充前端渲染测试。
- [ ] 升级至 `3.1.4` 后发布测试环境，以 `camera_id=111` 和已确认 Unix 时间窗实际验收并下结论。
- 验收判定：接收媒体包为 `0` 表示 WSS 已连接但上游未下发该时间窗的媒体数据或仍缺协议指令；媒体包大于 `0` 而 WASM 输出帧为 `0` 表示回放封装/转封装不兼容；WASM 输出帧大于 `0` 而 Canvas 渲染帧为 `0` 表示浏览器解码或渲染问题；Canvas 渲染帧大于 `0` 则回放首帧成功。
- [x] Chrome 实测发现原生 `datetime-local` 未声明秒级粒度，精确到秒的有效回放窗口会被浏览器清空；补 `step=1` 后升版 `3.1.5` 再验收。
- [x] Chrome 插件对原生日期时间控件的键盘事件无法同步 React 状态；为可复现实验，增加仅预填 `start_at` / `end_at` 查询参数的入口，仍需用户点击播放，且 URL 不包含 token 或凭据。
- [x] 测试环境 `3.1.6 / 5cb5f52` 实测 `camera_id=111`（治疗室4）回放窗口 `2025-08-20 12:25:40` 至 `12:38:24`：鉴权和 WSS 建连成功，连续 34 秒 `接收媒体包=0`，WASM runtime 已就绪，转封装/解码/Canvas 帧均为 `0`。
- 结论：当前问题不在二壮播放器、WASM、浏览器解码或渲染；该 WSS 回放会话未下发任何二进制媒体数据。下一步由接口方确认该 `camera_id + 时间窗` 是否实际有录像，以及 WSS 是否还要求客户端发送播放/订阅协议指令。
- [x] 对照验证 `camera_id=584`：绕过二壮与播放器，直接用实时短期 token 建立原始 WSS；20 秒收到 `923` 条消息、约 `5.31 MB` 二进制数据。通用 WSS 地址和流服务可用。
- 更新后的排查结论：`111` 零媒体包不属于前端播放器或通用 WSS 地址问题，优先由接口方核查其录像数据、摄像头映射和回放任务创建。
- [x] 使用运维提供的原始 NVRPlayer SDK 对照 `584`：8 秒收到 `329` 条消息、约 `1.87 MB`，`onFirstFrame` 已触发，无播放器错误。
- 修正结论：`584` 已验证“WSS 收包且 NVRPlayer 可首帧播放”；`111` 已验证“WSS 建连但零媒体包”。因此当前 111 问题在播放器之前的上游数据下发/录像会话层，不继续修改播放器。
- [x] 原始 SDK 复验 `camera_id=72`：9 秒收到 `477` 条消息、约 `2.21 MB`，`onFirstFrame` 已触发，无播放器错误；结论与 584 一致。

### 2026-08-25 `camera_id=111` 直播映射核验

- [x] 代码核查：实验页摄像头 ID 取自 `tb_crm_iot_device.id`，并原样传给鉴权接口的 `camera_id`；当前没有独立的 stream-camera-ID 字段或转换规则。
- [x] 测试同步库只读核验：`id=111` 为 `category=camera`、`provider=HikVisionNvrChannel`、`hardware_id=NVRCHANNEL:22-56`，名称为“治疗室4-客流测试”，不是普通非摄像头设备。
- [x] Chrome 测试环境打开 `/h5/nvr-lab/10001/cameras/111`，实时视频显示“画面已开始播放”，播放器 Canvas 为 `1920x1080`。
- [x] 刷新后复核同一路径，连续 10 秒保持播放状态、无页面错误，Canvas 尺寸保持 `1920x1080`。
- 结论：`111` 可用于当前取流服务，不能再将其历史回放零媒体包归因于“页面取错 ID”。直播可播放与指定历史时间窗是否有录像是独立验收。
- 回放窗口已由 2026 年实测确认可播放；后续继续保持旧 H5 Monitor 不变，也不猜测另一套 ID。

### 2026-08-25 `111` 2026 年历史回放验收

- [x] 使用 `camera_id=111`，回放窗口 `2026-08-25 11:03:38` 至 `11:10:18`，在测试环境 NVR 实验页发起回放。
- [x] 35 秒结果：接收媒体包 `219`、WASM 初始化 `1`、WASM 输出帧 `149`、解码输入帧 `74`、Canvas 已渲染帧 `71`，页面状态为“画面已开始播放”，无页面错误。
- 结论：10001 的 `111` 已通过测试环境桌面端直播和该 2026 历史窗口回放验收。此前 2025 窗口零媒体包是录像窗口/上游数据差异，不能再概括为回放功能故障。
- [ ] 尚未完成 iPhone Safari 与微信内直播验收；旧 H5 Monitor 不替换、正式环境不发布，直到移动端硬门槛完成。

## 2026-08-26 已确认方案：3.0 摄像头列表复用 2.x 最近截图

- 现状：3.0 摄像头列表只读业务库的 `tb_crm_iot_device`，没有截图字段，当前使用灰色占位；2.x 的 H5 Monitor 已从 `tb_channel_snapshots` 读取每个旧视频通道的最新 `thumbnail_path` 并返回预览图。
- 已确认的旧链路关系：`tb_stores.external_org_id -> tb_video_recorders -> tb_video_channels -> tb_channel_snapshots`；其中最新截图按 `channel_id`、`created_at desc, id desc` 选择。
- 关键约束：业务摄像头的 `id` 和旧 `tb_video_channels.id` 不是同一主键；`NVRCHANNEL:<业务NVR ID>-<通道号>` 中的业务 NVR ID、业务 NVR 序列号和旧 `tb_video_recorders.device_code` 均不能假定相等。因此不使用任何 NVR 序列号做截图映射依据。
- 已确认 v1：仅在 `external_org_id` 一致，且该旧门店恰好只有一台有效旧录像机时，按 `channel_no` 动态读取该旧通道的最新截图作为预览。此时“门店 + 通道号”唯一，不会混淆录像机。无旧门店、无旧录像机、多个旧录像机、通道不存在、通道号重复或无截图时一律保持灰色占位；不猜测、不写业务库、不复制 OSS 文件、不触发抓图。
- 安全边界：预览图访问必须复用门店监控范围授权，不能因用户可浏览资源列表就绕过摄像头查看权限；优先由后端受控代理图片，不将 OSS 原始路径或旧截图访问权直接暴露给前端。
- 待验证：统计单旧录像机门店数量和可命中截图覆盖率；实现时以测试覆盖单录像机命中、无截图、多录像机拒绝匹配、无旧门店和无监控权限五类情形，再发布测试环境验收。
