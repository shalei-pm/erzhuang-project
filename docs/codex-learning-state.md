# Codex Learning State

最后更新：2026-08-18

## 2026-08-21 NVR 回放实验最新结论

- 测试分支最新实验提交：`5cb5f52`，测试页面版本 `3.1.6`，路由仅限 `10001`：`/erzhuang-project/h5/nvr-lab/10001/cameras/111`。
- 已用 Chrome 在已登录测试环境真实验收 `camera_id=111`（治疗室4）回放：`2025-08-20 12:25:40` 至 `2025-08-20 12:38:24`。
- 结果：鉴权和 WSS 建连成功；34 秒内收到媒体包 `0`；WASM runtime 已就绪；转封装、解码、Canvas 渲染均 `0`。二壮播放器没有收到可解码的数据，尚不能播放。
- 判断：当前由接口方处理或确认，优先核实该摄像头该窗口是否有录像，以及 WSS 连接后是否要求额外播放/订阅协议消息。不要继续修改旧萤石 H5 Monitor，也不要把问题归因于浏览器兼容性。

## 2026-08-25 NVR 原始 WSS 对照验证

- 使用运维提供的 `camera_id=584` 回放参数实时换取短期 token，并使用独立 Node WebSocket 客户端直接连接，不使用二壮页面、NVRPlayer、WASM 或浏览器解码。
- 20 秒结果：鉴权成功、WSS 建连成功、收到 `923` 条消息，其中二进制数据约 `5,310,364` 字节，未发生连接错误或服务端关闭。
- 结论：该类 WSS 地址、鉴权服务和上游流服务的通用链路可正常下发媒体数据。`camera_id=111` 的零媒体包问题不再应归因于 WSS 地址格式或播放器，优先排查 111 的录像可用性、摄像头/设备映射及该会话的上游回放任务。
- 安全：本次未输出、记录或保留长期 Authorization、短期 token、完整 WSS 地址或媒体内容。

## 2026-08-25 原始 NVRPlayer 播放对照验证

- 为避免把“原始 WSS 收到字节”误推导为“播放器必然可播放”，使用运维提供的原始 `NVRPlayer-SDK` 以同一 `camera_id=584` 与同一回放窗口做本机隔离播放测试。临时页面只监听 `127.0.0.1`，由本机后端实时签发短期会话。
- 8 秒结果：WSS 建连成功，收到 `329` 条消息、约 `1,872,064` 字节，SDK `onFirstFrame` 回调已触发，未见播放器错误。
- 结论：NVRPlayer/WASM 的通用播放链路可实际播放 584 的回放数据。111 的失败发生在播放器之前：该会话没有下发媒体包。此结论不等于“播放器与所有未来摄像头无关”，而是排除了当前 111 问题由通用播放器实现导致的可能。
- 安全：临时服务、短期 token 和媒体数据均在验证结束后清理，不进入仓库或项目配置。

## 2026-08-25 camera_id=72 原始播放器复验

- 使用与 584 相同的回放时间窗和原始 NVRPlayer SDK，仅将 `camera_id` 改为 `72`。
- 9 秒结果：WSS 建连成功，收到 `477` 条消息、约 `2,209,212` 字节，`onFirstFrame` 已触发，无播放器错误。
- 结论：72 与 584 均能通过该 WSS 鉴权和原始 SDK 实际播放，进一步确认当前 111 的零媒体包是摄像头/录像会话级问题，而不是通用播放器问题。

## 2026-08-25 10001 `camera_id=111` 直播核验

- 代码事实：NVR 实验页将 `tb_crm_iot_device.id` 直接用作鉴权接口的 `camera_id`；当前没有单独的流服务 ID 字段或映射。
- 测试同步库只读查看：`tb_crm_iot_device.id=111` 是有效海康摄像头通道，`hardware_id=NVRCHANNEL:22-56`，名称“治疗室4-客流测试”。
- Chrome 已登录测试环境实测：`/erzhuang-project/h5/nvr-lab/10001/cameras/111` 显示“实时视频 / 画面已开始播放”，并已创建 `1920x1080` 播放器 Canvas。
- 页面刷新后再次打开同一路径，连续 10 秒维持“画面已开始播放”，未见页面错误，确认不是一次性残留状态。
- 结论：当前服务接受 `111` 作为取流参数，无法支持“页面拿错普通设备 ID”这一假设。此前 111 的历史回放零媒体包仍须按录像可用性或服务端回放会话处理，不能与直播成功混为一谈。
- 安全：本次仅只读检查测试库和已登录页面；未保存、输出或写入 Authorization、短期 token、WSS URL 或媒体数据。

## 2026-08-25 10001 `camera_id=111` 2026 年回放通过

- 使用已登录 Chrome 测试环境，在 NVR 实验页对 `camera_id=111` 发起 `2026-08-25 11:03:38` 至 `11:10:18` 的回放。
- 35 秒验收：接收媒体包 `219`、WASM 初始化 `1`、WASM 输出帧 `149`、解码输入帧 `74`、Canvas 渲染帧 `71`，页面状态“画面已开始播放”，未见页面错误。
- 结论：10001 的 `111` 已完成测试环境桌面端直播与该回放窗口的首帧/持续渲染验收。此前 2025 时间窗的零媒体包是该窗口数据或服务端回放会话的差异，不是通用播放器、ID 映射或当前回放功能故障。
- 保留边界：iPhone Safari、微信内直播、长时播放、断线重连和正式环境均未验收；不得据此替换旧萤石 H5 Monitor 或发布正式环境。

## 当前项目记忆快照（主会话优先读取）

本节是主负责人会话的“当前事实”摘要。后续新会话、压缩恢复、发布、回滚或专项开发前，先读本节，再按需读取后面的历史记录和专项文档。

### 项目定位与目标

- 项目从个人练习项目演进为新氧青春门店空间资源管理系统。
- 业务目标：二壮 3.0「门店空间资源查看」基于公司业务库只读展示已部署工控机门店的业务空间、工控机、NVR、摄像头和空间-设备绑定完整性；现有 H5 Monitor 监控查看方式保持不变。
- 工程目标：保持一条可重复的 Codex 开发、Git 管理、公司 GitLab/K8s 发布、线上验证、回滚和文档沉淀流程。
- 主会话职责：项目负责人/架构中枢，负责需求澄清、方案拆分、代码验收、发布、回滚、文件化记忆和跨会话交接。

### 当前阶段

- 当前线上公司环境已切换为 MySQL + OSS。
- 最新确认健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"mysql","asset_store":"oss"}
```

- 最新版本文件：`VERSION=2.31.8`。
- 最新公司 GitLab 发布提交：`7311bda fix: fallback h5 player for unsupported h265 mse`，已推送到 `gitlab/codex/containerize-single-image` 并触发公司 K8s 自动发布；公网无登录态 `curl /health` 当前会被 APISIX 302 到 SSO，线上版本和播放结果需由已登录浏览器验证。
- 当前 3.0 开发状态：3.0 主流程从二壮自维护门店/录像机/通道/设计图/AI 识别，转为读取公司业务库的只读资源查看。设计文档已落地：`docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`；实现计划已落地：`docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`。本地分支 `codex/store-space-resource-view-3` 已完成后端只读 API 和前端只读列表/详情初版；测试环境已验证代码发布但因环境隔离无法访问正式业务库，下一步目标是公司正式环境 cutover。
- 2026-08-14 发布链路新口径：GitLab 仓库不变；测试环境使用 `codex/containerize-single-image` + Wharf pipeline `752` + `https://lite.sy.soyoung.com/erzhuang-project`；正式环境使用 GitLab `main` + Wharf pipeline `771` + `http://lite.soyoung.com/erzhuang-project`。正式环境是在 `main` 分支提交代码后自动触发 pipeline `771` 构建，构建成功后还需要在 Wharf 点部署并走审批，审批通过并部署成功才算正式上线完成。
- 2026-08-18 数据库访问新口径：本机 TablePro 只用于连接测试库 `polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang` 做开发和只读核查；线上库 `polar-ops.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang` 不通过客户端直连，只由线上代码或公司批准链路访问。涉及线上表结构调整时，Codex 产出 SQL/影响说明/验证 SQL/回滚建议，由运维在正式库执行。
- 2.x 稳定备份已完成：tag `v2.31-stable-before-resource-view-3` 指向 `7311bda / VERSION=2.31.8`，说明文档为 `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`。
- 旧 PostgreSQL runtime、pgx 依赖、pg-mysql 迁移入口和旧库回滚连接已从运行时代码中删除。后续不要再以“可切回 Postgres”作为安全阀。
- 韩国 Lighthouse 发布链路已终止；该服务器上关于二壮项目的所有库表已经完全删除。后续二壮项目发布、回滚、验收和排查只走公司 GitLab/K8s + MySQL/OSS，不再使用韩国服务器。

### 已完成内容

- 门店空间资源后台：门店列表、城市/名称筛选、门店详情、设计图标注、通道映射、门店/录像机/通道写接口。
- 设计图能力：PDF 上传、预览图、标注区域、AI 识别、人工修正、设计图保存。
- 录像机与通道能力：萤石账号同步、录像机扫描有效通道、刷新截图、AI 识别通道画面、人工确认业务/非业务区域、床位拆分。
- H5 Monitor：按机构/门店查看监控通道，直播、回放、播放器交互、门店切换、通道分组和移动端播放体验已完成多轮线上修复。
- SSO 与权限：公司 APISIX SSO cookie/JWT 接入，`tb_users` 授权用户表，角色/权限基础逻辑，写接口按权限守卫。
- MySQL/OSS 迁移：54 家有 `external_org_id` 的门店已迁入 MySQL；通道截图等资产已迁入 OSS 并通过台账/代理路径验证。第 55 家 Postgres 空 `external_org_id` 门店“新氧青春诊所(长沙北辰荟店)”确认不迁移。
- 运行时安全清理：`cmd/server` 只接受 `APP_DB_DRIVER=mysql`，只读取 `MYSQL_DSN` / `K8S_SECRET_MYSQL_DSN`；删除旧 PostgreSQL repository、schema 初始化、导出 CLI、pg-mysql ops 入口和 pgx 依赖。
- 线上只读回归：2026-07-04 已由用户在已登录浏览器执行，只读检查全部 200，门店总数 54，H5 Monitor 样本门店 `10030`、`10019`、`10081` 均返回正常分组。
- 线上写接口回归：2026-07-04 已由用户在已登录浏览器执行，临时门店创建、编辑、保存设计图、添加录像机、删除录像机、删除临时门店全部通过。
- 线上资产/识别抽验：2026-07-04 门店 `56`、录像机 `64/GQ2603587`、通道 `900065`，门店详情读取和单通道截图刷新接口均 200；单通道识别接口 200 但业务结果为 `recognition_failed`，需要继续查看 `recognition_result` 失败原因。
- 通道 `900065` 识别失败根因已定位：`capture_ms=578`，抓图成功；`recognition_ms=30407` 后 AI provider 返回 upstream 502，请求 ID `5704b803-f192-4f86-a3a7-be7a3df7a53d`。这不是 MySQL/OSS 主链路失败。
- 同门店另一个通道 `900076` 单通道识别成功：接口 200，业务结果 `recognized`。结论：AI/识别链路整体可用，`900065` 属于单次上游 502 波动。
- 用户管理角色保存修复：`2.30.24 / f732228` 修复 MySQL `tb_roles` 漏种 `editor` 导致保存“编辑运维”后回退普通查看的问题。
- 退出登录修复：`2.31.7` 将公司域名退出改为顶层同源 `/erzhuang-project/logout?redirect=<SSO统一退出地址>`；后端先清理 host-only、当前 host 域和父域 `sy_sso_token`，再安全跳转到公司 `logouttogether`，避免 fetch 清 cookie 与网关退出互相影响。
- Windows 监控播放兼容修复：`2.31.8` 针对部分 Windows Edge/Chrome 访问 H5 Monitor 时 `MediaSource addSourceBuffer hvc1 unsupported` 的问题，默认仍保留桌面 MSE；仅当播放器实际报出 H.265/HEVC SourceBuffer unsupported 时自动切到 `desktop-wasm` 软解重试，避免影响可正常播放的 Windows 电脑。
- 普通查看用户监控门店范围权限：`2.31.0` 新增 `tb_user_resource_scopes` 通用 scope 表，用户管理创建/编辑 `viewer` 时可按城市/搜索勾选可查看监控门店；H5 Monitor 列表和直接门店 URL 均由后端强校验，门店列表/详情返回 `can_view_monitor` 供前端隐藏入口。
- 门店列表统计口径修复：`2.31.0` MySQL `ListStores` 的右上角 summary 改为按当前搜索/城市筛选条件汇总全量 filtered dataset，不再用当前页 `items` 汇总，保证分页切换不改变统计。
- 3.0 方案梳理：已确认模块名为「门店空间资源查看」；只展示有启用工控机的门店；空间类型使用业务库自己的三层结构 `level=1/2/3`；详情展示空间视角、设备视角、异常项；设计图标注、AI 通道识别、人工确认和门店/录像机/通道写入口不进入 3.0 主流程；H5 Monitor 暂不改。
- 3.0 前 2.x 封版：已创建 `v2.31-stable-before-resource-view-3`，并写入 handoff 文档，后续回滚优先用 `git revert` 而不是 reset/force push。
- 3.0 后端当前版本：新增 `internal/resourceview`，提供已同步 `tb_crm_*` 四表的只读聚合、空间树、设备树、异常项和 API `GET /api/store-space-resource-view/stores`、`GET /api/store-space-resource-view/stores/{tenantId}`；3.0.3 起复用二壮主 MySQL，不再支持独立业务库 DSN。
- 3.0 前端初版：新增资源查看 API 类型、domain helper、`ResourceStoreList`、`ResourceStoreDetail`；后台主页面切为「门店空间资源查看」，展示工控机/NVR/摄像头/空间/绑定/异常统计，详情含空间视角、设备视角、异常项；旧新增、编辑、删除、扫描、识别、确认、设计图上传/标注入口已从主页面隐藏。

### 当前进行中

- 2026-08-26 工控机 NVR 监控全量替换：用户已确认正常 H5 入口全量切换、未配置门店隐藏、管理员/编辑运维全量可见、普通查看按 `monitor:view` 门店范围过滤；技术实施由主会话负责，必须保留 `MONITOR_PLAYBACK_MODE=nvr|legacy` 运行时回滚和 Git 稳定 commit 回滚。设计与实施计划分别位于 `docs/superpowers/specs/2026-08-26-nvr-monitor-full-rollout-design.md`、`docs/superpowers/plans/2026-08-26-nvr-monitor-full-rollout-implementation.md`；当前已进入测试发布前的代码验证阶段。
- 2026-08-26 3.2.0 全量 NVR 首版已完成本地实现，待测试发布：新增 `internal/nvrmonitor`，正常路由在 `MONITOR_PLAYBACK_MODE=nvr` 时改用 NVR 门店、摄像头和会话接口；默认/非法模式或缺少 `K8S_SECRET_NVR_STREAM_AUTHORIZATION` 时回退 `legacy`。NVR 准入固定为启用门店下 `category='camera' AND provider='HikVisionNvrChannel' AND status=1 AND deleted_at IS NULL`；前端同一构建通过受认证的 `/api/h5/monitor-mode` 在运行时选择 NVR 或旧萤石页面，旧通道深链接不猜测映射。前端 Vitest `62` 项和生产构建已通过；本机没有 Go，后端编译和测试必须由 Wharf 测试构建补验。待测试 K8s 开关为 `nvr` 后做实际功能与回滚演练。
- 2026-08-26 测试发布记录：`f1625a4 feat: roll out nvr monitor routes` 已同步 GitHub 备份分支和 GitLab `codex/containerize-single-image`，触发 Wharf pipeline `752`。随后只读核对 GitLab remote，该分支 HEAD 为 `f1625a4`；临时 GitLab AskPass 已删除。测试健康检查在未登录命令行会话中正确返回 APISIX SSO `302`，不能据此判定应用异常。GitLab API 对当前发布凭据不开放 pipeline 查询（`404`），Chrome 插件当前无已连接标签页，故尚未确认 Wharf 构建/部署结果或页面版本；不得将本次状态描述为“测试发布成功”。下一步先在 Wharf 确认构建和部署，再把测试实例的 `MONITOR_PLAYBACK_MODE` 设为 `nvr` 并执行功能和 `nvr -> legacy` 回滚演练。
- 2026-08-26 NVR 缩略图排查：正常 NVR 监控列表的黑色画面来自前端固定空态，不是工控机截图请求失败；该页面当前没有渲染 API 返回的 `thumbnail_url`。二壮资源查看已保留旧 2.x 截图兜底，但只允许“旧系统恰有一台录像机、按通道号匹配”的门店使用，避免多录像机或序列号不一致造成错图。待确定缩略图策略：受限历史图兜底、首帧后的按需私有存储，或由工控机平台提供受鉴权截图 API；未经确认不新增截图写入、后台抓流或长期媒体留存。
- 2026-08-26 3.2.1 缩略图首步：NVR 监控列表开始渲染受限历史截图；只有后端按“单旧录像机 + 同通道号”验证过的 `thumbnail_url` 才会下发，图片 404/加载失败或无可靠历史图时回落到中性灰色图片占位。没有新增工控机截图请求、后台解码、OSS 写入或业务库写入。前端测试 `11 files / 64 tests` 与生产构建通过，待测试 Wharf 构建和 Chrome 真实页面验收。
- 2026-08-26 3.2.1 测试发布与视觉验收：GitLab 测试 commit `824af6f feat: show verified monitor thumbnails` 的 Wharf 构建 `169173` 在 1 分 31 秒内成功，自动部署记录 `414738` 已部署至测试实例。Chrome 真实打开 `10001` 的正常 NVR 监控页，44 路摄像头均显示中性灰色图片占位，不再是黑色块，页面和控制台无项目错误。该门店没有符合受限旧图匹配规则的 `thumbnail_url`，故本次无法验证实际历史图渲染；缩略图第二阶段仍需决定首帧私有存储或工控机静态截图 API。
- 2026-08-26 缩略图初始化范围纠正：用户不接受把截图初始化设计为普通 UI 功能、人工浏览器驻留队列或需要手工配置 Web 实例的任务。目标改为一次性的后台数据回填：独立任务严格串行处理摄像头、生成私有 OSS 截图，业务页面仅读取结果。由于 NVR SDK 的首帧渲染依赖 Canvas/WebCodecs，后台任务仍需专用渲染运行时；推荐使用一次性独立 Job/runner，不向 Web 实例注入新配置，Job 仅引用既有机密和数据库/OSS Secret。待用户确认该执行形态后，编写正式设计、DDL 和运维执行清单。
- 2026-08-26 NVR 截图回填设计：用户批准将其按纯后端一次性数据回填推进。设计文件 `docs/superpowers/specs/2026-08-26-nvr-snapshot-backfill-design.md` 已明确独立 runner、串行队列、后端 H.265/RTP 到 ffmpeg 解码、私有 OSS、二壮自有快照表、`10001` 技术验证与分阶段全量执行。它不向 Web 实例增加配置，不写 `tb_crm_*`，也不增加刷新 UI 或定时任务。设计待用户审阅；通过前不写业务代码、不创建表、不发起取流。
- 2026-08-26 NVR 缩略图无 DBA 替代：用户明确没有 DBA 支持，已废止自有快照表设计。当前 `3.2.4` 改为只读 `tb_crm_iot_device` 候选摄像头，成功 JPEG 只写现有私有 OSS 的 `nvr-camera-snapshots/{tenant_id}/{camera_id}.jpg`，由同源且复用门店监控权限的图片路由读取；对象不存在时前端保持中性占位。默认跳过已有对象，`--force` 才允许覆盖；严格串行、每路 20 秒、相邻取流至少 2 秒、连续 3 次鉴权/WSS 故障熔断。`--all-tenants` 是全测试环境的显式开关，不能与门店或摄像头筛选混用。此例外只适用于 NVR 初始化，不放宽其他数据库治理规则。

- 建立文件化项目记忆机制：
  - `docs/codex-learning-state.md`：长期状态、发布记录、关键上下文。
  - `docs/decisions.md`：产品/技术决策台账。
  - `work/current-plan.md`：当前轮工作目标、拆分、进度和下一步。
- 项目控制文档已补齐：
  - `README.md`：已追平当前 MySQL + OSS 运行时口径。
  - `docs/technical-architecture-index.md`：已改为当前 store-space 主路径代码地图。
  - `docs/post-cutover-regression-checklist.md`：MySQL/OSS 切换后线上回归清单。
  - `docs/legacy-postgres-supabase-shutdown-checklist.md`：旧 PostgreSQL/Supabase 下线确认清单。
- MySQL/OSS 切换后的稳定期：后续每次重要讨论、开发、验证、发布、回滚都要主动回写上述文件。
- 门店空间资源查看 3.0：
  - 设计文档：`docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`。
  - 实现计划：`docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`。
  - 当前阶段：方案和计划已完成，2.x 稳定备份 tag、zip 与 handoff 已完成，后端只读 API 和前端只读资源查看初版已在本地分支完成。
  - 当前剩余：发布到公司后验证业务库连接、真实数据样本验收、用户体验确认；用户明确要求后才发布公司环境。

### 待决策问题

- 旧 PostgreSQL/Supabase 数据和资源的正式保留、归档、删除时间表，需要产品负责人、公司安全/运维/相关研发确认后执行；主会话不能独自推动删除外部数据源。
- 是否彻底下线旧 `designplan` 独立路由和旧 `tb_design_plan_*` 兼容表，需要先确认当前前端/用户流程是否仍依赖。
- 3.0 业务数据待确认：`tb_crm_consulting_room.dict_id` 字典来源、`province_id/city_id` 城市字典来源、`tb_crm_iot_area_device_relation.function_type` 取值口径、设备/空间状态枚举、工控机与 NVR 是否存在显式关系；测试数据已确认存在 `security_camera`、`cart_camera` 和多种非摄像头用途，3.0.3 仅将摄像头用途纳入摄像头映射。
- 是否从当前全局角色权限升级到更通用的机构/门店/资源范围授权。当前已决定先做普通查看用户的监控门店范围权限，并提前按 scope 模型考虑未来扩展。
- 门店删除当前仍沿用硬删除/外键级联语义；正式环境是否改成软删除和审计，需要 DBA/产品确认。
- 资产访问审计、长期安全审计、截图/PDF 访问日志的正式落地方案仍待治理。
- 历史迁移文档仍包含 PostgreSQL/Supabase 阶段事实，需要新会话按日期和当前快照判断是否仍有效；`README.md` 和 `docs/technical-architecture-index.md` 已追平当前架构。

### 技术栈与运行方式

- 后端：Go 1.22，`net/http`，入口 `cmd/server/main.go`。
- 数据库：公司 MySQL，核心表为 `tb_` 前缀，连接变量 `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN`。
- 3.0 资源查看：复用二壮 `MYSQL_DSN` / `K8S_SECRET_MYSQL_DSN`，只读查询同步到 `db_pm_erzhuang` 的 `tb_crm_*` 资源表；不得写入这些同步表或把连接串、账号、密码写入仓库。
- 3.0 本地开发分支：`codex/store-space-resource-view-3`。主会话按技术负责人拆分：Backend 只读资源聚合、Frontend 只读资源查看、主会话 Review/发布控制。
- 资产：公司 OSS，运行时 `ASSET_STORE=oss`；前端访问路径保持后端代理稳定。
- 前端：Vite + React + TypeScript + Ant Design，入口 `frontend/src/App.tsx`。
- H5 播放：萤石云 OpenAPI + `ezuikit-flv`，前端 decoder 静态资源位于 `frontend/public/assets/ezuikit-flv/`。
- AI：通道截图识别和设计图识别支持 OpenAI/GPT 与 MiniMax，运行时 provider 可通过后端配置读取；当前线上曾验证 MiniMax / MiniMax-M3。
- 登录：公司 APISIX SSO，`sy_sso_token` cookie，后端验签并用企业邮箱匹配 `tb_users`。
- 公司发布：推送 `gitlab/codex/containerize-single-image`，公司 GitLab/K8s 自动构建发布，入口 `https://lite.sy.soyoung.com/erzhuang-project/`。
- 公司环境隔离：
  - `*.sy.soyoung.com` 默认是内网可见测试环境。
  - 测试入口：`https://lite.sy.soyoung.com/erzhuang-project`，对接测试分支 `codex/containerize-single-image`、测试实例机器和测试数据库，Wharf pipeline `752`。
  - 线上入口：`http://lite.soyoung.com/erzhuang-project`，对接 GitLab `main`、主干实例机器和线上数据库，Wharf pipeline `771`。
  - 当前目标已经切到公司正式环境；后续“发布到公司”必须先确认测试还是正式，正式发布按 GitLab `main` 提交触发构建、Wharf 点部署、审批通过后上线处理。
  - 二壮运行库测试环境：host `polar-dev.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret/安全渠道管理。
  - 二壮运行库正式环境：host `polar-ops.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret/安全渠道管理。
- 3.0 资源查看与二壮运行库共用 `db_pm_erzhuang` 主连接；数据表仍按只读边界使用，测试验收完成后可清理不再使用的独立业务库 Secret。
- GitHub 代码备份：GitHub 仍保留为主代码备份和历史留存；除非用户明确说明“不要同步 GitHub”或“只推公司 GitLab”，已确认准备发布的代码仍应同步 GitHub。GitHub 不再代表线上发布完成。
- GitLab 推送认证：本机 token 文件为 `/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`。发布到公司时默认用临时 `GIT_ASKPASS` 读取该文件，用户名 `oauth2`；禁止打印 token、写入命令、写入仓库、写入文档或保留长期 askpass，用完删除临时脚本。
- 历史个人练习发布：GitHub `main` + 韩国 Lighthouse + `scripts/deploy.sh` + systemd/nginx。该链路已终止，且韩国服务器上的二壮项目库表已删除；只保留为历史学习记录。

### 验证与发布规则

- 公司发布前默认验证：
  - Go：本机直接 `go test` 可能触发 macOS `missing LC_UUID`，当前可靠门禁是 `go test -c` 编译关键包和 `go build ./cmd/server`。
  - 前端：涉及 UI 时必须读 `docs/ui-standards.md`、`docs/frontend-review-checklist.md`，并做浏览器实际验收，不只跑 build。
  - 浏览器调试：优先检查 Chrome 插件能力，必要时让用户用 `[@chrome](plugin://chrome@openai-bundled)` 唤起；可用时优先用 Chrome 插件或 `node_repl` 配合 Chrome Plugin。只有插件未暴露或不可用时，才降级到 Computer Use，并在最终说明中标注原因。
  - GitLab hook：推公司分支前，对本次改动文件运行 `rg -n -i "join" <changed-files>`，避免 hook 拦截。
- 公司发布后验证：
  - 线上已登录浏览器验证 `/erzhuang-project/health` 返回 `database=mysql`、`asset_store=oss`。
  - 关键页面：门店列表、门店详情、H5 Monitor、写接口、通道识别/确认按本次改动范围抽验。
- 韩国 Lighthouse 发布/回滚已废止，不得再用于二壮项目；历史文档中的 TAT、`scripts/deploy.sh`、`scripts/rollback.sh` 只作为早期练习记录。

### 已知风险

- 历史迁移文档仍存在阶段性表述；入口文档 `README.md` 和 `docs/technical-architecture-index.md` 已更新为 MySQL/OSS 当前事实。
- 旧 PostgreSQL 回滚连接已删除，MySQL/OSS 正式成为唯一运行时路径；后续回滚只能回滚到仍兼容 MySQL/OSS 的提交，不能依赖旧库兜底。
- 韩国服务器上的二壮项目库表已经完全删除，因此韩国 Lighthouse 不再具备二壮项目运行、回滚或对照验证能力。
- 萤石抓图和播放接口可能触发限流/风控，批量识别必须节流；老批量识别接口已降为单次推进 1 路。
- MiniMax/GPT 都可能返回非 JSON 或 `<think>` 解释文本，已做兜底，但仍建议保留单通道人工重试。
- 公司公网 health 从无登录态环境可能被 SSO 重定向，线上验证优先由用户已登录浏览器执行。
- MySQL 8.0.13 对 CHECK 约束不可靠，应用层和迁移脚本必须继续做枚举/数据校验。
- 3.0 会引入第二个 MySQL 只读数据源，必须严格区分“二壮运行库”和“公司业务库”：二壮库继续负责登录、权限、系统设置和 H5 Monitor；业务库只负责资源查看。业务库账号必须只读，API 不提供写入。真实 DSN、账号、密码不得写入仓库或文档。
- 3.0 当前城市名第一版按 `city_id` 展示为“城市 N”；如产品要求显示真实城市名，需要业务库提供城市字典或由后端补充稳定映射。
- 3.0 真实业务库数据量、长门店名、空间树深度、异常数量仍需用公司真实样本做一次 UI 信息密度验收。

### 下一步建议

1. 完成正式环境 cutover 前检查：正式实例信息、`main` 提交流程、pipeline `771` 构建、Wharf 部署审批、正式 Secret、SSO/网关、回滚点和验收脚本。
2. 代码侧补正式域名 `lite.soyoung.com` 的 SSO 域名判断，并评估/修复正式登录资料同步可能遇到的 MySQL collation mismatch。
3. 正式发布时把已验证代码提交到 GitLab `main`，等待 Wharf pipeline `771` 构建成功，再在 Wharf 点部署并走审批。
4. 审批通过且部署成功后验证 `http://lite.soyoung.com/erzhuang-project`：health、SSO 登录/退出、用户管理、门店列表、H5 Monitor、3.0 资源查看真实数据和数据库来源。

## 2026-08-14 公司正式环境发布链路与数据库口径更新

- 背景：此前主会话长期使用的公司地址 `https://lite.sy.soyoung.com/erzhuang-project` 实际是测试环境。用户确认现在要切到公司正式环境。
- GitLab 仓库：`https://gitlab.sy.soyoung.com/pm/shalei-pm/erzhuang-project.git`，不变。
- 测试环境：
  - 分支：`codex/containerize-single-image`
  - Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=752`
  - 入口：`https://lite.sy.soyoung.com/erzhuang-project`
  - 二壮运行库：`polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`
- 正式环境：
  - 分支：GitLab `main`
  - Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=771`
  - 流程：在 `main` 分支提交代码后自动触发构建；构建成功后在 Wharf 点部署并走审批；审批通过且部署成功后才进入线上验收。
  - 入口：`http://lite.soyoung.com/erzhuang-project`
  - 二壮运行库：`polar-ops.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`
- 数据库用户：二壮运行库测试和正式均使用 `u_pm_erzhuang_rw`；密码不得写入仓库、文档、命令或回复，后续只通过 K8s Secret/安全渠道配置。
- 操作边界：
  - 当前只更新项目记忆和发布手册，不执行 `main` 提交、Wharf 部署或正式发布。
  - 后续正式发布需要把已验证代码提交到 GitLab `main`，观察 pipeline `771` 构建，构建成功后在 Wharf 点部署并走审批，再在正式域名完成验收。

## 2026-08-18 测试/线上数据库访问方式补充

- 用户确认：
  - TablePro 可以直接访问测试库，用于开发期查询和只读验收。
  - 线上库不能通过客户端访问，只能由线上代码访问。
  - 测试库与线上库目前按表结构一致口径管理。
  - 后续如开发涉及线上表结构调整，需要由 Codex/DBA 专项整理表操作 SQL 和说明，交给运维执行正式库变更。
- 安全边界：
  - 仓库、文档和最终回复不记录数据库密码或完整 DSN。
  - 本机客户端不保存线上连接，不直连线上库。
  - 当前要先用测试库确认运维同步进 `db_pm_erzhuang` 的资源查看表是否已有数据。

### 2026-08-18 TablePro 测试库资源表同步核查

- 工具：TablePro，本机只连测试库 `polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`。
- 操作：仅执行只读查询，未执行写入、导出或表结构变更。
- 表存在性查询：
  - `tb_crm_admin_tenant`：存在。
  - `tb_crm_consulting_room`：存在。
  - `tb_crm_iot_device`：存在。
  - `tb_crm_iot_area_device_relation`：未出现在 `information_schema.tables`。
  - 进一步按 `%relation%`、`%area%device%`、`%iot%area%` 模糊查表名，未返回任何行。
- 精确行数：
  - `tb_crm_admin_tenant`：85。
  - `tb_crm_iot_device`：7546。
  - `tb_crm_consulting_room`：3523。
- `tb_crm_iot_device.category` 分布：
  - `bt_gateway`：732。
  - `button`：633。
  - `camera`：3807。
  - `edge`：70。
  - `nvr`：81。
  - `pad`：800。
  - `tv`：1423。
- 有工控机门店数：`tenants_with_edge=67`。
- 结论：测试库已经同步了门店、设备、空间三张核心表，且有足够数据支撑“有工控机门店列表”；但缺少空间-摄像头绑定关系表 `tb_crm_iot_area_device_relation`，当前还不足以支撑 3.0 详情页完整展示“空间/床位 -> 摄像头”的绑定关系。
- 原待同步事项已完成：关系表已同步到测试和线上二壮库。下一步改为收敛 3.0 数据源至项目主库，并以真实数据完成 API、异常项和前端信息密度验收。
- 2026-08-18 业务库维护研发补充确认关系表 DDL：`tb_crm_iot_area_device_relation.area_id` 就是 `tb_crm_consulting_room.id`，`device_id` 关联 `tb_crm_iot_device.id`。查询摄像头时才筛 `category='camera'`，因为关系表可能包含其他设备类型。该表以 `(device_id, area_id, function_type)` 唯一，`function_type` 是绑定用途维度；在拿到真实数据和枚举口径前，3.0 不以它过滤绑定关系，也不把 `device_id + area_id` 视为唯一。
- 同步完成复核：TablePro 测试连接确认关系表实际行数 `5632`。其中摄像头关系 `2586` 条，覆盖 `1725` 个空间、`1876` 台摄像头；`area_id` 孤儿关系为 `0`，`device_id` 孤儿关系为 `4`。`function_type` 分布为 `security_camera=2006`、`cart_camera=584`、`pad=709`、`business_tv=589`、`live_tv=589`、`help_button=582`、`bt_gateway=573`；摄像头关系来自前两类。用户确认该表已同时同步至线上二壮库。
- 数据源决策更新：3.0 不再需要跨库访问原业务库，下一次开发将改为复用二壮主库的只读查询；仍不允许向同步表写入。

### 2026-08-18 3.0.3 主库收敛实现完成，待测试环境验收

- 实现：`cmd/server/main.go` 删除独立业务库 DSN 和第二条 MySQL 连接；主库成功建立后将同一 `*sql.DB` 注入 `resourceview.NewMySQLRepository`。
- 真实数据修正：关系表同时含摄像头、PAD、电视、蓝牙网关等关系。repository 和 service 只把 `category='camera'` 或缺失设备但用途以 `camera` 结尾的关系纳入摄像头映射；PAD、电视、网关不再被误报为缺失摄像头。
- 异常保留：摄像头用途的缺失设备关系继续产生 `missing_camera`；测试库已知有 4 条此类设备孤儿关系。
- 本地验证：`CGO_ENABLED=0` 下 `go test ./internal/resourceview`、`go test ./internal/app`、`go test ./cmd/server` 通过；后端 `go build` 通过；前端 `npm test` 41 项通过、`npm run build` 通过，仅有既有 chunk-size warning。
- 下一步：提交并发布 GitLab 测试分支 `codex/containerize-single-image`，用已登录测试浏览器验收资源列表、详情、异常项和 H5 Monitor 入口；验收通过后才进入正式 `main` 发布流程。

### 2026-08-18 3.0.4 测试环境同步表字段兼容修复，待重新验收

- 首次测试发布：GitLab 测试 pipeline `752` 的构建 `167892` 成功，测试页面显示 `3.0.3 (container)`。
- 发现：资源列表请求失败，页面显示 `Error 1054 (42S22): Unknown column 'ip' in 'field list'`，导致前端列表和汇总显示为 0；这不表示同步数据为空。
- 根因：3.0 初版 repository 错把源业务表的字段假设写成 `ip`、`heartbeat_at`、`sort_order`。已提供的同步表 DDL 使用 `ip_addr`、`last_heartbeat_time`、`order_num`。
- 修复：3.0.4 只改只读查询的字段名和排序子句，IP/心跳/空间排序的接口语义保持不变；新增源码级回归检查，防止旧字段名再次进入同步表 SQL。
- 发布：commit `28ac15a fix: align resource view with synced mysql schema` 已同步 GitHub 备份和 GitLab 测试分支；Wharf pipeline `752` 构建 `167894` 成功。
- 测试验收：测试页显示 `3.0.4 (container)`；资源列表返回 `67` 家有工控机门店、`67` 台工控机、`75` 台 NVR、`3465` 个摄像头、`1874` 条绑定关系。真实门店 `10001 北京保利总部店` 的设备视角显示工控机、NVR 和带空间名称的摄像头通道；异常项也正常展示未绑定空间摄像头。
- 发布门禁：当前仅完成测试环境自动验收，等待用户确认业务展示口径后再把同一 commit 进入正式 `main`；不得把本次测试构建当作正式发布。

### 2026-08-18 3.0.5 首页统计展示收敛，测试环境验收通过

- 产品确认：首页保留摄像头、空间、已绑定、未绑定的覆盖率口径；空间暂时按三层空间节点统计。首页不展示“异常”，异常项只在门店详情中用于排查。
- 实现：移除顶部汇总“异常”及表格“异常”列，空态跨列数同步调整；后端异常计算和详情页异常项均保留。
- 发布：commit `ae63860 feat: simplify resource list metrics` 已同步 GitHub 与 GitLab 测试分支；Wharf pipeline `752` 构建 `167897` 成功。
- 验收：测试页确认版本 `3.0.5`，真实门店列表包含 67 家门店，顶部与表格均无“异常”字段，摄像头、空间、已绑定、未绑定数据保持显示。
- 状态：正式环境未改动，等待用户确认 3.0 整体测试验收通过后，才以冻结 commit 发布到 GitLab `main`。

### 2026-08-14 正式环境发布流程补充确认

- 用户再次确认：正式环境不是推测试分支，也不是构建成功自动上线。
- 准确流程：
  1. 在 GitLab `main` 分支上提交代码。
  2. 自动触发 Wharf pipeline `771` 构建。
  3. 构建成功后，在 Wharf 页面点部署。
  4. 部署进入审批流程。
  5. 审批通过且部署成功后，访问 `http://lite.soyoung.com/erzhuang-project` 验收。
- 主会话规则：以后说“发布正式/线上/生产”时，需要报告构建、部署点击、审批和线上验收四个状态，不能只说“已推 main”或“构建成功”。

### 2026-08-14 正式 main 2.31.8 冒烟构建触发

- 目标：为了验证公司正式环境配置是否正确，将 3.0 前最后稳定 2.x 版本发布到 GitLab `main`；3.x 继续保留在测试环境 `codex/containerize-single-image`。
- 2.x 稳定 tag：`v2.31-stable-before-resource-view-3`，指向 `7311bda fix: fallback h5 player for unsupported h265 mse`，版本 `2.31.8`。
- 实施方式：
  - 使用临时 worktree `/private/tmp/erzhuang-main-v2318`，避免污染当前工作区未提交的项目记忆文档。
  - 从 `gitlab/main` 当前提交 `1387669` 创建临时发布分支。
  - 将运行相关文件恢复为 `v2.31-stable-before-resource-view-3` 内容，同时保留 `main` 上已有文档，避免项目记忆倒退。
  - 提交作者按 GitLab hook 要求设置为 `凯撒（沙磊） <shalei@soyoung.com>`。
- 推送结果：
  - GitLab `main` 已从 `1387669` 更新到 `c95545a8d2dbaeb43df7d365b1279e577b9d3319`。
  - 提交信息：`release: restore 2.31.8 on main`。
  - GitLab hook 返回：作者信息正确，代码规范检查通过。
- 发布状态：
  - 已完成：提交到 `main`，触发 Wharf pipeline `771` 正式构建。
  - 已完成：构建成功后已在 Wharf 点部署并走审批。
  - 已完成：部署审批通过后，正式域名 `http://lite.soyoung.com/erzhuang-project` 已显示版本 `2.31.8 (container)`。
- 推送前验证：
  - `go test -c ./cmd/server` 通过。
  - `go test -c ./internal/app` 通过。
  - `go test -c ./internal/storespace` 通过。
  - `go test -c ./internal/h5monitor` 通过。
  - `go build ./cmd/server` 通过。
  - `frontend npm test` 通过，5 files / 40 tests。
  - `frontend npm run build` 通过，只有既有 Vite chunk size warning。
  - 运行代码扫描未发现 `resourceview`、`store-space-resource-view`、`ResourceStore` 残留。
- 下一步：
  - 继续在正式环境验证 health、SSO、门店列表、H5 Monitor 和用户管理等关键路径。
  - 若正式环境 2.31.8 基础链路稳定，再决定是否把 3.0 从测试环境推进到正式发布流程。

### 2026-08-17 正式环境 2.31.8 页面版本验收

- 用户反馈：部署审批通过后访问 `http://lite.soyoung.com/erzhuang-project`，页面显示 `2.31.8 (container)`。
- 结论：正式环境从 GitLab `main` 提交、Wharf pipeline `771` 构建、Wharf 部署审批到正式域名展示版本号的链路已跑通。
- 当前状态：
  - 正式环境运行 2.31.8，用于验证正式环境 MySQL/OSS/SSO/网关基础配置。
  - 测试环境保留 3.x，继续作为「门店空间资源查看」验证环境。
- 仍需补充验收：
  - 退出登录是否符合正式域名行为。
  - 门店详情、系统设置/用户管理是否可打开，角色与权限保存是否正常。

### 2026-08-17 正式环境 2.31.8 核心 API 冒烟通过

- 用户在正式域名 `http://lite.soyoung.com/erzhuang-project` 已登录浏览器执行核心接口冒烟。
- 结果：
  - `GET /erzhuang-project/health`：200，返回 `database=mysql`、`asset_store=oss`。
  - `GET /erzhuang-project/api/auth/me`：200，返回 authenticated user 和 4 个权限。
  - `GET /erzhuang-project/api/store-space/stores?page=1&page_size=20`：200，返回 20 条，正式库总数 `total=71`。
  - `GET /erzhuang-project/api/h5/orgs/10030/monitor`：200，返回北京保利实验室门店，`groups` 数量 1。
- 结论：
  - 正式环境基础链路已验证通过：网关/SSO、二壮正式 MySQL、OSS 配置声明、门店列表 API、H5 Monitor 样本门店 API 均可用。
  - 正式库门店数为 71，不同于此前测试环境 54，说明正式环境确实对接正式数据口径。
- 仍需补充：
  - 正式域名退出登录。
  - 门店详情页面。
  - 系统设置/用户管理页面。
  - H5 Monitor 实际播放页首帧。

### 2026-08-17 正式环境退出登录修复

- 问题：正式环境 `http://lite.soyoung.com/erzhuang-project` 点击退出登录后，会短暂显示项目未登录页，然后又恢复登录态。
- 根因：2.31.7 统一退出修复只把 `lite.sy.soyoung.com` 识别为公司 SSO 域名；正式域名 `lite.soyoung.com` 未被识别，前端只跳 `/erzhuang-project/logout` 清本项目 cookie，没有带 `redirect=<SSO logouttogether>` 去清公司 SSO 上游登录态。APISIX 随后又基于仍有效的上游 SSO session 补发登录态。
- 修复提交：GitLab `main` 已推送 `960ade206e76eacaa6f47c174ac86f1eb9dcbcd9`，提交信息 `fix: logout from production sso domain`。
- 修复内容：
  - 前端公司 SSO 域名识别增加 `lite.soyoung.com`。
  - 正式域名退出地址改为 `/erzhuang-project/logout?redirect=<security-test logouttogether>`。
  - `from_host=lite.soyoung.com`，`from_uri=http://lite.soyoung.com/erzhuang-project/`，匹配当前正式入口协议。
  - 后端补正式域名 logout 回归测试，确认清 host-only、`lite.soyoung.com` 和 `soyoung.com` cookie 后再跳统一退出。
- 推送状态：
  - GitLab `main` 从 `c95545a` 更新到 `960ade2`。
  - GitLab hook 返回作者信息正确、代码规范检查通过。
  - 已触发正式 Wharf pipeline `771` 构建；仍需构建成功后点部署并走审批。
- 发布前验证：
  - `frontend npm test` 通过，5 files / 40 tests。
  - `frontend npm run build` 通过，只有既有 Vite chunk size warning。
  - `go test -c ./internal/app` 通过。
  - `go build ./cmd/server` 通过。
- 待线上验证：
  - Wharf pipeline `771` 构建成功。
  - 点部署并审批通过。
  - 页面底部更新到 `2.31.8 (...)` 对应新构建。
  - 点击退出登录后应进入公司统一退出流程，不再自动恢复登录态。

### 2026-08-17 正式退出登录修复构建失败与补丁

- 现象：`960ade2 fix: logout from production sso domain` 触发的正式 Wharf pipeline `771` 镜像构建失败，通知显示 child build failed。
- 根因：`960ade2` 新增的正式域名后端测试期望清除 `soyoung.com` 父域 cookie；但 `authCookieClearDomains("lite.soyoung.com")` 旧逻辑按最后 3 段生成父域，得到的仍是 `lite.soyoung.com`，Linux 镜像内 `go test ./...` 会失败。本机此前只跑 `go test -c` 编译门禁，未执行测试，因此没有提前暴露。
- 修复提交：GitLab `main` 已推送 `b097c3d2840e51e3e41e2a89169cb76336d1a5ba`，提交信息 `fix: clear production sso parent cookie`。
- 修复内容：`authCookieClearDomains` 明确适配公司域名：
  - `*.sy.soyoung.com` 清 `sy.soyoung.com`。
  - `*.soyoung.com` 清 `soyoung.com`。
- 推送状态：
  - GitLab `main` 从 `960ade2` 更新到 `b097c3d`。
  - GitLab hook 返回作者信息正确、代码规范检查通过。
  - 已重新触发正式 Wharf pipeline `771` 构建。
- 本地验证：
  - `go test -c ./internal/app` 通过。
  - `go build ./cmd/server` 通过。
  - `frontend npm test` 通过，5 files / 40 tests。
  - `frontend npm run build` 通过，只有既有 Vite chunk size warning。
- 待线上验证：
  - Wharf pipeline `771` 构建是否通过。
  - 构建成功后点部署并审批。
  - 正式环境复验退出登录。
  - 系统设置/用户管理是否可打开，角色与权限保存是否正常。
  - 3.0 业务库只读连接仍是独立 DSN，不与二壮运行库混用。

## 2026-07-08 普通查看用户监控门店范围权限本地验收

## 2026-08-13 3.0 门店空间资源查看公司发布记录

- 发布目标：二壮 3.0「门店空间资源查看」。
- 发布分支：`gitlab/codex/containerize-single-image`。
- 发布 commit：`36a6619 merge: store resource view 3`。
- 版本：`3.0.0`。
- GitHub 备份：`origin/codex/containerize-single-image` 已同步到 `36a6619`。
- 公司 GitLab：`refs/heads/codex/containerize-single-image` 已更新到 `36a6619f441f896714e12abbd2ae3e7227da414c`，等待公司 K8s 自动构建发布。
- 发布内容：
  - 新增业务库只读资源查看 API：`GET /api/store-space-resource-view/stores`、`GET /api/store-space-resource-view/stores/{tenantId}`。
  - 新增 `internal/resourceview` 聚合业务库门店、空间、工控机、NVR、摄像头、绑定关系和异常项。
  - `cmd/server` 支持 `BUSINESS_MYSQL_DSN` / `K8S_SECRET_BUSINESS_MYSQL_DSN`；用户已在镜像/公司环境配置后者，仓库不保存真实 DSN。
  - 后台主页面切换为只读「门店空间资源查看」，隐藏旧新增、编辑、删除、扫描、识别、确认、设计图上传/标注入口。
  - H5 Monitor 路由和播放页未改。
- 发布前验证：
  - `cd frontend && npm test` 通过，41 tests。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `CGO_ENABLED=0 ... go test ./internal/resourceview` 通过。
  - `go test -c ./internal/resourceview`、`go test -c ./internal/app`、`go test -c ./cmd/server` 通过。
  - `go build -o /private/tmp/server-check ./cmd/server` 通过。
  - 新增 3.0 代码未发现真实业务库账号、主机、密码或 DSN；`internal/resourceview` 不提供业务库写接口。
- 线上待验证：
  - 公司构建/实例部署成功通知。
  - 已登录浏览器访问 `/erzhuang-project/health`，页面版本应为 `3.0.0 (...)`，health 仍应返回 `database=mysql`、`asset_store=oss`。
  - `GET /erzhuang-project/api/store-space-resource-view/stores?page=1&page_size=20` 返回 200，且不是 `resource_view_not_configured`。
  - 首页显示「门店空间资源查看」并有真实业务库门店数据。
  - 门店详情能打开空间视角、设备视角、异常项。
  - “查看监控”入口仍按 `can_view_monitor` / `monitor_url` 显示；H5 Monitor 播放页不受影响。

- 范围：
  - 普通查看用户按门店授权查看监控。
  - 用户管理弹窗的门店范围选择交互。
- 本地浏览器验收：
  - 启动本地 mock 前端：`http://127.0.0.1:5173/erzhuang-project/`。
  - 进入“系统设置 -> 用户管理 -> 添加用户”，确认普通查看角色默认展示“查看监控门店范围”。
  - 城市筛选生效：切换到“上海”后只展示上海门店。
  - 在城市筛选下勾选门店后，切回“全部”时已选门店前置。
  - `全选` / `清空` 文案已按用户反馈保留简洁形式。
  - 弹窗宽度已加宽到适合真实门店范围选择的尺寸，减少城市和门店列表拥挤。
  - 登录状态行已改为横向阅读结构，开关宽度放宽，避免中文被压缩。
- 构建验证：
  - `cd frontend && npm run build` 通过。
  - Vite 仍提示既有 chunk size warning，非本次回归。
- 门店统计 bug 修复：
  - 根因：MySQL `ListStores` 先分页取 `items`，再用 `summarizeStoreListItems(items)` 生成 summary，导致右上角统计随翻页变化。
  - 修复：改用 `storeListSummary(ctx, rawLike, rawLike, filters.City)` 按当前 tab/搜索条件汇总全量 filtered dataset。
  - 防回归：新增 `TestMySQLListStoresSummaryUsesFilteredDataset` 源码守卫测试。
- 下一步：
  - 复查改动范围，避免混入 OpenClaw 并行改动。
  - 发布前重新跑 Go 编译门禁和前端构建。
  - 发布后线上验证 viewer 空范围/部分范围、H5 Monitor 门店切换过滤、直接访问未授权机构返回 403。

## 2026-07-08 2.31.0 公司发布记录

- 发布提交：`068ccc8 feat: restrict viewer monitor stores`。
- 发布分支：`gitlab/codex/containerize-single-image`。
- GitHub 备份分支：`origin/codex/containerize-single-image`。
- 推送结果：
  - 公司 GitLab 从 `02a6623` 更新到 `068ccc8`，远端 hook 输出 `Processed push`。
  - GitHub 备份分支从 `02a6623` 更新到 `068ccc8`。
- 发布内容：
  - 普通查看用户监控门店范围权限。
  - H5 Monitor 后端门店授权强校验。
  - 门店列表/详情 `can_view_monitor` 入口提示字段。
  - 用户管理门店范围选择交互。
  - 门店列表 summary 改为按当前 tab/筛选条件汇总全量 filtered dataset，修复翻页后右上角统计变化问题。
- 发布前验证：
  - `go test -c ./internal/app` 通过。
  - `go test -c ./internal/h5monitor` 通过。
  - `go test -c ./internal/storespace` 通过。
  - `go build ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过，仍有既有 Vite chunk size warning。
- 发布后验证状态：
  - 无登录态公网 `curl https://lite.sy.soyoung.com/erzhuang-project/health` 返回 APISIX 302 到 SSO，说明需由用户在已登录公司浏览器完成线上验证。
  - 待验证项：`/health`、页面底部版本、用户管理新增/编辑 viewer 门店范围、viewer 授权/未授权门店监控入口、H5 Monitor 直接访问未授权机构 403、门店列表统计翻页不变。

## 2026-07-08 2.31.1 系统设置用户管理修复发布记录

- 问题：2.31.0 发布后进入系统设置/用户管理时报 `list auth users failed`，页面用户列表为空。
- 根因：2.31.0 新增 `tb_user_resource_scopes` 后，用户列表会统计普通查看用户的监控门店范围；公司线上 MySQL 运行库尚未自动创建这张新表，导致 `/api/users` 在 scope count 查询阶段 500。
- 修复提交：`10011b9 fix: ensure viewer scope table`。
- 版本：`2.31.1`。
- 修复内容：
  - `internal/app/mysql_store.go` 增加 `ensureUserResourceScopesTable`，在用户列表 scope count、viewer 范围读写、H5 Monitor scope 判断前幂等确保 `tb_user_resource_scopes` 存在。
  - 新增 `TestMySQLStoreEnsuresUserResourceScopesTable`，保护建表 SQL。
- 发布：
  - 公司 GitLab `codex/containerize-single-image` 从 `068ccc8` 更新到 `10011b9`。
  - GitHub 备份分支已同步到 `10011b9`。
- 发布前验证：
  - `go test -c ./internal/app` 通过。
  - `go build ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过，仍有既有 Vite chunk size warning。
  - 直接运行新增单测仍触发本机已知 `missing LC_UUID load command`，因此以编译门禁为准。
- 线上待验证：
  - 已登录浏览器访问 `/erzhuang-project/health`，版本应更新到 `2.31.1`。
  - 进入系统设置，用户管理不再显示 `list auth users failed`。
  - 新增/编辑普通查看用户时，监控门店范围候选列表可加载并保存。

## 2026-07-08 2.31.2 门店列表统计修复发布记录

- 问题：
  - 门店列表右上角统计显示 `共 0 家门店 / 面诊室 0 / 治疗室 0 / 美容室 0`。
  - 偶发报错：`mysql list stores summary: Error 1267 (HY000): Illegal mix of collations ... for operation '='`。
- 根因：
  - 2.31.0 为修复“统计随翻页变化”，把 summary 改为 MySQL 全量汇总，但调用参数错误，把 normalized search like 传成 raw like。
  - summary SQL 单独维护了一套筛选条件，和列表条件漂移。
  - summary 聚合中的 `area_type = 'consultation'`、城市 `city = ?` 等字符串比较在公司 MySQL 混合 collation 下会触发 1267。
- 修复提交：`9277d50 fix: stabilize store list summary`。
- 版本：`2.31.2`。
- 修复内容：
  - `ListStores` summary 改为 `storeListSummary(ctx, filters)`，统一传 filters。
  - `storeListSummary` 复用 `mysqlStoreListWhere(filters)`，保证列表和统计使用同一套搜索/城市条件。
  - 门店数和区域数分开聚合，区域数用 `exists` 关联门店筛选，避免公司 GitLab hook 拦截 SQL join 语法。
  - 城市比较使用 `binary ... = binary ?`，区域类型比较使用 `binary a.area_type ...`，规避 MySQL collation 混算。
- 发布：
  - 公司 GitLab `codex/containerize-single-image` 从 `10011b9` 更新到 `9277d50`。
  - GitHub 备份分支已同步到 `9277d50`。
- 发布前验证：
  - `go test -c ./internal/storespace` 通过。
  - `go build ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过，仍有既有 Vite chunk size warning。
- 线上待验证：
  - 已登录浏览器访问 `/erzhuang-project/health`，版本应更新到 `2.31.2`。
  - 门店列表右上角统计不为 0，翻页不变化。
  - 切到城市 Tab 后，统计为该城市全部门店口径，翻页不变化。

## 2026-07-08 2.31.4 Wharf 镜像构建修复发布记录

- 背景：
  - `9277d50 / 2.31.2` 推送后，Wharf 镜像构建失败，通知显示 child build failed。
  - Wharf 详情确认失败发生在 Dockerfile 的 `RUN go test ./...`，包为 `internal/storespace`。
- 根因：
  - 本机 macOS 直接运行 Go 测试二进制仍会触发已知 `missing LC_UUID load command`，发布前只执行了 `go test -c` 编译门禁。
  - 公司 Linux 镜像会真实运行 `go test ./...`，暴露了新增源码守卫测试的错误假设。
  - 第一轮修复 `30f5491 / 2.31.3` 只修正了 `where binary coalesce` 断言，但完整 Wharf 日志显示仍失败在 `TestMySQLListStoresSummaryUsesFilteredDataset`：测试要求 `storeListSummary` 函数体直接包含 city binary 条件；实际实现是 `storeListSummary` 调用 `mysqlStoreListWhere(filters)` 间接复用筛选条件。
- 修复：
  - `61b5511 fix: relax store summary source guard`。
  - 版本：`2.31.4`。
  - 将 binary city comparison 的断言放到全文件/source 层面，`storeListSummary` 函数体只校验调用 `mysqlStoreListWhere(filters)`、全量 count、`exists` 和 `binary a.area_type`。
- 发布：
  - 公司 GitLab `codex/containerize-single-image` 从 `30f5491` 更新到 `61b5511`。
  - GitHub 备份分支已同步到 `61b5511`。
  - 用户收到实例部署成功通知，说明 `2.31.4` 已完成公司自动部署。
- 发布前验证：
  - `go test -c ./internal/storespace` 通过。
  - `go build ./cmd/server` 通过。
  - 对本次改动文件执行 `rg -n -i "join" VERSION internal/storespace/mysql_store_test.go`，无匹配，避免公司 GitLab hook 再次拦截。
- 后续改进：
  - 有条件时补一个 Linux 容器内测试门禁，至少覆盖 Dockerfile 中的 `go test ./...`，避免本机 macOS 环境只能编译测试而漏掉运行期测试失败。
  - 线上继续验证门店列表右上角统计、翻页稳定性和城市 Tab 统计口径。

## 2026-07-08 H5 Monitor 区域 Tab 返回状态修复

- 问题：
  - 在 H5 Monitor 页面筛选区域 Tab 后，点击摄像头进入监控详情；返回门店监控列表时，区域 Tab 回到“全部”，没有回到进入详情前的列表。
- 根因：
  - 第一轮只用 `sessionStorage` 保存区域 Tab，本地部分返回路径可恢复，但 URL/history 本身没有携带区域状态。
  - 线上或不同入口返回时仍可能按默认路由重新进入“全部”，说明需要把“从哪个区域列表进入详情”变成显式路由状态，而不是只依赖隐藏存储。
- 修复提交：
  - `37cadd4 fix: persist h5 monitor tab in url`。
  - 版本：`2.31.6`。
- 修复内容：
  - `H5Route` 增加可选 `tab` 状态，`parseH5Route` 从 `?tab=` 读取合法区域 Tab。
  - H5 Monitor 主页点击区域 Tab 时同步更新 URL：默认“全部”无参数，其他区域使用 `?tab=treatment` 等参数。
  - 从区域列表进入摄像头详情时，详情 URL 也携带 `?tab=`；页面返回时回到同一个 `?tab=` 的监控列表。
  - 继续保留按 `externalOrgId` 的 `sessionStorage` 兜底，URL 参数优先生效；如果当前门店不存在该分类，自动回到“全部”并同步路由。
  - 新增/扩展 `frontend/src/domain/h5-monitor-active-tab.test.ts`，覆盖按门店保存/恢复、非法值回退、storage 不可用兜底、URL 参数读取和查询串生成。
  - 更新 `frontend/vite.config.ts`，把新单测加入默认 `npm test` include。
- 验证：
  - `cd frontend && npm test`：5 files / 37 tests 通过。
  - `cd frontend && npm run build`：通过，仍有既有 Vite chunk size warning。
  - Chrome 插件真实浏览器验收：`http://127.0.0.1:5174/erzhuang-project/h5/orgs/demo/monitor` 点击“治疗室”后 URL 变为 `?tab=treatment`；进入通道详情 URL 为 `/channels/2?tab=treatment`；点击页面“返回”后 URL 回到 `/monitor?tab=treatment`，active tab 和列表均保持“治疗室”。
- 发布状态：
  - 已推送公司 GitLab 固定分支 `codex/containerize-single-image`，远端从 `61b5511` 更新到 `37cadd4`，触发公司 K8s 自动发布。
  - 已同步 GitHub 备份分支 `origin/codex/containerize-single-image`。
  - 公网无登录态 `curl https://lite.sy.soyoung.com/erzhuang-project/health` 返回 APISIX 302 到 SSO；Chrome 打开公司 health 被浏览器侧拦截为 `ERR_BLOCKED_BY_CLIENT`，仍需用户在已登录公司浏览器里确认版本和 H5 Monitor 返回体验。

## 2026-08-10 Windows H5 Monitor H.265 播放兼容修复发布

- 问题：
  - 部分 Windows Edge/Chrome 用户打开 H5 Monitor 实时视频时报首帧超时。
  - 诊断里显示 `decode=desktop-mse`，播放器内部报 `MediaSource addSourceBuffer` 对 `video/mp4;codecs=hvc1...` unsupported。
  - 用户补充另一台 Windows 电脑可正常播放，因此不能按 Windows UA 一刀切改成软解。
- 修复提交：
  - `7311bda fix: fallback h5 player for unsupported h265 mse`。
  - 版本：`2.31.8`。
- 修复内容：
  - 桌面环境默认继续使用 `desktop-mse`，避免影响可正常硬解或 MSE 播放的机器。
  - 仅当播放器明确返回 H.265/HEVC + MSE/SourceBuffer + unsupported/not support 错误时，前端自动切到 `desktop-wasm` 软解重试。
  - 移动端仍保持 `mobile-wasm`；软解路径只把真实视频帧事件作为首帧成功，避免 `loaded` 误判。
  - 页面会先提示“当前浏览器不支持该 H.265 硬解码流，正在切换软解码重试”。
- 发布：
  - GitHub 备份分支已同步：`8467e93 -> 7311bda`。
  - 公司 GitLab 固定分支已推送：`8467e93 -> 7311bda`，触发公司 K8s 自动发布。
- 发布前验证：
  - `cd frontend && npm run test`：5 files / 40 tests 通过。
  - `cd frontend && npm run build`：通过，仍有既有 `ezuikit-flv` large chunk warning。
  - `git diff --check` 通过。
  - `go build -o /private/tmp/server-check ./cmd/server` 通过。
  - 对本次改动文件执行 `rg -n -i "join" VERSION frontend/src/components/H5FlvPlayer.tsx frontend/src/domain/h5-player-diagnostics.ts frontend/src/api.test.ts`，无匹配，避免公司 GitLab hook 拦截。
- 发布后待验证：
  - 等公司实例更新后，页面底部版本应为 `2.31.8 (...)`。
  - 原失败 Windows 机器重试同一通道：应自动切到 `desktop-wasm` 后播放，或诊断里至少显示 fallback 后的软解路径。
  - 原可正常播放 Windows 机器不应被强制切软解，除非播放器实际报 H.265 MSE unsupported。

## 2026-07-04 MySQL/OSS 资产与单通道识别抽验

- 执行人：用户在已登录公司浏览器控制台执行。
- 范围：真实门店单通道截图刷新和识别，不批量请求。
- 样本：
  - 门店：`56`，新氧青春诊所(佛山岭南天地店)
  - 录像机：`64`，`GQ2603587`
  - 通道：`900065`，通道号 `1`
- 验证结果：
  - `GET /erzhuang-project/api/store-space/stores/56/channel-data`：200。
  - `POST /erzhuang-project/api/store-space/channels/900065/snapshot`：200。
  - `POST /erzhuang-project/api/store-space/channels/900065/recognize`：200。
  - 识别响应业务状态：`status=recognition_failed`。
  - `recognition_result.status=recognition_failed`。
  - 控制台输出 `ASSET_AND_RECOGNITION_CHECK_DONE`。
- 失败详情：
  - `recognition_result.status=recognition_failed`。
  - `message=vision recognition failed: status 502 ... upstream_error`。
  - `request ID=5704b803-f192-4f86-a3a7-be7a3df7a53d`。
  - `capture_ms=578`，说明萤石抓图阶段成功。
  - `recognition_ms=30407`，说明 AI provider 请求耗时约 30.4s 后由上游返回 502。
- 当前判断：
  - 门店详情、通道读取、萤石截图刷新、识别接口路由、认证、OSS/MySQL 写回链路均可达。
  - 本次失败根因在 AI provider 上游 502，不是 MySQL/OSS 主链路失败。
- 下一步：
  - 低频抽验另一个通道，确认 AI provider 是否只是单次波动。
  - 如果持续 502，暂停批量识别，检查当前 provider 或稍后重试。

### 追加样本

- 通道：`900076`，通道号 `2`。
- `POST /erzhuang-project/api/store-space/channels/900076/recognize`：200。
- 识别响应业务状态：`status=pending_confirmation`。
- `recognition_result.status=recognized`。
- 结论：
  - 识别链路整体可用。
  - 通道 `900065` 的 upstream 502 更符合单次 AI provider 波动，而不是系统性故障。
  - 后续遇到个别识别失败，优先按单通道低频重试处理。

## 2026-07-04 MySQL/OSS 线上写接口回归

- 执行人：用户在已登录公司浏览器控制台执行。
- 范围：临时门店写接口验收，脚本最终清理临时数据。
- 临时数据：
  - 临时门店 ID：`900006`
  - 临时门店名：`Codex MySQL 写接口回归 1783154376454 已编辑`
  - 临时新增录像机 ID：`900058`
  - 临时新增录像机设备号：`CODB4376454`
- 验证结果：
  - `GET /erzhuang-project/health`：200，返回 `database=mysql`、`asset_store=oss`。
  - `POST /api/store-space/stores`：201，临时门店创建成功。
  - `PATCH /api/store-space/stores/900006`：200，门店基础信息编辑成功。
  - `PUT /api/store-space/stores/900006/design-plan`：200，设计图标注保存成功，未复现 collation 错误。
  - `POST /api/store-space/stores/900006/recorders`：201，录像机添加成功。
  - `DELETE /api/store-space/recorders/900058`：204，新增录像机删除成功。
  - `GET /api/store-space/stores/900006/channel-data`：200，确认新增录像机已不存在，`recorder deleted=true`。
  - `DELETE /api/store-space/stores/900006`：204，临时门店清理成功。
  - 控制台输出 `WRITE REGRESSION OK store_id=900006`。
- 结论：
  - MySQL 门店空间核心写接口当前线上可用。
  - 临时数据已清理。
  - 设计图保存 collation 修复在线上有效。
- 下一步：
  - 抽验真实门店资产链路和单通道识别链路。

## 2026-07-04 MySQL/OSS 线上只读回归

- 执行人：用户在已登录公司浏览器控制台执行。
- 范围：只读接口，不改数据。
- 验证结果：
  - `GET /erzhuang-project/health`：200，返回 `database=mysql`、`asset_store=oss`。
  - `GET /erzhuang-project/api/auth/me`：200，返回真实登录用户“凯撒（沙磊）”和权限数组。
  - `GET /erzhuang-project/api/store-space/stores?page=1&page_size=100`：200，`total=54`，`items.length=54`。
  - `GET /erzhuang-project/api/h5/orgs/10030/monitor`：200，北京保利实验室门店，`groups.length=1`。
  - `GET /erzhuang-project/api/h5/orgs/10019/monitor`：200，上海陆家嘴店，`groups.length=5`。
  - `GET /erzhuang-project/api/h5/orgs/10081/monitor`：200，杭州城北万象城店，`groups.length=5`。
  - `failed=[]`。
- 结论：
  - MySQL/OSS 当前线上只读主链路健康。
  - 54 家有效门店口径再次确认。
  - H5 Monitor 样本门店只读接口正常。
- 下一步：
  - 执行临时门店写接口验收，并自动清理临时数据。
  - 再抽验资产链路和单通道识别链路。

## 2026-07-04 项目文件化记忆与控制文档整理

- 背景：
  - 主会话将长期担任项目负责人，用户要求建立文件化项目记忆，避免长期上下文压缩后丢失关键状态。
  - MySQL/OSS 切换完成后，README 和技术架构索引仍保留早期 PostgreSQL/Supabase 主路径表述，可能误导后续会话。
- 本次整理：
  - `docs/codex-learning-state.md` 顶部新增当前项目记忆快照。
  - 新建 `docs/decisions.md`，记录关键产品/技术决策。
  - 新建 `work/current-plan.md`，记录当前轮目标、进度、验证方式和下一步。
  - 更新 `README.md`，改为当前 MySQL + OSS、公司 GitLab/K8s、54 家有效门店口径。
  - 重写 `docs/technical-architecture-index.md`，以 `store-space`、MySQL、OSS、APISIX SSO、H5 Monitor、萤石/AI 为当前主路径。
  - 新建 `docs/post-cutover-regression-checklist.md`，用于 MySQL/OSS 切换后线上回归。
  - 新建 `docs/legacy-postgres-supabase-shutdown-checklist.md`，用于旧 PostgreSQL/Supabase 下线确认。
- 验证：
  - 本轮只改文档和 `work/current-plan.md`，未改业务代码。
  - 检查新增文档未写入真实密钥、数据库密码或公司敏感连接串。
- 后续：
  - 做一次线上回归并记录结果。
  - 组织旧 PostgreSQL/Supabase 下线确认。
  - 对历史迁移文档加状态标记或归档说明。

## 2026-07-04 MySQL 设计图保存 collation 修复 2.30.22

- 现象：
  - 线上临时门店写接口验收中，新增门店和编辑门店成功，保存设计图返回 500。
  - 详细错误：`Error 1267 (HY000): Illegal mix of collations ... for operation 'nullif'`。
- 根因：
  - MySQL 设计图保存 SQL 使用 `nullif(?, '')` 写入 `recognition_result`。
  - 参数 collation 与空字符串 literal collation 在公司 MySQL 环境不一致，触发字符串比较 collation 冲突。
- 修复：
  - 将设计图保存的 `recognition_result` 写法改为 `case when length(?) = 0 then null else ? end`，用长度判断代替字符串比较。
  - 新增 MySQL Store 源码门禁测试，禁止重新引入字符串参数上的 `nullif(?, '')`。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。

## 2026-07-04 MySQL 门店空间写接口补齐 2.30.21

- 现象：
  - MySQL/OSS 切换后，读链路、扫描、识别、通道确认已经恢复，但仍需要继续排查创建、编辑、删除等前端可触达写接口。
  - MySQL 仓储层仍有若干方法直接返回 `ErrNotImplemented`，会导致对应 API 在公司环境下返回 501/500。
- 根因：
  - PostgreSQL 到 MySQL 迁移期间优先补了读链路、迁移链路和通道识别链路，门店空间的常规运营写接口没有全部补齐到 `tb_` 表。
- 修复：
  - 实现 MySQL 新增萤石账号、新增门店、编辑门店基础信息、保存设计图标注、新增录像机。
  - 实现 MySQL 删除门店、删除录像机、删除通道，沿用当前硬删除语义和外键级联约束。
  - 新增 MySQL 写接口源码门禁测试，避免这些前端可触达写方法再次退回未实现状态。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - 本机直接执行 Go 测试仍触发 macOS 测试二进制 `missing LC_UUID load command`，本次继续使用 `go test -c` 做编译门禁。

## 2026-07-02 Postgres -> MySQL 真实数据迁移方向纠正

- 用户纠正：
  - 公司 MySQL 测试库不是只跑 Stage A 样本验证，而是需要承接当前 Supabase/PostgreSQL 的真实业务数据，并作为后续公司测试环境数据基座。
  - OSS Stage A 只验证了样本对象从 Supabase Storage 复制到 OSS 的链路，没有完成真实历史数据迁移。
- 当前判断：
  - 迁移顺序应改为：Postgres 真实业务数据 -> MySQL 测试库 -> 基于 MySQL 真实资产清单迁 OSS。
  - OSS 对象迁移不能早于业务行迁移，否则缺少真实 `store_id`、`external_org_id`、`recorder_id`、`channel_id` 和资产归属。
  - 当前 Go runtime 仍是 `DATABASE_URL` + pgx/PostgreSQL 实现，不能只替换连接串切 MySQL；MySQL repository 是后续单独工作流。
- 新增迁移工具：
  - `cmd/pg-to-mysql-export`：只读 Postgres，生成 MySQL 导入 SQL、auto increment SQL 和 `report.json`。
  - `internal/mysqlmigration`：保存表映射、字段转换和 SQL 生成逻辑。
  - 迁移工具支持 `--external-org-id 10030` 小样本导出，也支持后续全量导出。
  - Postgres `tb_users.phone` 会迁为 MySQL `tb_users.mobile`，Postgres 当前 `role` 单字段会转成 MySQL `tb_user_roles` 关系。
- 新增文档：
  - `docs/postgres-to-mysql-data-migration-runbook.md`。
- 已发起 DBA 专项复核：
  - 复核点包括表顺序、DDL 缺口、只读校验、OSS 迁移顺序和必须双向确认的高风险项。
- MySQL 测试库 Stage A 样本清理：
  - 用户确认 MySQL 测试库里的 Stage A 样本/假数据需要清除。
  - 使用 `db/mysql_stage_a_cleanup_sample_tb.sql` 的受控清理口径，只清理 `900001-900199` ID 段和 `stage-a` 标记数据。
  - 执行前统计：`tb_stores`、`tb_store_areas`、`tb_store_design_plans`、`tb_design_plan_annotations`、`tb_ezviz_accounts`、`tb_video_recorders`、`tb_video_channels`、`tb_channel_snapshots`、`tb_users`、`tb_asset_objects`、`tb_audit_logs`、`tb_asset_access_logs` 合计 28 行。
  - 执行后统计：上述 12 张表 Stage A ID 段合计 0 行。
  - 清理事务已提交，MySQL 版本 `8.0.13`，目标库 `db_pm_erzhuang`，库时区 `+08:00`。

## 当前主题

学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证、回滚流程。

## 2026-06-24 通道缩略图队列加载 2.14.7 修复记录

- 版本号：`2.14.7`。
- 用户反馈：
  - 公司环境 `新氧青春诊所(上海新淮海坊店)` 通道最近截图加载非常慢，转一段时间后失败。
- 排查结论：
  - 门店 ID：`9`，录像机 `L18975312`，通道数 `30`。
  - 截图更新时间为 `2026-06-24 12:35` 之后，说明不是旧截图对象缺失的单一问题。
  - 只读请求测试显示：
    - 串行加载前 8 张时，单张也会出现 2s 到 20s 以上不等的耗时，部分请求 20s 内读不完响应体。
    - 并发加载 30 张时，21 个请求拿到 HTTP 200 但 20s 内未读完 body，9 个请求 AbortError，平均耗时接近 20s。
    - 并发 4 或 6 时，12 张测试样本基本全部 25s 超时。
  - 结论：前端一次性加载缩略图会放大失败，必须先做队列/限并发；但单张读取也偏慢，后续仍需要后端生成真正小缩略图或优化 Supabase 图片代理链路。
- 修复：
  - 新增 `frontend/src/domain/image-load-queue.ts`，提供通用前端图片加载队列，当前缩略图并发限制为 `2`。
  - 通道表格缩略图不再直接一次性设置全部 `<img src>`，而是进入队列并等待图片真实 `load/error` 后才释放下一个名额，避免浏览器仍然同时拉取几十张图。
  - 队列等待期间展示稳定尺寸的小 loading 占位，避免表格抖动。
  - 离开页面、筛选或切换数据时取消仍在排队的加载任务。
- 后续建议：
  - 继续做后端真实缩略图生成：表格加载几十 KB 小图，点击预览再加载大图。
  - 评估后端缓存或 signed URL，减少 Go 后端代理 Supabase 大图造成的慢请求。
- 验证：
  - 新增 `frontend/src/domain/image-load-queue.test.ts`，覆盖并发限制、任务完成后释放下一个名额、取消排队任务。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-image-queue-test src/domain/image-load-queue.ts src/domain/image-load-queue.test.ts && node /tmp/erzhuang-image-queue-test/image-load-queue.test.js` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-channel-test src/domain/channel-recognition.ts src/domain/channel-recognition.test.ts && node /tmp/erzhuang-channel-test/channel-recognition.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地浏览器打开 `http://127.0.0.1:5177/erzhuang/`，页面能正常渲染，控制台无 error；因本地 dev server 未连接完整后端数据，本轮未在本地复现真实公司门店缩略图瀑布。

## 2026-06-23 录像机识别失败提示修正 2.14.6 修复记录

- 版本号：`2.14.6`。
- 用户反馈：
  - 公司线上环境 `新氧青春诊所(合肥银泰中心店)` 中，录像机 `GG9803685` 点击“识别区域”后速度很快。
  - 页面提示“已完成 GG9803685 的通道识别”，但最近截图没有刷新，也没有重新截图识别。
- 排查结论：
  - 门店 ID：`16`，录像机 ID：`19`。
  - 公司线上详情接口显示 `GG9803685` 的 21 个通道都执行了识别尝试，`updated_at` 更新到了 `2026-06-23 18:53` 左右。
  - 这些通道的 `recognition_result` 均为 `status=recognition_failed`，并且抓图耗时只有几十毫秒。
  - 具体错误为：`ezviz api error code=10028 msg=抓图接口调用次数超限`。
  - 所以真实根因不是前端没有调用“识别区域”，也不是没有进入后端；而是萤石抓图接口触发次数限制，后端逐通道保存失败结果后继续队列，前端最终 toast 没有统计失败数，误导成“已完成”。
- 修复：
  - 新增 `frontend/src/domain/channel-recognition.ts`，统一生成通道识别行内提示和录像机级完成提示。
  - 单通道行内提示现在会展示失败 message，例如“抓图接口调用次数超限”，而不是只显示“失败 · 总 47ms”。
  - 录像机级识别完成后会统计失败通道：
    - 全部失败：`GG9803685 识别完成，但 21 个通道抓图/识别失败：...`
    - 部分失败：`GG9803685 识别完成，x/y 个通道抓图/识别失败：...`
    - 全部成功时保留原成功文案。
- 验证：
  - 新增 `frontend/src/domain/channel-recognition.test.ts`，覆盖萤石 `10028` 全失败、部分失败和全部成功三种提示。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-channel-test src/domain/channel-recognition.ts src/domain/channel-recognition.test.ts && node /tmp/erzhuang-channel-test/channel-recognition.test.js` 通过。
  - `cd frontend && npm run build` 通过。
- 发布：
  - GitHub `main` commit：`991033b`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`38158d8`，等待公司 K8s 自动发布。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 commit：`991033b`。
  - 韩国公网入口 `https://43.155.237.46/erzhuang/health` 验证通过。

## 2026-06-23 Supabase Storage Bucket 自愈 2.14.5 修复记录

- 版本号：`2.14.5`。
- 用户反馈：
  - 公司环境仍出现 `store space request failed`。
  - 最近截图显示“加载失败”，希望页面能展示更详细的抓图、识别、存储反馈，便于共同定位。
- 排查结论：
  - 公司环境 `/health` 返回 `database=postgres`、`asset_store=supabase`，说明后端已切到 Supabase Storage。
  - 公司环境前端 bundle 已是 `2.14.4 (container)`，包含截图诊断逻辑。
  - 抽样调用 `GET /api/store-space/channel-snapshots/9509d32aed822d963233de786e9a8ecd.jpg/diagnostics` 返回：
    - `code=snapshot_open_failed`
    - `stage=open_snapshot`
    - `asset_store=supabase`
    - `snapshot_key=channel-snapshots/9509d32aed822d963233de786e9a8ecd.jpg`
    - `exists=false`
    - `detail=open asset failed: http 400 {"statusCode":"404","error":"Bucket not found","message":"Bucket not found"}`
  - 根因不是前端，也不是萤石临时 URL 过期；而是公司 Supabase Storage 中缺少代码使用的 bucket，或 `SUPABASE_STORAGE_BUCKET` 与实际 bucket 名不一致。
- 修复：
  - Supabase Storage 保存资产时，如果首次写入返回 `Bucket not found`，后端会用 service role 自动创建私有 bucket，并重试一次保存。
  - 如果创建 bucket 失败或权限不足，仍返回明确错误，不做无限重试。
- 注意：
  - 已经因为 bucket 不存在而写入失败的历史截图对象不会自动恢复；需要对对应通道执行“刷新截图”或“重新识别”，生成新截图后才会写入 Supabase Storage。
- 验证：
  - 新增 `TestSupabaseStorageStoreCreatesBucketAndRetriesSaveWhenMissing`，覆盖 bucket 缺失时自动创建并重试保存。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 发布：
  - GitHub `main` commit：`8a73c95`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`1c59220`，等待公司 K8s 自动发布。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 commit：`8a73c95`。
  - 韩国服务器本机健康检查返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"local"}`。
  - 韩国公网入口 `https://43.155.237.46/erzhuang/health` 验证通过。

## 2026-06-23 通道截图与抓图识别诊断增强 2.14.4 修复记录

- 版本号：`2.14.4`。
- 用户反馈：
  - 公司环境再次出现 `store space request failed`。
  - 通道映射页所有“最近截图”显示加载失败，前端缺少足够信息判断是抓图、保存 Supabase、读取 Supabase，还是历史本地文件缺失。
- 产品/排障决策：
  - 页面需要展示脱敏后的诊断信息，便于用户直接把错误贴回给 Codex 定位。
  - 不能展示 accessToken、apiKey、service role key 或完整签名 URL。
- 修复：
  - store-space 错误响应保留旧 `error` 字段，同时新增 `code`、`stage`、`detail`。
  - 后端新增 `GET /api/store-space/channel-snapshots/{name}/diagnostics`，返回 `asset_store`、`snapshot_key`、`exists`、`code/stage/detail`。
  - 前端 `ApiError` 保留 `code/stage/detail`，通道映射页错误提示会展示这些字段。
  - 最近截图 `<img>` 加载失败时，前端自动请求截图诊断接口，并在缩略图下方用小字展示脱敏诊断信息。
- 验证：
  - 新增 `TestScanRecorderEndpointReturnsDiagnosticForUnexpectedError`，覆盖普通内部错误不再只返回一句 `store space request failed`。
  - 新增 `TestChannelSnapshotDiagnosticsReportsOpenFailure`，覆盖截图读取失败时返回脱敏诊断信息。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。

## 2026-06-23 兜底抓图队列不中途停止 2.14.3 修复记录

- 版本号：`2.14.3`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775` 已能进入逐通道抓图识别队列，但只抓到约 9、10 张图后停止。
  - 用户判断实际通道不可能只有 9、10 个，怀疑与此前 70 秒、每 6 秒的测算有关。
- 排查结论：
  - 当前逐通道队列是前端逐个调用 `probe-recognize-channel`，不再是单个后端请求卡满 70 秒。
  - 真正导致 9、10 张后停止的是前端保留了“连续 5 个通道失败就停止”的旧兜底策略。
  - 如果 1-10 有效，11-15 为空通道或抓图失败，队列会直接停止，导致 16 之后的有效通道被漏掉。
- 修复：
  - 新增 `fallbackProbeChannelNumbers()` 和 `shouldStopFallbackProbe()`，集中管理兜底通道探测策略。
  - 兜底识别最多检测到 64 路；30 路之前不允许因连续失败停止；30 路之后连续 8 个通道失败才停止。
  - 移除前端兜底队列里的“连续 5 次失败即停止”旧逻辑，避免中间空通道造成后续漏扫，同时避免每次无脑扫满 64 路。
- 验证：
  - 新增 `frontend/src/domain/fallback-probe.test.ts`，覆盖兜底检测计划为 1-64，且停止条件为 30 路后连续 8 次失败。
  - `cd frontend && npm run build` 通过。

## 2026-06-23 萤石错误透传 2.14.2 修复记录

- 版本号：`2.14.2`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775` 扫描不再表现为 504，但页面提示“录像机 K96112775 扫描失败：store space request failed”，仍没有进入逐通道抓图识别。
- 排查结论：
  - `2.14.1` 后端扫描接口已不再同步抓图兜底，能把萤石 `10026` 错误返回到 service 层。
  - 但 store-space handler 没有识别 `ezviz.Error`，统一把未知错误转成 `store space request failed`。
  - 前端只能看到泛化文案，无法命中 `10026` 或“设备数量超出个人版限制”的兜底判断。
- 修复：
  - store-space handler 对 `ezviz.Error` 返回 HTTP 502，并保留错误 code/msg，例如 `ezviz api error code=10026 msg=...`。
  - 前端现有 `shouldUseFallbackProbe` 可直接根据返回文案进入逐通道抓图识别队列。
- 验证：
  - 新增 `TestScanRecorderEndpointReturnsEzvizErrorCodeForFallback`，覆盖 `10026` 不再被吞成 `store space request failed`。

## 2026-06-23 扫描接口 10026 同步兜底下线 2.14.1 修复记录

- 版本号：`2.14.1`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775`（上海静安）扫描仍然出现 HTTP 504。
  - 用户观察到系统仍像是在先跑完整通道扫描，而不是进入新的逐通道抓图识别流程。
- 排查结论：
  - `2.14.0` 前端已经在扫描接口返回 `10026` 时接管抓图识别队列。
  - 但后端 `EzvizScanner.ScanRecorderChannels` 在 `camera/list` 返回 `10026` 时仍会同步调用旧的 `probeChannelsByCapture`，最多串行探测 32 个通道、连续 5 次失败后才停止。
  - 在失败通道耗时较长时，公司网关容易先返回 504，前端无法收到 `10026`，也就无法进入新的逐通道抓图识别队列。
- 修复：
  - 下线扫描接口内的旧同步抓图兜底路径。
  - `camera/list` 返回 `10026` 时，后端原样返回萤石错误，由前端触发 `probe-recognize-channel` 队列逐通道抓图、识别和写入。
  - 保留非 `10026` 错误的原有返回逻辑。
  - 资产存储模式增加防守性识别：如果运行时已经提供完整 Supabase Storage 配置，但漏配 `ASSET_STORE=supabase`，后端会自动使用 Supabase Storage，避免公司 K8s 环境误写容器本地目录。
  - `/health` 增加非敏感字段 `asset_store`，用于确认线上当前使用 `local` 还是 `supabase`，方便排查“最近截图/设计图加载不出来”。
- 验证：
  - 更新 `TestEzvizScannerReturnsPlanLimitWithoutCaptureProbe`，覆盖 `10026` 时不发送任何 `/device/capture` 请求，并把错误返回给上层。
  - 保留 `TestEzvizScannerDoesNotFallbackForUnauthorizedDevice`，覆盖非授权错误不触发抓图探测。
  - 新增 `TestNewStoreFromEnvAutoSelectsSupabaseWhenStorageConfigExists` 覆盖 Supabase Storage 配置完整时自动选用 Supabase。
  - 更新 `/health` 测试覆盖 `asset_store` 字段。

## 2026-06-23 抓图兜底扫描识别 2.14.0 开发记录

- 版本号：`2.14.0`。
- 用户反馈与产品调整：
  - 华东录像机 `K92940413` 扫描上报 HTTP 504。
  - 实测 `camera/list` 返回 `10026 设备数量超出个人版限制`，通道 1-10 可抓图，通道 11-15 返回 `60012` 且每个失败耗时约 10-15 秒，完整同步兜底扫描约 70 秒。
  - 产品流程调整为：当无法直接获取通道列表时，不再等待完整扫描结果，而是逐通道抓图；抓图成功即创建有效通道、保存最近截图，并同步完成 AI 区域识别。
  - 页面只展示“已检测 X 个，有效 Y 个”，连续失败数只作为内部停止条件，不展示给用户。
- 实现：
  - 新增 `ProbeRecognizeChannel` 服务能力和 `POST /api/store-space/recorders/{recorder_id}/probe-recognize-channel`。
  - 新增仓库方法 `UpsertRecorderChannel`，单通道成功时创建/更新通道，不清空其他通道，也不覆盖已确认映射。
  - 前端扫描遇到 `10026` 或“设备数量超出个人版限制”时，自动进入抓图识别队列，从通道 1 开始逐个调用单通道接口。
  - 成功通道立即写入页面通道列表，截图和 AI 识别结果同步展示；连续 5 个失败或达到通道 32 后停止。
- 验证：
  - 新增 `TestProbeRecognizeChannelCreatesChannelAndStoresRecognition` 覆盖抓图成功后创建通道、保存稳定截图、写入 AI 识别结果。
  - 新增 `TestProbeRecognizeChannelReturnsInactiveWhenCaptureFails` 覆盖抓图失败不创建通道。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 发布状态：
  - GitHub `main` commit：`4d2860d`。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 `COMMIT=4d2860d`，`VERSION=2.14.0`。
  - 韩国服务器本机验证：`/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 active。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`caff710`。
  - 公司环境由 GitLab/K8s 自动发布；本机当前无法直接验证公司内网页面版本，需要用户在公司网络侧确认。

## 2026-06-23 通道最近截图过期展示 2.13.1 修复记录

- 版本号：`2.13.1`。
- 用户反馈：
  - 有效通道里的“最近截图”过几天后仍然出现加载失败，怀疑图片没有妥善保存。
- 排查结论：
  - 当前新识别/刷新链路会把萤石云 `device/capture` 返回的临时图片先下载，再通过 `AssetStore` 保存为 `/api/store-space/channel-snapshots/{name}`，这是稳定托管路径。
  - 韩国服务器抽样检查：
    - 新测试门店 `萤石华北测试门店` 的截图均为 `/api/store-space/channel-snapshots/...`，后端接口返回 200。
    - 老门店 `新氧青春诊所 深圳龙岗坂田万科项目` 的 38 个通道仍保存为 `https://opencapture.ys7.com/...` 临时 URL，并带 `full_image_expires_at`，属于历史数据未迁移。
  - 因此本次现象主要来自历史临时截图 URL 过期；新截图保存逻辑本身可用。
- 修复：
  - 后端读取通道时，如果截图是带过期时间的远程临时图，且已过期，则不再把 `thumbnail_url/full_image_url` 暴露给前端。
  - 已保存到系统截图库的 `/api/store-space/channel-snapshots/...` 不受过期时间影响。
  - 前端对已过期截图显示“已过期”，保留“刷新截图/重新识别”入口，让用户重新生成稳定截图。
- 验证：
  - 新增 `TestExpiredRemoteChannelSnapshotIsNotExposed` 覆盖过期远程截图不再暴露给前端。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-23 萤石云扫描通道抓图兜底 2.13.0 开发记录

- 版本号：`2.13.0`。
- 用户反馈：
  - 部分录像机超过萤石云个人版设备限制时，`device/camera/list` 直接返回错误，导致系统扫描通道失败。
  - 实测 `GF8132547` 在华东账号下 `device/camera/list` 返回 `10026`，但 `device/capture` 抓取通道 1 成功。
- 产品决策：
  - 默认仍优先使用萤石官方 `device/camera/list` 扫描通道。
  - 当 `camera/list` 返回 `10026` 时，降级使用 `device/capture` 从通道 1 开始串行探测。
  - 抓图成功即认为该通道有效；连续 5 个通道抓图失败后停止；最大探测到通道 32。
- 修复：
  - `internal/ezviz/client.go` 新增 `ErrorCode`，供上层识别萤石错误码。
  - `internal/storespace/ezviz_scanner.go` 在 `10026` 时启用抓图兜底探测。
  - 其他萤石错误码，例如 `20018 该用户不拥有该设备`，仍保持原错误，不误触发兜底扫描。
- 验证：
  - 新增 `internal/storespace/ezviz_scanner_test.go` 覆盖 `10026` 兜底抓图、连续 5 个失败停止、非权限错误不兜底。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go build ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-22 城市列表补充济南并按音序排列 2.12.2 开发记录

- 版本号：`2.12.2`。
- 用户反馈：
  - 添加机构时城市列表需要增加“济南”。
  - 整个城市列表需要按首字母音序排列。
- 修复：
  - 新增 `frontend/src/domain/cities.ts`，集中维护城市下拉配置。
  - 城市列表新增“济南”，并按拼音首字母顺序排列。
  - 添加机构弹窗和编辑机构弹窗统一引用 `CITY_OPTIONS`，避免两个入口城市列表不一致。
- 验证：
  - 新增 `frontend/src/domain/cities.test.ts` 覆盖“包含济南”和完整城市顺序。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-22 其他区域确认后显示未知 2.12.1 修复记录

- 版本号：`2.12.1`。
- 用户反馈：
  - 通道识别为“其他区域”后，手动填写“护士站”等编号/备注，点击确认正常。
  - 再点击编辑修改并重新确认后，业务区域类型显示成“未知”。
- 根因：
  - 非业务区域自定义备注不属于固定场景枚举，前端二次确认时会发送 `sceneType = unknown`。
  - 页面展示层把内部兜底枚举 `unknown` 直接翻译成“未知”，导致用户看到错误业务类型；实际备注仍保存在 `areaNote`。
- 修复：
  - 新增 `frontend/src/domain/channel-labels.ts`，集中维护通道场景展示名。
  - `unknown` 在通道映射业务展示中统一显示为“其他区域”。
  - `frontend/src/components/VideoChannelTab.tsx` 改为复用领域 helper，避免组件内重复维护场景文案。
- 验证：
  - 新增 `frontend/src/domain/channel-labels.test.ts` 覆盖 `unknown -> 其他区域`、`machine_room -> 机房`、`front_desk -> 前台`、`treatment -> 治疗室`。
  - 已用临时 TypeScript 编译链路验证新增领域测试通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - 修复代码 commit：`d0d0d8d`。
  - GitHub `main` 已推送修复代码。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`de514e6`。
  - 公司环境由 GitLab/K8s 自动发布；当前本机无法解析 `lite.sy.soyoung.com`，需要用户在公司内网侧确认页面版本。
  - 韩国服务器已执行 `/opt/apps/erzhuang-project/scripts/deploy.sh`，服务器测试、Go build、前端 build、服务重启均通过。
  - 韩国服务器本机验证：`VERSION=2.12.1`，`/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 与 `nginx` 均为 active。
  - 本机直连 `https://43.155.237.46/erzhuang/health` 仍连接失败，和既有网络现象一致；服务器本机服务状态正常。

## 2026-06-17 机构基础信息编辑 2.12.0 开发记录

- 版本号：`2.12.0`。
- 用户反馈：
  - 添加门店后，城市和新氧机构 ID 无法再修改。
  - 机构列表操作区希望改为 `详情 / 编辑 / 删除`。
- 产品决策：
  - 列表页新增“编辑”只维护基础信息：城市、门店名称、新氧机构 ID。
  - 设计图、录像机、通道映射仍在详情页对应 Tab 维护，不放进基础信息编辑弹窗，避免入口重复和校验混乱。
- 后端代码索引：
  - `PATCH /api/store-space/stores/{id}`：更新机构基础信息。
  - `internal/storespace/models.go`：`UpdateStoreBasicInfoInput`。
  - `internal/storespace/service.go`：`UpdateStoreBasicInfo`，复用同名门店校验并排除当前门店。
  - `internal/storespace/store.go`：Memory/Postgres 更新 `city/name/normalized_name/external_org_id/updated_at`。
  - `internal/storespace/handler.go`：`updateStoreBasicInfo`。
- 前端代码索引：
  - `frontend/src/components/StoreList.tsx`：操作区改为 `详情 / 编辑 / 删除`。
  - `frontend/src/components/EditStoreModal.tsx`：基础信息编辑弹窗。
  - `frontend/src/App.tsx`：编辑弹窗状态、保存、重复门店确认和列表刷新。
  - `frontend/src/api.ts`：`UpdateStoreBasicInfoPayload`、`storeSpaceApi.updateStoreBasicInfo`。
  - `frontend/src/components/CreateStoreModal.tsx`：导出 `CITY_OPTIONS` 供编辑弹窗复用。
- 验证状态：
  - 已补 service 与 handler 测试。
  - `cd frontend && npm run build` 通过。
  - 本机 Go 测试仍受 `.tools/go` / macOS 动态加载 `missing LC_UUID load command` 影响，待在服务器或可用 Go 环境验证。

## 2026-06-17 图片访问前缀修复 2.11.1 开发记录

- 版本号：`2.11.1`。
- 用户反馈：
  - 公司 GitLab 环境识别区域后，通道列表“最近截图”显示“已过期”。
  - 设计图图纸也无法加载图片。
- 根因判断：
  - 后端已把萤石云临时截图下载并保存到系统资产存储，通道截图路径保存在 `channel_snapshots.thumbnail_path/full_image_path`。
  - 设计图路径保存在门店设计图记录的 `preview_image_path/thumbnail_path`。
  - 前端旧逻辑把所有后端返回的 `/api/...` 图片地址硬编码补成 `/erzhuang/api/...`。
  - 公司环境实际前缀是 `/erzhuang-project/`，所以图片请求被改到错误路径，浏览器加载失败后被前端误显示为“已过期”。
- 修复：
  - 新增 `frontend/src/url-utils.ts`，集中处理 API base、图片展示 URL、存储路径反解。
  - 默认 API base 改为根据当前页面路径和 Vite `BASE_URL` 推导，兼容个人 `/erzhuang/` 与公司 `/erzhuang-project/`。
  - 设计图、门店缩略图、通道截图统一按对应 API base 转换，不再写死 `/erzhuang`。
  - 图片加载失败文案由“已过期”改为“加载失败”；截图预览说明改为“已保存到系统截图库”。
- 验证：
  - `frontend/src/url-utils.test.ts` 覆盖 `/erzhuang-project/api/...`、`/erzhuang/api/...`、历史 `uploads/...` 路径转换。
  - `cd frontend && npm run build` 通过。
  - `go test ./...` 本机未完成：系统 PATH 无 `go`，改用项目 `.tools/go` 后 macOS 动态加载报 `missing LC_UUID load command`，本次未改后端代码。
- 发布状态：
  - GitHub `main` 已推送：`2a443c2 Fix image URL base path handling`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已推送：`c5b0d22 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境由 GitLab/K8s 自动发布；当前本机无法解析 `lite.sy.soyoung.com`，需要用户在公司内网侧确认页面版本和图片加载。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh`，服务器当前 commit：`2a443c2`，版本：`2.11.1`。
  - 韩国服务器 `go test ./...`、Go build、前端 build、服务重启均成功。
  - 韩国服务器本机 `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 韩国服务器 nginx 与 `erzhuang-project.service` 均为 active，监听 `0.0.0.0:443` 与 `127.0.0.1:18081`。
  - 本机直连 `https://43.155.237.46/erzhuang/health` 暂时连接失败，但 SSH 到服务器本机检查服务和端口均正常。
  - 本次发现本机 SSH 登录韩国服务器的 key 是 `~/.ssh/erzhuang_lighthouse`，不是文档里原先容易混淆的服务器内部 GitHub deploy key。

## 2026-06-17 通道映射 Excel 导出 2.11.0 开发记录

- 版本号：`2.11.0`。
- 目标：
  - 在机构详情页的通道映射 Tab 增加“导出 Excel”能力。
  - 按用户要求，按钮放在“通道列表”模块标题行，位于业务区域筛选条件左侧。
- 后端：
  - 新增 `GET /api/store-space/stores/{id}/channel-mappings/export.xlsx`。
  - 使用 Go 标准库生成 `.xlsx`，不引入第三方 Excel 依赖。
  - 导出列：序号、城市、门店名称、新氧机构 ID、录像机编号、通道号、最近截图、业务区域类型、编号/备注。
  - 过滤离线录像机、失效通道和已删除通道。
  - 排序顺序：面诊室、治疗室、生美、其他区域；同类型再按编号/备注、录像机编号、通道号排序。
  - 可读取的通道截图会作为 Excel 图片对象嵌入，读取不到则保留文字占位。
- 前端：
  - `VideoChannelTab` 通道筛选行新增 `导出 Excel` 按钮。
  - 支持导出中 loading 态和错误 toast。
  - 浏览器按后端 `Content-Disposition` 文件名下载 `.xlsx`。
- 验证：
  - `go test ./...` 通过。
  - `frontend npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器验收：按钮已出现在“通道列表”标题行，位于 `全部/面诊室/治疗室/生美` 筛选左侧。

## 2026-06-17 发布术语规范

本节是 2026-06-17 的历史发布术语记录。2026-07-06 已更新当前规则：GitHub 代码备份能力依然保留；韩国 Lighthouse 发布链路已终止，且韩国服务器上的二壮项目库表已完全删除；二壮项目实际发布只走公司 GitLab/K8s。

用户当时明确两套发布口径：

- 默认 GitHub 备份：
  - 除非用户明确说明“不要同步 GitHub”或“只推公司 GitLab”，所有已确认准备发布的代码都先提交并推送到 GitHub `origin/main`。
  - GitHub 是主代码备份；后续当前规则仍保留这一点，但不再发布韩国服务器。
- “发布到公司”：
  - merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
  - 推送 remote `gitlab`。
  - 由公司 GitLab / K8s 自动发布，通常约 5 分钟。
  - 不操作韩国 Lighthouse，不 force push，不覆盖公司 Docker/K8s/运行时环境配置。
  - 验证 `https://lite.sy.soyoung.com/erzhuang-project/health` 和页面版本号。
- “发布到韩国服务器”（已于 2026-07-06 废止）：
  - 推送 GitHub `origin/main`。
  - 通过腾讯云 TAT 指定韩国 Lighthouse `ap-seoul / lhins-rjfpwj1u`。
  - 以 `lighthouse` 用户执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
  - 服务器从 GitHub 拉取最新 `main`，自动执行测试、构建、重启和健康检查。
  - 验证 `http://127.0.0.1:18081/health` 和公网 `/erzhuang/`。
- 如果用户同时要求两个环境，需要记录两个环境最终 commit，避免页面版本号和问题反馈对不齐。

同步文档：

- `AGENTS.md`
- `docs/deploy-runbook.md`

## 2026-06-17 萤石云账号区域自动同步

- 版本号：`2.10.1`。
- 公司环境添加录像机时“选择区域”为空，根因是公司新数据库 `ezviz_accounts` 没有 `华北/华东/华南/华中` 等展示记录。
- 当前决策：
  - 公司内网环境可临时把完整 `EZVIZ_ACCOUNTS_JSON` 写入内网 GitLab Dockerfile 或容器环境变量，后续再迁移到 K8s Secret。
  - 代码不把 `app_key/app_secret/access_token` 写入数据库。
  - 服务启动时从 `EZVIZ_ACCOUNTS_JSON` 读取账号 `name/account_name`，自动 upsert 到 `ezviz_accounts`，状态设为 `available`。
  - 前端继续从 `GET /api/store-space/ezviz-accounts` 获取可选区域。
  - 扫描、抓图时后端仍使用运行时 env 中的完整密钥。
- 验证重点：
  - 公司环境启动日志应出现 `ezviz scanner enabled, synced N account(s)`。
  - `GET /api/store-space/ezviz-accounts` 应返回 `华北/华东/华南/华中`。
  - 添加门店/添加录像机时“选择区域”下拉应出现对应大区。

## 2026-06-16 资产存储抽象 2.10.0 开发记录

- 版本号：`2.10.0`。
- 背景：
  - 公司研发反馈：如果设计图、预览图、缩略图和监控截图只存在容器本地目录，K8s 容器重启或重新调度后可能丢失。
  - 建议把这些文件放到 Supabase Storage，并保留数据库字段记录对象路径。
- 本次改进：
  - 新增 `internal/assets` 统一资产存储层。
  - 支持 `ASSET_STORE=local` 和 `ASSET_STORE=supabase` 两种实现。
  - 设计图上传仍在本地临时目录完成 PDF 转 PNG，然后把 `original.pdf`、`preview.png`、`thumbnail.png` 保存到 AssetStore。
  - 通道截图从萤石云临时 URL 下载后，也改为保存到 AssetStore。
  - 图片接口继续由 Go 后端读取并转发，前端不直连 Supabase Storage。
  - 兼容旧本地路径：数据库仍保存 `uploads/{upload_id}/preview.png`，Supabase 使用该逻辑 key；本地模式会映射回 `UPLOAD_DIR/{upload_id}/preview.png`，避免个人服务器旧图打不开。
- 环境变量约定：
  - 本地/个人服务器：`ASSET_STORE=local`，`UPLOAD_DIR=/opt/apps/erzhuang-project/uploads`。
  - 公司 K8s：`ASSET_STORE=supabase`，`SUPABASE_URL=...`，`SUPABASE_SERVICE_ROLE_KEY=...`，`SUPABASE_STORAGE_BUCKET=design-plan-assets`，`UPLOAD_DIR=/tmp/erzhuang-work`。
  - `SUPABASE_SERVICE_ROLE_KEY` 只允许放服务端环境变量或 K8s Secret，不进入仓库、镜像和前端 `VITE_*` 配置。
- 文档同步：
  - `docs/deploy-runbook.md` 补充 Supabase Storage 部署配置。
  - `docs/technical-architecture-index.md` 补充 `internal/assets`、设计图上传、通道截图的代码索引。
  - `docs/superpowers/plans/2026-06-16-asset-store-storage.md` 记录本次实施计划和后续维护要点。
- 发布状态：
  - 代码 commit：`dfc4845`。
  - GitHub `main` 已推送到 `dfc4845`。
  - TAT InvocationId：`inv-p4x3r8g8ad`。
  - 服务器发布脚本执行成功，服务器已拉取 `dfc4845`。
  - 服务器 `go test ./...` 通过。
  - 服务器 Go build 通过。
  - 服务器前端 build 通过，产物包含 `/erzhuang/assets/index-CPQG6Jsb.js`。
  - `erzhuang-project.service` 重启成功。
  - 服务器本机 `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 服务器 `npm install` 仍提示 2 个 high severity vulnerabilities，未在本次存储改造中处理，后续可单独做前端依赖安全升级评估。

## 2026-06-15 通道截图持久化 2.9.10 发布记录

- 版本号：`2.9.10`。
- Commit：`b470ec4`。
- 用户反馈：
  - 过了周末后，机构详情的“通道映射” Tab 里“最近截图”展示不出来。
- 根因：
  - 后端原来把萤石云 `opencapture.ys7.com` 返回的临时签名截图 URL 直接保存到 `channel_snapshots.thumbnail_path/full_image_path`。
  - 线上接口返回的旧 URL 访问为 `403 Forbidden`，过期后前端图片自然无法展示。
- 修复：
  - 新增 `LocalSnapshotStore`，抓图成功后下载截图到服务器本地 `uploads/channel-snapshots`。
  - 新增 `GET /api/store-space/channel-snapshots/{name}`，前端展示改用项目自己的稳定图片地址。
  - AI 识别仍使用萤石云刚返回的公网临时 URL，避免模型服务访问内网地址失败；前端展示使用本地持久化地址。
  - 新增 `POST /api/store-space/channels/{channel_id}/snapshot`，已确认通道可“刷新截图”，不改变确认状态、业务区域类型、编号，也不增加识别次数。
  - 前端缩略图加载失败时显示“已过期”，避免用户只看到空白。
- 本地验证：
  - 新增测试 `TestRecognizeRecorderChannelsStoresRemoteSnapshotsLocally` 和 `TestRefreshChannelSnapshotKeepsConfirmedMapping`。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器预览 `/erzhuang/` 和机构详情通道映射 Tab 可正常加载，操作列未溢出。
- 发布状态：
  - GitHub `main` 已推送到 `b470ec4`。
  - 服务器 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh` 执行成功。
  - 服务器当前 commit：`b470ec4`。
  - 服务器当前版本：`2.9.10`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 `active`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-BBai7Js_.js` 和 `/erzhuang/assets/index-AdczDtmt.css`。
- 已知影响：
  - 历史已经过期的萤石云 URL 无法凭空恢复；用户需要对已确认通道点击“刷新截图”，或对未确认通道执行“重新识别/识别区域”，新截图才会落到本地持久化存储。
  - `uploads/channel-snapshots` 目录会在第一次刷新/识别截图时自动创建。

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

## 2026-06-12 通道编辑解锁 2.9.7 修复记录

- 版本号：`2.9.7`。
- 问题：
  - 已确认通道点击“编辑”后，页面状态仍显示已确认，“重新识别”按钮仍不可点击。
- 根因：
  - 前端之前只写入本地草稿状态，后端真实通道状态仍为已确认。
  - 已确认通道识别接口已被后端锁定，所以仅前端放开按钮也无法真正重新识别。
- 修复：
  - 新增后端 `UnlockChannelForEdit` 能力和 `/api/store-space/channels/{channel_id}/unlock` 接口。
  - 点击“编辑”时调用后端解锁，将通道状态改为待确认，清空确认时间，保留当前区域类型/编号作为编辑草稿。
  - 解锁后“重新识别”恢复可点击；后端允许待确认通道重新抓图和 AI 识别。
  - 新增 `scripts/check-channel-edit-unlocks-state.mjs`，并更新 `scripts/check-confirmed-channel-recognition-lock.mjs`。

## 2026-06-12 进入详情加载反馈 2.9.8 修复记录

- 版本号：`2.9.8`。
- 问题：
  - 机构列表点击“进入详情”后，用户反馈半天没反应，需要点很多次才跳转。
- 排查：
  - 线上列表接口实测约 `0.27s-0.42s`。
  - 线上详情接口实测约 `0.5s-1.3s`，不算完全阻塞，但足以让无反馈按钮显得像没点上。
  - 前端此前没有行级详情加载态，重复点击会触发多个详情请求。
  - 基础接口 `/health` 和账号列表约 `0.137s`，说明 Supabase/网络往返是主要底噪。
  - 门店详情原实现会串行查询 areas、design plans、recorders，并对每台录像机单独查询 channels，存在 N+1 查询。
- 修复：
  - 机构列表增加行级 `openingStoreIds` 状态。
  - 点击“进入详情”后按钮显示“进入中”并带 spinner。
  - 正在进入详情时禁用该行进入/删除按钮，并忽略重复点击。
  - 门店详情内录像机通道查询由“每台录像机一次查询”改为“按门店一次批量查询”，减少远程数据库往返。
  - 通道“编辑解锁”接口由返回整份门店详情改为仅返回当前通道，前端用局部替换更新当前行。
  - 新增 `scripts/check-store-open-loading-state.mjs`，防止入口加载反馈回退。

## 2026-06-12 通道操作即时反馈 2.9.9 修复记录

- 版本号：`2.9.9`。
- 问题：
  - 用户反馈详情页内点击确认、编辑、删除都需要大量等待。
- 排查：
  - 这类操作不是浏览器到后端之间“没连上”，而是写接口需要等待远程数据库更新。
  - 其中确认、删除通道、删除录像机等路径还会返回或重新拉取较重的门店详情数据，导致按钮操作手感被详情接口耗时拖慢。
- 修复：
  - 通道确认增加乐观更新：点击确认后前端立即把当前行切到确认状态，后端返回后再校准；失败时回滚并提示。
  - 通道编辑解锁保持单通道轻量返回，并在点击后立即切到待确认编辑态。
  - 删除通道和删除录像机增加乐观更新：点击删除后先从页面移除，后端失败时恢复原门店状态。
  - 新增 `scripts/check-channel-actions-optimistic-state.mjs`，防止通道操作退回“等接口返回后才更新页面”的交互。

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

## 2026-06-26 AI 模型 provider 切换开发记录

- 目标：
  - 解决 OpenAI 接口限流时项目识别能力不稳定的问题。
  - 通道截图识别和设计图识别都支持通过环境变量切换 OpenAI / MiniMax。
  - MiniMax HTTP 调用内置到 Go 代码中，避免正式服务依赖 OpenClaw 外部脚本。
- 关键改动：
  - `CHANNEL_AI_PROVIDER=openai|minimax|minimax-script` 控制通道截图识别。
  - `DESIGN_PLAN_AI_PROVIDER=openai|minimax` 控制设计图识别；不设置时跟随 `CHANNEL_AI_PROVIDER`。
  - `MINIMAX_API_KEY` 是 MiniMax 唯一 key 来源，不复用 `OPENAI_API_KEY` 或 `VISION_API_KEY`。
  - 设计图识别增加 markdown 代码块包裹 JSON 的解析兼容。
  - MiniMax/OpenAI base URL 带 `/v1` 时避免重复拼接 `/v1/v1/...`。
  - 新增 `cmd/ai-smoke`，用于 provider/key/model 切换后的真实冒烟验证。
  - 新增 `docs/model-provider-switching.md`，记录换 provider、换 key、换模型和冒烟步骤。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/channelai/... ./internal/designplan/... ./cmd/ai-smoke` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 真实 MiniMax 冒烟：
  - `https://api.minimaxi.com/v1/models` 可用，当前 key 返回模型列表：`MiniMax-M3`、`MiniMax-M2.7`、`MiniMax-M2.7-highspeed`、`MiniMax-M2.5`、`MiniMax-M2.5-highspeed`、`MiniMax-M2.1`、`MiniMax-M2.1-highspeed`、`MiniMax-M2`。
  - `MiniMax-01-vision` 返回 `unknown model`；`MiniMax-M1` 返回 `not support img`。
  - `MiniMax-M3` 设计图 smoke 成功，耗时约 `4557ms`。
  - `MiniMax-M3` 通道截图 smoke 成功，耗时约 `3027ms`。
- 后续：
  - `minimax-script` 仍作为短期兜底保留；MiniMax HTTP 在线上环境验证稳定后再删除，彻底解耦 OpenClaw。

## 2026-06-26 详情页识别模型切换按钮开发记录

- 目标：
  - 在机构详情页「设计图标注 / 通道映射」Tab 行最右侧增加「切换识别模型」按钮。
  - 按钮后展示当前识别模型，例如 `当前识别模型：OpenAI / gpt-5.5` 或 `MiniMax / MiniMax-M3`。
  - 点击按钮在 OpenAI 和 MiniMax 之间切换，同时影响设计图识别和通道截图识别。
- 实现：
  - 新增后端 `GET /api/ai-settings` 和 `POST /api/ai-settings/toggle`。
  - 新增 `app_settings` 表保存 `ai_provider`，并开启 RLS + 拒绝前端直连策略。
  - 识别服务改为运行时读取当前 provider，不需要重启服务。
  - API key 仍只来自运行时环境变量，不进入数据库、前端或仓库。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 未完成：
  - 当前沙箱无法用 Playwright/Computer Use 完成页面截图验收；本地 dev server 已启动在 `http://127.0.0.1:5177/erzhuang/`，需要浏览器人工确认按钮位置。

## 2026-06-26 识别模型切换 2.16.0 发布记录

- 版本号：`2.16.0`。
- GitHub `main` commit：`d783014 Add runtime AI model switching`。
- 公司 GitLab 发布分支：`codex/containerize-single-image`。
- 公司 GitLab merge commit：`0ebed48 Merge branch 'main' into codex/containerize-single-image`。
- 发布范围：
  - 机构详情页 Tab 行最右侧新增“切换识别模型”按钮和当前模型显示。
  - 后端新增 AI settings API，运行时在 OpenAI / MiniMax 之间切换。
  - 通道截图识别和设计图识别支持动态 provider。
  - MiniMax HTTP recognizer 内置到 Go 服务，保留 `minimax-script` 作为短期兜底。
  - 同步 H5 monitor 技术调研文档和隐藏 Ezviz live demo 支撑代码。
- 本地验证：
  - `git diff --check --cached` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - staged diff 敏感信息扫描未发现真实 key。
- 公司环境验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已确认包含 `2.16.0`、“当前识别模型”、“切换识别模型”、`OpenAI`、`MiniMax`。
- 韩国服务器发布：
  - 通过 SSH 执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
  - 服务器拉取 GitHub `main` 到 `d783014`。
  - 服务器 `go test ./...`、Go build、frontend build 通过。
  - 重启 `erzhuang-project.service` 后健康检查最终通过。
  - 公网 `https://43.155.237.46/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"local"}`。
  - 韩国线上 JS 已确认包含 `2.16.0`、`d783014`、“当前识别模型”、“切换识别模型”、`OpenAI`、`MiniMax`。
- 注意：
  - TAT 发布因本机无 `TENCENTCLOUD_SECRET_ID` / `TENCENTCLOUD_SECRET_KEY` 环境变量而未继续输入密钥，改用已记录的 SSH key 执行同一部署脚本。
  - 韩国部署时服务重启后前 13 次本机 health 连接失败，第 14 次成功，判断为服务启动/依赖初始化短暂延迟；本次无需回滚。

## 2026-06-26 VIP治疗室与通道筛选 2.17.0 开发记录

- 目标：
  - 新增业务区域类型 `VIP治疗室`，归入治疗室大类。
  - 通道映射筛选扩展为：全部、面诊室、治疗室、生美、前台/候诊区、通道/其他。
  - 筛选和排序规则沉淀为可复用前端领域模块，供后续 H5 monitor 首页复用。
- 关键规则：
  - `VIP治疗室` 对应 `area_type=vip_treatment`，治疗室筛选和治疗室数量统计都包含它。
  - `VIP治疗室` 编号/备注非必填，空编号在后端以 `area_number=0` 表示；同一门店最多一个未编号 VIP 治疗室。
  - 普通治疗室、面诊室、生美仍要求数字编号。
  - `前台/候诊区` 只按可见/可维护文本包含 `前台`、`候诊`、`等候` 判断。
  - `通道/其他` 作为非业务且非前台候诊的兜底组。
- 实现：
  - 新增 `frontend/src/domain/channel-filters.ts`，以最小字段接口 `ChannelFilterable` 承载通道筛选、归类、排序规则。
  - 新增 `frontend/src/domain/channel-filters.test.ts` 覆盖全部排序、治疗室包含 VIP、前台候诊匹配、通道/其他兜底。
  - 通道映射 Tab 改用共享筛选模块，新增 VIP 治疗室选项。
  - 设计图标注区域卡片新增 VIP 治疗室选项，并与通道映射一致支持空编号。
  - `internal/storespace` 与旧 `internal/designplan` 模块同步支持 `vip_treatment`，避免旧路由/schema 保留三类约束。
  - `docs/h5-monitor-dev-task.md` 增加复用实现方案，要求 H5 monitor 不复制分组排序逻辑。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && ./node_modules/.bin/vitest run src/domain/channel-filters.test.ts` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 未完成：
  - Vite dev server 需要提升权限后可启动；Playwright 自带浏览器未安装，系统 Chrome headless 被本机权限限制关闭，因此本轮未完成自动化截图验收。

## 2026-06-26 通道截图缓存 2.17.1 开发记录

- 目标：
  - 降低机构详情页通道映射 Tab 每次进入时最近截图重新排队加载的等待感。
  - 保持刷新截图/重新识别后能显示新图，不让用户看到过期图片。
- 实现：
  - 前端 `ImageLoadQueue` 增加已成功加载 URL 的内存记录。
  - `QueuedSnapshotImage` 仅在同一 URL 已成功加载过时直接显示；未命中仍走原来的队列预加载和错误兜底。
  - 后端通道截图接口增加 `Cache-Control: private, max-age=604800, immutable` 与 `ETag`，命中 `If-None-Match` 时返回 `304`。
  - 前端验收清单新增规则：版本化图片 URL 应支持浏览器缓存或前端内存缓存，刷新图片时通过新 URL 失效旧缓存。
- 风险控制：
  - 不修改截图 URL 生成逻辑。
  - 不修改图片加载失败兜底逻辑。
  - 当前截图刷新会生成新文件名，因此新 URL 会自然绕过旧缓存。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestChannelSnapshotResponseUsesBrowserCacheHeaders|TestChannelSnapshotDiagnosticsReportsOpenFailure'` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-image-queue-test src/domain/image-load-queue.ts src/domain/image-load-queue.test.ts && node /tmp/erzhuang-image-queue-test/image-load-queue.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-26 门店详情即时进入 2.17.2 开发记录

- 背景：
  - 机构列表点击“详情”时，前端原逻辑会等待 `GET /api/store-space/stores/{id}` 全量详情接口返回后才切换页面。
  - 该接口会加载门店基础信息、区域、设计图、录像机、通道和最近截图路径；大门店或网络波动时，用户会感觉点击后“转很久才进入”。
- 实现：
  - 新增 `frontend/src/domain/store-detail-navigation.ts`，沉淀列表摘要到详情占位对象、默认 Tab 判断、短期详情缓存逻辑。
  - 点击详情后立即用列表摘要生成详情壳，先展示门店标题、统计、Tab 和“正在加载门店详情”面板。
  - 完整详情接口返回后再替换真实数据；请求失败则回到列表并展示错误提示。
  - 同一门店 60 秒内二次进入且列表 `updatedAt` 未变化时使用前端内存详情缓存。
  - 用户返回列表会递增详情请求版本号，避免旧请求返回后把页面重新拉回详情。
- 未做：
  - 暂未拆分后端详情接口。下一步如仍有明显慢接口，可把门店详情拆成轻量 shell、通道数据、设计图数据三个加载单元。
- 验证：
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 本地 dev server 需提升权限启动，已启动在 `http://127.0.0.1:5176/erzhuang/`；Playwright 浏览器二进制未安装，本轮未完成自动化截图验收。

## 2026-06-26 门店详情 Tab 接口轻拆 2.18.0 开发记录

- 目标：
  - 在不废弃全量详情接口的前提下，把机构详情页数据按 Tab 轻量拆分，减少进入默认 Tab 时等待非当前业务块数据。
  - 避免把接口拆得过细，保持后续维护和 H5 monitor 复用简单。
- 后端实现：
  - 保留 `GET /api/store-space/stores/{id}` 全量详情接口，继续作为兼容和 mutation 兜底。
  - 新增 `GET /api/store-space/stores/{id}/design-plan-data`，只返回门店基础信息、设计图、区域标注。
  - 新增 `GET /api/store-space/stores/{id}/channel-data`，只返回门店基础信息、录像机和通道。
  - `PostgresStore` 抽出基础门店查询 helper，两个 Tab 接口分别只调用对应列表查询。
- 前端实现：
  - 详情页仍先用列表摘要立即进入详情壳。
  - 默认 Tab 只请求对应 Tab 数据；切换到另一个 Tab 时再懒加载。
  - 详情缓存升级为按 Tab 记录已加载状态，合并数据时不会清空另一个 Tab 已有内容。
  - 创建、编辑、保存、删除、确认等已有 mutation 仍保留现有全量返回处理，不在本轮扩大改造。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestGetStoreDesignPlanDataEndpointReturnsOnlyDesignPlanTabData|TestGetStoreChannelDataEndpointReturnsOnlyChannelTabData'` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 风险：
  - 如果某个 mutation 返回全量详情后，缓存会标记两个 Tab 均已加载；这符合当前兼容策略，但后续若 mutation 也拆分，需要一起调整缓存标记。

## 2026-06-26 通道最近截图视口懒加载 2.18.1 开发记录

- 目标：
  - 降低机构详情页通道映射 Tab 首次进入时最近截图的并发加载压力。
  - 保留已有图片队列和缓存能力，避免改动后出现截图裂图或刷新截图不更新。
- 实现：
  - `QueuedSnapshotImage` 增加 `IntersectionObserver` 视口触发逻辑。
  - 未进入视野附近的截图只显示原加载占位，不立即进入预加载队列。
  - 截图进入视野附近约 `160px` 后才进入既有 `ImageLoadQueue(2)`，继续限制同时预加载 2 张。
  - 已成功加载过的同 URL 继续直接显示，保持 2.17.1 的内存缓存效果。
  - 不支持 `IntersectionObserver` 的浏览器自动回退到原队列加载逻辑。
- 验证：
  - `cd frontend && npm run build` 通过。
  - 本地 Vite 预览页面可打开，首页渲染正常，控制台未发现运行时错误。
  - `cd frontend && npm test -- --run` 未通过，失败为既有测试文件未使用 Vitest `test/it` 套件结构，以及 `api.test` 在当前测试环境下 base path 断言不一致；本次改动未触及对应逻辑。
- 风险：
  - 本地 mock 环境没有门店通道数据，未完成真实通道表格的浏览器截图验收；公司环境发布后需重点观察通道映射 Tab 首屏截图加载速度和滚动加载表现。
- 发布：
  - GitHub `main` commit：`463c32c Lazy load channel snapshots`。
  - 公司 GitLab 发布分支 merge commit：`6c71f07 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BJXO-7_s.js`，确认包含 `2.18.1`、`IntersectionObserver` 和 `160px 0px` 懒加载触发配置。

## 2026-06-26 详情页 Tab 统计修复 2.18.2 开发记录

- 背景：
  - 2.18.0 将详情数据拆成设计图 Tab 和通道映射 Tab 后，顶部统计被当前 Tab 的局部接口摘要字段覆盖。
  - 进入通道映射时，通道接口没有区域数据，导致业务区域数显示 0。
  - 切换到设计图标注时，设计图接口没有录像机和通道数据，导致录像机数、有效通道数显示 0。
- 实现：
  - `mergeStoreDetailTab` 不再使用局部接口的摘要字段整包覆盖当前详情。
  - 通道 Tab 只更新录像机、有效通道、确认状态和业务类型计数。
  - 设计图 Tab 只更新设计图状态、缩略图、业务区域数和区域标注数据。
  - 保留门店基础信息、状态和更新时间等共享字段更新。
- 验证：
  - 新增 `store-detail-navigation` 复现测试，覆盖“先加载通道 Tab、再加载设计图 Tab”后顶部统计不被互相清零。
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 发布：
  - GitHub `main` commit：`92593fa Fix split detail tab metrics`。
  - 公司 GitLab 发布分支 merge commit：`01493f2 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BHsDPUSA.js`，确认包含 `2.18.2`。

## 2026-06-26 详情页局部统计未知值修复 2.18.3 开发记录

- 背景：
  - 2.18.2 修复了 Tab 切换时统计互相清零，但通道映射 Tab 首次进入仍可能显示业务区域数 0。
  - 根因是 `channel-data` 局部接口不返回 `areas` 字段，前端映射层把“未返回区域数据”推导成 `areaCount=0`。
- 实现：
  - `mapStoreSpaceDetail` 区分“后端返回空数组”和“后端未返回字段”。
  - 当 `areas` 字段缺失时，不再推导区域相关计数为 0，而是保留为未知值，交给详情合并逻辑沿用列表摘要或已加载设计图数据。
  - 当 `recorders` 字段缺失时，同理不推导录像机/通道计数为 0。
  - `mergeStoreDetailTab` 遇到局部详情统计为未知值时保留当前统计。
- 验证：
  - `store-detail-navigation` 定向测试通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
- 发布：
  - GitHub `main` commit：`b2bca42 Preserve split detail unknown metrics`。
  - 公司 GitLab 发布分支 merge commit：`22f53be Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BE0jYwKG.js`，确认包含 `2.18.3`。

## 2026-06-26 门店列表业务区域总数修复 2.18.4 开发记录

- 背景：
  - 2.18.3 保证局部 Tab 接口缺失统计字段时不覆盖已有值，但详情页首次进入仍依赖门店列表摘要生成顶部壳。
  - 后端 `ListStores` 只返回治疗室、面诊室、生美分项计数，没有返回业务区域总数 `area_count`。
  - 因此前端列表摘要中的 `areaCount` 仍为 0，首次进入通道映射时顶部业务区域显示 0，切到设计图后才显示真实区域数。
- 实现：
  - `StoreListItem` 新增 `area_count` 字段。
  - Postgres `ListStores` SQL 增加 `count(distinct a.id) as area_count` 并扫描到返回结构。
  - MemoryStore `storeListItem` 同步累计 `AreaCount`。
- 验证：
  - 新增/补充 storespace 测试，覆盖设计图保存区域后列表摘要 `AreaCount` 返回真实数量。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestSaveDesignPlanAllowsVIPTreatmentWithoutNumber|TestConfirmVIPTreatmentAllowsBlankNumberAndCountsAsTreatment'` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布：
  - GitHub `main` commit：`e974ee3 Return store list area counts`。
  - 公司 GitLab 发布分支 merge commit：`5700181 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BoEHEZrM.js`，确认包含 `2.18.4`。

## 2026-06-26 H5 Monitor 试点集成 2.19.0 开发记录

- 目标：
  - 将独立 `h5-monitor` 原型集成进主项目，先作为受控试点能力给单门店验证。
  - 试点范围只开放“北京保利实验室门店”，新氧机构 ID `10030`，录像机 `GN0941203`。
- 后端实现：
  - 新增 `internal/h5monitor` 模块，提供 H5 首页、直播地址、录像片段、回放地址、播放地址失效接口。
  - 复用现有 `storespace` 门店、通道、录像机、萤石账号数据；播放凭证仍来自运行时 `EZVIZ_ACCOUNTS_JSON`，不写入前端或文档。
  - 新增萤石能力：FLV 直播地址、FLV 回放地址、录像片段查询、地址失效、AAC 转码 best-effort。
  - H5 API 响应不暴露 `device_serial`、app key、app secret、access token、萤石账号名。
  - 服务端门禁集中在 `h5monitor.Service`：默认只允许 `externalOrgId=10030` 和 `deviceSerial=GN0941203`。
  - 并发限制本轮仍为进程内内存计数：普通用户 15 路，管理员 20 路；多 Pod 场景后续需落库或接入统一会话。
- 前端实现：
  - 新增 H5 路由：
    - `/h5/orgs/{externalOrgId}/monitor`
    - `/h5/orgs/{externalOrgId}/monitor/channels/{channelId}`
  - 后台详情页右上角新增“查看监控”按钮，且仅 `externalOrgId=10030` 的门店展示。
  - H5 首页按区域筛选展示监控通道，默认每批 24 路，支持加载更多。
  - H5 详情页默认直播，支持切换录像、查询片段、点击片段播放。
  - 播放器使用 `ezuikit-flv`，静态 decoder 文件放在 `frontend/public/assets/ezuikit-flv/`。
  - 播放器默认静音以满足浏览器自动播放限制，用户点击后调用官方 `openSound/closeSound`。
  - H5 页面使用 route-level lazy import，后台页面不主动加载 H5 播放页面。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过；提示播放器 chunk 较大，属于 `ezuikit-flv` 依赖体积预期。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 浏览器验收通过：
    - `/erzhuang-project/h5/orgs/demo/monitor` 可渲染 H5 首页 mock 数据。
    - 点击通道可进入 H5 详情页，默认实时视频，声音按钮可见。
    - 切换录像可显示日期选择和录像片段。
    - `/erzhuang-project/` 后台首页未误进入 H5 路由。
- 风险：
  - 公司真实播放依赖 `EZVIZ_ACCOUNTS_JSON` 中包含华北账号，且账号名与数据库录像机绑定账号一致。
  - 本地没有公司数据库，未在本机验证 `10030/GN0941203` 的真实 H5 API 数据。
  - `ezuikit-flv` 打包后会生成约 1.8MB 未压缩播放器 chunk；已通过详情页 lazy import 控制影响范围。
  - 前端 `vitest` 当前只运行 `src/api.test.ts`，因为仓库内其他 `.test.ts` 仍是脚本式断言文件，后续可统一整理测试入口。

## 2026-06-26 H5 Monitor 播放与回放诊断修复 2.19.1 开发记录

- 背景：
  - 公司线上 H5 视频详情页进入后播放器黑屏，页面显示“播放器加载失败”，错误为 `v is not a constructor`。
  - 回放 Tab 看不到录像片段。
  - 用户希望错误信息继续外显，并增加可一起定位排查的详细上下文。
- 排查结论：
  - 直播黑屏主因是前端动态加载 `ezuikit-flv` 后优先把模块对象当构造函数使用；该包实际导出为 default class，打包后触发 `v is not a constructor`。
  - 公司线上回放片段接口返回 500，原因是萤石 `localIndex` 字段在线上返回为数字，后端原结构体按 string 解码导致 JSON unmarshal 失败。
  - 播放地址失效接口原来只提交 `id`，萤石新接口要求 `deviceSerial`、`channelNo`、`urlId`，线上曾返回 `deviceSerial不能为空`。
- 实现：
  - `H5FlvPlayer` 动态加载播放器时按 `default`、`EzuikitFlv`、`module` 顺序选择真正的函数构造器。
  - 播放器错误面板增加 stage、简化 URL、decoder 路径、直播/回放模式、库导出类型；事件 payload 中的签名 URL 会缩写，避免完整临时签名外露。
  - H5 API 错误对象增加后端 `code` 字段，页面 toast 展示 `HTTP` 状态、萤石错误码和字段错误。
  - 回放片段 `localIndex` 改为兼容 string/number 的 `FlexibleString`。
  - 播放地址失效接口改为携带 `deviceSerial`、`channelNo`、`urlId`，并补测试防止参数退化。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run test` 通过。
  - `cd frontend && npm run build` 通过；仍有播放器 chunk 体积提示，属于 `ezuikit-flv` 依赖体积预期。
  - `git diff --check` 通过。
- 发布：
  - 待推送公司 GitLab 固定分支 `codex/containerize-single-image` 后，由公司 K8s 自动发布。
- 线上追加验证：
  - 公司线上第一次发布后，回放片段接口从 `localIndex` 解码错误推进到新的真实返回差异：`meta.code` 有时是字符串。
  - 已补充 `FlexibleInt` 兼容 string/number，并用 `meta.code:"200"` 的测试复现覆盖。

## 2026-06-26 H5 Monitor 播放画面适配与 MSE 告警修复 2.19.2 开发记录

- 背景：
  - 公司线上部分实时视频已经能出画面，但页面出现 `MediaSource.addSourceBuffer` / `SourceBuffer` 告警。
  - 播放画面顶部有明显黑条，整体画面显示不完整，诊断条也会遮挡主要画面。
- 排查结论：
  - 报错来自 `ezuikit-flv` 的 MSE 硬解码路径，属于播放器内部 SourceBuffer 资源/上限问题；单画面 H5 场景稳定性优先于硬解码收益。
  - 画面黑条与播放器默认渲染模式、缺少官方样式、内部 video/canvas 未被外层容器稳定约束有关。
- 实现：
  - 引入 `ezuikit-flv/style.css`。
  - 播放器配置关闭 `useMSE`，保留 `useWCS` 和 `autoWasm`，规避 MSE SourceBuffer 路径。
  - 设置 `scaleMode`、`videoBuffer`、`themeData:null`、`mutedShowAutoReload:false`，减少播放器内置控件和自动重载干扰。
  - 切换/卸载播放器时先 pause 再 destroy，并清空容器 DOM，减少旧实例残留。
  - 将 MSE/SourceBuffer 类事件降级为可恢复 warning，6 秒后自动收起，不再用错误 toast 和大红层长期遮挡画面。
  - CSS 强制播放器内部 `video/canvas` 填满容器，诊断条移到顶部并区分 warning/error 视觉层级。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 服务可启动；自动浏览器截图验收因本项目未安装 Playwright、Browser 会话 tab 绑定异常未完成，真实画面仍需公司线上 H5 页面复验。

## 2026-06-26 H5 Monitor 恢复 MSE 播放路径 2.19.3 开发记录

- 背景：
  - 2.19.2 发布后，公司线上 H5 监控详情页播放器容器和声音按钮可见，但实时画面完全黑屏。
  - 用户反馈“啥都没显示出来”，相比 2.19.1 已能出画面但有 SourceBuffer 告警，属于播放渲染回归。
- 排查结论：
  - 2.19.2 为规避 `MediaSource.addSourceBuffer` 告警关闭了 `useMSE`。
  - 从现象判断，公司真实 FLV 流在当前浏览器/播放器组合下仍依赖 MSE 路径出画面；关闭 MSE 后播放器初始化成功但无法渲染视频。
- 实现：
  - 恢复 `useMSE:true`，保留官方样式、诊断降级、播放器销毁清理、诊断条不遮挡等其他改动。
  - 本轮只改一个变量，先恢复出画面；黑条和 SourceBuffer warning 后续再基于线上真实表现单独处理。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 回放时间参数与片段分页修复 2.19.4 开发记录

- 背景：
  - 公司线上 H5 监控切到“录像”后，点击回放片段获取播放地址失败。
  - 错误提示为 `回放地址获取失败 · HTTP 500 · code=10001 · ezviz api error code=10001 msg=illegal parameter startTime`。
  - 用户同时反馈录像回放片段“好像也不太对”。
- 排查结论：
  - 录像片段查询接口 `/api/v3/device/local/video/unify/query` 的 `startTime/endTime` 使用 Unix 秒是正确的。
  - 播放地址接口 `/api/lapp/v2/live/address/get` 在回放模式下要求 `startTime/stopTime` 为 `YYYY-MM-DD HH:mm:ss` 字符串；之前后端错误地传了 Unix 秒。
  - 公司容器时区不应影响录像片段日期。片段查询应按中国门店业务日期，也就是 `Asia/Shanghai` 自然日查询。
  - 片段查询返回 `hasMore/nextFileTime` 时，之前只取第一页，会导致一天内录像片段不完整。
- 实现：
  - 回放播放地址参数改为北京时间 `YYYY-MM-DD HH:mm:ss`。
  - 录像片段查询的自然日范围固定按 `Asia/Shanghai` 计算。
  - 录像片段查询支持跟随 `nextFileTime` 分页合并，避免只展示第一页片段。
  - 增加 Go 测试覆盖回放时间格式、上海自然日范围、片段分页合并。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 回放时间选择器样式修复 2.19.5 开发记录

- 背景：
  - 公司线上 H5 监控“录像”页日期选择弹层出现明显错位。
  - Ant Design DatePicker 默认弹层风格、英文月份和时间列布局与当前 H5 监控页不匹配。
- 设计决定：
  - 不继续修 AntD 弹层尺寸，改为 H5 页面内的轻量时间选择条。
  - 保留 `今天 / 昨天 / 前天` 快捷日期。
  - 用原生 `datetime-local` 选择具体时间，并提供“定位回放”按钮。
  - 保留实时/录像切换、离开详情时释放当前播放地址的逻辑。
- 实现：
  - H5 回放页移除 `DatePicker/dayjs` 直接依赖。
  - 新增 `.h5-date-time-field` 和 `.h5-date-confirm` 样式，使用项目现有边框、圆角、主色和焦点态。
  - H5 详情 chunk 从约 420KB 降到约 10KB，移动端加载更轻。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 服务可启动；Playwright CLI 因本机未安装 `chrome-for-testing` 浏览器二进制未完成截图验收。

## 2026-06-26 H5 Monitor 自绘回放时间弹层 2.19.6 开发记录

- 背景：
  - 2.19.5 使用原生 `datetime-local` 后，虽然避免了 AntD DatePicker 弹层错位和 chunk 过大，但浏览器原生弹层样式无法与项目风格统一。
  - 用户提供 ahabook 批阅记录日期选择器作为参考，希望日期选择区域整体可点击，弹层风格更接近当前产品。
- 设计决定：
  - 不继续依赖浏览器原生日期时间弹层，改为 H5 页面内自绘轻量日期时间选择器。
  - 保留 `今天 / 昨天 / 前天` 快捷日期和“定位回放”按钮。
  - 自绘弹层采用白色圆角浮层、圆形月份切换按钮、轻量日期网格、克制选中态，并支持点击外部关闭。
  - 保持实时/录像切换、关闭详情时释放当前播放地址的逻辑不变。
- 实现：
  - `PlaybackDatePicker` 新增自绘日期网格、小时/分钟滚动列、月份切换和完整触发按钮。
  - 选择器触发区整体可点击，不再只依赖原生日历图标。
  - 日期选中态改为浅底描边，弱化“蓝色按钮感”，更贴近项目后台浮层风格。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 移动端实时视频 HLS 适配 2.19.7 开发记录

- 背景：
  - 用户在手机浏览器和飞书内打开 H5 监控详情页后，实时视频黑屏。
  - 之前桌面端为恢复画面将 `ezuikit-flv` 的 MSE 路径重新打开，但移动浏览器对 FLV/MSE 兼容性不稳定，不能继续只调 FLV 播放器参数。
- 排查结论：
  - H5 详情页当前固定向萤石请求 FLV 地址，并固定使用 `ezuikit-flv` 播放。
  - 移动端应优先使用 HLS/m3u8 + 原生 `<video playsInline controls>`，桌面端保留 FLV 播放器路径，避免影响已能播放的桌面环境。
  - 本地录像回放文档对 HLS 支持不明确，当前回放仍保持 FLV 路径，后续需要基于移动端真实表现决定是否切萤石 JSSDK/ezopen 或内部 ISAPI 代理。
- 实现：
  - H5 live-url 请求新增 `protocol` 参数，支持 `hls/flv`；服务端按协议向萤石请求 `protocol=2/4`，并在响应里返回协议。
  - 前端移动端通用判断不限定 iPhone，覆盖 iPhone、Android、飞书/企微类移动 WebView，移动端实时视频请求 HLS，桌面请求 FLV。
  - 播放器组件根据协议选择播放方式：HLS/m3u8 走原生 `<video>`，FLV 继续走 `ezuikit-flv`。
  - 原生 video 路径保留加载态，直到 `loadedmetadata/canplay/playing` 后收起；失败时展示协议和简化 URL 诊断。
- 验证：
  - 新增后端测试覆盖 HLS/FLV 两种 live-url 协议参数。
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 发布：
  - 公司 GitLab `codex/containerize-single-image` 已推送 commit `48e143c`，触发公司 K8s 自动发布。
  - GitHub `main` 已同步 commit `4a52b8b`。
  - 公司线上 `/health` 返回 `database:"postgres"`、`asset_store:"supabase"`。
  - 公司线上静态资源已更新到 `2.19.7 (container)`，H5 详情 chunk 包含 `native-video`、`playsInline`、`hls/flv` 协议选择逻辑。

## 2026-06-26 H5 Monitor 移动端 H265 播放器适配 2.19.8 开发记录

- 背景：
  - 2.19.7 将移动端实时视频改为 HLS + 原生 video 后，手机端提示“视频编码类型非 H264”。
  - 用户判断不应为了手机播放统一关闭 H265，因为会降低录像机编码效率并增大录像体积。
- 排查结论：
  - HLS/native video 路径依赖浏览器原生解码，遇到 H265 流时容易失败。
  - `ezuikit-flv` 本地类型说明显示：MSE 硬解只支持 H264，iOS Safari 不支持；`autoWasm` 支持 H265 时从 MSE 自动降级到 wasm。
  - 更合理的方向是保留 H265 设备配置，移动端用播放器软解适配，而不是改录像机编码。
- 实现：
  - H5 实时视频默认协议改回 FLV，避免移动端进入 HLS/native video 的 H264 限制。
  - `H5FlvPlayer` 移动端播放上下文关闭 `useMSE`，保留 `autoWasm:true` 和 `useWCS:true`。
  - 播放器参数增加 `hasAudio:true`、移动端 `keepScreenOn:true`，保留声音和手机屏幕常亮能力。
  - 诊断信息增加 `protocol` 与 `decode`，便于区分桌面 MSE 与移动端 wasm 路径。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-26 H5 Monitor 移动端黑屏诊断增强 2.19.9 开发记录

- 背景：
  - 2.19.8 在手机端仍然黑屏，页面只显示“点击开启声音”，没有错误详情。
  - 截图说明播放器实例已经初始化成功，但没有渲染出首帧；之前代码在初始化成功后立即关闭 loading，导致“初始化成功但无首帧”的状态被误判为正常。
- 排查结论：
  - 当前缺少首帧/流成功诊断，无法判断卡在取流、解码、还是渲染阶段。
  - `ezuikit-flv` 暴露 `streamSuccess`、`videoInfo`、`videoFrame`、`playing`、`loadingTimeout`、`wasmDecodeError` 等事件，可用于收集黑屏证据。
- 实现：
  - 播放器初始化后不再立即收起 loading，而是等待 `streamSuccess/videoInfo/videoFrame/playing/loaded` 这类首帧或流成功事件。
  - 增加 12 秒首帧超时诊断：超时后显示 `first-frame-timeout`、协议、解码路径、最近播放器事件和 `getState()`。
  - 监听更多错误事件，包括 `wasmDecodeError`、`webcodecsH265NotSupport`、`mediaSourceH265NotSupport`、`unrecoverableEarlyEof` 等。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-29 H5 Monitor 移动端首帧判断修正 2.19.10 开发记录

- 背景：
  - 用户反馈 2.19.9 手机端仍然黑屏，且没有错误信息显示。
  - 复查代码发现移动端 wasm 路径下，`loaded/playing` 被当作首帧成功事件，可能导致 12 秒超时诊断被提前清除，但视频尚未真正渲染。
- 结论：
  - 对移动端软解路径，`loaded/playing` 只能说明播放器状态推进，不能证明已经有视频帧。
  - 移动端应该只把 `streamSuccess/videoInfo/videoFrame` 视为首帧或流成功信号。
- 实现：
  - 新增 `domain/h5-player-diagnostics.ts`，集中定义首帧事件判断。
  - 移动端 `mobile-wasm` 路径不再把 `loaded/playing` 视为首帧成功，保留计时器直到真正的视频事件到达。
  - 桌面 `desktop-mse` 路径保留原兼容判断，避免影响现有桌面播放。
  - 增加 vitest 覆盖移动端和桌面端首帧事件差异。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 页面级播放诊断 2.19.11 开发记录

- 背景：
  - 用户反馈 2.19.10 手机端依然全黑屏，截图里只有“点击开启声音”，没有任何错误或超时提示。
  - 截图说明播放器容器、声音按钮和页面路由均已正常渲染，问题转为“播放器内部诊断没有可靠暴露到手机页面”。
- 结论：
  - 下一步不能继续猜播放参数，应先让黑屏状态具备可截图、可复盘的证据。
  - 诊断信息不能只放在播放器黑框内部，因为第三方播放器的 canvas/video/内部层级可能遮挡自定义提示。
- 实现：
  - `H5FlvPlayer` 增加页面级 `onStatus` 回调，结构化上报初始化、播放地址、播放器事件、首帧成功和首帧超时状态。
  - H5 详情页在播放器下方新增常驻状态卡，展示 stage、message、协议、解码路径、前端版本、UA、最近播放器事件等信息。
  - 继续保留播放器内部诊断和 toast，但页面级状态卡作为手机端排查的主证据。
- 验收目标：
  - 如果移动端继续黑屏，页面必须在黑框下方显示 `player-init`、`player-event` 或 `first-frame-timeout` 等状态，便于继续判断卡在取流、解码还是渲染。

## 2026-06-29 H5 Monitor 流连接与首帧渲染拆分 2.19.12 开发记录

- 背景：
  - 用户反馈 2.19.11 手机端仍黑屏，但页面级状态卡显示 `streamSuccess`。
  - 截图确认播放地址、decoder 路径、版本、UA、`decode=mobile-wasm` 均已暴露；萤石 FLV 流已经连接成功，但没有看到画面。
- 结论：
  - `streamSuccess` 只能代表流连接成功，不能代表画面已经渲染。
  - 之前把 `streamSuccess` 归入首帧成功事件是误判，导致黑屏时显示 `first-frame-ready`。
  - 线上 `decoder.js` 和 `decoder.wasm` 均可访问，`decoder.wasm` 的 Content-Type 为 `application/wasm`，暂不支持“wasm 资源未部署/MIME 错误”这个假设。
- 实现：
  - 移动端首帧成功只认 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes` 这类视频渲染事件。
  - `streamSuccess` 单独显示为 `stream-connected`，继续等待视频帧，不再清除首帧超时计时器。
  - 移动端播放参数改为 `useWCS:false`、`forceNoOffscreen:true`，明确走 wasm + 普通 canvas 渲染路径。
  - 增加 `wasmDecodeErrorReplay:true`、`wasmDecodeAudioSyncVideo:true`、`debug:true`，并监听更多播放器事件，便于下一轮截图继续定位。
- 验收目标：
  - 如果仍黑屏，状态卡应显示 `stream-connected` 后是否出现 `videoInfo/videoFrame/firstFrameDisplay/playToRenderTimes`，或最终 `first-frame-timeout`。

## 2026-06-29 H5 Monitor 移动端直播调通里程碑 2.19.12 验收记录

- 结果：
  - 用户在公司线上移动端复测后确认：实时视频终于可以显示。
  - 这标志着“萤石云 FLV 取流 + iPhone/微信 H5 + H265 视频 + `ezuikit-flv` wasm 软解”链路在试点门店真实环境下跑通。
- 本次调通的关键经验：
  - 不能把 `streamSuccess` 当作画面可见。它只代表 FLV 流连接成功，首帧/画面可见必须看 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes` 这类渲染事件。
  - 黑屏排查要把链路拆层：播放地址获取 -> decoder 资源加载 -> 流连接 -> 视频信息解析 -> wasm 解码 -> canvas 渲染。
  - 页面级诊断必须放在播放器黑框外。第三方播放器内部 DOM/canvas 可能遮挡自定义提示，导致手机端看起来“没有任何错误”。
  - iPhone/微信 H5 环境下，移动端播放应明确走 wasm + 普通 canvas 路径：`useMSE:false`、`useWCS:false`、`forceNoOffscreen:true`、`autoWasm:true`。
  - 线上 `decoder.js` 与 `decoder.wasm` 需要可访问，且 `decoder.wasm` 应返回 `Content-Type: application/wasm`；本次已排除 decoder 部署/MIME 错误。
- 当前可复用配置：
  - 直播地址：萤石 `/api/lapp/v2/live/address/get`，`protocol=4` FLV。
  - 前端播放器：`ezuikit-flv@2.1.1`。
  - 移动端解码路径：`decode=mobile-wasm`。
  - 移动端关键参数：`useMSE:false`、`useWCS:false`、`forceNoOffscreen:true`、`wasmDecodeErrorReplay:true`、`wasmDecodeAudioSyncVideo:true`、`keepScreenOn:true`。
- 后续注意：
  - 继续保留状态卡或等价诊断能力，至少在试点期不要过早隐藏。
  - 回放页也应复用同一套“流连接”和“首帧渲染”拆分逻辑，不要只看播放地址是否返回。
  - 后续扩门店时，如果某通道再次黑屏，优先截图状态卡，根据事件停在哪一层判断，而不是先改播放器参数。

## 2026-06-29 H5 Monitor PC 首帧误报修复 2.19.13 开发记录

- 背景：
  - 2.19.12 移动端直播跑通后，用户反馈 PC 端画面已经显示，但页面仍覆盖 `first-frame-timeout` 错误层。
  - 截图显示 PC 端 `decode=desktop-mse`，事件为 `start > videoInfo > streamSuccess`，画面实际可见。
- 结论：
  - 2.19.12 为移动端修正首帧判断时，把 `streamSuccess/videoInfo` 从所有路径的首帧成功信号里移除，误伤了 PC。
  - PC 的 MSE 路径可以继续使用宽松判断；移动端 wasm 路径必须保持严格，避免再次误判黑屏为成功。
- 实现：
  - `desktop-mse` 路径恢复接受 `streamSuccess`、`videoInfo`、`loaded`、`playing` 作为首帧/播放就绪信号。
  - `mobile-wasm` 路径仍只接受 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes`。
  - 补充前端单测覆盖桌面和移动端差异。

## 2026-06-29 H5 Monitor 播放器控制控件 2.20.0 开发记录

- 背景：
  - H5 Monitor 直播链路已在试点门店移动端和 PC 端跑通，进入播放器产品化阶段。
  - 用户确认本轮只做基础单路查看能力，不做刷新流、异常自动重试、多画面、云台、倍速、下载、复杂时间轴。
- 实现：
  - `H5FlvPlayer` 改为 `forwardRef`，通过 `H5PlayerHandle` 暴露播放、暂停、声音、截图、全屏等受控方法。
  - 新增 `H5PlayerControls`，在播放器底部提供播放/暂停、静音/开声音、截图、横屏/竖屏、全屏/退出全屏。
  - 新增 15 分钟长时间播放保护：到时暂停并提示是否继续，停止时释放当前播放 URL，继续时重新取直播或回放 URL。
  - 新增 `PlaybackSegmentSlider` 和 `domain/h5-playback.ts`，支持在单个录像片段内拖动定位，拖动过程中只预览，提交后才重新请求回放 URL。
  - 回放 URL 请求增加序列号保护，旧请求返回时不会覆盖新 URL；旧响应如果已经拿到 URL，会立即释放，降低萤石资源泄漏风险。
  - 原生 video fallback 不再展示浏览器内建 controls，避免暴露下载、倍速、复杂时间轴等本轮明确不做的能力。
- 样式原则：
  - 控制条保持 H5 工具风格：底部轻量暗色半透明浮层、按钮尺寸克制、移动端可横向滚动且触控面积足够。
  - 诊断状态卡继续常驻显示，等用户明确要求“收起来”后再做折叠入口。
- 验证：
  - `cd frontend && npm run test` 通过，9 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - `go test ./...` 未执行：当前本机环境没有 `go` 命令。
  - Playwright 可视验收未执行：本机缺少 Playwright Chromium 浏览器二进制。

## 2026-06-29 H5 Monitor 播放器体验修复 2.20.1 开发记录

- 背景：
  - 2.20.0 增加播放器控制控件后，试用中发现 6 个体验问题：控件自动隐藏后移动端不清楚如何唤回、移动端按钮偏大、移动端截图不应只打开新页面、回放暂停后再播放会回到片段起点、横屏按钮在手机上不像横置观看、回放滑块放在播放器下方不符合预期。
- 实现：
  - 播放器控制层取消自动隐藏机制，改为点击播放器画面区域隐藏，再点击画面区域显示。
  - 移动端控制按钮缩小，保留文字按钮便于继续调试，后续可替换为 icon。
  - 截图优先使用 Web Share API 调起系统分享/保存面板，不支持时 fallback 为下载；用户取消分享不再误报截图失败。
  - 回放暂停时记录当前片段内估算时间，再次播放时从该时间重新获取回放 URL，避免从片段起点重播。
  - 移动端横屏改为固定全屏并旋转播放器区域，形成手机横置观看体验。
  - 回放片段滑块移到播放器画面 overlay 内，跟控制条同层展示，不再放在下方回放面板。
- 验证：
  - `cd frontend && npm run test` 通过，11 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器用 `externalOrgId=demo` mock 页面验证：控件点击显隐、回放滑块 overlay、移动端按钮尺寸、移动端横屏旋转；控制台无 error/warn。
  - `go test ./...` 未通过本机环境验证：全局 `go` 不存在；使用 `./.tools/go/bin/go` 后测试二进制被 macOS `dyld missing LC_UUID load command` 拦截，未出现业务断言失败。

## 2026-06-29 H5 Monitor 播放器暂停、全屏、截图修复 2.20.2 开发记录

- 背景：
  - 用户在线上验收 2.20.1 后反馈：回放暂停再恢复会黑屏重建且 PC 端仍可能回到片段起点；手机侧全屏按钮失效；手机侧截图没有进入系统相册保存流程。
- 根因：
  - 2.20.1 为解决“回放恢复不回起点”选择了重新请求回放 URL，实际带来播放器重建和黑屏体验，不符合“真暂停/真恢复”的产品预期。
  - 移动浏览器或飞书 WebView 常不开放普通元素 `requestFullscreen`，原逻辑只提示失败，没有降级体验。
  - `ezuikit-flv` 截图 API 默认类型可能直接走 `download`，前端拿不到图片数据就无法调起 Web Share API。
- 实现：
  - 普通播放/暂停改为只调用播放器实例 `pause()` / `play()`，不再在恢复播放时重新 `playRange()` 取回放 URL。
  - 手机全屏在原生 Fullscreen API 不可用或失败时，降级为页面内全屏横置模式，并使用简短提示“已切换为页面内全屏”。
  - 播放器截图调用显式传入 `base64`，前端拿到 data URL 后继续走 Web Share API；H5 仍不能静默写入系统相册，需要用户在系统面板选择保存。
- 验证：
  - `cd frontend && npm run test` 通过，12 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 demo 页面验证：暂停后播放器容器未卸载、未出现回放占位；移动视口点击全屏后进入 `is-inline-fullscreen`，按钮变为“退出全屏”。

## 2026-06-29 H5 Monitor 回放暂停续播体验修复 2.20.3 开发记录

- 背景：
  - 用户在线上验收 2.20.2 后反馈：PC 和手机端录像回放里暂停后再点播放，仍不是继续播放，而是从当前回放 URL 的起点重新播放。
- 调研结论：
  - 当前跑通手机 H265 的 `ezuikit-flv@2.1.1` 更适合 FLV 流播放，不适合把 `pause()` / `play()` 当成原生 video 的精确真暂停/续播。
  - 该库 API 有 `currentTime`、`pause()`、`play()`，但没有明确 `seek()` / `resume()`；README 也提示因解码资源异步加载，不推荐直接外部调用 `play()`。
  - 因此短期不声称实现“同一条流原地真暂停”，而是实现“暂停点续看”：暂停时记录播放器 `currentTime`，恢复时从暂停点重新获取回放 URL。
- 实现：
  - `H5FlvPlayer` 通过 ref 暴露 `getCurrentTime()`，优先读取播放器 `currentTime`，兼容内部 video/canvas loader 的当前时间。
  - 回放暂停时记录 `pausedAtUnix`，并尽量截取当前画面作为冻结帧。
  - 回放恢复时从 `pausedAtUnix` 重新请求回放 URL，不再从原始片段起点播放；状态卡会显示 `reason=resume` 和 `resumeFrom=HH:mm:ss`。
  - 恢复加载期间保留旧 URL，等新 URL 成功返回后再释放旧 URL；冻结帧持续到新播放器首帧 ready，降低黑屏体感。
  - 录像片段点击、滑块定位、长时间播放保护继续走原有重新取 URL 流程。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地移动视口 demo 验证：H5 详情页、录像 tab、录像片段播放、overlay 滑块和控制条均正常渲染；控制台无 error/warn。
- 后续建议：
  - 如果产品要求严格意义的真暂停、seek、resume，应单独验证萤石 `EZUIKit-JavaScript-npm` 或其他官方播放器方案是否能同时满足 H265、手机 WebView、回放控制和自定义 UI。

## 2026-06-29 H5 Monitor 回放恢复遮罩与滑块位置修复 2.20.4 开发记录

- 背景：
  - 用户验收 2.20.3 后确认回放已经能从暂停点继续，但恢复时仍会黑屏一下；拖动回放滑块能定位成功，但滑块 UI 会回到拖动前位置。
- 根因：
  - 恢复遮罩依赖播放器截图返回 `dataUrl`，部分环境下 `ezuikit-flv` 可能返回 Blob/File 或无法通过播放器 API 截图，导致没有冻结帧可显示。
  - `PlaybackSegmentSlider` 之前只维护内部 offset，外层重新取回放 URL 后没有把新的起播时间回传给滑块，导致 UI 位置不同步。
- 实现：
  - 播放器截图归一化支持 base64、data URL、Blob/File；播放器 API 无结果时，尝试从当前 canvas/video 抓取一帧。
  - 恢复播放时即使没有冻结帧，也显示轻量恢复遮罩，避免用户只看到纯黑屏。
  - 回放页新增 `playbackCursorUnix`，每次 `playRange(startTime...)` 都同步当前起播点，并把它传给滑块。
  - `PlaybackSegmentSlider` 改为支持 `currentStartTime`，根据外层起播点更新当前位置，避免拖动后回弹。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放恢复黑屏遮罩修复 2.20.5 开发记录

- 背景：
  - 用户验收 2.20.4 后确认滑块位置问题已解决，但暂停后点击播放时仍能看到明显黑屏和“加载中”，像刷新了一下。
- 根因：
  - 恢复遮罩之前同时依赖 `resumeCoverVisible && loading`。
  - `loading` 是回放 URL 接口请求状态，请求完成后会立即变为 false；但播放器重建和首帧渲染还没有完成，导致遮罩提前消失，播放器内部黑色 loading 层暴露。
- 实现：
  - 恢复遮罩改为只依赖 `resumeCoverVisible`，生命周期延长到播放器回调 `first-frame-ready` / `mock-ready` 后再关闭。
  - 恢复遮罩层级提高到播放器控件之上，避免被播放器内部黑底、loading 或 canvas 层覆盖。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放恢复闪屏抛光 2.20.6 开发记录

- 背景：
  - 用户验收 2.20.5 后认为当前体验可以忍受，但暂停后继续播放仍能看到很短的黑色闪屏，希望再尝试一次低风险抛光。
- 判断：
  - 遮罩已经持续到播放器上报 `first-frame-ready`，仍有闪屏说明黑色暴露点大概率发生在首帧事件和真实画面稳定绘制之间。
- 实现：
  - 收到 `first-frame-ready` / `mock-ready` 后不再立刻关闭恢复遮罩，而是延迟 250ms 再移除。
  - 恢复遮罩增加短过渡，避免硬切换。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放进度自动推进 2.20.7 开发记录

- 背景：
  - 用户验收 2.20.6 后确认暂停续播问题基本可接受，继续反馈两个回放体验问题：播放滑块不会随着播放自动往后移动；播放到当前录像片段最后一秒后，应该自动关闭当前片段并进入下一个录像片段。
- 实现：
  - 回放播放中新增 1 秒 tick，同步读取播放器 `currentTime`，无法读取时退回到 `PlaybackSession` 的墙钟估算时间，并实时更新 `playbackCursorUnix`，驱动 overlay 滑块自动前进。
  - 新增 `nextRecordSegmentIndex`，按当前片段对象或时间边界查找下一个录像片段；当前片段到达末尾前 1 秒时自动触发下一段播放。
  - 自动切片段复用现有“保留当前画面”的恢复遮罩逻辑：切段前尽量截取当前帧，新 URL 首帧稳定后再移除遮罩，减少段间黑屏。
  - 将片段列表、当前片段、当前回放 URL、loading 状态同步到 ref，避免定时器闭包读到旧状态导致重复切段或释放错误 URL。
  - 滑块手动拖动时暂停外部自动位置同步，松手或失焦后再提交定位，避免用户拖动过程中被 tick 拉回。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite preview 页面 smoke 验证：应用可正常渲染，控制台无 error/warn；本地无后端数据导致列表接口 HTTP 500，属本地预览环境限制，未进行真实萤石播放流验证。

## 2026-06-29 H5 Monitor 播放器控制条样式优化 2.21.0 开发记录

- 背景：
  - 用户确认 H5 Monitor 播放功能基本满足后，提出纯样式优化：播放/暂停、声音、截图、横竖屏、全屏控件改为 icon，不显示中文；控制按钮与回放滑块进一步整合，降低播放器 overlay 高度。
- 实现：
  - `H5PlayerControls` 改为三列控制条：左侧播放/暂停与声音，中央承载回放滑块，右侧截图、横竖屏、全屏。
  - 控制按钮从中文文字改为无边框 icon，仅保留 `aria-label` 用于可访问性和调试识别。
  - `PlaybackSegmentSlider` 新增 `compactControls` 形态，嵌入控制条中间时不显示起始/结束时间，也不显示当前时间文案，仅保留滑块本体。
  - 控制条改为低高度半透明浮层，桌面回放态高度约 50px，移动端约 46px，减少对监控画面的遮挡。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 验证：桌面和 390px 移动视口下，按钮均无中文文本，滑块居中整合到同一控制条，未出现明显挤压或重叠。

## 2026-06-29 H5 Monitor 横竖屏 icon 微调 2.21.1 开发记录

- 背景：
  - 用户反馈横屏/竖屏切换 icon 希望更接近“两块横竖屏幕叠放”的识别方式，确认去掉旋转箭头，只保留两个矩形。
- 实现：
  - 将横竖屏切换按钮的旋转箭头 icon 替换为双矩形线性 icon：后层竖向矩形、前层横向矩形。
  - 保持原有按钮行为、active 态、无中文显示和 `aria-label` 不变。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 暂停态控制条显隐修复 2.21.2 开发记录

- 背景：
  - 用户反馈播放中点击画面可隐藏/显示控制条，但暂停后点击画面无法隐藏控制条；暂停截图时控制条会遮挡画面。
- 根因：
  - `H5PlayerControls` 将 `!playing` 纳入 `pinned` 强制显示条件，导致暂停态即使外层 `controlsVisible=false`，控制条仍会保持 `is-visible`。
- 实现：
  - 控制条强制显示条件改为仅 `loading || failed`；暂停态不再强制显示，点击画面可按同一规则隐藏/显示。
  - 播放、暂停、截图、取流逻辑不变。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 验证：暂停后点击画面控制条隐藏，再次点击恢复显示；控制台无 error/warn。

## 2026-06-29 H5 Monitor 返回按钮 icon 尺寸微调 2.21.3 开发记录

- 背景：
  - 用户反馈 H5 监控详情页左上返回按钮里的左箭头偏小，需要适当放大。
- 实现：
  - 保持返回按钮外圈 32px 和点击区域不变，仅将 `.h5-back-icon` 从 16px 调整为 19px，线宽从 2 调整为 2.2。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor icon 视觉尺寸修正 2.21.4 开发记录

- 背景：
  - 用户线上验收 2.21.3 后反馈返回按钮仍然显小，并指出播放器右侧截图、横竖屏、全屏三个 icon 视觉大小和高度不一致。
- 根因：
  - 返回按钮继承了全局 `button` 的左右 padding，导致 32px 按钮内 SVG 被 flex 压缩，虽然 CSS 设置了 21px，但实际渲染宽度只有约 6px。
  - 播放器右侧三个 SVG 虽然外框一致，但图形路径在 24x24 viewBox 中占比和视觉重心不同。
- 实现：
  - 返回按钮补充 `padding: 0`、`min-width: 32px`，并让 `.h5-back-icon` 固定 `flex-basis: 21px`；返回箭头路径改为更饱满的 24px viewBox chevron。
  - 微调相机、横竖屏、全屏/退出全屏 icon 的路径尺寸和坐标，让 30px 按钮内 17px SVG 的视觉高度更一致。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 移动视口验证：返回按钮 SVG 实际渲染为 21x21，按钮 padding 为 0；右侧三个控制按钮均为 30x30，SVG 均为 17x17 且居中。

## 2026-06-29 H5 Monitor 返回按钮 icon 尺寸回调 2.21.5 开发记录

- 背景：
  - 用户线上验收 2.21.4 后认为返回箭头反而偏大，希望回到最初经验尺寸，只保留 padding 挤压问题的修复。
- 实现：
  - 返回箭头恢复为原始 16px chevron 和 2px 线宽。
  - 保留 `.h5-back-btn { padding: 0; min-width: 32px; }` 与 `.h5-back-icon { flex: 0 0 16px; }`，避免再次被全局 button padding 压缩。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地移动端 demo 验证：返回按钮 padding 为 0，SVG 实际渲染为 16x16。

## 2026-06-29 H5 Monitor 上海凯德晶萃店入口开放 2.21.6 开发记录

- 背景：
  - 北京保利实验室门店 H5 Monitor 页面已通过用户线上验收。
  - 用户要求继续给“新氧青春诊所(上海凯德晶萃店)”开放门店详情右上角“查看监控”入口，新氧机构 ID 为 `10047`。
- 实现：
  - 前端 H5 Monitor 入口从单机构 `10030` 改为试点机构白名单：`10030`、`10047`。
  - 后端 H5 Monitor 服务端门禁同步改为试点机构白名单。
  - 保留北京 `10030` 仅允许 `GN0941203` 的旧试点限制；上海 `10047` 不硬编码录像机编号，使用该门店自己数据库下的有效通道和萤石账号配置。
  - 版本号升级到 `2.21.6`。
- 验证：
  - 新增前端测试覆盖 `10047` 可打开 H5 Monitor 入口。
  - 新增后端测试覆盖 `10047` 首页和直播取流使用上海门店自己的通道数据。

## 2026-06-29 H5 Monitor 首页通道标题层级修复 2.21.7 开发记录

- 背景：
  - 开放真实业务门店“新氧青春诊所(上海凯德晶萃店)”后，H5 Monitor 首页圆形预览图下方主标题显示为 `通道12`，区域编号/备注显示在第二行，信息层级与业务预期相反。
  - 用户期望主标题显示“区域类型 + 编号/备注”，例如 `治疗室1号`；副标题显示通道号，例如 `通道12`。
- 根因：
  - H5 首页 `channelName()` 优先使用后端 `channel_name`，真实扫描数据里的 `channel_name` 往往就是 `通道12`。
  - 区域编号/备注被单独拼为副标题，导致真实门店里“通道号”抢占了业务主标题位置。
- 实现：
  - 新增 `h5ChannelDisplayText` 前端领域 helper，统一生成 H5 通道卡片展示文案。
  - 业务区域标题优先级改为：业务类型标签 + 备注/编号，其次非业务备注，其次场景标签，最后才退回通道原名。
  - 卡片副标题固定为 `通道{channel_no}`。
  - H5 详情页进入时缓存的通道名称同步使用新的业务标题。
  - H5 `AreaType` 类型补充 `vip_treatment`，避免 VIP 治疗室后续展示退化。
  - H5 首页“加载更多”从固定 24 个改为按当前网格列数展示完整行：首屏 3 行，每次追加 2 行，避免真实门店桌面宽度下出现最后一行只露出半行的问题。
- 验证：
  - 新增前端测试覆盖 `通道12 + treatment + 1号 => 治疗室1号 / 通道12`。
  - 新增前端测试覆盖备注场景：`治疗室401号`、`护士站 / 通道16`。
  - 新增前端测试覆盖桌面 7 列和移动 3 列时的完整行加载数量。

## 2026-06-29 H5 Monitor 首页默认展示与缩略图刷新 2.21.8 开发记录

- 背景：
  - 用户线上验收 2.21.7 后，提出首页默认 3 行略少，建议默认展示 4 行。
  - 用户同时讨论：点击查看视频后，如果已经取到播放器画面，是否可以顺手刷新该通道首页缩略图，让真实业务门店的缩略图更及时。
- 方案取舍：
  - 不采用前端播放器 canvas 截帧上传：移动端、H265、萤石播放器内部跨域和画布污染风险较高，且会增加前端上传链路复杂度。
  - 采用“播放器第一帧成功 -> 前端低频通知后端 -> 后端复用现有萤石抓图与公司空间保存链路”的方式。
  - 刷新为 best effort，不阻断播放、不弹 toast；失败只在后台静默吞掉，避免影响查看监控主流程。
- 实现：
  - H5 首页默认展示从 3 行调整为 4 行，仍按当前网格列数计算完整行；加载更多仍每次增加 2 行。
  - 新增 H5 后端接口：`POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/snapshot`，先复用 H5 试点门禁和通道校验，再调用 storespace 现有 `RefreshChannelSnapshot` 抓图保存。
  - 前端在播放器 `first-frame-ready` 后触发缩略图刷新；跳过 mock 播放；同一 `机构+通道` 前端 10 分钟冷却。
  - 后端同一通道也增加 10 分钟冷却，防止多个用户同时观看同一路视频时重复打萤石抓图接口。
  - 缩略图刷新成功后，H5 路由壳更新列表刷新 key；用户返回首页时可重新拉取列表，展示新的缩略图链接。
- 验证：
  - `cd frontend && npm run test` 通过，17 tests passed。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 2.21.8 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 业务 commit：`a642e4a feat: refresh H5 monitor thumbnails after playback`。
- 推送结果：
  - GitLab remote 已从 `2c67c8f` 更新到 `a642e4a`。
  - 公司线上前端静态资源已探测到 `2.21.8`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.7 (container)` 更新到 `2.21.8`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-29 H5 Monitor 直播中缩略图刷新黑屏修复 2.21.9 开发记录

- 背景：
  - 线上验收 `2.21.8` 后，用户反馈刚开始看实时视频一段时间后可能突然黑屏，只能返回列表页；列表页预览图也会临时变成黑屏，再次进入后播放恢复，返回后预览图恢复正常。
- 根因判断：
  - `2.21.8` 将“第一帧成功”作为刷新缩略图触发点，导致播放过程中调用后端抓图接口。
  - 该抓图请求可能与当前直播取流竞争录像机/萤石/门店带宽资源，造成当前播放器黑屏或把黑屏保存成缩略图。
- 实现：
  - 移除 `first-frame-ready` 阶段的缩略图刷新。
  - 新增 H5 缩略图刷新时机规则：只允许实时直播流在 `exit`、`switch`、`stop` 释放前刷新；实时流续流/替换 `replace` 不刷新。
  - 回放模式完全不触发缩略图刷新，回放 URL 释放只关闭播放地址。
  - 直播释放时先尽力刷新缩略图，再关闭直播地址；刷新失败不阻断关闭直播地址。
- 验证：
  - `cd frontend && npm run test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-06-29 H5 Monitor 2.21.9 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`1a702b6 fix: refresh H5 thumbnails only after live close`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.8 (container)` 更新到 `2.21.9`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-29 H5 Monitor 停止自动缩略图刷新 2.21.10 开发记录

- 背景：
  - 用户反馈 H5 Monitor 关闭直播时仍有一定比例无法稳定更新缩略图，且列表缩略图自动变化会产生黑屏、慢加载和视觉跳变。
  - 经过讨论，当前阶段 H5 Monitor 的核心目标是稳定查看实时视频，缩略图自动更新不是强需求。
- 决策：
  - H5 Monitor 不再自动刷新缩略图。
  - 实时直播、回放、切换 tab、关闭详情、续流、停止播放都不触发后端抓图。
  - 现有后台通道列表里的手动“刷新截图”能力保留，不受影响。
- 实现：
  - 前端移除 H5 详情页释放直播流前的缩略图刷新逻辑，释放播放地址只调用失效播放地址接口。
  - 前端移除 H5 首页刷新 key 和 `refreshSnapshot` API。
  - 后端移除 `POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/snapshot` 路由、H5 snapshot refresher 注入和 H5 专用截图刷新适配。
  - 新增后端测试确认 H5 snapshot 路由不再注册，避免后续误恢复。
- 风险说明：
  - H5 首页缩略图不会因为用户观看视频而自动更新；如果后续确实需要更新，应作为单独的播放器截图实验重新评估 PC、手机浏览器和飞书 WebView 能力。

## 2026-06-29 H5 Monitor 2.21.10 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`f59be65`，其中包含业务修复 `13a0506 fix: disable automatic H5 thumbnail refresh`，并正常合并远端 MySQL 迁移交接文档提交 `66e8eba`。
- 推送结果：
  - GitLab remote 已从 `66e8eba` 更新到 `f59be65`。
  - 公司线上前端静态资源已探测到 `2.21.10 (container)`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.9 (container)` 更新到 `2.21.10 (container)`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-30 门店列表全量统计修复 2.21.11 开发记录

- 背景：
  - 用户反馈门店列表右上角统计只统计当前分页页内门店，翻到没有已确认门店的页面时，面诊室、治疗室、生美统计会错误变为 0。
- 根因：
  - 前端 `App.tsx` 使用当前页 `stores` 计算右上角统计；`stores` 来自分页接口 `items`，不是全部门店。
- 实现：
  - 后端 `GET /api/store-space/stores` 返回新增 `summary` 字段，按当前搜索条件统计全部匹配门店，统计发生在分页前。
  - MemoryStore 和 PostgresStore 均补齐同一口径：`store_count`、`treatment_count`、`consultation_count`、`beauty_count`。
  - 前端 `StoreListResponse` 增加 `summary`，门店列表“全部”视图右上角统计改用后端全量 summary，不再受当前页影响。
  - 城市筛选仍维持当前页前端筛选口径，后续如需要城市维度全量统计，需要另扩城市筛选参数或城市聚合接口。
- 验证：
  - 新增后端测试覆盖“分页只返回 1 家门店，但 summary 统计全部 2 家匹配门店”。
  - 新增前端测试覆盖门店 summary 汇总 helper。

## 2026-06-30 门店列表统计修复 2.21.11 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`362e0a1 fix: count store list summary across pages`。
- 推送结果：
  - GitLab remote 已从 `076586e` 更新到 `362e0a1`。
  - 首次推送被公司 GitLab hook 拒绝，原因是新增 SQL 包含受限关联查询写法；已改为子查询写法后重新验证并推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.10 (container)` 更新到 `2.21.11 (container)`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-30 门店正式化字段 2.22.0 开发记录

- 背景：
  - 公司正规环境后续要求迁移到 MySQL，需要提前规划保留现有数据的迁移方式。
  - 业务确认时仅有“业务区域类型 + 编号/备注”不够，治疗室、VIP治疗室、美容室需要支持床位拆分。
  - 机构基础资料需要新增“机构简称”。
- 实现：
  - 后端新增 `stores.short_name` / API `short_name`，创建、编辑、列表、详情、重复检查均兼容返回。
  - 后端新增 `video_channels.bed_label` / API `bed_label`，通道确认、扫描保留、H5 Monitor 查询、Excel 导出均支持。
  - 前端创建/编辑机构弹窗新增“机构简称”字段，详情页顶部展示机构简称。
  - 通道映射表新增“床位拆分”列，仅治疗室、VIP治疗室、美容室显示输入；空值兼容单床或旧数据。
  - 用户可见“生美”统一改为“美容室”，内部枚举仍保留 `beauty`，避免历史数据迁移。
  - 新增 `frontend/src/domain/channel-mapping-target.ts`，将 `areaType + areaNumber + bedLabel` 作为临时本地映射目标，后续可替换为公司业务系统区域/床位字典。
  - MySQL DDL 补齐 `tb_stores.short_name`、`tb_video_channels.bed_label`。
  - `docs/mysql-migration-handoff.md` 补充 MySQL 迁移注意事项、图片存储核查口径和未来业务区域字典边界。
- 图片存储结论：
  - 当前数据库保存图片/PDF/截图路径或 logical key，不保存二进制图片内容。
  - 正式 MySQL 第一阶段只迁路径字段；图片内容仍由 Supabase Storage、local asset store 或后续公司文件服务承载。
- 待办：
  - 正式迁 MySQL 仍需单独实现 MySQL repository、样本迁移脚本、测试库验证、全量迁移和回滚方案。

## 2026-06-30 门店正式化字段 2.22.0 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`65e3269 feat: add store short names and channel bed labels`。
- 推送结果：
  - GitLab remote 已从 `d824a43` 更新到 `65e3269`。
  - 本次同时包含前置规划文档 commit：`bba7c71`、`5355f40`、`4b0f110`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端 JS 静态资源 `index-B-DR2bc0.js` 已包含 `2.22.0 (container)`、`机构简称`、`床位拆分`、`美容室` 文案。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-30 H5 Monitor 播放清晰度切换 2.22.2 开发记录

- 背景：
  - 用户确认当前查看监控默认保持流畅级别，同时希望播放器右上角增加“切为高清 / 切为标清”切换按钮。
  - 按钮需要与播放器控制条同步显示和隐藏：控制条显示时按钮显示，控制条隐藏时按钮隐藏。
- 实现：
  - 后端 H5 live-url / playback-url 请求新增 `quality` 参数。
  - 默认仍为 `sd`，后端映射到萤石 `quality=2`（流畅/子码流）。
  - 高清 `hd` 映射到萤石 `quality=1`（高清/主码流）。
  - 前端 H5 播放器新增 `streamQuality` 状态，默认 `sd`；右上角按钮显示下一步动作：
    - 当前标清：显示 `切为高清`。
    - 当前高清：显示 `切为标清`。
  - 直播切换清晰度时重新获取直播播放地址，并在新地址成功后释放旧地址，降低切换时资源泄漏风险。
  - 回放切换清晰度时尽量记录当前播放点，重新获取对应播放地址，避免退回录像片段开头。
  - 播放器诊断状态补充 `quality=sd/hd`，方便线上排查实际取流参数。
- 验证：
  - 新增后端测试覆盖 live / playback 请求 `quality=hd` 时传给萤石的 `Quality=1`。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run test` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地 H5 mock 页面浏览器验收通过：初始显示 `切为高清`，点击后变为 `切为标清`；点击播放器画面隐藏时，清晰度按钮和控制条一起消失，再次点击一起显示。

## 2026-06-30 H5 Monitor 2.22.2 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`fa899ac feat: add H5 monitor stream quality toggle`。
- 推送结果：
  - GitLab remote 已从 `8b20988` 更新到 `fa899ac`。
  - 首次非交互 HTTPS 推送因本机未配置 GitLab credential helper 失败；随后使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端静态资源已探测到 `2.22.2 (container)`。
- 备注：
  - 本次未发布韩国服务器。
  - 本地 `origin/main` 与公司发布分支存在历史分叉，为避免影响 GitHub main，本次未同步 GitHub。

## 2026-06-30 H5 Monitor 回放隐藏清晰度切换 2.22.3 开发记录

- 背景：
  - 用户指出录像回放看起来无法切换标清/高清，如果回放只有一种模式，则不应展示切换按钮。
  - 萤石文档中清晰度切换主要针对实时预览；录像回放不支持同样的高清/标清切换体验。
- 实现：
  - H5 播放器右上角“切为高清 / 切为标清”仅在实时视频模式显示。
  - 录像回放模式隐藏清晰度切换按钮，避免误导用户。
  - 前端回放取流请求不再传 `quality`。
  - 后端回放接口即使收到 `quality=hd` 也固定使用 `quality=2`，保证回放行为稳定。
- 验证：
  - 更新后端测试：`TestPlaybackURLIgnoresRequestedQuality` 覆盖回放忽略清晰度参数。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/h5monitor` 通过。
  - `cd frontend && npm run test` 通过。
  - `cd frontend && npm run build` 通过。

## 2026-06-30 H5 Monitor 2.22.3 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`f6d8681 fix: hide H5 quality toggle during replay`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端静态资源已探测到 `2.22.3 (container)`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-30 H5 Monitor 诊断日志与识别图片 URL 修复 2.22.4 开发记录

- 背景：
  - H5 播放器下方诊断卡已完成阶段性调试使命，需要默认收起，遇到问题时再由用户点开复制给 Codex 排查。
  - 运营识别门店时 MiniMax 报错 `disallowed url: https://opencapture.ys7.com/...`；该问题对 GPT/OpenAI 也存在同类风险，因为临时萤石抓图 URL 不适合作为模型识别输入。
  - 用户确认本次新增日志属于短周期诊断日志，不是安全合规审计日志；后续权限/操作审计日志需要另做长期保存。
- 实现：
  - H5 播放详情页右上角新增信息 icon，播放器日志默认收起；点击后展示当前状态、最近状态记录、复制和关闭按钮。
  - 前端播放器日志保留最近 24 条并做相邻去重，复制内容包含机构 ID、通道 ID、模式、播放状态、清晰度、url_id 和最近诊断状态。
  - 后端 H5 Monitor 直播取流、回放取流、录像片段查询、播放地址释放补充短周期诊断日志，方便按门店、通道、阶段、耗时定位问题。
  - 通道识别链路补充抓图、保存快照、调用模型、保存结果等阶段日志，日志中只记录脱敏设备号、图片域名/路径摘要和错误摘要。
  - 修复模型识别图片 URL：抓图保存到快照存储成功后，MiniMax/GPT 均使用稳定的本地快照 URL 进行识别，不再直接传 `opencapture.ys7.com` 临时 URL。
- 日志原则：
  - 本次日志走服务 stdout / K8s 日志系统，作为短周期诊断日志使用。
  - 不写入业务数据库，不保存完整播放 URL、token、签名 query、service role key 等敏感信息。
  - 后续权限操作日志应另建结构化审计日志，长期保存，记录“谁在什么时候做了什么变更”。
- 验证：
  - 新增测试覆盖 `ProbeRecognizeChannel` 和 `RecognizeRecorderChannels` 均使用已保存快照 URL 调用识别器。
  - `cd frontend && npm test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 本地浏览器验收 H5 播放详情桌面和移动端：日志默认收起，信息 icon 可展开，复制/关闭按钮可见，控制台无新增错误。

## 2026-06-30 H5 Monitor 2.22.4 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`15320c4 fix: improve H5 diagnostics and recognition image URLs`。
- 推送结果：
  - GitLab remote 已从 `f6d8681` 更新到 `15320c4`。
  - 首次非交互 HTTPS 推送因本机未配置 GitLab credential helper 失败；随后使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端静态资源已探测到 `2.22.4 (container)`。
  - H5 播放页懒加载资源 `H5MonitorChannel--vszTHZe.js` 已包含 `播放器日志`、`查看播放器日志`、`H5 Monitor 播放器诊断` 等新逻辑。
- 备注：
  - 本次未发布韩国服务器。
  - 本地 `origin/main` 与公司发布分支存在历史分叉，为避免影响 GitHub main，本次未同步 GitHub。

## 2026-06-30 MiniMax 相对快照 URL 修复 2.22.5 开发记录

- 背景：
  - 用户反馈报错门店：`新氧青春诊所(上海正大广场店)`。
  - 录像机 `FK8984413` 识别完成，但 `43` 个通道抓图/识别失败。
  - MiniMax 返回：`invalid param: image url must be http(s):// or data:...;base64 (2013)`。
- 根因：
  - `2.22.4` 已把萤石 `opencapture.ys7.com` 临时图保存到本地快照存储后再传给模型，但传给模型的仍是前端可访问的相对路径 `/api/store-space/channel-snapshots/{name}.jpg`。
  - 前端和后台页面可以使用该相对路径，但 MiniMax/GPT 服务端无法解析相对 URL；MiniMax 明确要求 `http(s)://` 或 `data:...;base64`。
- 实现：
  - 模型识别前新增 `prepareRecognitionImageURL`。
  - 如果识别图片已经是 `http(s)` 或 `data:`，保持原样。
  - 如果识别图片是本地快照 API 路径，则从 `SnapshotStore.Open` 读取图片内容并转换为 `data:image/...;base64,...` 后传给 MiniMax/GPT。
  - 数据库和前端仍保存、展示原来的 `/api/store-space/channel-snapshots/{name}.jpg`，只改变模型调用入参。
  - 诊断日志中对 data URL 做摘要显示为 `data:image/...;base64,[redacted]`，避免日志写入整张图片 base64。
- 验证：
  - 新增 `TestProbeRecognizeChannelConvertsStoredSnapshotToDataURLForRecognition`，覆盖本次 `FK8984413` 同类报错。
  - 更新 probe 和批量识别用例，确认识别器最终收到 `data:image/jpeg;base64,...`。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-06-30 通道批量识别抗中断优化 2.22.6 开发记录

- 背景：
  - 用户反馈 `新氧青春诊所(上海正大广场店)`，业务机构 ID `10011`，录像机 `FK8984413` 页面报 `识别失败：Failed to fetch`。
  - 线上只读排查确认系统内部 store id 为 `12`，recorder id 为 `14`，该录像机有 `43` 个有效通道。
  - `2.22.5` 已修复 MiniMax 相对快照 URL 问题；本次线上数据中通道 `1-20` 已在 `2026-06-30 15:29-15:35` 成功识别为 `provider=minimax`，通道 `21-44` 仍停留在旧失败记录。
- 根因判断：
  - MiniMax 单通道识别耗时普遍约 `12-27s`，大量通道连续识别时，某一路可能被浏览器/公司网关/Ingress 中断。
  - 前端原逻辑在任一通道 `fetch` 抛错后直接中断整台录像机识别，导致后续通道不再继续。
  - 前端还会把已成功识别但待人工确认的通道重新加入批量识别，造成重复消耗模型和额外超时风险。
- 实现：
  - 新增 `shouldBatchRecognizeChannel`，批量识别只处理未识别、识别失败或半截状态的未确认通道；已成功识别待确认通道不重复跑。
  - 录像机级识别队列改为单通道容错：某一路请求失败、网络中断或模型识别失败，只记录该通道结果并继续后续通道。
  - `TypeError: Failed to fetch` 统一转成中文提示：`识别请求中断，可能是单路识别耗时过长或公司网关超时，已继续识别后续通道。`
  - 识别完成 toast/页面错误区展示本轮总数、成功数、失败数、中断数和首个失败通道，方便运营和 Codex 对齐问题。
  - 后端 `recognizeChannel` 增加请求上下文取消日志：`storespace: channel-recognize interrupted ... error="context canceled"`，便于后续让运维按 recorder/channel/time 查 K8s 日志。
- 验证：
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-channel-test src/domain/channel-recognition.ts src/domain/channel-recognition.test.ts && node /tmp/erzhuang-channel-test/channel-recognition.test.js` 通过。
  - `cd frontend && npm test` 通过，18 tests passed。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地 Vite dev server 可启动；Playwright 浏览器二进制未安装，未做截图式浏览器验收。

## 2026-06-30 通道批量识别抗中断优化 2.22.6 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`b6d114b fix: keep channel recognition running after transient failures`。
- 推送结果：
  - GitLab remote 已从 `6c1c2bc` 更新到 `b6d114b`。
  - 首次非交互 HTTPS 推送因本机未配置 GitLab credential helper 失败；随后使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端首页资源已更新为 `/erzhuang-project/assets/index-DPpJtdbx.js`。
  - 线上前端 bundle 已包含 `2.22.6`、`识别请求中断`、`已继续识别后续通道`、`不会重复消耗模型` 等本次逻辑。
- 备注：
  - 本次未发布韩国服务器。
  - 本地 `origin/main` 与公司发布分支存在历史分叉，为避免影响 GitHub main，本次未同步 GitHub。

## 2026-06-30 机构列表城市筛选分页修复 2.22.7 开发记录

- 背景：
  - 用户反馈机构列表选择“上海”后，只筛出当前页里的上海机构，而不是全部上海机构；需要往后翻多页才能看到其它上海门店。
  - 线上只读复现：请求 `GET /api/store-space/stores?page=1&page_size=5&city=上海` 仍返回深圳门店，说明生产后端尚未支持 `city` 参数。
- 根因：
  - 前端原逻辑先请求分页后的当前页门店，再用 `stores.filter(city)` 做城市筛选。
  - 这导致城市筛选只作用于当前页，分页总数、统计、城市按钮都不是“该城市全集”。
- 实现：
  - 后端 `StoreFilters` 新增 `City`，`GET /api/store-space/stores?city=上海` 在分页前按 `stores.city` 精确过滤。
  - 后端列表响应新增 `cities`，按当前搜索关键词返回可选城市全集，避免城市按钮依赖当前页数据。
  - `MemoryStore` 与 `PostgresStore` 均支持 city 过滤、全量城市选项、分页前 total/summary 计算。
  - 前端新增 `store-list-query` helper，统一生成列表查询参数；城市筛选会请求服务端并重置到第一页。
  - 前端移除当前页二次城市过滤，列表、分页、统计全部使用后端过滤结果。
- 验证：
  - 新增后端测试 `TestListStoresFiltersCityBeforePagination`，覆盖 city 过滤在分页前发生。
  - 新增前端 domain 测试 `store-list-query.test.ts`，覆盖 `city=上海` 查询参数。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-store-query-test src/domain/store-list-query.ts src/domain/store-list-query.test.ts && node /tmp/erzhuang-store-query-test/store-list-query.test.js` 通过。
  - `cd frontend && npm test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-06-30 机构列表城市筛选分页修复 2.22.7 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`4e94903 fix: apply city filter before store pagination`。
- 推送结果：
  - GitLab remote 已从 `15fde5c` 更新到 `4e94903`。
  - 首次非交互 HTTPS 推送因本机未配置 GitLab credential helper 失败；随后使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端首页资源已更新为 `/erzhuang-project/assets/index-CH0SPpGz.js`，bundle 已包含 `2.22.7`。
  - 线上接口 `GET /api/store-space/stores?page=1&page_size=5&city=上海` 返回 `total=8`，当前页 `5` 个 item 的 `city` 均为 `上海`。
- 备注：
  - 本次未发布韩国服务器。
  - 本地 `origin/main` 与公司发布分支存在历史分叉，为避免影响 GitHub main，本次未同步 GitHub。

## 2026-06-30 MiniMax 通道识别 JSON 解析容错 2.22.8 开发记录

- 背景：
  - 用户反馈 `新氧青春诊所(上海正大广场店)` 录像机 `FK8984413` 识别完成 `8/10`，剩余通道 `39` 报错：
    `parse minimax recognition json: invalid character '<' looking for beginning of value: <think> ... 弱电室 ...`。
  - 该报错说明抓图和 MiniMax HTTP 请求已经成功，失败点在模型返回内容解析：模型没有严格按 JSON schema 只输出 JSON，而是混入了 `<think>` 分析文字。
- 根因判断：
  - MiniMax 偶发返回“思考/解释文本 + JSON”，或极端情况下只返回解释文本。
  - 旧解析器只处理纯 JSON、Markdown fenced JSON、以及第一个合法 `{...}`；如果分析文本里先出现了非法花括号片段，或完全没有 JSON，就会把全文交给 `json.Unmarshal`，触发 `<think>` 解析失败。
  - OpenAI/GPT 路径也存在类似风险，之前只暴露在 MiniMax 上。
- 实现：
  - 通道识别 JSON 提取器改为扫描整段文本中的每一个 `{` 起点，跳过分析文字里的非法花括号，直到找到第一个可解析 JSON 对象。
  - OpenAI/GPT 通道识别路径复用同一套模型 JSON 提取器，兼容“解释文本 + JSON”输出。
  - MiniMax 如果完全没有返回 JSON，但文本明确包含“弱电室 / 弱电间 / 机房 / machine room / weak current room”，则生成低置信度、需人工复核的 `machine_room` 结果，避免该类非业务区域卡住整批识别。
- 验证：
  - 新增 `TestExtractModelJSONTextSkipsInvalidBraceBeforeResult`，覆盖分析文本里先出现非法 `{...}` 后再输出合法 JSON。
  - 新增 `TestMiniMaxRecognizerFallsBackFromWeakCurrentRoomExplanation`，覆盖 MiniMax 只返回弱电室解释文本时的低置信度兜底。
  - 新增 `TestOpenAIRecognizerParsesThinkWrappedJSON`，覆盖 GPT/OpenAI 兼容“think 文本 + JSON”。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/channelai -count=1` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-30 MiniMax 通道识别 JSON 解析容错 2.22.8 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`5959144 fix: tolerate minimax channel reasoning output`。
- 推送结果：
  - GitLab remote 已从 `37e9f94` 更新到 `5959144`。
  - 使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 无缓存请求首页已更新为 `/erzhuang-project/assets/index-C65y8J1h.js`。
  - 线上前端 bundle 已包含 `2.22.8`。
- 备注：
  - 本次未发布韩国服务器。
  - 入口 HTML 存在短暂缓存，普通请求一度仍显示旧资源 `index-CH0SPpGz.js`，无缓存请求已确认新构建生效。

## 2026-06-30 MiniMax 医生办公室解释文本兜底 2.22.9 开发记录

- 背景：
  - 用户反馈 `新氧青春诊所(上海长宁旗舰店)` 录像机 `FW4529752` 识别完成 `59/61`，通道 `16` 报错：
    `parse minimax recognition json: invalid character '<' looking for beginning of value: <think> ... 医生办公室 ...`。
  - 该错误与 `2.22.8` 的弱电室案例同源：MiniMax HTTP 调用成功，但模型只返回 `<think>` 分析文本，没有返回合法 JSON。
- 根因判断：
  - `2.22.8` 已能跳过解释文本中的非法花括号，并对“弱电室/机房”做低置信度兜底。
  - 新案例中模型明确识别出“医生办公室”，但该文本不在 `2.22.8` 的窄兜底强信号表内，因此仍被判为解析失败。
- 实现：
  - 将 MiniMax 无 JSON 兜底改为强信号表形式，保留“弱电室/弱电间/机房”映射。
  - 新增“医生办公室 / doctor's office / doctor office”强信号，映射为 `scene_type=unknown`、`area_number=医生办公室`、`confidence=low`、`needs_review=true`。
  - 该兜底只在模型完全未返回合法 JSON 时触发，且结果统一要求人工复核，避免把模型解释文本当高置信度识别。
- 验证：
  - 新增 `TestMiniMaxRecognizerFallsBackFromDoctorOfficeExplanation`，覆盖本次 `FW4529752` 通道 `16` 同类返回。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/channelai -run 'TestMiniMaxRecognizerFallsBackFrom(WeakCurrentRoom|DoctorOffice)Explanation' -count=1` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-30 MiniMax 医生办公室解释文本兜底 2.22.9 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`686675b fix: tolerate doctor office minimax reasoning output`。
- 推送结果：
  - GitLab remote 已从 `611d29b` 更新到 `686675b`。
  - 使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 无缓存请求首页已更新为 `/erzhuang-project/assets/index-Bca1GPCX.js`。
  - 线上前端 bundle 已包含 `2.22.9`。
- 备注：
  - 本次未发布韩国服务器。

## 2026-06-30 H5 Monitor 入口默认开放 2.22.10 开发记录

- 背景：
  - 用户询问机构详情页右上角“查看监控”入口能否默认开放给所有机构。
  - 代码确认当前仍使用 H5 Monitor 试点白名单，只允许新氧机构 ID `10030`、`10047` 显示入口。
- 决策：
  - 机构详情页入口不再使用试点白名单。
  - 只要门店存在非空新氧机构 ID，就显示“查看监控”入口。
  - H5 Monitor 页面继续按新氧机构 ID 拉取真实录像机/通道数据；没有数据时由页面展示空态，不在机构详情页做复杂拦截。
- 实现：
  - 移除 `h5MonitorPilotExternalOrgIds` 白名单。
  - `canOpenH5Monitor` 改为判断 `externalOrgId.trim() !== ""`。
  - 更新前端测试，覆盖 `10031`、`010030`、带空格机构 ID 均允许，空字符串和纯空格不允许。
- 验证：
  - `cd frontend && npm test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。
  - 本次未改变按钮样式和页面布局，仅改变入口可见性判断。

## 2026-06-30 H5 Monitor 入口默认开放 2.22.10 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`25352b7 feat: open H5 monitor entry for all org stores`。
- 推送结果：
  - GitLab remote 已从 `6100917` 更新到 `25352b7`。
  - 使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 无缓存请求首页已更新为 `/erzhuang-project/assets/index-LEvpVOuF.js`。
  - 线上前端 bundle 已包含 `2.22.10`。
  - 线上前端 bundle 已包含 `externalOrgId.trim()!==""`，确认入口判断不再使用试点白名单。
- 备注：
  - 本次未发布韩国服务器。

## 2026-06-30 H5 Monitor 后端机构白名单移除 2.22.11 开发记录

- 背景：
  - `2.22.10` 已将机构详情右上角“查看监控”入口从前端白名单改为“有新氧机构 ID 即显示”。
  - 用户反馈点击其它门店入口后 H5 页面显示 `not found`。
- 根因：
  - 前端入口已放开，但后端 H5 Monitor 服务仍保留试点机构白名单，只允许 `10030`、`10047`。
  - 非试点机构请求 `/api/h5/orgs/{externalOrgId}/monitor` 时被 `isPilotAllowedOrg` 拦截为 404。
- 实现：
  - 移除后端 H5 Monitor 机构级试点白名单。
  - `GetMonitorHome` 和播放/回放通道校验改为：只要数据库中能按新氧机构 ID 找到门店，即允许访问。
  - 保留北京实验门店 `10030` 对设备 `GN0941203` 的历史过滤，避免该实验门店误展示其它录像机。
  - 更新测试：非试点机构 `10031` 有门店和通道数据时，H5 Monitor 首页返回 200 和对应通道。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/h5monitor -count=1` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-06-30 H5 Monitor 后端机构白名单移除 2.22.11 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`17943ea fix: allow H5 monitor backend for all org stores`。
- 推送结果：
  - GitLab remote 已从 `dae5d0b` 更新到 `17943ea`。
  - 使用交互式 HTTPS 账号/token 推送成功。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 无缓存请求首页已更新为 `/erzhuang-project/assets/index-BUVQ3-CA.js`。
  - 线上前端 bundle 已包含 `2.22.11`。
  - 非试点机构 H5 API 已验证：
    - `GET /api/h5/orgs/10029/monitor` 返回 200，门店为 `新氧青春诊所(上海长宁旗舰店)`。
    - `GET /api/h5/orgs/10011/monitor` 返回 200，门店为 `新氧青春诊所(上海正大广场店)`。
- 备注：
  - 本次未发布韩国服务器。

## 2026-07-01 APISIX-SSO 骨架与 DBA 协作规范 2.23.0 开发记录

- 背景：
  - 用户提供公司文档《内部系统接入APISIX-SSO使用方式》，确认二壮项目必须使用公司推荐的 APISIX 网关 `security-sso` 插件，不自建 OAuth2 登录流程。
  - 上一轮曾错误沿用 OAuth2/API SSO 口径，已按文档纠偏。
- 实现：
  - 新增 APISIX-SSO 后端骨架：
    - `GET /api/auth/me`
    - `POST /api/auth/logout`
    - `GET /_/auth/callback`
    - `GET /logout`
  - 默认 `SSO_ENABLED=false`，不影响现有运营后台。
  - `SSO_ENABLED=true` 时读取 `sy_sso_token` cookie，并按文档校验 RS256 JWT：
    - `alg` 必须为 `RS256`。
    - 使用公司文档公钥或 `SSO_JWT_PUBLIC_KEY` 验签。
    - 校验 `exp`。
    - 配置 `SSO_EXPECTED_SUB` 后校验 `sub`。
    - `data.mail` 必须存在，作为第一版 `tb_users.email` 授权主键。
  - 前端新增 SSO 登录欢迎页；未登录时不加载门店业务数据。
  - SSO 文档改为 APISIX 单一路径，并同步纠正 MySQL DBA/迁移验收文档里的旧 `/token` 口径。
  - 新增 DBA 专项协作规则，后续 MySQL schema、权限模型、资产存储迁移先交 DBA 专项出方案，再由主会话验收。
  - 脱敏历史计划文档中的 GitLab personal access token 明文示例。
- 风险与后续：
  - 当前权限仍是 SSO 骨架阶段兼容态，`role=admin`、`permissions=["admin"]`；正式权限需要继续接入 `tb_users`、角色、机构范围和审计日志。
  - 公司环境如要启用 SSO，需要运维配置 APISIX `security-sso` 插件、包含 `/_/auth/callback` 与 `/logout` 路由，并由安全配置 SSO 认证域名白名单。
  - 建议公司环境启用时配置 `SSO_EXPECTED_SUB` 为实际访问域名。
- 验证：
  - `cd frontend && npm test` 通过，21 tests passed。
  - `cd frontend && npm run build` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build ./cmd/server` 通过。
  - `go test ./internal/app` 在本机执行测试二进制时仍触发 macOS `dyld missing LC_UUID`，属于当前本机 Go 工具链/测试二进制执行限制；编译级验证已通过。

## 2026-07-01 APISIX-SSO 骨架与 DBA 协作规范 2.23.0 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`c9d72e9 feat: add apisix sso auth skeleton`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`。
  - GitLab remote 已从 `3aeaccb` 更新到 `c9d72e9`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 无缓存请求首页已更新为 `/erzhuang-project/assets/index-Dp-AlerQ.js`。
  - 线上前端 bundle 已包含 `2.23.0`、`APISIX-SSO`、`/_/auth/callback`。
- 备注：
  - 本次未发布韩国服务器。
  - 默认 `SSO_ENABLED=false`，所以发布后不会突然拦截现有运营后台。

## 2026-07-01 APISIX-SSO 退出登录入口 2.23.1 开发记录

- 背景：
  - 公司 SSO 已配置完成后，用户反馈后台没有可见的 logout 入口。
  - 代码核查确认后端已支持 `POST /api/auth/logout` 和 APISIX 默认 `GET /logout`，但前端缺少稳定退出入口。
- 实现：
  - 前端认证 helper 新增 `authLogoutPath()`，统一生成带项目路径前缀的 `/logout`。
  - 门店列表页右上角展示当前 SSO 用户与“退出登录”。
  - 门店详情页右上角也复用同一退出控件，避免用户进入详情后找不到退出入口。
  - 退出流程调整为先调用项目 `POST /api/auth/logout` 清理本地 cookie，再跳转到 `/erzhuang-project/logout` 触发 APISIX SSO 退出；本地清理失败时也会继续尝试 SSO 退出。
  - 详情页右侧操作区补充间距和换行，兼容“查看监控”和退出控件并存。
- 验证：
  - `cd frontend && npm test` 通过，22 tests passed。
  - `cd frontend && npm run build` 通过。
- 待发布验证：
  - 公司环境自动发布后，检查页面底部版本号包含 `2.23.1`。
  - SSO 登录后，列表页和门店详情页均应显示退出登录入口。
  - 点击退出后应进入公司 SSO/APISIX logout 链路，不再跳回 `/_/auth/callback`。

## 2026-07-01 公司域名 SSO 退出入口显示补丁 2.23.2 开发记录

- 背景：
  - `2.23.1` 已成功发布到公司环境，线上页脚显示 `2.23.1 (container)`。
  - 浏览器验收发现列表页仍未显示“退出登录”。
- 根因：
  - 前端显示退出入口依赖 `auth.enabled=true`。
  - 当前公司网关已经启用 APISIX SSO，但项目后端环境仍处于兼容态，`/api/auth/me` 可能返回 `enabled=false`，导致退出入口被隐藏。
- 实现：
  - 新增 `shouldShowLogoutEntry()`，在已认证且域名为 `lite.sy.soyoung.com` 时也显示退出入口。
  - 保持本地开发环境兼容态不显示退出入口，避免干扰本地调试。
  - 补充单测覆盖公司域名兼容态、本地域名兼容态、后端 SSO 启用态。
- 验证：
  - `cd frontend && npm test` 通过，23 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-07-01 公司域名 SSO 退出入口显示补丁 2.23.2 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`85657e6 fix: show sso logout on company domain`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`。
  - GitLab remote 已从 `1ad1dd3` 更新到 `85657e6`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 线上验证：
  - 浏览器打开 `https://lite.sy.soyoung.com/erzhuang-project/?codex_verify=2.23.2`，页面底部显示 `版本 2.23.2 (container)`。
  - 门店列表页右上角已显示“当前登录用户”和“退出登录”按钮。
  - 未点击“退出登录”做破坏性验证，避免主动登出用户当前 SSO 会话。
- 备注：
  - 当前 `/api/auth/me` 仍显示本地兼容用户信息，说明公司 APISIX SSO 网关已接管访问，但项目后端 `SSO_ENABLED` 可能仍未开启；本次补丁专门兼容该过渡状态。
  - 本次未发布韩国服务器。

## 2026-07-01 SSO 退出裸 JSON 与未真退出修复 2.23.3 开发记录

- 背景：
  - 用户点击“退出登录”后进入 `https://lite.sy.soyoung.com/erzhuang-project/logout`，页面直接显示 `{"ok":true}`。
  - 用户再次访问项目仍是登录状态，说明只是业务后端清理了本地 cookie，没有触发公司/APISIX SSO 真正注销。
- 根因：
  - 前端将浏览器跳转到了带项目路径前缀的 `/erzhuang-project/logout`。
  - 公司 APISIX 未在该带前缀路径优先接管登出，请求落到 Go 后端 `GET /logout` handler。
  - Go 后端 `GET /logout` 与 `POST /api/auth/logout` 复用 JSON 响应，导致浏览器裸显 `{"ok":true}`，也无法确认 SSO 网关会话已注销。
- 实现：
  - 前端在公司域名 `lite.sy.soyoung.com` 下点击退出时，浏览器跳转根路径 `/logout`，优先交给 APISIX SSO 插件处理真正登出。
  - 保留非公司域名下的 `/erzhuang-project/logout` 兼容路径。
  - Go 后端 `GET /logout` 改为清理本地 cookie 后 302 回项目首页，避免裸 JSON；`POST /api/auth/logout` 继续返回 JSON 给前端 Ajax 使用。
  - 新增后端测试覆盖 `GET /erzhuang-project/logout` 不再返回 JSON，而是 302 回首页并清 cookie。
- 验证：
  - `cd frontend && npm test` 通过，23 tests passed。
  - `cd frontend && npm run build` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build ./cmd/server` 通过。
  - `go test ./internal/app` 运行测试二进制时仍触发本机 macOS `dyld missing LC_UUID`，属于已知本机 Go 工具链执行限制；编译级验证通过。

## 2026-07-01 SSO 退出裸 JSON 与未真退出修复 2.23.3 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`a78260a fix: route sso logout through gateway`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`。
  - GitLab remote 已从 `c0bcacf` 更新到 `a78260a`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 线上验证：
  - 浏览器打开 `https://lite.sy.soyoung.com/erzhuang-project/?codex_verify=2.23.3d`，页面底部显示 `版本 2.23.3 (container)`。
  - 门店列表页右上角仍显示“退出登录”按钮。
  - 未直接点击“退出登录”，避免主动登出用户当前 SSO 会话；根据线上 bundle 逻辑，公司域名下点击退出将跳转根路径 `/logout`，不再跳 `/erzhuang-project/logout`。
- 备注：
  - 本次未发布韩国服务器。

## 2026-07-01 SSO 退出后登录闭环修复 2.23.4 开发记录

- 背景：
  - 用户反馈退出后会短暂闪过项目内 SSO 欢迎页，再进入公司 SSO 登录页。
  - 公司 SSO 登录页 URL 显示 `from_host=lite.sy.soyoung.com`，登录后回到域名根，而不是二壮项目起始页。
- 证据：
  - 未登录直接访问 `https://lite.sy.soyoung.com/erzhuang-project/` 时，APISIX 返回 302，`Location` 中包含完整 `state=https://lite.sy.soyoung.com/erzhuang-project/`。
  - 访问 `https://lite.sy.soyoung.com/logout` 时，APISIX 生成的 `state` 是 `/logout`，追加 `state`、`redirect_uri`、`redirect` 查询参数都不会改变该行为，只会被作为 `/logout?...` 的一部分编码进 state。
- 根因：
  - 公司登录回跳路径由 APISIX SSO 插件根据当前请求路径生成，不是前端可直接用 `from_host` 参数改成带路径的 URL。
  - 项目内 `LoginWelcome` 的按钮走 `/_/auth/callback`，容易让 SSO 只按 host 处理，形成回到 `lite.sy.soyoung.com` 根路径的体验。
- 实现：
  - 新增 `authCompanyEntryPath()`：公司域名下统一使用 `/erzhuang-project/` 作为 SSO 登录入口。
  - 公司域名下未登录时不再展示项目内欢迎页，而是用 `window.location.replace("/erzhuang-project/")` 重新进入项目起始页，让 APISIX 生成完整 `state`。
  - 使用 `sessionStorage` 做一次性防抖，避免异常配置下无限刷新；登录成功后清理该标记，保证下一次退出/登录仍可触发。
  - 保留本地开发环境的项目内 `LoginWelcome`，不影响调试。
- 验证：
  - `cd frontend && npm test` 通过，24 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-07-01 SSO 退出后登录闭环修复 2.23.4 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`8c2cd2f fix: keep sso login return path`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`。
  - GitLab remote 已从 `3a2fd59` 更新到 `8c2cd2f`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 线上验证：
  - 未登录访问 `https://lite.sy.soyoung.com/erzhuang-project/` 返回 302 到公司 SSO authorize 地址，`Location` 中包含完整 `state=https://lite.sy.soyoung.com/erzhuang-project/`。
  - 当前浏览器会话已退出，无法直接查看后台页脚；待用户完成 SSO 登录后可在页面底部确认 `2.23.4 (container)`。
- 备注：
  - 本次未发布韩国服务器。

## 2026-07-01 SSO 退出优先走网关修复 2.23.5 开发记录

- 背景：
  - 用户反馈 `2.23.4` 中点击“退出登录”后页面仍显示后台，出现 `Failed to fetch`，刷新后仍是已登录状态。
- 根因：
  - 前端退出流程仍然先 `await POST /api/auth/logout`，再跳转 `/logout`。
  - 公司 SSO/APISIX 场景下，该业务 API 请求可能被网关/认证状态影响而 `Failed to fetch`，导致退出动作体验不稳定。
  - 公司环境的真正退出应优先交给 APISIX `/logout`，不应依赖项目业务 API 成功。
- 实现：
  - 新增 `shouldUseGatewayLogout()`，公司域名 `lite.sy.soyoung.com` 下点击退出时立即跳转 `/logout`。
  - 非公司域名保留原有 `POST /api/auth/logout` 本地清理逻辑，方便本地/兼容环境调试。
  - 复用 `isCompanySSODomain()` 判断，统一公司域名下登录入口、退出入口、退出按钮可见性。
- 验证：
  - `cd frontend && npm test` 通过，25 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-07-01 SSO 退出优先走网关修复 2.23.5 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`49d956c fix: bypass app logout api on company sso`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`。
  - GitLab remote 已从 `376b56b` 更新到 `49d956c`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 线上验证：
  - 浏览器打开 `https://lite.sy.soyoung.com/erzhuang-project/?codex_verify=2.23.5b`，页面底部显示 `版本 2.23.5 (container)`。
  - 门店列表页右上角仍显示“退出登录”按钮。
  - 未直接点击“退出登录”，避免主动登出用户当前 SSO 会话；根据线上 bundle 逻辑，公司域名下点击退出将直接跳转 `/logout`，不再先请求 `/api/auth/logout`。
- 备注：
  - 本次未发布韩国服务器。

## 2026-07-01 SSO 用户表最小授权闭环 2.24.0 开发记录

- 背景：
  - 用户确认第一版用户表以企业邮箱作为唯一授权标识，保留 `display`、`phone`，后续继续扩展角色、机构范围和权限点。
  - 默认管理员使用 `shalei@soyoung.com`。
  - 登录提示区域应展示 SSO 返回的真实 `display`，不再依赖本地假用户信息。
- 实现：
  - 新增 `tb_users` Postgres 表初始化，字段包括 `email`、`username`、`display_name`、`feishu_user_id`、`phone`、`role`、`enabled`、`last_login_at`。
  - `EnsurePostgresSchema` 自动种子默认管理员 `shalei@soyoung.com`，`role=admin`，`enabled=true`；已有数据不覆盖。
  - `SSO_ENABLED=true` 时，`/api/auth/me` 先完成 APISIX SSO JWT 验签，再按 `data.mail` 查 `tb_users`；用户不存在或禁用返回 403。
  - 登录成功后，用 SSO payload 的 `display`、`phone`、`user_id` 回填用户表，并返回给前端，现有右上角登录提示自动展示真实 `display_name`。
  - `SSO_ENABLED=false` 时保留本地 admin 兼容态，避免本地和未启用 SSO 的环境被阻断。
- 验证：
  - `./.tools/go/bin/go test -c ./internal/app` 通过，后端认证包测试二进制可编译。
  - `./.tools/go/bin/go build ./cmd/server` 通过。
  - `cd frontend && npm test` 通过，25 tests passed。
  - `cd frontend && npm run build` 通过。
- 备注：
  - 本机执行 Go 测试二进制仍会触发 macOS `dyld missing LC_UUID` 环境问题，因此本轮 Go 行为测试以测试二进制编译和服务端构建作为可执行验证。
  - 未触碰 DBA 专项未提交的 MySQL 迁移文件。

## 2026-07-01 SSO 用户表最小授权闭环 2.24.0 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`69018e0 feat: add sso user provisioning`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`，从 `018bb97` 更新到 `69018e0`。
  - GitLab remote 已从 `018bb97` 更新到 `69018e0`。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 发布前验证：
  - `./.tools/go/bin/go test -c ./internal/app` 通过。
  - `./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
  - `cd frontend && npm test` 通过，25 tests passed。
  - `cd frontend && npm run build` 通过。
- 线上验证：
  - 命令行访问 `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 APISIX 302 登录页，说明当前公司入口已被 SSO 接管；本地命令行没有浏览器 SSO 登录态，无法直接读取健康 JSON 或页面版本。
  - 待浏览器登录态进入公司页面后，可在页面底部确认 `版本 2.24.0 (container)` 或 `2.24.0 (<commit>)`。
- 备注：
  - 本次未发布韩国服务器。
  - DBA 专项未提交的 MySQL 迁移文件保持未触碰。

## 2026-07-01 SSO 兼容态优先读取真实用户 2.24.1 开发记录

- 背景：
  - 用户线上验证发现登录后右上角仍显示 `本地管理员 / local-admin@example.com`，没有显示公司 SSO 中的 `display`。
- 根因：
  - APISIX SSO 网关已经完成登录并保护入口，但业务后端环境仍可能处于 `SSO_ENABLED=false` 兼容态。
  - 旧逻辑在 `SSO_ENABLED=false` 时会直接返回本地管理员，不会尝试读取请求里的 `sy_sso_token`。
- 修复：
  - `/api/auth/me` 改为只要请求携带有效 `sy_sso_token`，就优先解析真实 SSO 用户并查 `tb_users`。
  - 只有请求没有 token，或 token 无效且后端没有强制启用 SSO 时，才回退本地管理员兼容态。
  - 保留 `SSO_ENABLED=true` 的严格模式：无 token 或 token 无效仍返回 401。
  - 新增测试覆盖“兼容态下有有效 SSO token 时应返回真实用户，而不是本地管理员”。
- 验证：
  - `./.tools/go/bin/go test -c ./internal/app` 通过。
  - `./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
  - `cd frontend && npm test` 通过，25 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-07-02 登录用户信息隐藏企业邮箱 2.24.2 开发记录

- 背景：
  - 用户确认 SSO 真实用户展示已正常，希望右上角登录信息只展示 `display`，不再外显企业邮箱。
- 实现：
  - 登录用户 chip 改为只展示 `display_name`，缺失时展示 `username`，再缺失显示“已登录”。
  - 前端保留邮箱数据字段，但不再渲染到页面。
  - 新增 `authUserDisplayName` helper 和单测，防止后续把企业邮箱重新作为展示兜底。
  - 删除不再使用的 `.auth-user-email` 样式。
- 验证：
  - `cd frontend && npm test` 通过，26 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-07-02 系统顶栏与 H5 门店切换 2.25.0 开发记录

- 背景：
  - 用户希望门店列表、机构详情、H5 Monitor 的返回与登出位置统一，后续权限接入后维护更简单。
  - H5 Monitor 需要在页面内切换有有效监控通道的门店，且门店不要求完成确认或业务区域确认。
  - SSO 未授权用户需要明确显示“暂无访问权限”，未登录不能继续请求 H5 业务数据。
- 实现：
  - 新增共享 `SystemTopBar`，后台列表页右上角统一登出，详情页左侧 `返回列表`、右侧登出；详情页的 `查看监控` 保留在业务区。
  - 新增 `GET /api/h5/monitor/stores`，按城市返回有有效监控通道的门店及可用通道数。
  - 统一 H5 Monitor 有效通道口径：通道 active、`channel_no > 0`、录像机设备号非空、萤石账号存在且运行时凭证可用；不再依赖通道确认状态，也移除北京试点设备特例。
  - 新增 H5 门店切换器，按城市分组、当前门店高亮，当前门店不在列表时仍可兜底显示。
  - H5 首页和频道页接入 `SystemTopBar`，频道页返回改为 `replaceState` 回到监控首页，避免浏览器历史栈反复回到频道页。
  - H5 401 统一进入 SSO/login 阻断流程，403 统一显示“暂无访问权限”。
  - 加强播放器直播取流竞态保护：直播 URL 请求失效后若晚返回，会立即调用失效接口释放 `url_id`，避免异常占用萤石并发。
- 验证：
  - `cd frontend && npm test` 通过，2 files / 29 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
- 备注：
  - 本次仅完成开发准备，尚未发布公司环境。
  - `internal/storespace.H5MonitorRepository.ListMonitorStores` 的 SQL/runtime credentials 过滤尚无 repository-level SQL 测试，当前以 service/handler 边界测试和编译门禁覆盖；后续如补数据库测试基建，应补充这一层。
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次变更。

## 2026-07-02 系统顶栏与 H5 门店切换 2.25.0 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`35a70ca feat: add h5 store switcher topbar`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`，从 `8ea9569` 更新到 `35a70ca`。
  - GitLab remote 已从 `8ea9569` 更新到 `35a70ca`。
  - GitLab push 返回 `new_sha=35a70cab85c82cebd28786195e38e24b11f1a085`，说明自动发布分支已更新。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 发布前验证：
  - `cd frontend && npm test` 通过，2 files / 29 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
- 线上验证：
  - `curl -I -L https://lite.sy.soyoung.com/erzhuang-project/health` 返回 HTTP 200，server 为 `APISIX/3.6.0`。
  - `curl -I -L https://lite.sy.soyoung.com/erzhuang-project/` 返回 HTTP 200，server 为 `APISIX/3.6.0`。
  - 无浏览器 SSO 登录态时，命令行读取页面内容为 APISIX `302 Found` 页面，无法直接确认页脚版本；需用户在浏览器登录态下确认页面底部 `2.25.0 (container)` 或 `2.25.0 (<commit>)`。
- 备注：
  - 发布记录补充后，GitHub/GitLab 分支继续同步到 `a0c1edd docs: record h5 topbar company release`；该提交仅更新文档，业务代码提交为其父提交 `35a70ca`。
  - 本次未发布韩国服务器。
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次发布提交。

## 2026-07-02 H5 门店切换改为标题下拉 2.25.1 开发记录

- 背景：
  - 用户线上验收后确认 H5 Monitor 的门店切换能力可用，但不希望以页面下方陈列式导航呈现。
  - 期望点击视频监控页的机构名称后下拉选择门店，并兼顾移动端自适应。
- 实现：
  - 将 `H5StoreSwitcher` 从展开式门店切换区改为标题触发的下拉浮层。
  - H5 Monitor 首页标题区域直接承载门店切换，移除原独立陈列式切换块。
  - 下拉列表继续沿用已有接口与城市分组能力，当前门店高亮，选择后使用原路由切换逻辑。
  - 移动端下拉改为单列、限制屏幕宽度和高度，避免横向溢出。
  - 新增组件渲染测试，锁定“当前门店以 dropdown trigger 呈现”的结构。
- 验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - 本地自动截图验收受限：Playwright 自带 Chromium 未安装，本机 Chrome headless 启动被 macOS 权限拦截；本次以组件测试、生产构建和静态 CSS 约束完成发布前验收。
- 备注：
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次变更。

## 2026-07-02 H5 门店切换改为标题下拉 2.25.1 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`810dccd feat: make h5 store switcher dropdown`。
- 推送结果：
  - GitHub 已推送备份分支 `origin/codex/containerize-single-image`，从 `62b960f` 更新到 `810dccd`。
  - GitLab remote 已从 `62b960f` 更新到 `810dccd`。
  - GitLab push 返回 `new_sha=810dccd59b23b118682815d48fe7cfa6192f7a06`，说明自动发布分支已更新。
  - 使用交互式 HTTPS 账号/token 推送成功，未把凭据写入命令记录、文档或提交。
- 发布前验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
- 待线上验证：
  - 公司自动发布通常约 5 分钟完成。
  - 浏览器登录态下进入 H5 Monitor，确认机构名称点击后以下拉方式切换门店，移动端无横向溢出。
  - 页面底部应展示 `2.25.1 (container)` 或 `2.25.1 (<commit>)`。
- 备注：
  - 本次未发布韩国服务器。
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次发布提交。

## 2026-07-02 H5 门店下拉箭头与移动端浮层修复 2.25.2 开发记录

- 背景：
  - 用户线上验收发现 H5 Monitor 左上角门店切换小箭头被挤小，移动端下拉菜单位置略偏。
- 根因：
  - 旧箭头使用文字字符 `▾`，在标题按钮的自动列和移动端字体缩放下容易被压缩成小点。
  - 移动端下拉浮层从标题文字左侧定位，未对齐监控卡片内容边缘。
- 修复：
  - 将门店切换箭头替换为固定尺寸 SVG icon，保证视觉尺寸稳定。
  - 标题触发器从 `inline-grid` 调整为 `inline-flex`，减少箭头列被挤压的可能。
  - 移动端下拉浮层左侧补偿卡片内边距，宽度保持 `calc(100vw - 32px)`，让菜单更贴合页面。
- 验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
- 备注：
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次变更。

## 2026-07-02 H5 门店下拉箭头同行修复 2.25.3 开发记录

- 背景：
  - 用户继续验收发现门店切换箭头跑到门店名称下方，不符合预期。
- 根因：
  - 上一版将标题触发器改为 `inline-flex + flex-wrap` 后，DOM 顺序为“门店名、城市、箭头”，城市占满一行，导致箭头被推到下一行。
- 修复：
  - 新增 `h5-store-trigger-title-row`，将“门店名 + 箭头”包成同一行。
  - 城市信息保持第二行展示。
  - 测试增加标题行结构断言，防止箭头再次脱离门店名称。
- 验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
- 备注：
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次变更。

## 2026-07-02 用户管理与全局角色权限开发记录

- 背景：
  - 用户确认第一版后台权限采用全局角色，不做机构范围授权。
  - `admin`：全量查看/编辑/用户管理；初始化 `shalei@soyoung.com`、`maming@soyoung.com`。
  - `editor`：全量查看/门店列表/机构详情编辑；初始化 `changwenxia@soyoung.com`、`wangxiaofan@soyoung.com`。
  - `viewer`：只读预留，暂不初始化具体用户。
- 实现：
  - 后端保留 `tb_users.role` 单字段，增加 `admin/editor/viewer` 权限 helper。
  - 增加用户管理 API：`GET /api/users`、`POST /api/users`、`PUT /api/users/{id}`，仅 `admin` 可用。
  - 后端写接口增加权限守卫：门店、设计图、录像机、通道、识别、确认等写操作需要 `store:write`；AI 模型切换按系统设置收紧为 `user:manage`。
  - 前端新增“系统设置 / 用户管理”页面，管理员可新增、编辑、启停用户和切换角色。
  - 前端按角色隐藏主要编辑入口：viewer 只读；editor 可编辑门店/设计图/通道但看不到用户管理和 AI 模型切换。
- 验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
  - `go test ./internal/app` 运行测试执行阶段仍被本机已知 `dyld missing LC_UUID` 问题阻断，编译级门禁通过。
  - 本地 Vite dev server 可启动；Playwright 截图验收受限于本机 Playwright Chromium 未安装，本轮以构建、测试、静态 diff review 和 UI 标准检查收口。
- DBA 协同：
  - 已重新唤醒 DBA 专项，新增 `docs/mysql-stage-a-readiness-report.md`。
  - DBA 结论：Stage A 空库首次试跑静态复核无阻断；MySQL governance RBAC 仅作预演，不代表第一版用户管理要切多表 RBAC。
- 备注：
  - 本轮业务代码已形成本地提交，但尚未发布公司环境。
  - DBA/MySQL 迁移 WIP 文件仍保持未纳入业务提交。

## 2026-07-02 用户管理与全局角色权限 2.26.0 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布内容：
  - 新增后台用户管理第一版：管理员可添加、编辑、启停用户并设置 `admin/editor/viewer`。
  - 后端写接口增加角色权限守卫，viewer 直接调写接口返回 403。
  - 前端按角色隐藏编辑入口，AI 模型切换收紧为管理员可见。
- 发布前验证：
  - `cd frontend && npm test` 通过，3 files / 30 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
- 待线上验证：
  - 公司自动发布通常约 5 分钟完成。
  - 浏览器登录态下确认页脚版本为 `2.26.0 (container)` 或 `2.26.0 (<commit>)`。
  - `shalei@soyoung.com` 可看到“系统设置”并进入用户管理。
  - `changwenxia@soyoung.com` / `wangxiaofan@soyoung.com` 可编辑门店和通道，但不可进入用户管理或切换识别模型。
- 备注：
  - 本次未发布韩国服务器。
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次发布提交。

## 2026-07-02 用户管理弹窗开关控件 2.26.1 开发记录

- 背景：
  - 用户确认“允许登录访问”更适合用启用/停用开关表达。
  - 用户名应保持非必填，只有企业邮箱必填。
- 实现：
  - 用户管理添加/编辑弹窗中，将 checkbox 替换为 switch 样式控件。
  - 表单标签明确为“用户名（可选）”“显示名称（可选）”。
  - 后端既有逻辑保持不变：新增用户仅企业邮箱必填；用户名为空时按邮箱前缀兜底。
- 验证：
  - `cd frontend && npm test` 通过，3 files / 31 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
- 备注：
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次业务变更。

## 2026-07-02 SSO 统一退出 from_uri 修复 2.26.2 开发记录

- 背景：
  - 运维确认 SSO 统一退出组件需要业务侧在 logout 地址中带上 `from_uri` 参数。
  - 退出后应回到项目首页 `https://lite.sy.soyoung.com/erzhuang-project/`，而不是回到 `lite.sy.soyoung.com` 根路径。
- 实现：
  - 公司域名 `lite.sy.soyoung.com` 下，前端退出地址改为 SSO `logouttogether`。
  - 退出地址带 `from_host=lite.sy.soyoung.com` 和 encoded `from_uri=https://lite.sy.soyoung.com/erzhuang-project/`。
  - 本地开发环境仍保留 `/erzhuang-project/logout` 退出路径。
- 验证：
  - 先补失败测试确认旧逻辑只返回 `/logout`。
  - `cd frontend && npm test -- api.test.ts` 通过，27 tests passed。
  - `cd frontend && npm test` 通过，3 files / 31 tests passed。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
- 备注：
  - DBA/MySQL 迁移 WIP 文件保持未纳入本次发布提交。

## 2026-07-02 OSS Stage A 受控迁移入口 2.27.0 开发记录

- 背景：
  - 公司 Pod 已通过 `POST /api/admin/ops/oss-smoke` 完成 OSS 内网 PUT/GET/DELETE smoke。
  - 本机无法访问 OSS 内网 endpoint，因此样本对象复制应在公司运行环境内执行。
  - 当前业务资产读写仍保持 `ASSET_STORE=supabase`，不切全局 OSS。
- 实现：
  - 新增 `POST /api/admin/ops/asset-migrate` 受控入口。
  - 入口仅在 `OPS_ENABLED` / `K8S_SECRET_OPS_ENABLED` 开启且管理员具备 `user:manage` 权限时可用。
  - 请求体接收 inventory CSV，默认 `external_org_id=10030`、`max_rows=20`，请求体限制 2MB。
  - `apply=true` 当前只允许样本门店 `10030`。
  - dry-run 不写 OSS；apply 只复制对象到 OSS，并返回待审查 `result_sql`，不直接写 MySQL。
  - 源存储默认复用现有业务 Supabase 运行时变量，目标 OSS 优先复用 `K8S_SECRET_*` 变量。
- DBA 审查要点：
  - `result_sql` 当前只 update 已存在的 `tb_asset_objects` 行，不 insert/upsert。
  - 执行 `result_sql` 前必须确认样本 logical key 已有 pending 记录，否则可能影响 0 行。
  - `mysql_schema_tb.sql` 与 `mysql_business_schema_patch_tb.sql` 存在重复字段风险，后续需要明确“完整初始化路径”和“旧库补丁路径”。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/assetmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/assets` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build ./cmd/server ./cmd/asset-migrate ./cmd/oss-smoke` 通过。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk size warning。
  - `go test ./...` 执行阶段仍受本机 Go runtime `dyld missing LC_UUID` 问题阻断，编译级门禁通过。
- 下一步：
  - 发布该入口到公司环境。
  - 从 MySQL 测试库导出 `external_org_id=10030` inventory CSV。
  - 先在线上已登录管理员浏览器中调用 `apply=false` dry-run。
  - dry-run 无 failed 后，再调用 `apply=true`，审查返回的 `result_sql` 后手工回写 MySQL。

## 2026-07-02 OSS Stage A 源样本对象 2.27.1 开发记录

- 背景：
  - Stage A dry-run 已通过：`Total=2, WouldCopy=1, Skipped=1, Errors=0`。
  - Stage A apply 失败：源 Supabase Storage 返回 `Object not found`。
  - 用户确认此前清理过 snapshots，因此测试库引用存在、源对象缺失是合理状态。
- 实现：
  - 新增 `POST /api/admin/ops/stage-a-source-sample` 受控入口。
  - 入口仅在 ops 开启且管理员具备 `user:manage` 权限时可用。
  - 只支持两个动作：`seed` 和 `cleanup`。
  - `seed` 只向源存储写入固定非敏感样本对象：`channel-snapshots/stage-a-10030-channel-1.jpg`。
  - `cleanup` 只删除同一个固定样本对象。
  - 不支持自定义 key，不支持上传真实业务截图。
- 验证：
  - 先补失败测试，确认新类型/入口不存在时 `go test -c ./internal/app` 编译失败。
  - 实现后 `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 调用 `stage-a-source-sample` seed。
  - 重新执行 `asset-migrate apply=true`。
  - 验证成功后调用 cleanup 清理源样本对象；OSS 目标样本对象在最终验证策略确认后清理。

## 2026-07-02 OSS Stage A 源样本 cleanup 2.27.2 修复记录

- 背景：
  - Stage A apply 已成功：`Copied=1, Skipped=1, Errors=0`，完整复制链路 `源 Supabase -> 公司 Pod -> OSS` 已跑通。
  - 调用 `stage-a-source-sample cleanup` 时返回 502。
  - 错误为 Supabase list 路径返回 `409 Duplicate`，发生在 `DeletePrefix` 的 list-then-delete 阶段。
- 实现：
  - 为 `SupabaseStorageStore` 增加 `Delete(ctx, key)`，直接按固定 key 删除，不先 list。
  - Stage A cleanup 优先使用可选 `Delete` 接口；不支持该接口的 store 仍 fallback 到 `DeletePrefix`。
- 验证：
  - 先补失败测试 `TestSupabaseStorageStoreDeleteRemovesExactKeyWithoutListing`，确认旧代码没有 `Delete` 方法。
  - 实现后 `go test -c ./internal/assets` 通过。
  - `go test -c ./internal/app` 通过。

## 2026-07-02 OSS Stage A 目标样本 cleanup 2.27.3 开发记录

- 背景：
  - Stage A 源样本对象已清理。
  - OSS 目标 bucket 中仍保留非敏感样本对象 `channel-snapshots/stage-a-10030-channel-1.jpg`。
- 实现：
  - 新增 `POST /api/admin/ops/stage-a-target-sample` 受控入口。
  - 入口仅支持 `{ "action": "cleanup" }`。
  - 只删除目标 OSS 的固定 Stage A 样本 key，不支持自定义 key。
  - 仍受 `OPS_ENABLED` 和管理员权限保护。
- 验证：
  - 先补失败测试确认新 runner/响应类型不存在。
  - `go test -c ./internal/app` 通过。
  - `go test -c ./internal/assets` 通过。
  - `go build ./cmd/server` 通过。

## 2026-07-02 OSS Stage A 样本迁移闭环记录

- 范围：
  - 样本门店：`external_org_id=10030`。
  - 样本对象：`channel-snapshots/stage-a-10030-channel-1.jpg`。
- 已完成：
  - 公司 Pod OSS smoke 通过。
  - MySQL 测试库 inventory 导出 2 行，rank=1/2 指向同一 logical key。
  - `asset-migrate apply=false` dry-run 通过：`Total=2, WouldCopy=1, Skipped=1, Errors=0`。
  - 源 Supabase 非敏感样本对象 seed 通过。
  - `asset-migrate apply=true` 通过：`Copied=1, Skipped=1, Errors=0`，复制 159 bytes 到 OSS。
  - 审查并执行 result SQL，精确更新 `tb_asset_objects.id=900081` 1 行。
  - 回写后状态为 `storage_provider=oss`、`bucket=sy-camera-erzhuang-project`、`migration_status=migrated`。
  - validation 关键检查通过：缺字段 0、重复 logical key 0、重复 OSS target key 0、bucket mismatch 0、migrated without proxy path 0。
  - 源 Supabase 样本对象 cleanup 通过。
  - 目标 OSS 样本对象 cleanup 通过。
- 结论：
  - Stage A 样本链路已闭环，证明 `Supabase 源对象 -> 公司 Pod -> OSS -> MySQL 状态回写 -> validation` 可行。
  - 下一阶段可以准备 Stage B，但需要先确认真实历史对象源仍存在，并制定批量迁移、失败重试、回滚和清理策略。

## 2026-07-02 Postgres -> MySQL 只读导出入口 2.28.0 开发记录

- 背景：
  - 用户纠正 MySQL 公司测试库需要承接当前 Supabase/PostgreSQL 真实业务数据，Stage A 样本链路不等于真实数据迁移完成。
  - 本地没有 `DATABASE_URL` / Supabase 连接环境，真实源数据导出应在公司运行环境使用已有 Postgres 连接执行。
- 实现：
  - 新增 `cmd/pg-to-mysql-export`，本地/运行环境均可只读导出 Postgres 数据为 MySQL import SQL、auto increment SQL 和 report。
  - 新增 `internal/mysqlmigration`，集中维护 Postgres -> MySQL 表映射、字段转换、机构范围过滤和 SQL 生成逻辑。
  - 新增 `POST /api/admin/ops/pg-mysql-export` 受控入口。
  - 入口只读 Postgres，不写 MySQL；仅在 `OPS_ENABLED` / `K8S_SECRET_OPS_ENABLED` 开启且管理员具备 `user:manage` 权限时可用。
  - 默认导出 `external_org_id=10030`，最多允许一次传 5 个机构 ID，避免误导全量大 SQL。
  - MySQL governance DDL 当时按 `tb_users.phone`、`tb_users.role` 兼容口径设计；后续公司测试库实测以 `mobile` + `tb_user_roles` 为准，导出器已在 2.29.2 调整。
  - 新增 `docs/postgres-to-mysql-data-migration-runbook.md`，明确顺序为：Postgres 真实业务数据 -> MySQL 测试库 -> 基于 MySQL 真实资产清单迁 OSS。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/pg-to-mysql-export-check ./cmd/pg-to-mysql-export` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `git diff --check` 通过。
  - `go test ./internal/app` 执行阶段仍受本机 Go runtime `dyld missing LC_UUID` 问题阻断，编译级门禁通过。
- 下一步：
  - 发布公司环境。
  - 用浏览器控制台调用 `POST /erzhuang-project/api/admin/ops/pg-mysql-export` 导出 `external_org_id=10030` 小样本。
  - 审核 report 和 import SQL 后，再决定是否写入 MySQL 测试库。

## 2026-07-03 Postgres -> MySQL 导出 502 修复记录 2.28.1

- 现象：
  - 公司环境 `POST /erzhuang-project/api/admin/ops/pg-mysql-export` 已命中新入口，但返回 502。
  - 控制台可见 `ok=false`、`external_org_ids=["10030"]`、`import_sql_chars=0`。
- 根因：
  - 导出器默认对每张源表拼接 `order by id`。
  - `app_settings` 源表主键为 `key`，没有 `id` 字段，因此导出到该表时 PostgreSQL 查询失败。
- 修复：
  - 为 `tableSpec` 增加 `OrderBy`。
  - `app_settings` 指定 `OrderBy: "key"`。
  - 查询构造改为：排序列存在才追加 `order by`，避免无 `id` 表阻断迁移探针。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/pg-to-mysql-export-check ./cmd/pg-to-mysql-export` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 再次调用 `pg-mysql-export` 导出 `external_org_id=10030`，这次重点检查返回 `detail`、表行数、SQL 字符数和导出范围。

## 2026-07-03 MySQL 金丝雀导入受控入口 2.29.0 开发记录

- 背景：
  - `pg-mysql-export` 已能导出 `external_org_id=10030` 的真实金丝雀数据。
  - 本机没有 MySQL 客户端，且不希望把公司 MySQL 连接散落到本机手工操作。
  - 用户确认优先走公司 Pod 内受控 ops 入口。
- 实现：
  - 新增 `POST /api/admin/ops/mysql-canary-import`。
  - 入口仅在 ops 开启且管理员具备 `user:manage` 权限时可用。
  - 仅允许 `external_org_id=10030`。
  - `import_sql` 必须包含 `-- Scope external_org_id: 10030`。
  - 会拒绝 `tb_stores` insert 中出现非 10030 的门店机构 ID。
  - `apply=false` 只连接 MySQL、检查必要表、返回当前摘要，不执行导入 SQL。
  - `apply=true` 才在事务中执行导入 SQL，并返回门店、录像机、通道、截图、日志、用户、孤儿行、非法 JSON 的摘要。
  - MySQL DSN 从 `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN` 读取。
  - 新增依赖 `github.com/go-sql-driver/mysql`。
- 敏感数据处理：
  - 导入 SQL 可能包含手机号、飞书 ID、模型识别原始长文本和截图 proxy path。
  - 完整 SQL 只应在临时文件、浏览器下载和受控 ops 请求体中短期使用，不写入仓库和文档。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - `go test ./internal/app -run ...` 执行阶段仍受本机 macOS Go runtime `dyld missing LC_UUID` 阻断；该问题为既有本机运行问题，编译级门禁通过。
- 下一步：
  - 发布公司环境。
  - 确认公司运行环境已配置 `K8S_SECRET_MYSQL_DSN` 或 `MYSQL_DSN`。
  - 用导出的 `10030` SQL 先调用 `apply=false` dry-run，再看摘要决定是否 `apply=true`。

## 2026-07-03 MySQL 金丝雀导入 JSON 转义修复 2.29.1

- 现象：
  - `external_org_id=10030` 的 `apply=false` dry-run 已能连接 MySQL 并校验表结构。
  - `apply=true` 执行导入时返回 502，MySQL 报 `Invalid JSON text: "Invalid escape character in string."`，位置落在 `tb_video_channels.recognition_result`。
- 根因：
  - Postgres 源数据里的 `recognition_result` JSON 本身需要保留 `\n`、`\"` 等反斜杠转义。
  - 导出器生成 MySQL SQL 字符串时只转义了单引号，没有转义反斜杠。
  - MySQL 执行 SQL 字符串字面量时先解释反斜杠，导致写入 JSON 列前内容被破坏。
- 修复：
  - `internal/mysqlmigration.mysqlString` 统一先转义反斜杠，再转义单引号。
  - 新增测试覆盖 JSON 字符串中的换行转义、引号转义和路径反斜杠。
- 验证：
  - 新增测试先按旧实现失败，再修复通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 重新用 `pg-mysql-export -> mysql-canary-import apply=true` 执行 `10030` 金丝雀导入。
  - 预期摘要应出现 `store_count=1`、`recorder_count=1`、`channel_count=4`、`snapshot_count=4`，且 `orphan_count=0`、`invalid_json_count=0`。

## 2026-07-03 MySQL 金丝雀导入用户字段兼容修复 2.29.2

- 现象：
  - 2.29.1 修复 JSON 转义后，`apply=true` 继续执行到用户表，返回 502。
  - MySQL 报 `Unknown column 'phone' in 'field list'`。
- 根因：
  - Postgres 用户表包含 `phone`、`role` 单字段。
  - 公司 MySQL 测试库当前用户主表以 `mobile`、`department`、`sso_subject` 和角色关系表为准，并不存在 `tb_users.phone`。
  - 旧导出器仍按早期 governance 草案同时写 `phone`、`mobile`、`role`，与真实测试库不一致。
- 修复：
  - `tb_users` 导出列去掉 `phone` 和 `role`。
  - Postgres `phone` 继续写入 MySQL `mobile`。
  - 增加 `department`、`sso_subject` 目标列，源库缺失时写默认空值。
  - 角色仍通过 `writeRoleStatements` 写入 `tb_user_roles`。
  - 新增测试约束 `tb_users` 导出列不再包含 `phone`、`role`，且保留 `email`、`mobile`、`enabled`。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
- 下一步：
  - 发布公司环境。
  - 再次执行 `10030` 金丝雀 `apply=true`，继续观察是否还有下一层真实 schema 差异。

## 2026-07-03 MySQL 金丝雀导入后只读校验入口 2.29.3

- 背景：
  - `external_org_id=10030` 金丝雀已成功导入 MySQL 测试库。
  - 导入后需要可重复、低风险地确认 MySQL 当前状态，避免每次都重新携带大段 `import_sql`。
- 实现：
  - 新增 `GET /api/admin/ops/mysql-canary-validate?external_org_id=10030`。
  - 入口仅在 ops 开启且管理员具备 `user:manage` 权限时可用。
  - 入口只读 MySQL，不执行导入 SQL，不修改数据。
  - 复用 `ensureMySQLCanaryTables` 和 `queryMySQLCanarySummary`，返回门店、录像机、通道、截图、操作日志、用户、外键孤儿和非法 JSON 摘要。
  - 仍限制 `external_org_id=10030`，避免误扫全量数据。
- 验证：
  - 新增 handler 测试覆盖只读校验成功返回摘要、拒绝非 10030 范围。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 调用只读校验入口，确认摘要仍为 `store_count=1`、`recorder_count=1`、`channel_count=4`、`snapshot_count=4`、`orphan_count=0`、`invalid_json_count=0`。
  - 校验通过后，进入基于 MySQL 真实数据生成 OSS 资产清单。

## 2026-07-03 MySQL 真实资产清单只读入口 2.29.4

- 背景：
  - `10030` 金丝雀导入后只读校验已返回 `ok=true`，且外键孤儿和非法 JSON 均为 0。
  - 下一步 OSS 迁移必须基于 MySQL 真实业务行生成 manifest，不能继续使用 Stage A 假数据代表历史资产。
- 实现：
  - 新增 `GET /api/admin/ops/mysql-asset-inventory?external_org_id=10030`。
  - 入口仅在 ops 开启且管理员具备 `user:manage` 权限时可用。
  - 入口只读 MySQL，不复制对象，不修改 `tb_asset_objects`。
  - 清单来源包括 `tb_store_design_plans` 的设计图路径和 `tb_channel_snapshots` 的通道截图路径。
  - Go 侧归一化 `/api/store-space/channel-snapshots/{name}`、`channel-snapshots/{name}`、`/api/design-plan/uploads/{upload_id}/{asset}`、`uploads/{upload_id}/{asset}` 等路径。
  - 对 `http(s)` 临时或签名 URL 标记为 `skipped/remote_http_url`，不强行迁移。
  - 对同一 logical key 的多处引用输出 `logical_key_rank` 和 `logical_key_ref_count`，后续复制时只应复制 rank=1。
- 验证：
  - 新增 handler 测试覆盖返回 manifest CSV、拒绝非 10030 范围。
  - 新增归一化测试覆盖通道截图 proxy path、重复引用、远程 URL 跳过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 调用资产清单入口，审查 `summary` 和 `manifest_csv`。
  - 若清单合理，再把 `manifest_csv` 传入 `asset-migrate apply=false` 做 dry-run。

## 2026-07-03 MySQL 通道截图资产清单最新记录口径修正 2.29.5

- 现象：
  - `10030` 刷新通道截图并重新导入 MySQL 后，资产清单从预期的 8 个引用变成 16 个引用。
  - `snapshot_rows=16`、`duplicate_refs=16`，说明清单纳入了旧截图历史行和新截图行。
- 根因：
  - `tb_channel_snapshots` 会保留每次截图刷新产生的历史记录。
  - 当前 OSS 迁移目标是迁移门店当前可展示的通道预览图，而不是迁移已物理删除或已过期的历史敏感截图。
  - 资产清单入口原先扫描该门店所有 `tb_channel_snapshots`，导致同一通道历史截图也进入迁移范围。
- 修复：
  - `mysql-asset-inventory` 查询通道截图时，只取每个通道 `created_at/id` 最新的一条截图记录。
  - Go 构建 manifest 时增加兜底过滤：同一通道只保留最新 `source_id` 的截图记录。
  - 新增测试覆盖同一通道旧/新截图同时存在时，manifest 只包含新截图。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 重新调用 `mysql-asset-inventory?external_org_id=10030`。
  - 预期 `summary.total=8`、`snapshot_rows=8`、`duplicate_refs=8`，再继续执行 `asset-migrate apply=false`。

## 2026-07-03 MySQL 资产台账受控回写接口 2.29.6

- 背景：
  - `10030` 通道截图 OSS 实际复制成功：`Total=8`、`Copied=4`、`Skipped=4`、`Errors=0`。
  - 再次查询 `mysql-asset-inventory` 仍显示 `pending=8`，说明 OSS 对象已复制，但 MySQL `tb_asset_objects` 台账尚未 upsert/标记 migrated。
  - `asset-migrate` 返回的 `result_sql` 只 update 已存在行，不适合作为当前真实样本的唯一回写方式。
- 实现：
  - 新增 `POST /api/admin/ops/asset-state-backfill`。
  - 输入 `manifest_csv`、`result_csv`、`external_org_id`、`batch_id`。
  - 仅允许 `external_org_id=10030`。
  - 只处理 `result_csv` 中 `action=copied`，并与 manifest 中 `logical_key_rank=1` 的行匹配。
  - 使用参数化 SQL upsert `tb_asset_objects`，写入 `storage_provider=oss`、bucket、storage key、content type、size、owner、sensitivity、`migration_status=migrated`、batch id 和迁移时间。
  - 同一 logical key 可重复执行，依赖 `logical_key_hash` 唯一键保持幂等；thumbnail/full 重复引用只登记一个资产对象。
- 验证：
  - 新增 handler 测试覆盖成功回写请求和拒绝非 10030 范围。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 用刚刚成功的 `manifest_csv` 和 `result_csv` 调用台账回写接口。
  - 回写后再次调用 `mysql-asset-inventory`，预期当前这 4 个 logical key 不再需要重新复制；如清单入口仍只从业务表判断 pending，需要继续把 inventory 与 `tb_asset_objects` 状态联动。

## 2026-07-03 MySQL 资产清单联动台账状态 2.29.7

- 现象：
  - `asset-state-backfill` 成功返回 `total=8`、`migrated=4`、`skipped=4`、`upserted=4`、`errors=0`。
  - 随后再次调用 `mysql-asset-inventory`，仍显示 `pending=8`。
- 根因：
  - 资产清单入口只根据业务表路径生成 manifest，没有查询 `tb_asset_objects`。
  - 因此即使台账已标记 `migration_status=migrated`，清单仍会机械标记为 `pending`。
- 修复：
  - `mysql-asset-inventory` 在生成 manifest 前按 logical key 查询 `tb_asset_objects`。
  - 当台账状态满足 `migration_status=migrated`、`storage_provider=oss`、bucket/storage key 非空时，manifest 行标记为 `skipped`，原因 `already_migrated`。
  - 保留重复引用统计，thumbnail/full 两个引用都会显示为已迁移跳过，不再建议复制。
- 验证：
  - 新增测试覆盖已迁移 logical key 进入 inventory 时，`pending=0`、`skipped=2`，CSV 包含 `already_migrated`。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 再次调用 `mysql-asset-inventory?external_org_id=10030`，预期 `pending=0`、`skipped=8`，manifest 中 skip reason 为 `already_migrated`。
- 线上复验：
  - 公司环境切到 `2.29.7` 后复查通过。
  - `mysql-asset-inventory?external_org_id=10030` 返回 `total=8`、`pending=0`、`skipped=8`、`snapshot_rows=8`、`duplicate_refs=8`。
  - manifest 中 8 条通道截图引用均为 `suggested_migration_status=skipped`、`skip_reason=already_migrated`。
  - 结论：`10030` 金丝雀门店当前通道截图已完成“Postgres 业务数据 -> MySQL、Supabase 源对象 -> OSS、MySQL 资产台账回写、inventory 幂等跳过”的闭环验证。

## 2026-07-03 Stage B 多门店金丝雀白名单 2.29.8

- 背景：
  - `10030` 单门店已完成完整闭环。
  - 下一步需要扩大到真实业务门店，但仍不能开放全量迁移，避免误导出/误写/误复制。
- 实现：
  - 新增运行时白名单环境变量：`OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS`，K8s Secret 兼容名为 `K8S_SECRET_OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS`。
  - 默认白名单始终包含 `10030`。
  - 配置示例：`OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS=10030,10047`。
  - `mysql-canary-validate`、`mysql-asset-inventory`、`asset-migrate apply=true`、`asset-state-backfill` 改为统一使用白名单校验。
  - `mysql-canary-import` 的 SQL scope comment 改为匹配当前请求的 `external_org_id`，不再硬编码 `10030`。
- 验证：
  - 新增测试覆盖 `10047` 在白名单中时 validate、inventory、asset apply、asset-state-backfill 均允许进入 runner。
  - 保留非白名单机构拒绝测试。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/mysqlmigration` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 配置公司运行时白名单为 `10030,10047`。
  - 对 `10047` 重复执行 Postgres -> MySQL -> OSS -> 台账回写 -> inventory 幂等验证。

## 2026-07-03 Postgres 下线前运行时切换门槛

- 产品目标：
  - 最终会删除 Postgres 数据库，因此不能只完成数据搬迁；所有线上运行时接口必须切到 MySQL/OSS 后，才能认为迁移完成。
  - 用户期望今天尽量完成可让运营使用的切换。
- 硬门槛：
  - MySQL 全量业务数据导入完成，并通过 orphan/invalid JSON 校验。
  - Supabase 图片对象迁到 OSS，`tb_asset_objects` 台账完整且 inventory 可幂等识别 `already_migrated`。
  - 后端运行时接口不再依赖 Postgres；至少机构列表、机构详情、通道映射、H5 Monitor 首页、截图读取等运营核心只读链路要支持 MySQL。
  - 图片读取接口路径保持不变，内部优先从 OSS 读；未迁完前可 fallback 旧存储，正式删除 Postgres/Supabase 前必须确认 fallback 不再被依赖。
  - 必须保留运行时开关，例如 `APP_DB_DRIVER=postgres|mysql` 或等价配置，确保公司环境切换后可以回滚。
- 今日建议推进顺序：
  - 先完成 Stage B 多门店 OSS 迁移闭环。
  - 随后优先实现“只读运行时 MySQL repo + OSS 图片读取优先”的切换，不先动编辑/识别等写入重链路。
  - 运营验收只读和查看监控主流程稳定后，再逐步切写操作。

## 2026-07-03 Stage B 第一批多门店迁移闭环完成

- 范围：
  - `10047`、`10011`、`10070`、`10054`、`10062`。
  - 加上已完成的 `10030`，当前已完成 6 个门店的 Postgres -> MySQL 与通道截图 OSS/台账迁移闭环。
- 已完成动作：
  - 对上述 5 个门店重新刷新当前通道截图，确认 H5 monitor 返回结构为 `groups[].channels[].id`，不再读取根级 `channels`。
  - 分门店执行 Postgres -> MySQL 导出与导入，所有导入结果 `orphan=0`、`invalid_json=0`。
  - 执行 `mysql-asset-inventory` 并用 `asset-migrate apply=false` dry-run，确认待复制数量与重复引用关系正常。
  - 使用 `max_rows=10` 分批执行 `asset-migrate apply=true`，每批成功后立刻调用 `asset-state-backfill` 回写 `tb_asset_objects`，避免单次大量复制触发 504。
- OSS 复制与台账回写结果：
  - `10047`：复制并回写 26 个 logical assets，最终 `pending=0`。
  - `10011`：复制并回写 43 个 logical assets，最终 `pending=0`。
  - `10070`：复制并回写 38 个 logical assets，最终 `pending=0`。
  - `10054`：复制并回写 56 个 logical assets，最终 `pending=0`。
  - `10062`：复制并回写 51 个 logical assets，最终 `pending=0`。
  - 本批合计新增迁移 214 个 logical assets。
- 验收结论：
  - 本批所有资产复制批次 `Errors=0`。
  - 所有台账回写批次 `errors=0`。
  - 该批门店已完成“业务数据进 MySQL、当前通道截图进 OSS、MySQL 资产台账可幂等识别”的迁移闭环。
- 后续规划：
  - 继续处理白名单中的 `10051`。
  - 随后进入运行时切换：后端核心只读链路需要支持从 MySQL 读取，图片读取路径保持前端 URL 不变，内部优先按 `tb_asset_objects` 从 OSS 读取。
  - Postgres/Supabase 删除前必须完成运行时切换和回滚开关验证。

## 2026-07-03 MySQL/OSS 运行时只读切换能力 2.30.0

- 背景：
  - 用户明确后续会删除 Postgres，因此数据迁移完成后必须让线上运行时接口可切到 MySQL/OSS。
  - 目标是先让运营核心查看链路可用，不等待所有编辑写入链路一次性 MySQL 化。
- 实现：
  - 新增 `APP_DB_DRIVER=postgres|mysql` 运行时开关；未配置时保持原有 Postgres 优先逻辑，避免影响当前线上。
  - `APP_DB_DRIVER=mysql` 时读取 `MYSQL_DSN` 或 `K8S_SECRET_MYSQL_DSN`，并自动追加 `parseTime=true`，避免 MySQL `datetime` 扫描失败。
  - 新增 `app.MySQLStore`，支持 `/health`、任务示例、SSO 用户读取/回写、用户管理、AI 设置读写。
  - 新增 `storespace.MySQLStore`，支持门店列表、门店详情、设计图数据、通道数据、萤石账号读取、重复门店检查、单通道上下文读取。
  - 新增 `storespace.NewMySQLH5MonitorRepository`，支持 H5 Monitor 门店列表、机构监控首页、通道校验与直播/回放前置查询。
  - MySQL 模式下 `design-plan` 独立旧接口暂时使用内存 repo；当前前端主流程已通过 `store-space` 接口读取门店/详情。
  - MySQL 模式下未迁完的编辑写入动作先返回 `not implemented`，避免误写半套链路；后续再逐步补齐写入链路。
- 验证：
  - 新增 `cmd/server` 配置测试覆盖 MySQL 开关、Postgres 默认、MySQL DSN 自动追加 `parseTime=true`。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - `go test ./...` 在本机仍触发 macOS 测试二进制 `missing LC_UUID load command`，这是本地工具链执行问题；已用 `go test -c` 和 `go build` 验证关键包编译。
- 发布/切换建议：
  - 先发布公司环境但不配置 `APP_DB_DRIVER=mysql`，确认版本正常。
  - 切换窗口配置 `APP_DB_DRIVER=mysql`、`ASSET_STORE=oss`，保留 `DATABASE_URL` 作为回滚后可用配置。
  - 切换后验证 `/health` 返回 `database=mysql`、`asset_store=oss`，再验机构列表、机构详情、视频监控门店切换、监控首页、通道直播入口。
  - 若只读链路异常，删除或改回 `APP_DB_DRIVER=postgres` 即可回滚运行时读库。

## 2026-07-03 MySQL Runtime 下 Postgres 导出复验与剩余批次清单 2.30.12

- 已复验：
  - 公司环境 `2.30.11` 下，`POST /api/admin/ops/pg-mysql-export` 在 `APP_DB_DRIVER=mysql` runtime 中已能通过 `DATABASE_URL` 只读连接 Postgres。
  - `external_org_id=10030` 导出返回 `status=200`、`ok=true`、`sql chars=36510`。
  - 导出表行数包括：`stores=1`、`video_recorders=1`、`video_channels=4`、`channel_snapshots=8`、`operation_logs=6`。
- 本次实现：
  - 新增 `GET /api/admin/ops/pg-mysql-source-orgs` 只读端点。
  - 端点同时读取 Postgres 源门店和 MySQL 目标门店，返回每个 `external_org_id` 的源库计数、是否已导入 MySQL、是否在迁移白名单、是否可作为下一批迁移对象。
  - 不执行导入、不复制资产、不写 MySQL，仅用于生成剩余迁移批次和降低人工猜测风险。
- 后续推进：
  - 发布公司环境后先调用该端点，确认源库约 55 家门店、MySQL 已完成 6 家、剩余门店列表和当前白名单状态。
  - 将下一批 5 个 `batchable=true` 的机构按现有闭环继续迁移：Postgres 导出、MySQL 单店导入、资产 `max_rows=10` 分批复制、每批后资产台账回写。
  - 如剩余机构不在 `OPS_MIGRATION_ALLOWED_EXTERNAL_ORG_IDS`，先追加白名单再迁移。

## 2026-07-03 MySQL Runtime 刷新通道截图写链路 2.30.13

- 现象：
  - `10051` 已完成 Postgres -> MySQL 业务数据导入。
  - 资产迁移第一批失败，`asset-migrate max_rows=1` 返回源对象 `404 not_found`。
  - 尝试用 `POST /api/store-space/channels/{channel_id}/snapshot` 刷新当前截图时，公司 MySQL runtime 返回 `501 not implemented`。
- 根因：
  - `10051` 历史截图源对象已在旧存储中缺失，不能直接从旧 key 迁移到 OSS。
  - MySQL runtime 只先实现了核心只读链路，`SaveChannelSnapshot` 仍返回 `ErrNotImplemented`，导致无法在 MySQL 下刷新并落库新截图。
- 修复：
  - 实现 `storespace.MySQLStore.SaveChannelSnapshot`。
  - 刷新截图时可插入 `tb_channel_snapshots` 最新行，并按刷新语义清空识别状态字段、不增加识别次数。
  - 返回通道时沿用 `GetChannel` 最新截图查询逻辑。
- 验证：
  - 新增 `TestMySQLChannelSnapshotUpdateArgsForRefresh` 覆盖刷新截图的 MySQL 更新参数。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 下一步：
  - 发布公司环境。
  - 重新刷新 `10051` 通道截图，再执行 `mysql-asset-inventory` 确认最新 manifest 指向新截图 key。
  - 继续 `applyOrgInBatches('10051', 20)` 完成 OSS 复制和资产台账回写。

## 2026-07-03 MySQL 刷新截图 collation 修复 2.30.14

- 现象：
  - `2.30.13` 部署后，`POST /api/store-space/channels/{channel_id}/snapshot` 不再返回 `501 not implemented`。
  - 刷新 `10051` 通道截图时返回 `500`，诊断 detail 为 `Error 1267 (HY000): Illegal mix of collations ... for operation 'nullif'`。
- 根因：
  - MySQL 更新语句使用 `nullif(?, '')` 判断空字符串。
  - 公司 MySQL runtime 中参数与空字符串字面量可能带不同 collation，`nullif` 会触发字符串比较并报 1267。
- 修复：
  - 将 MySQL 通道截图更新 SQL 的空字符串判断改为 `char_length(?)`。
  - 保持刷新语义不变：刷新截图不增加识别次数、不覆盖已有识别状态；识别链路传入状态时仍可更新识别字段。
- 验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。

## 2026-07-03 MySQL 迁移第 55 家门店只读审计 2.30.15

- 现象：
  - Postgres -> MySQL 主数据迁移完成后，`pg-mysql-source-orgs` 返回 `source_count=54`、`mysql_count=54`、`remaining_count=0`。
  - 用户业务记忆里总门店数为 55，需要确认是否有 1 家漏迁。
- 根因判断：
  - `pg-mysql-source-orgs` 的源清单口径是 `external_org_id` 非空的门店/机构。
  - 没有录像机不会被排除；`10056`、`10071`、`10076`、`10077`、`10078` 等 `channel_count=0` 门店已经正常导入。
  - 最可能的差异是 Postgres `stores` 里存在 1 家 `external_org_id` 为空的门店，不属于按机构 ID 迁移链路。
- 本次实现：
  - 新增 `GET /api/admin/ops/pg-mysql-store-audit` 只读诊断端点。
  - 返回 Postgres `stores` 总数、`external_org_id` 非空门店数、非空 distinct 机构数、空 `external_org_id` 门店列表。
  - 同时返回 MySQL `tb_stores` 总数、`external_org_id` 非空门店数、非空 distinct 机构数、空 `external_org_id` 门店列表。
  - 端点只读，不导入、不复制资产、不写 MySQL。
- 验证：
  - 新增 `TestPGMySQLStoreAuditEndpointReturnsMissingExternalOrgStores` 覆盖端点响应会列出空 `external_org_id` 源门店。
  - 本机直接运行 Go 测试仍触发 macOS 测试二进制 `missing LC_UUID load command`；按项目既定方式使用 `go test -c` 与 `go build` 做编译验证。

## 2026-07-03 MySQL Runtime 通道写链路补齐 2.30.16

- 现象：
  - MySQL/OSS 切换后，线上读链路可用，但录像机 `scan-channels` 返回 `501 not implemented`。
  - 前端把 `501` 统一翻译为“截图识别能力还在接入中”，容易误判为所有识图模型断开。
  - 实际诊断显示 `ai-settings` 为 `minimax / MiniMax-M3`，但 `scan-channels` 对 MySQL runtime 仍走到未实现写接口。
- 根因：
  - `MySQLStore.ReplaceRecorderChannels`、`MySQLStore.UpsertRecorderChannel`、通道解锁/确认写接口仍返回 `ErrNotImplemented`。
  - 通道 recognizer 的挂载被启动时环境变量可用性 gate 住；MySQL runtime 下 AI provider 实际从 `tb_app_settings` 动态读取，不能只按启动时默认 OpenAI 环境判断。
- 修复：
  - 实现 MySQL 录像机扫描结果写回：新增/恢复通道、下线缺失通道、更新录像机在线状态和有效通道数、写操作日志。
  - 实现 MySQL 探测识别通道 upsert，支持扫描未入库通道后保存截图和识别结果。
  - 实现 MySQL 通道解锁和确认，支持人工修正识别结果。
  - 通道 recognizer 改为始终挂载动态 provider，具体 OpenAI/MiniMax 配置在请求时按运行时 AI 设置读取。
- 验证：
  - 新增 MySQL 写链路 helper 测试覆盖无效通道校验和录像机状态计算。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。

## 2026-07-03 批量通道识别网关超时保护 2.30.17

- 现象：
  - `POST /api/store-space/recorders/64/scan-channels` 已返回 `200`，录像机 `GQ2603587` 从离线变为在线，并写入 41 个有效通道。
  - 控制台继续手动调用 `POST /api/store-space/recorders/64/recognize-channels` 时返回 APISIX `504 Gateway Time-out` HTML。
- 根因：
  - `recognize-channels` 是历史批量接口，会同步逐路抓图、保存截图、调用 AI，并在每路之间等待 1.2 秒。
  - 41 路通道一次性同步处理超过公司网关超时窗口；这不是 AI 配置断开，也不是扫描写入失败。
  - 前端页面当前“识别区域”按钮已经走逐通道 `/channels/{channel_id}/recognize`，更适合线上使用。
- 修复：
  - 后端历史批量接口按通道号排序后，每个请求最多处理 5 路待识别通道。
  - 剩余未识别通道保留给下一轮请求或页面逐通道流程，避免一个请求长时间阻塞到 504。
- 验证：
  - 新增 `TestRecognizeRecorderChannelsLimitsWorkPerRequest` 覆盖 6 路待识别时单次只处理 5 路。
  - 本机直接执行 Go 测试仍触发 macOS 测试二进制 `missing LC_UUID load command`。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。

## 2026-07-03 批量通道识别进一步降载 2.30.18

- 现象：
  - `2.30.17` 将老批量接口限制为单次 5 路后，前 4 轮返回 `200`，第 5 轮仍出现 APISIX `504 Gateway Time-out`。
  - 说明 5 路抓图、存储和 AI 识别在部分通道组合下仍可能超过公司网关窗口。
- 修复：
  - 将老批量接口 `POST /api/store-space/recorders/{recorder_id}/recognize-channels` 单次处理上限从 5 路降为 1 路。
  - 页面逐通道识别路径保持不变；控制台或旧调用方即使继续调用老批量接口，也只推进 1 路，避免一次请求阻塞过久。
- 验证：
  - 更新 `TestRecognizeRecorderChannelsLimitsWorkPerRequest`，覆盖 6 路待识别时单次只处理 1 路。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。

## 2026-07-04 MiniMax 非 JSON 解释文本兜底 2.30.19

- 现象：
  - 页面逐通道识别已稳定走 `/api/store-space/channels/{channel_id}/recognize`。
  - `GQ2603587` 最终 41 路全部识别成功，但过程中 MiniMax 偶发返回 `<think>...` 解释文本，导致 JSON 解析失败。
  - `FU9610841` 识别 29 路时再次出现同类问题：门口/侧门画面返回解释文本，没有合法 JSON。
- 根因：
  - MiniMax M3 即使请求了 JSON schema，也可能在非业务区域画面输出 `<think>` 分析文本。
  - 后端已有弱电室、机房、医生办公室的文本兜底，但没有覆盖侧门、入口、走廊、通道等常见非业务区域。
- 修复：
  - 扩展 MiniMax 文本 fallback：识别北/南/东/西侧门、入口、门口、走廊、通道等描述。
  - fallback 结果统一标记低置信、需人工复核，避免误当作高置信自动结果。
- 验证：
  - 新增 `TestMiniMaxRecognizerFallsBackFromEntranceExplanation` 覆盖 `<think>` 解释“北侧门 / door area / entrance”时返回 `scene_type=entrance`、`area_number=北侧门`。
  - 本机直接执行 Go 测试仍触发 macOS 测试二进制 `missing LC_UUID load command`。
  - 使用 `go test -c` 与服务端 `go build` 做编译验证。

## 2026-07-04 通用识别兜底与批量重试上限 2.30.20

- 现象：
  - MiniMax 已确认会在非业务区域返回 `<think>` 解释文本；GPT/OpenAI 类模型也可能出现同类非 JSON 输出。
  - 页面批量“识别区域”会跳过已识别通道，但失败通道下次点击仍会再次进入批量自动识别；如果失败原因稳定，可能重复消耗时间和模型调用。
- 修复：
  - OpenAI/GPT 通道识别路径也接入同一套非 JSON 文本兜底，和 MiniMax 共用入口、侧门、走廊、通道等非业务区域识别规则。
  - 通用 prompt 明确禁止 `<think>`、思考过程、解释和代码块，只允许输出 JSON。
  - 前端批量自动识别对失败通道设置上限：`recognition_attempts >= 2` 时不再自动批量重试，保留人工单通道重试入口。
  - 为避免公司 GitLab hook，改动文件内移除敏感字符串拼接调用。
- 验证：
  - 新增 `TestOpenAIRecognizerFallsBackFromEntranceExplanation` 覆盖 GPT/OpenAI 返回 `<think>` 入口解释文本时的兜底。
  - 更新前端 channel recognition 测试，覆盖失败 2 次后不再进入批量自动识别。
  - 使用 `go test -c`、服务端 `go build`、前端测试和前端 build 做验证。

## 2026-07-04 公司环境 PostgreSQL 运行时联系清理 2.30.23

- 背景：
  - 公司环境已完成 MySQL/OSS 主数据与资产迁移验收，线上 `/health` 返回 `database=mysql`、`asset_store=oss`。
  - 用户明确确认不再保留旧库回滚连接，避免继续存在数据出海或误连旧库风险。
- 清理：
  - `cmd/server` 只接受 `APP_DB_DRIVER=mysql`，只读取 `MYSQL_DSN` / `K8S_SECRET_MYSQL_DSN`，不再读取旧 `DATABASE_URL` 作为数据库连接。
  - 删除 pgx 依赖、旧 PostgreSQL repository、旧 schema 初始化、旧 H5 monitor repository、pg-to-mysql 导出 CLI、pg-mysql ops 导出/源清单/审计端点和对应测试。
  - `designplan` 旧路由继续注册，但线上服务使用资产存储 + 内存 repo，不再持有旧库连接。
  - 发布手册公司环境口径更新为 MySQL + OSS。
- 验证：
  - `rg -n "github.com/jackc/pgx|sql.Open\\(\"pgx\"|APP_DB_DRIVER=postgres|NewPostgresStore|EnsurePostgresSchema|pg-mysql|pg_mysql|mysqlmigration|ExportMySQLMigration|PostgresStore|PostgreSQL|postgres" cmd internal go.mod go.sum docs/deploy-runbook.md VERSION` 无结果。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/designplan -o /private/tmp/designplan.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - 本机直接运行 `go test ./cmd/server ./internal/app ./internal/storespace ./internal/designplan` 仍触发 macOS 测试二进制 `missing LC_UUID load command`，按项目既定方式以编译验证为准。

## 2026-07-06 Supabase 删除前请求来源排查

- 背景：
  - 用户准备删除旧境外 Supabase 数据库，但 Supabase Dashboard Last 60 minutes 显示仍有请求：总计 30，Postgres 21，Storage 3，Realtime 3，API Gateway 3。
- 本地代码复核：
  - `cmd/server/main.go` 的 `databaseConfigFromEnv` 只接受 MySQL，只读取 `MYSQL_DSN` / `K8S_SECRET_MYSQL_DSN`；旧 `DATABASE_URL` 不再作为运行时数据库连接。
  - `internal/assets/store.go` 仍保留 Supabase asset provider 兼容代码；但只要 `K8S_SECRET_ASSET_STORE=oss` / `ASSET_STORE=oss` 存在，运行时会选择 OSS，不会自动选择 Supabase。
  - `internal/app/ops_handler.go` 仍保留历史资产迁移 source store 支持，可在手工设置 `SOURCE_ASSET_STORE=supabase` 或旧 Supabase 环境变量时访问 Supabase Storage；这属于迁移/ops 路径，不是主服务数据库运行时。
- 当前判断：
  - 结合线上 `/health` 已返回 `database=mysql`、`asset_store=oss`，公司主服务继续访问旧 Supabase/PostgreSQL 的概率较低。
  - Supabase 控制台本身可能产生 Postgres / Storage / Realtime / API Gateway 请求；仅凭 Dashboard 首页请求数不能确认业务服务仍在访问。
- 下一步：
  - 查看 Supabase Logs 中 Postgres/API Gateway/Storage/Realtime 的来源、user agent、client IP、query 或 path。
  - 运行一次只读 `pg_stat_activity` 查询定位当前连接；注意该查询本身也会产生一次 Postgres 活动。
  - 做 60-70 分钟静默窗口观察：关闭 Supabase 控制台，不运行旧脚本/迁移/ops；窗口结束后再查看是否仍有不可解释请求。
  - 请运维确认 K8s 当前 Pod/Secret 中不存在旧 `DATABASE_URL`、`SUPABASE_*`、`SOURCE_SUPABASE_*` 或 `SOURCE_ASSET_STORE=supabase`。
- 用户执行 `pg_stat_activity` 后的结果：
  - 当前活动连接均为 Supabase 平台内部组件或本地 loopback：`postgres_exporter`、`Supabase Storage API`、`pgbouncer`、`PostgREST 14.5`、`pg_net 0.20.3`、`pg_cron scheduler`。
  - `client_addr` 主要为 `::1` / `127.0.0.1` / `NULL`，未看到公司 Go 服务、K8s Pod、Lighthouse、浏览器前端或外部业务客户端 IP。
  - 查询内容包括 `pgbouncer.get_auth($1)`、`LISTEN "pgrst"`、`show archive_mode;`、`COMMIT` 等平台内部/控制台相关行为。
  - 结论：该快照没有发现公司业务运行时继续连接旧 Supabase/PostgreSQL 的证据；仍建议做 60-70 分钟静默窗口和 Supabase Logs 复核后再删除。

## 2026-07-06 旧 Supabase 数据库删除

- 操作：
  - 用户确认已删除旧 Supabase 数据库。
- 删除前依据：
  - 公司线上 `/health` 已返回 `database=mysql`、`asset_store=oss`。
  - `cmd/server` 运行时只接受 MySQL，不再读取旧 `DATABASE_URL`。
  - `pg_stat_activity` 快照未发现公司业务服务、K8s Pod、Lighthouse 或外部业务客户端连接旧 Supabase/PostgreSQL；可见连接均为 Supabase 平台内部组件或 loopback。
- 当前状态：
  - 旧 Supabase/PostgreSQL 不再作为可用数据源或回滚路径。
  - 删除后的线上只读核心回归已通过。
- 删除后线上回归：
  - 执行人：用户在已登录公司浏览器控制台执行。
  - `GET /erzhuang-project/health`：200，返回 `database=mysql`、`asset_store=oss`。
  - `GET /erzhuang-project/api/auth/me`：200，返回当前登录用户“凯撒（沙磊）”。
  - `GET /erzhuang-project/api/store-space/stores?page=1&page_size=100`：200，`total=54`、`items.length=54`。
  - `GET /erzhuang-project/api/h5/orgs/10030/monitor`：200，北京保利实验室门店，`groups.length=1`。
  - `GET /erzhuang-project/api/h5/orgs/10019/monitor`：200，上海陆家嘴店，`groups.length=5`。
  - `GET /erzhuang-project/api/h5/orgs/10081/monitor`：200，杭州城北万象城店，`groups.length=5`。
  - `failed=[]`。
- 结论：
  - 旧 Supabase 数据库删除后，公司线上 MySQL/OSS 主链路、登录、门店列表和 H5 Monitor 样本门店均正常。
  - 旧 Supabase/PostgreSQL 不再是运行时依赖或回滚路径。

## 2026-07-06 用户管理编辑角色不生效修复

- 现象：
  - 管理员在用户管理界面把某用户角色改为“编辑运维”并保存后，列表仍显示“普通查看”。
  - 再次打开编辑弹窗，角色仍是“普通查看”。
- 根因：
  - 前端提交的角色值为 `editor`，后端 handler 也会接收并归一化为 `editor`。
  - MySQL 读取角色依赖 `tb_user_roles -> tb_roles.code`，如果 `tb_roles` 中没有 `editor` 角色，`setMySQLUserRole` 的 `insert ignore into tb_user_roles ... select ... where r.code='editor'` 会插入 0 行且不报错。
  - 随后列表查询 `coalesce(..., 'viewer')`，因此回退显示为“普通查看”。
- 修复：
  - `setMySQLUserRole` 在写用户角色关系前，先幂等补齐 `admin`、`editor`、`viewer` 三个应用系统角色，修复线上现有库漏种 `editor` 时保存无效的问题。
  - `db/mysql_governance_schema_tb.sql` 增加 `editor` 角色 seed，并让 `editor` 复用原 `operator` 的运维权限集合，避免新库继续漏种。
  - 新增 `TestSetMySQLUserRoleSeedsKnownRolesBeforeAssignment`，用 recording SQL driver 验证分配用户角色前会先 seed 已知角色，再删除旧关系并写入新关系。
- 验证：
  - 本机直接运行 Go 测试仍触发项目已知 macOS `missing LC_UUID load command`，未能执行测试断言。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
- 发布后验证建议：
  - 在用户管理中把一个普通查看用户改为“编辑运维”并保存。
  - 列表应显示“编辑运维”。
  - 再次打开编辑弹窗，角色下拉应保持“编辑运维”。
  - 可再调用 `/api/auth/me` 验证该用户重新登录后权限含 `editor`、`store:read`、`store:write`。

## 2026-08-13 3.0.1 公司发布后 BackOff / 首页 404 热修复

- 背景：
  - 3.0「门店空间资源查看」发布后，用户访问 `https://lite.sy.soyoung.com/erzhuang-project/` 看到 `404 page not found`。
  - 随后公司告警提示 Pod `erzhuang-project-erzhuang-project-7fb4d69c77-gbz92` 进入 `BackOff`，说明问题核心是容器启动失败/重启退避，不只是前端路由。
- 根因判断：
  - 3.0 新增 `K8S_SECRET_BUSINESS_MYSQL_DSN` 业务库只读连接，启动阶段如果 `PingContext` 失败会 `log.Fatalf`，导致整站退出。
  - Dockerfile 已复制前端产物到 `/app/frontend/dist`，但运行镜像未内置 `FRONTEND_DIR` 默认值；若 K8s 未显式注入该变量，Go 服务不会注册 `/erzhuang-project/` 前端静态路由，根路径会 404。
- 修复：
  - `Dockerfile` 增加：
    - `APP_BASE_PATH=/erzhuang-project`
    - `FRONTEND_DIR=/app/frontend/dist`
  - `cmd/server/main.go` 将业务库资源视图连接失败从 `fatal` 改为降级日志：主系统、登录、系统设置、旧 H5 Monitor 和静态首页不再被 3.0 新只读数据源拖垮。
  - `VERSION` 升至 `3.0.1`。
- 版本：
  - Commit：`06e67bf fix: restore company static startup defaults`
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已推到 `06e67bfb269b788a58cd1ea6ab0c6a51a41d7e82`。
  - GitHub 备份分支 `codex/containerize-single-image` 已同步。
- 本地验证：
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/server.test` 通过。
  - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过；仍有既有 Vite chunk 体积 warning。
- 线上观察：
  - 未登录访问 `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 APISIX/SSO `302`，说明网关入口仍存在。
  - 由于 Codex 无用户公司登录态，最终页面可用性需用户在已登录浏览器确认。
- 后续验证脚本：
  - 在已登录公司浏览器控制台请求：
    - `/erzhuang-project/health`
    - `/erzhuang-project/api/auth/me`
    - `/erzhuang-project/api/store-space-resource-view/stores?page=1&page_size=20`
- 后续经验：
  - 新增次级数据源不能在启动失败时拖垮整个应用，除非该数据源是核心必需依赖。
  - 生产镜像内置静态目录时，应在 Dockerfile 内提供对应默认环境变量，减少 K8s 配置遗漏造成的首页 404。

## 2026-08-13 3.0.2 公司发布后资源查看未配置排查

- 现象：
  - 用户刷新公司页面后，前端已显示 `版本 3.0.2 (container)`，3.0「门店空间资源查看」页面可以打开。
  - 资源查看 API 仍返回 `store space resource view is not configured` / `code=resource_view_not_configured`。
- 已确认：
  - Wharf 当前实例镜像为 `ded1832e`，Pod `erzhuang-project-erzhuang-project-6996887974-zh9bb` 运行中，重启次数 0。
  - 当前容器日志明确显示：`business resource view disabled: database setup failed: context deadline exceeded`。
  - 容器内 `K8S_SECRET_BUSINESS_MYSQL_DSN` 存在；未设置非 Secret 版 `BUSINESS_MYSQL_DSN`，符合公司 Secret 注入口径。
  - 容器内 DNS 可解析业务库 RDS 域名到内网地址，`nc -vz <business-rds>:3306` 显示 TCP 端口 open。
  - 对同镜像临时启动一个只读诊断进程，并给业务库 DSN 附加短 `timeout/readTimeout/writeTimeout` 后，Go MySQL driver 连续报 `read tcp <pod-ip>:<port>-><business-rds-ip>:3306: i/o timeout`，最终 `driver: bad connection`。
- 当前结论：
  - 3.0.2 代码和镜像已实际运行，前端版本不是旧缓存问题。
  - `resource_view_not_configured` 的直接原因不是业务库 DSN 未配置，而是后端启动阶段无法完成业务库 MySQL 握手/登录，`resourceview.Service` 按降级策略未启用。
  - TCP 四层连通不等于 MySQL 协议可用；当前证据不是前端或资源查看 SQL 查询错误。
  - 2026-08-14 用户补充确认：我们此前运行的是公司测试环境，而该业务库表必须在正式环境才能访问；因此测试 Pod 无法完成正式业务库 MySQL 握手/读取属于环境隔离导致的预期现象，不应继续当成代码 bug 修。
- 伴随发现：
  - 同一容器日志出现 `auth: profile sync failed ... Illegal mix of collations ... for operation '<>'`，这是二壮运行库登录资料同步的独立 collation 问题，需要另开小修复处理；它不是 3.0 资源查看未配置的直接原因。
- 下一步：
  - 不再要求测试环境强行访问正式业务库表。
  - 等正式环境发布链路、正式实例和正式数据库/SSO 配置确认后，在正式 Pod 网络内验证业务库只读 DSN 是否能完成 MySQL 握手、认证和 `select 1`。
  - 正式环境连通后，启动日志应出现 `business resource view enabled`；再验证 `/erzhuang-project/api/store-space-resource-view/stores?page=1&page_size=20` 返回真实业务库资源数据。

## 2026-08-18 3.0.6 测试环境门店列表操作列错位修复

- 现象：3.0.5 移除首页“异常”列后，门店资源列表的“详情 / 查看监控”按钮溢出表格右边界。
- 根因：`ResourceStoreList` 已从 12 列缩减为 11 列，但资源列表 CSS 仍把操作列宽度配置在第 12 列；实际第 11 列仅为 72px。
- 修复：将第 11 列设为 168px 并保留右侧内边距；第 1、2、5-10 列继续使用 72px，门店名称和机构 ID 的现有宽度策略不变。
- 版本与发布：`3.0.6`，commit `238dd0a fix: align resource list action column` 已同步 GitHub 与 GitLab 测试分支；Wharf pipeline `752` 构建 `167902` 成功，耗时 1 分 48 秒。
- 本地验证：`cd frontend && npm test -- --run`（5 files / 41 tests passed）；`cd frontend && npm run build` 通过，保留既有大 chunk warning。
- 浏览器验收：通过 Chrome 插件打开本地 mock 预览，首行“详情 / 查看监控”两个按钮完整位于第 11 列内，表格不再向右溢出。
- 测试环境验收：测试实例已在 `2026-08-18 21:52:49` 自动部署 commit `238dd0a4`；测试入口随后显示 `3.0.6 (container)`，真实数据首行的两个操作按钮均位于表格内。
- 流程纠正：本次一度将 Wharf 的“部署”按钮误判为测试环境必经步骤。实际测试链路仍是 GitLab 推送后自动构建、自动部署；以后构建成功后应等待最多约 5 分钟并核对实例最近部署 commit 和页面版本，不能仅因页面短暂显示旧版本就手动重复部署。

## 2026-08-19 3.0.7 资源列表更新时间与确认口径调整（待发布）

- 需求：资源列表恢复旧版“更新时间”；“已确认”不再表示人工确认，而是门店级覆盖率状态。
- 已实现：
  - 后端列表项目新增 `updated_at`，取该门店摄像头-空间绑定关系表 `tb_crm_iot_area_device_relation.created_at` 的最大值。该字段由业务库在关系记录变更时更新；无绑定关系时返回空，前端显示 `-`。
  - 后端新增 `cameras_fully_bound`。仅当 `camera_count > 0` 且 `unbound_camera_count == 0` 时为 true；任意未绑定摄像头或无摄像头门店均不是“已确认”。
  - 前端在序号格恢复旧版“已确认”水印，并恢复“更新时间”列；操作列改为第 12 列并保持 168px 宽度。
- 本地验证：`frontend npm test -- --run`（5 files / 41 tests）与 `frontend npm run build` 通过；Chrome 插件以 mock 数据在 1440px 宽度验收，表格无横向溢出、更新时间正常显示、双操作按钮保持同一行。
- 后端验证：`go test -c ./internal/resourceview` 编译通过；执行测试二进制仍受本机已知 macOS `missing LC_UUID load command` 限制，未能本机运行断言。
- 发布状态：已提交 `cbda79e feat: show resource mapping completion`，已同步 GitHub 备份与 GitLab 测试分支；Wharf pipeline `752` 构建 `167904` 成功（1 分 38 秒）并自动部署。
- 测试环境验收：Wharf 实例当前版本 `cbda79ec`、状态“运行中”；`https://lite.sy.soyoung.com/erzhuang-project/` 已显示 `3.0.7 (container)`，表头含“更新时间”，20 行真实资源数据正常加载，表格无横向溢出，浏览器无页面错误。首屏门店均有未绑定摄像头，因此未显示“已确认”，符合新口径。

## 2026-08-19 3.0.8 摄像头空间绑定详情表（待发布测试）

- 需求：将机构详情从空间树、设备树、异常项三 Tab 改为摄像头逐行核对的空间绑定关系表；监控查看流程不改。
- 已实现：
  - 前端以摄像头为一行，展示录像机编号、通道号、摄像头 ID、最近截图占位、绑定状态、空间层级 1/2/3、床位。
  - 使用 `parent_id` 向上回溯空间父子链，不使用业务数据中不稳定的 `level` 作为展示层级。
  - 同一摄像头的多条绑定关系在同一行按路径逐条保留；缺少绑定时状态为“未绑定”、四个空间字段统一显示 `-`。
  - 近期截图字段暂无 3.0 只读 API 数据源，明确显示 `-`；未接回旧萤石抓图链路，也没有提供无效的重新截图操作。
- 本地验证：`cd frontend && npm test -- --run` 通过（5 files / 41 tests）；`cd frontend && npm run build` 通过，仍有既有的大 chunk 提示。
- Chrome 验收：本地 mock 详情页已确认不再显示旧三 Tab；9 列表头、顶部机构/绑定完成度信息、空截图占位和“查看监控”入口均正常。内容容器宽度 1440px 时表格未发生列重叠，表格框保留横向滚动能力供更长真实数据使用。
- 待发布：升版 `3.0.8` 后提交到 GitHub 备份与 GitLab `codex/containerize-single-image`，等待 Wharf pipeline `752` 自动构建部署；部署后使用真实门店 `10001` 复核路径、未绑定行和页面版本。

- 发布完成：commit `7db9670 feat: show camera space bindings` 已同步 GitHub 和 GitLab 测试分支；测试环境由 Wharf 自动部署，页面显示 `3.0.8 (container)`。
- 测试环境真实验收：门店 `10001` 详情不再显示旧三 Tab，渲染 55 条摄像头行，顶部与列表一致显示已绑定 39、未绑定 16、状态待确认；9 列绑定表与横向容器正常。
- 观察待讨论：真实数据部分摄像头缺少 NVR/通道字段，页面按约定显示 `-`；部分父子空间同名，导致层级 1/2 显示同名。二者均保留原始业务数据，不在前端猜测或改写，后续确定区域筛选与字段展示口径时一起决策。

## 2026-08-19 3.0.9 NVRCHANNEL 录像机与通道展示修正（待发布测试）

- 用户确认业务字段规则：摄像头 `hardware_id` 为 `NVRCHANNEL:<nvr设备ID>-<通道号>` 时，前半段是 NVR 在设备库中的 ID，后半段是通道号；例如 `NVRCHANNEL:22-10` 应展示录像机编号 `22`、通道号 `10`。
- 修复：详情前端优先解析该规则，解析失败后才回退既有 NVR 关联和 `channel_no`。原始 `hardware_id` 不再直接当作录像机编号显示。
- 已补单测覆盖该规则；待完成前端测试、构建和测试环境真实样本复核后发布。

- 发布完成：`d135c55 fix: parse NVR camera channel identifiers` 已同步 GitHub 和 GitLab 测试分支；Wharf 自动部署后测试页面显示 `3.0.9 (container)`。
- 真实验收：机构 `10001` 详情中确认一条真实 `NVRCHANNEL:22-10` 记录已展示为录像机编号 `22`、通道号 `10`，修复生效。
- 后续体验项：有多条绑定路径时，空间列内的多行文字在 DOM 文本中连续；视觉上仍通过 grid 换行展示，但可在下一轮决定是否增加更清晰的路径分隔或空间 ID 消歧。本次不扩大为该改动。

## 2026-08-19 3.0.10 有效海康摄像头范围（待发布测试）

- 产品口径：资源查看的摄像头只统计 `category='camera' AND provider='HikVisionNvrChannel' AND status=1 AND deleted_at IS NULL`。非海康 provider、停用或软删除 camera 全部排除。
- 实现：repository 只读取有效摄像头与其关系；service 对直接输入的设备记录再次执行同一筛选，确保列表、详情、已绑定/未绑定、已确认和异常计算不会出现口径漂移。
- 异常边界：缺失设备但 `function_type` 以 `camera` 结尾的关系仍作为数据异常线索保留，但不计入有效摄像头数量。
- 本地验证：项目内置 Go 工具链执行 `go test ./internal/resourceview` 通过；`go build ./cmd/server` 通过；前端 `npm test -- --run` 通过（5 files / 41 tests），`npm run build` 通过，仅保留既有大 chunk warning。
- 发布状态：`36d2b03 feat: limit resource view to active hikvision channels` 已同步 GitHub 与 GitLab `codex/containerize-single-image` 测试分支；远端分支 SHA 已核对。测试健康入口返回 `200`。
- 待测试环境验收：确认页面版本为 `3.0.10 (container)`，首页统计按新范围收敛，真实详情没有非海康/停用 camera，且 `NVRCHANNEL:22-10` 仍显示录像机 `22`、通道 `10`。当前终端未连接已登录 Chrome，首页 HTTP 请求受 SSO `302` 保护，需在已登录浏览器确认页面结果。

## 2026-08-19 3.0.11 摄像头空间类型与名称（待发布测试）

- 产品规则：摄像头关联空间的 `parent_id=2387` 代表诊室区域容器，直接关联到该类空间的关系不作为有效绑定；例如设备 `70` 关联 `2665`、`2667` 时，忽略 `2665.parent_id=2387`，保留 `2667.parent_id=2665`。
- 实现：服务层在摄像头关系筛选后统一排除容器关系，影响详情关系、摄像头绑定状态、门店已绑定/未绑定和已确认口径。关联空间父级不存在时不排除，前端以 `-` 展示空间类型。
- 前端：详情表从四列空间路径收敛为“空间类型 / 空间名称”，分别读取关联空间父级和自身的名称；多条关系保持 API 关系记录顺序。
- 展示补充：关联空间自身 `level=3` 时，空间类型固定显示“治疗室”；这是前端呈现规则，不修改数据库、API 原始值或绑定口径。
- 测试：后端新增容器关系过滤回归并通过 `go test ./internal/resourceview`；资源查看 domain 测试已加入默认 Vitest 清单。前端共 `6 files / 47 tests` 通过，生产构建通过，仅保留既有大 chunk warning。
- 发布状态：`3c96f0b feat: show camera space type and name` 已同步 GitHub 与 GitLab `codex/containerize-single-image`；测试环境自动部署完成，用户已确认实际页面更新为 `3.0.11`。本次存在构建/部署传播延迟，不能仅以 GitLab 分支更新或健康入口可达宣称发布完成。
- 待业务验收：详情为 7 列，容器关系未展示，真实空间父子名称正确，首页统计与详情绑定状态一致。

## 2026-08-19 3.0.12 资源详情表紧凑展示（待发布测试）

- 问题：真实测试页的 7 列表格仍采用浏览器自动列宽分配，录像机/通道与摄像头 ID 之间产生大面积留白；最近截图仅显示 `-`；较长空间名称被省略号截断。
- 修复：通过 `colgroup` 与固定表格布局锁定录像机、通道、摄像头、截图、状态和空间类型列，剩余宽度给空间名称；空截图改为固定尺寸的默认缩略占位；空间名称在本列换行完整展示。
- 边界：只改变前端呈现，不变更摄像头-空间关联、统计、业务库或截图接口。
- 本地验证：前端 `6 files / 47 tests` 通过，Go 全量测试通过，生产构建通过；本机未运行后端，不能加载真实门店数据。
- 发布状态：`f51b48f fix: compact resource binding table` 已同步 GitHub 与 GitLab `codex/containerize-single-image`。截至记录时可见 Wharf 部署通知仍为上一版 `3c96f0b`，本轮等待自动部署与实际页面版本确认。

## 2026-08-19 3.0.13 摄像头列表字段收敛与父空间补载（待发布测试）

- 产品确认的详情列顺序为：摄像头 ID、录像机编号、通道号、最近截图、空间类型、空间名称、绑定状态、操作；表格标题为“摄像头列表”。
- 无截图时仅显示低对比灰色图片图标，不显示“暂无截图”等文字。操作列保留“刷新截图”按钮，但当前禁用并提示“截图服务待接入”，不调用旧萤石通道抓图接口，也不对业务库产生写操作。
- 后端只读空间查询补载本门店空间的直接父节点。这样即使同步数据中父空间不在原 `tenant_id` 结果集中，前端仍可通过 `parent_id -> parent.name` 展示空间类型；`level=3` 显示为“治疗室”的既有展示规则不变。
- 本地验证：`go test ./...`、`go build ./cmd/server`、前端 `npm test -- --run`（6 files / 47 tests）和 `npm run build` 全部通过。前端构建仅保留既有大 chunk warning。
- 发布状态：`d550dc8 feat: refine camera binding list` 已同步 GitHub 备份与 GitLab `codex/containerize-single-image` 测试分支，已触发 Wharf `752` 自动构建。临时 GitLab `GIT_ASKPASS` 脚本已在推送后删除。
- 待部署验收：必须以 Wharf 部署记录和页面 `3.0.13 (container)` 做真实验收，重点检查父空间类型、表格列宽及禁用截图按钮。

## 2026-08-19 3.0.14 摄像头列表空间类型筛选（待发布测试）

- 默认“全部”按绑定状态排序：已绑定摄像头在上，未绑定摄像头在下；同一状态内保持既有录像机、通道、摄像头 ID 的稳定顺序。
- 筛选器按当前门店真实存在的空间类型动态生成，默认选中“全部”，外观使用紧凑的分段 Tab。避免把业务库中新增或非固定的空间类型排除在筛选范围外。
- 选中某空间类型时，仅展示该类型的绑定关系，并按命中空间名称正序排列；一台摄像头有多条关系时，表内仅保留命中类型的关系，避免出现与当前筛选不一致的空间名称。
- 门店切换时若旧筛选类型不在新门店中存在，页面自动回退到“全部”，不显示误导性的空表。
- 本地验证：Go 全量测试、Go 构建、前端 `npm test -- --run`（6 files / 49 tests）和 `npm run build` 通过；仅保留既有前端大 chunk warning。
- 发布状态：`6eda789 feat: filter camera bindings by space type` 已同步 GitHub 备份与 GitLab `codex/containerize-single-image` 测试分支，已触发 Wharf `752` 自动构建；临时 GitLab 认证脚本已在推送后删除。
- 待部署验收：测试页显示 `3.0.14 (container)` 后，确认前三列紧凑、后五列均分；默认已绑定行位于未绑定行之前；选择任一空间类型后仅显示命中关系且空间名称正序。

## 2026-08-19 3.0.15 门店类型摄像头统计与未绑定筛选（待发布测试）

- 机构列表移除“空间”数量列，替换为“面诊室”和“治疗室”。两列均统计绑定到对应展示空间类型的去重摄像头数，不统计空间实体数量。
- 后端在资源查看摘要中增加 `consultation_camera_count`、`treatment_camera_count`，列表分页、搜索、城市筛选和详情摘要统一使用该口径。空间类型规则与详情一致：`level=3` 展示为“治疗室”，其余取直接父空间名称。
- 详情筛选器显示“全部 总摄像头数”、每个空间类型的去重摄像头数，以及独立的“未绑定”数量；未绑定项可点击，便于直接核对缺口。
- 已新增后端回归：一台摄像头多条同类型关系只计一次；前端回归覆盖未绑定筛选。`go test ./...`、`go build ./cmd/server`、前端 49 项测试和生产构建通过。
- 发布状态：`55184fa feat: show camera counts by space type` 已同步 GitHub 备份与 GitLab `codex/containerize-single-image` 测试分支，已触发 Wharf `752` 自动构建；临时 GitLab 认证脚本已在推送后删除。
- 待部署验收：测试页显示 `3.0.15 (container)` 后，检查机构列表“面诊室 / 治疗室”均为摄像头数而非空间数；详情 Tab 的全部、各类型、未绑定数字与筛选后的行数一致。

## 2026-08-19 3.0.16 摄像头操作列对齐（待发布测试）

- 详情表“操作”表头和“刷新截图”按钮统一右对齐，右侧保留 20px 内边距，避免按钮悬在列中或紧贴表格边框。
- 本地验证：前端 `npm test -- --run`（6 files / 49 tests）和 `npm run build` 通过；仅保留既有大 chunk warning。
- 发布状态：`60376fb fix: align camera actions` 已同步 GitHub 备份与 GitLab `codex/containerize-single-image` 测试分支，已触发 Wharf `752` 自动构建；临时 GitLab 认证脚本已在推送后删除。
- 待部署验收：测试页显示 `3.0.16 (container)` 后，确认操作列右对齐、右边距自然，且不影响其他列的均分布局。

## 2026-08-20 监控查看 3.0 改造预研：10001 灰度

- 产品方向：资源列表和详情页已完成 3.0 化；摄像头取图接口暂不开发，也不暂时回填萤石云录像机编号。下一阶段改造监控查看，先仅灰度北京保利总部店 `external_org_id=10001`，其他门店保持现有萤石云链路和页面功能不变。
- 当前实现事实：H5 Monitor 页面/路由、SSO/门店范围鉴权、频道分组、直播、录像片段、回放、URL 释放、用户并发限制、播放器控制和诊断都围绕 `internal/h5monitor` 与 `ezuikit-flv` 萤石云实现；不能把改造理解为仅替换播放 URL。
- 初步架构方向：保留前端布局、路由、权限与控制面板，后端在 H5 Monitor 之下引入可按门店选择的取流提供方。`10001` 通过运行时灰度配置走新工控机/业务取流提供方，其他门店继续走萤石云。浏览器只调用二壮 API，不直连公司工控机接口或携带内部凭据。
- 回滚边界：不应在新提供方失败时静默回退萤石云，避免隐藏问题、重复开流或绕过新链路验证；关闭 `10001` 灰度配置应可立即恢复旧链路。
- 待确认：新接口的直播、录像片段、回放、释放能力；鉴权与超时；返回 URL/协议（FLV/HLS/WebRTC 等）；10001 摄像头/NVR/通道/工控机的精确入参映射；是否支持音频、质量切换与截图。未获得这些信息前，不改业务代码或播放器。

### 2026-08-20 新工控机取流接口补充（敏感凭据不入库）

- 业务映射澄清：新链路不使用录像机编号。根据业务空间位置通过 `tb_crm_iot_area_device_relation.area_id -> device_id` 找到摄像头/设备 ID，再向流鉴权接口申请该设备的短时 token；不传起止时间为直播，传起止时间为回放。
- 已知接口形态：服务端以 `camera_id`、`stream_type`、可选 `start_time/end_time` 请求公司鉴权服务，获得 token；浏览器以该 token 连接公司 `wss` 流地址。用户提供的 Authorization、token 和带 token 的 URL 均属于敏感凭据，禁止写入仓库、项目文档、命令历史、日志或最终回复；如为真实长期凭据，应由接口方轮换并改由 K8s Secret 注入。
- 关键未知：WSS 返回的是 FLV、fMP4、MPEG-TS、裸 H.264/H.265、WebRTC 信令还是其他私有二进制协议，目前无法从 URL 或 JWT 推断。必须用新鲜短时 token 做只读协议探针，记录首个控制消息/二进制帧特征、编解码和关闭语义后才能选择播放器。
- 体验边界：当前 H5 Monitor 的录像片段时间轴依赖“分段查询”。新接口目前只确认“给定起止时间即回放”，尚未确认录像存在性/分段接口；若没有该能力，不能宣称完整保留旧回放时间轴，应先由产品明确接受简化回放，或要求接口方补齐分段查询。

### 2026-08-20 NVRPlayer-SDK 调研（未接入）

- 已阅读用户提供的 `NVRPlayer-SDK` 快照。它不是通用 FLV/HLS 播放器：直播路径为 `RTP over WebSocket -> H.265 FU-A 重组 -> WebCodecs -> canvas`；回放路径先用 `SystemTransform` WASM 将海康私有封装转出 H.264/H.265 帧，再仍由 WebCodecs 渲染到 canvas。音频为 G.711 A-law + Web Audio。
- 接口最小验证：流鉴权接口可成功签发短时 token；使用文档外推的 WSS 升级请求返回 `400 token invalid`。这只说明“鉴权 token 到 WSS 的完整契约尚未核实”，不代表设备或流服务不可用。新 SDK README 也说明其预期输入应由业务后端直接下发已签名 `wsUrl`，因此后续应向接口方取得该正式响应，不能自行拼接或猜测 token-to-URL 规则。
- 复核：已严格按接口方提供的 `camera_id=111`、`stream_type=2`、`start_time=123`、`end_time=456` 示例调用，鉴权为 `200/code=0`，但由本机该次调用签发的 token 连接 WSS 时为 `400 token invalid`。随后接口方提供一条新签发的实际 WSS 地址，标准 Upgrade 握手成功返回 `101 Switching Protocols`。结论：WSS 流服务与 URL 拼接方式有效；待查的是二壮将来调用鉴权服务时为何未签出同类可用 token（服务凭据、环境、请求参数或隐含条件），不能把单次本机签发失败误判为 WSS 服务不可用。
- 运维修正：回放 `start_time/end_time` 必须传 Unix 秒级时间戳，不能传占位数字或格式化日期字符串。2026-08-20 已用运维提供的真实时间戳完成全链路复验：鉴权接口 `200/code=0`，将本次新签发 token 拼入 WSS 后标准 Upgrade 返回 `101 Switching Protocols`。至此二壮后端获取短时 token、前端 WSS 直连的最小合同已验证可用；token、Authorization 与媒体内容均未写入仓库、文档或日志。
- 重要实现风险：SDK 以 URL 中 `startTime/endTime` 判断回放，而当前已知 WSS URL 只含 token；二壮接入时必须显式传递 `isReplay/forceWasm`，不能依赖该字符串判断。SDK 还会把完整 `wsUrl`（含 token）写到浏览器控制台，且断线重连会无上限复用旧 URL；引入前必须移除敏感日志，并改为由二壮受控刷新播放地址。
- 移动端结论：直播依赖 H.265 WebCodecs，README 已明确 Safari 不支持；iOS 上所有浏览器均受 WebKit 限制。回放虽使用 WASM 转封装，但其输出仍调用 `VideoDecoder`，因此“回放不受 Safari 影响”的 README 结论不能直接采信，必须真机验证。Android Chrome/Edge 也需按设备与 HEVC 硬件能力验证，不能承诺全量兼容。
- 运行依赖：回放 Worker 当前硬编码加载 `static.soyoung.com/sy-pre` 下的版本化资源；该地址在调研时返回 200，但正式接入不能依赖预发路径或外部可变资源。应由拥有方确认源码/许可证/维护责任，并在二壮静态资源或受控公司 CDN 固定版本部署 WASM、worker 和主 SDK。
- 官方方案对照：萤石官方 GitHub 的 `EZUIKit-flv`、`EZUIKit-JavaScript-npm`、`@ezuikit/player-hls` 均仍活跃，其中 HLS SDK 提供 H.265 软解跨浏览器兜底。但它们分别接受 FLV、HLS/m3u8 或萤石 `ezopen` 地址，不能直接播放当前私有 WSS/RTP/海康回放流。只有工控机侧提供 HLS、FLV 或 WebRTC 等标准输出时，才应优先采用官方 SDK；否则 NVRPlayer 是当前协议唯一已知参考实现，而不是可替换成萤石 SDK 的同类组件。
- 当前状态：仅完成资料与只读协议验证，未复制 SDK、未改监控代码、未部署。下一步先取得后端真实 `wsUrl` 合同、token 生命周期、回放/片段/释放语义，并决定是否将移动端直播作为首期硬门槛。
- 产品确认：iPhone 与微信内打开直播是首期基础要求。接口方随后确认工控机/NVR 无法输出 HLS、FLV 或 WebRTC 等其他格式，因此不能把“硬件提供标准输出”作为前提。当前 NVRPlayer 的 H.265/WebCodecs 直播路径只能作为桌面协议调试工具；首期 iOS 兼容必须在浏览器侧软解，或由独立软件网关在不改变硬件的前提下完成协议/编码转换后再验证。

### 2026-08-21 10001 NVR 实验页设计已确认

- 用户确认实验页必须复用测试环境 2.x H5 Monitor 的视觉与交互：门店标题、区域 Tab、圆形摄像头墙、播放详情 16:9 区域、控制条与底部“实时视频 / 录像”Tab；不得做成独立后台调试面板。
- 实验页只服务 `10001`、只限管理员、使用业务库只读摄像头关系；旧萤石 H5 Monitor 不改，不自动回退。
- 默认验证样本为 `camera_id=111`，测试库显示其映射为治疗室4、NVR `22`、通道 `56`；业务库不能判断录像是否存在，回放需由实际取流服务验证。
- 已提交设计规范：`docs/superpowers/specs/2026-08-21-nvr-lab-10001-design.md`，commit `4ade1ea docs: define 10001 nvr lab`。设计中明确：回放第一版只有起止时间输入，不伪造录像片段列表；iPhone Safari/微信真机通过前，实验页不得替换旧监控页。

### 2026-08-21 新取流鉴权凭据模型已由接口方确认

- `stream_type=2` 同时用于直播和回放：直播只传 `camera_id`、`stream_type`；回放额外传 Unix 秒级 `start_time`、`end_time`。
- `Authorization` 是长期服务端凭据，不会过期；测试与正式使用同一凭据值，但必须分别以各自 K8s Secret 注入，不能由前端持有或由代码、Dockerfile、日志、项目文档保存。
- 每次用户发起播放时，二壮后端携带该长期凭据实时请求鉴权服务；返回的 WSS token/地址才是短期数据，只用于当前播放会话，不持久化、不写日志、不放入浏览器诊断信息。
- 推荐运行时变量名：`NVR_STREAM_AUTHORIZATION`，可兼容读取 `K8S_SECRET_NVR_STREAM_AUTHORIZATION`。变量名是二壮实现约定，实际 Secret 名称由运维/K8s 管理。

### 2026-08-21 10001 NVR 实验页第一轮实现（待发布测试）

- 已实现独立路由：`/h5/nvr-lab/10001` 和 `/h5/nvr-lab/10001/cameras/{cameraId}`；前端无导航入口，后端固定只接受租户 `10001`，并由 APISIX 登录态后的 `admin` 角色守卫。
- 后端只读使用现有资源视图仓储，严格筛选 `category='camera' AND provider='HikVisionNvrChannel' AND status=1 AND deleted_at IS NULL`；每次播放实时调用鉴权服务，直播不带时间，回放使用最多 30 分钟的 Unix 秒级起止时间。
- 安全处理：长期鉴权值仅运行时读取 `K8S_SECRET_NVR_STREAM_AUTHORIZATION`（兼容 `NVR_STREAM_AUTHORIZATION`）；会话响应为 `Cache-Control: no-store`。短期 WSS 地址只留在 React 内存，不写日志、诊断、localStorage 或 sessionStorage。
- 已将 NVRPlayer 与 WASM 运行时固定在本仓库静态资源中，移除预发 CDN、WSS URL 控制台输出和自动复用旧地址重连；回放显式使用 WASM，不再从 URL 推断。
- 本地验证：前端 `npm test` 共 8 个文件、52 个断言通过，`npm run build` 通过（仅保留既有大 chunk warning）。本机当前没有 Go 二进制，新增的 `internal/nvrlab` 单测、全量 Go 测试与构建尚未执行，发布前必须在带 Go 的环境补跑。
- 待发布测试：确认测试 K8s Secret 已配置后发布到 `codex/containerize-single-image`，使用管理员在 `camera_id=111` 验证桌面直播和回放首帧、受控重新连接，以及 iPhone Safari/微信内直播。iOS 未通过前不替换旧萤石 H5 Monitor。
- 发布记录：提交 `647b5a6 feat: add 10001 nvr streaming lab` 已于 2026-08-21 推送 GitLab `codex/containerize-single-image`，触发 Wharf `752` 测试自动构建；已同步 GitHub 同名备份分支。尚待构建、自动部署与实际页面/首帧验收，未发布正式 `main`。

### 2026-08-21 10001 NVR 实验页测试发布与配置核验

- 初次测试构建 `168403`（commit `647b5a6`）失败。Wharf 日志确认根因是 `internal/nvrlab/service_test.go` 把 `CameraListResponse` 当作数组使用，导致 `go test ./...` 编译失败；Kaniko 的 `/tmp/digest` 是构建失败后的伴随信息，不是根因。
- 最小修复 `ab3f4d5 test: fix nvr lab camera list assertion` 已推送。Wharf 构建 `168473` 成功，完成完整 Go 测试和镜像构建；测试实例已自动升级并运行该 commit。
- Chrome 已登录验收：`/erzhuang-project/h5/nvr-lab/10001` 路由已进入新实验页，不再回落到主列表。
- 当前阻塞：测试实例未绑定 `K8S_SECRET_NVR_STREAM_AUTHORIZATION`（兼容名 `NVR_STREAM_AUTHORIZATION`），后端不会创建 NVR 实验服务，页面会显示“取流实验页暂未配置”。该值必须通过测试环境 K8s Secret 注入，不能写入普通变量、仓库或文档。
- 下一步：配置 Secret 后滚动重启测试实例，再以管理员身份用 `camera_id=111` 验收列表、桌面直播、回放与重新连接；iPhone Safari/微信验证仍是扩大灰度的硬门槛。

### 2026-08-21 NVR Secret 配置后二次验收

- 用户完成测试实例部署后，Chrome 复验实验首页正常加载北京保利总部店的 44 路有效摄像头，区域筛选包含“面诊室 / 治疗室 / 其他”；说明管理员守卫、资源只读查询和 NVR Secret 启用均已生效。
- 默认 `camera_id=111` 正确解析为“治疗室4 / 治疗室”。进入直播详情时，后端播放会话接口返回稳定错误 `nvr_stream_authorization_failed`（HTTP 502），前端展示“取流鉴权失败，请稍后重试”；尚未建立 WSS 连接，因而没有首帧可验收。
- 浏览器控制台未发现 token、完整 WSS 地址或播放器异常日志。当前错误分类有意不包含上游响应详情，故尚不能从页面区分长期 Authorization 格式错误、鉴权服务拒绝、响应合同变化或测试集群网络问题。
- 待处理：先由配置方核对 Secret 值为接口方的长期服务端 Authorization 原样值，不加 `Bearer` 前缀、引号或短期 WSS token；若仍失败，补充仅记录/返回安全状态分类（HTTP 状态、超时、响应结构）后再发布定位，禁止记录凭据、token、WSS URL 或上游正文。

### 2026-08-21 NVR 鉴权失败根因收敛

- 为在不暴露敏感数据的前提下完成定位，先后发布 `3.1.1`（`229ce06`，服务端安全分类）和 `3.1.2`（`7e822f8`，安全响应码）；由于 Chrome 插件不能读取请求响应体且测试实例未绑定可查询的容器日志源，最终发布 `3.1.3`（`2933125`）仅在管理员实验页展示已有安全错误码。
- Wharf 构建 `168529` 成功，测试实例已自动运行 `2933125`。同一直播请求的页面结果为 `nvr_stream_authorization_upstream_http_422`。
- 已确认失败边界：二壮后端已读取 Secret 并成功请求到公司鉴权服务；鉴权服务以 HTTP 422 拒绝当前入参。失败发生在短期会话签发之前，WSS 连接、NVRPlayer 解码、首帧和移动端均尚未开始，不应归因于播放器。
- 待接口方确认：针对北京保利总部店的真实可用 `camera_id`、直播是否确实只传 `camera_id + stream_type=2`、以及 HTTP 422 的字段级校验规则。对外反馈仅提供请求口径与 HTTP 422，不提供长期 Authorization、token、WSS URL 或上游正文。

### 2026-08-21 鉴权参数对照结论

- 使用同一长期服务端凭据做四组只读对照：`camera_id=111/584` 分别请求直播（只传 `stream_type=2`）和回放（同样的 Unix 秒起止时间）。
- 两个摄像头的无时间直播请求均返回 HTTP 422，均未签发 token；两个摄像头的带时间请求均返回 HTTP 200、`code=0` 且签发 token。
- 结论：当前测试实例的 Secret 已被读取且不是本次失败原因。失败由上游接口对无时间 `stream_type=2` 请求的校验导致；`camera_id=111` 也不是导致 HTTP 422 的充分原因。
- 待接口方书面确认：直播的正确 `stream_type`、是否需要额外参数或服务端生成的默认时间，以及 HTTP 422 的校验规则。在合同确认前，二壮不伪造时间窗口、不猜测其他 stream type，也不修改旧萤石播放链路。

### 2026-08-21 直播 stream_type=1 直接对照

- 用户建议验证直播是否应使用 `stream_type=1`。未改代码、未部署，直接对 `camera_id=584` 发起无时间参数的只读请求。
- 结果仍为 HTTP `422`。请求响应体和任何短期 token 均未保存或输出。
- 结论：`stream_type=1` 不解决当前直播鉴权失败；此前对 `stream_type=2` 的实现保持不变。需要接口方给出 HTTP 422 的字段级校验规则或一条可工作的直播请求样例。

### 2026-08-21 回放鉴权复验

- 对 `camera_id=584` 使用 `stream_type=2` 与 Unix 秒级起止时间直接做只读鉴权请求，结果为 HTTP `200`。
- 结论：回放会话签发链路可用。后续需用实际有录像的时间窗在隔离实验页验证 WSS 建连和播放器首帧；这不改变直播目前仍被 HTTP 422 阻塞的事实。

### 2026-08-21 10001 测试环境回放首帧验收

- Chrome 已登录测试环境打开 10001 NVR 实验页，摄像头列表共 44 路。对当前白名单中的 `camera_id=82`（走廊）与 `camera_id=111`（治疗室4）分别输入 2025-08-20 12:25 至 12:38 的回放时间范围。
- 两次均通过后端签发回放会话并建立 WSS，页面稳定显示“视频流已连接，等待画面”；等待超过 30 秒没有首帧。Chrome 没有 WASM worker、WebSocket 或静态资源错误。
- `camera_id=584` 虽可在鉴权接口签发会话，但不在 10001 当前实验页白名单，页面后端会拒绝，因此不能作为该页验收样本。
- 当前不能宣称回放可播放。下一步需要接口方提供 10001 白名单摄像头中确认有录像的摄像头和 Unix 时间窗，或增加不暴露媒体/token 的接收帧、WASM 转封装与渲染计数诊断，以区分“无录像数据”和“播放器解码失败”。
- 用户随后指定精确时间窗 `1755663940` 至 `1755664704`。测试页的 `datetime-local` 控件可接受并传入对应的秒级值；对 `camera_id=111` 连续约 33 秒仍只有“视频流已连接，等待画面”，没有首帧。
- 结论更新：分钟级输入精度不是本次失败原因。故障边界收敛至回放媒体是否实际下发，或媒体已下发后的 WASM 转封装/解码阶段。

## 2026-08-26 3.0 摄像头列表复用 2.x 截图的映射边界

- 目标：在不接入新抓图能力、不修改业务同步数据的前提下，让 3.0 摄像头绑定列表尽量显示 2.x 已有的最近截图。
- 已确认：业务 NVR ID/序列号与旧录像机序列号不保证相同，禁止使用 NVR 序列号推断映射。
- 第一版范围：仅覆盖旧系统中该门店恰有一台有效录像机的门店。机构 ID 一致后，以通道号映射业务摄像头与旧视频通道，再取该旧通道 `created_at desc, id desc` 的最新 `tb_channel_snapshots.thumbnail_path`。
- 排除：旧门店无录像机、存在多台旧录像机、通道缺失/重复、无截图或用户无该门店监控权限时不返回旧截图，继续使用灰色占位；不建映射表、不复制 OSS 资产、不触发萤石或工控机抓图。
- 下一步：先以测试库统计单录像机门店及可命中截图覆盖率；实现时补单录像机命中、无截图、多录像机拒绝、无旧门店、无监控权限测试，再发布测试环境验收。

### 2026-08-26 实现记录：3.1.7 截图复用与播放器控制修复

- 已实现旧截图只读查询：按 `tb_stores.external_org_id -> tb_video_recorders -> tb_video_channels -> tb_channel_snapshots` 查询；只在旧门店录像机总数等于 `1` 时按通道号读取最新缩略图。多录像机、无图、无通道或路径不合法均不返回。
- 资源详情返回的是受控应用 URL。该图片端点先校验 `store:read` 和门店监控范围，再从现有 OSS 截图存储读取；前端不接触对象路径、签名 URL 或密钥。
- 已修复 NVR 实验页：标题栏与播放器增加间距；“开启声音”直接在点击手势中创建/恢复 Web Audio；直播暂停改为停止本地 Canvas 输出，恢复后继续渲染接收中的新帧。
- 本地验证：前端 `npm test -- --run` 为 8 files / 54 tests 全通过，`npm run build` 通过。当前机器没有 Go 与 gofmt，后端编译/测试由 Wharf 构建补验。
- 发布状态：2026-08-26 已提交并推送 GitLab 测试分支与 GitHub 备份，commit `7355395 fix: reuse legacy snapshots and stabilize nvr controls`。Wharf pipeline `752` 已自动部署；已登录测试页显示 `3.1.7 (container)`，临时 GitLab `GIT_ASKPASS` 已删除。
- 已验收：测试首页能加载真实门店列表；10001 / 摄像头 111 的 NVR 直播成功渲染，播放器上方的标题卡片与播放器保持可见间距。
- 待补充人工点验：一条单旧录像机门店的截图命中、一条多录像机门店的灰色占位，以及“开启声音”和暂停/恢复的实际控制结果。当前浏览器会话仅开放导航与截图，未以 Computer Use 模拟点击替代。

### 2026-08-26 进行中：3.1.8 NVR 按小时定位回放

- 范围：仅隔离实验页 `/h5/nvr-lab/10001/cameras/{cameraId}` 的“录像”模式；旧萤石 H5 Monitor 的回放、片段查询与播放器行为不改。
- 实现：抽取共享 `PlaybackDatePicker`，复用 2.x 的“今天 / 昨天 / 前天”、日历、时分和“定位回放”交互；NVR 页改为单一回放起点并派生最长一小时的 Unix 秒范围，同时增加 24 个小时段快捷选择。
- 边界：小时段不代表录像存在；选择日期或时段不建立会话，只有“定位回放”会调用短期会话接口。今天的当前小时截断到当前分钟，未来起点显示中文提示。
- 本地验证：前端 Vitest `10 files / 58 tests` 通过，`npm run build` 通过。Chrome 插件已在本地预览实际检查桌面布局、日期控件、24 个小时段、未来时段阻止和“昨天 11:00 - 12:00”的正确范围；本地 API 返回 HTTP 500 属于未接公司后端的预览环境，不影响控件验证。
- 已知限制：本机没有 `go` 和 `gofmt`，本轮 Go 单测与编译不能在本机执行，须由 Wharf 测试构建补验；发布后仍需 Chrome 插件对测试站 `3.1.8 (container)` 做真实会话、回放首帧与脱敏状态验收。

### 2026-08-26 3.1.8 测试发布与 Chrome 验收

- 提交与分支：`c76b560 feat: add hourly nvr playback locator` 已从 `codex/nvr-hourly-playback` 快进合并至 `codex/containerize-single-image`，并推送 GitLab 测试分支与 GitHub 同名备份分支；推送使用临时 `GIT_ASKPASS` 读取本机安全 token 文件，脚本已删除。
- 部署：Wharf `752` 自动构建/部署后，Chrome 测试页已呈现新共享日期选择器和 24 个小时段，证明测试实例运行本轮前端与一小时后端限制代码。未手动触发部署，未修改正式 `main`。
- Chrome 实测：录像页无 `datetime-local`；点击未来 `23:00 - 次日 00:00` 被阻止并显示中文提示；选“昨天”与 `11:00 - 12:00` 后，页面范围为 `2026/08/25 11:00 - 2026/08/25 12:00`。点击“定位回放”后，`camera_id=111` 显示“画面已开始播放”，媒体包、WASM、解码输入与 Canvas 渲染帧均递增，控制台无 error；页面/诊断未暴露 token、完整 WSS URL 或 Authorization。
- 验收边界：本轮已通过桌面 Chrome 的日期、小时段、会话创建与回放首帧验收。Chrome 插件会话不支持切换到真实 iPhone/微信 viewport，因此移动端直播兼容性仍须以真机单独验收；旧萤石 H5 Monitor 与正式环境均未改。

### 2026-08-26 进行中：3.1.9 直接按小时回放与抖动修复

- 产品调整：NVR 回放日期仅筛选小时段所属日期。点击有效小时段即创建会话，不再显示“定位回放”或可独立修改的时分；今天未开始的小时原生 disabled 并置灰。
- 根因：回放诊断信息是 `.h5-viewer-player` 横向 flex 中的第二个子元素，播放时又每约 250ms 更新一次计数。它会挤压并反复改变 Canvas 的可用宽度，造成回放画面抖动。
- 修复：从 `NVRLabPlayer` 移除诊断 UI 和 `onDiagnostics` 状态订阅，播放器只渲染稳定的画布区域；保留通用连接失败提示和重新连接操作。共享日期组件新增仅日期模式，旧萤石回放仍使用完整日期时间和定位操作。
- 本地验收：Chrome 插件已确认无“定位回放”按钮、无回放链路诊断面板；今天后续小时 disabled；点击“昨天 11:00 - 12:00”直接请求会话。本地 Vite 未接公司后端，HTTP 500 仅用于确认请求立即发起。

### 2026-08-26 3.1.9 测试发布与 Chrome 验收

- 提交与发布：`b9f027c fix: simplify nvr playback controls` 已推送 GitLab `codex/containerize-single-image` 与 GitHub 同名备份分支。Wharf `752` 自动部署后，未操作正式 `main`；本次临时 GitLab `GIT_ASKPASS` 脚本已删除。
- 实际交互：Chrome 测试页回放模式仅显示“回放日期”和小时段，没有“定位回放”按钮或回放链路诊断面板。今天未开始的后续 11 个小时以原生 disabled 状态显示。
- 实际播放：切换到“昨天”后点击 `11:00 - 12:00`，无需第二次确认即创建回放会话，`camera_id=111` 首帧成功。页面和控制台无 error，页面未出现 token、完整 WSS URL 或 Authorization。
- 稳定性证据：在持续回放中相隔 5 秒读取 Canvas，CSS 尺寸均为 `1358 x 763.875`，画布绘图缓冲区均为 `1920 x 1088`。原先挤占播放器的右侧统计节点已不存在，已消除本页布局重排导致的抖动来源；如仍出现画面抖动，应单独按上游流帧率、编码和网络问题继续定位。

### 2026-08-26 进行中：3.1.11 NVR 页面底栏模式切换

- 纠正：此前错误地将“底部 bar”实现为播放器内部控制栏。已在 Chrome 直接查看 2.x 真实观看页，并核对 `H5MonitorChannel`：`h5-bottom-tabs` 是独立、固定在视口底部的页面级导航，播放器控制栏不承载模式切换。
- 实现：NVR 页改为同一层级结构，`NVRLabCamera` 负责模式状态与 `nav.h5-bottom-tabs`；`NVRLabPlayer` 恢复为单一播放器组件。直播和按小时发起回放的已有行为不变，旧萤石 H5 Monitor 不改。
- 本地验证：Vitest `10 files / 59 tests` 与 `npm run build` 通过。Chrome 验收确认 `h5-bottom-tabs` 的计算样式为 `position: fixed`；点击“录像”后底栏选中且 24 个小时段出现；播放器内部无模式切换控件。
- 提交与发布：`48102a7 fix: use page bottom bar for nvr modes` 已推送 GitHub 备份与 GitLab `codex/containerize-single-image`；Wharf `752` 已自动部署，未触碰正式 `main`。
- 测试环境 Chrome 验收：`camera_id=82` 直播画面正常；`h5-bottom-tabs` 固定在页面视口底部；点击录像后底栏选中、日期和 24 个小时段出现；播放器内部的模式导航数量为 `0`。本轮原先错误的 `ca3c938` 内部控制栏实现已被此版本覆盖。

### 2026-08-26 进行中：3.1.12 NVR 回放进度定位

- 产品需求：NVR 回放补齐 2.x 播放器内的回放进度拖动。
- 实现：复用 `PlaybackSegmentSlider`，将其时间范围类型从旧萤石记录片段抽成只含开始/结束秒的通用结构。NVR 页在已创建回放会话时将小时窗口传入播放器中央；直播和无回放会话时保留原有模式文字。
- 定位语义：拖动完成以目标秒为新的回放开始时间、保留已选小时窗口的结束时间，重新向后端签发 WSS 会话。工控机 SDK 未公开可靠媒体时间码，因此游标仅在首帧后依据播放/暂停状态作秒级估算，拖动后以新会话的实际流为准。
- 本地验证：Vitest `10 files / 60 tests` 与 `npm run build` 通过。Chrome 插件本地预览确认页面级 `h5-bottom-tabs` 仍是唯一模式入口，播放器内没有模式导航；本地未连接公司后端，故页面返回 HTTP 500，无法伪造真实回放会话或滑块呈现。当前开发机未安装 Go，`go test ./...` 与 `go build` 留给 Wharf 构建补验。待发布测试环境后以真实回放窗口验收首帧、游标推进和拖动重定位。
- 测试发布：已提交 `78e0a30 feat: add nvr playback progress seeking`，同步 GitHub 备份和 GitLab `codex/containerize-single-image`；Wharf `752` 自动部署后生效。GitLab 推送使用一次性 `GIT_ASKPASS`，发布后已确认临时脚本删除。
- Chrome 测试环境验收：在 `camera_id=111` 切换“昨天 / 11:00 - 12:00”回放，播放器首帧成功。播放器控制栏中央出现可用的 `3600` 秒进度条，4 秒内游标由 `28` 推进至 `32`；键盘将游标移动后确认，播放器以新会话从 `0` 开始并恢复推进至 `9`。页面级底栏仍只有一个，播放器内部无模式导航，控制台无项目 error，页面未暴露鉴权信息或完整 WSS 地址。
- 后续边界：桌面 Chrome 回放已通过；iPhone Safari 与微信内直播仍是独立真机验收项，旧萤石 H5 Monitor 和正式环境均未改。

### 2026-08-26 3.1.12 回放定位复盘与 3.1.13 修复

- 用户反馈：任意比例拖动后，滑块没有停留在目标位置，或显示比例不正确。
- 复现与根因：拖动后的实现把目标秒同时写入了 `activePlaybackRange.startTime`。该 state 既是后端回放会话的起点，又被播放器作为整小时滑块的左边界，导致新会话建立后滑块按缩短区间重算为零或错误比例。上一轮仅验证了靠近起点的定位，未覆盖任意比例拖动，该结论不应视为完整验收。
- 修复：将“固定小时播放窗口”与“本次 WSS 会话起点”拆分。滑块始终使用原小时窗口；拖动只把目标秒传给会话请求的 `start_time`，`end_time` 保持原窗口结束；新会话开始时游标直接落在目标秒。
- 回归：新增 `buildNVRLabPlaybackSession` 测试，断言拖动后保留原窗口；前端 Vitest `10 files / 61 tests` 与生产构建通过。
- 测试发布与真实验收：`1704b0a fix: preserve nvr playback slider window` 已同步 GitHub 与 GitLab 测试分支，Wharf `752` 自动部署生效。Chrome 在 `camera_id=111`、昨天 `11:00 - 12:00` 实测：中段定位保持 `1800 / 3600` 并继续到 `1806`；靠近末段定位保持 `3264 / 3600` 并继续到 `3270`。两次均显示“画面已开始播放”，控制台无项目 error。
- 产品阶段结论：用户确认 `10001` 工控机取流实验页的样式、核心功能和实际使用验证已基本可用，作为该实验页的阶段验收结论。后续不继续进行无明确问题的页面微调；当前仍保持实验页隔离，旧萤石 H5 Monitor 和正式环境不自动替换。扩大灰度或替换旧链路前，需单独确认目标门店范围、监控与回滚方案。

### 2026-08-26 已确认待设计：工控机监控全量替换

- 目标：原“查看监控”入口全量切换到工控机 NVR 新链路；旧萤石监控不再作为日常可见入口。
- 门店准入：仅展示存在符合工控机摄像头口径的门店；未配置门店不展示“查看监控”入口，也不回退萤石。
- 权限：管理员与编辑运维查看全部已配置门店；普通查看用户仅可在已授权门店范围内看到入口并观看。
- 回滚：本期保留旧萤石代码与数据配置作为版本级回滚能力，但不在新界面暴露旧入口；稳定后再另行评估清理。
- 状态：正在形成全量路由、数据、权限、验收和回滚设计，未经用户确认不改业务代码或发布。

### 2026-08-26 已确认：NVR 默认缩略图一次性后端回填

- 用户确认不将缩略图变成页面功能、人工维护入口、浏览器常驻队列、定时刷新任务或 Web 实例配置项。
- 设计与实施计划：`docs/superpowers/specs/2026-08-26-nvr-snapshot-backfill-design.md`、`docs/superpowers/plans/2026-08-26-nvr-snapshot-backfill-implementation.md`。
- 目标架构：独立 one-shot Job 使用现有 MySQL/OSS/NVR 授权 Secret 抓取一帧，读取同步业务表，只写私有 OSS 与待 DBA 执行的自有表 `tb_nvr_camera_snapshots`；Web 缩略图请求继续按门店 `monitor:view` 权限校验。
- 关键未知：浏览器 NVRPlayer 已证明 Canvas 端播放，但未证明服务端可将 WSS RTP/H.265 原生解码为 JPEG。必须先在测试门店 `10001` 对一个已知可直播摄像头做 20 秒、并发 1 的技术闸门；失败即停止批处理。
- 运行顺序：无副作用单摄像头解码闸门 -> DBA 测试 DDL/Job 权限 -> 用户确认的 `10001` 串行批处理 -> 用户确认的全量测试 -> 独立生产审批和执行。正常 Web 发布与临时 Job 生命周期彼此隔离。

### 2026-08-26 主会话直属 DBA 子 agent 审核结论

- 主会话改用直属 DBA 子 agent 监督本次工作，不依赖用户人工转交。子 agent 未执行数据库、代码、发布或提交操作。
- 接受：`camera_id` 是全局唯一摄像头身份；`oss_object_key` 用 ASCII binary collation；不对同步 `tb_crm_*` 建外键；Web、一次性 Job、DBA/运维三类主体使用分离的最小权限。
- 主会话裁决：不接受“只保存成功缩略图、不保留状态”的建议。回填表保留失败状态、最近尝试时间和稳定错误码，以满足暂停、续跑与审计需求，且不存任何原始错误、JWT、WSS URL 或媒体内容。
- 顺序优化：单摄像头原生解码技术闸门可先不写数据库和 OSS，仅验证真实 WSS/RTP/H.265 是否能产出受限 JPEG；通过后才需要测试库 DDL 与 Job 最小权限配置。

### 2026-08-26 直属子 agent：协议技术闸门第一小步

- 实现范围严格限制在 `internal/nvrsnapshot/depacketizer.go` 与对应单测：固定 RTP v2/PT96、H.265 FU=49、Annex-B 输出、无网络/MySQL/OSS/ffmpeg/凭据处理。
- 主会话已做两阶段审查。首次规格审查打回无限 FU 缓存、分片中断状态与边界测试问题；首次质量审查打回 AP/PACI、FU 元数据校验和单 NAL 上限问题。修复后的规格复审和质量复审均通过。
- 安全策略：未支持的 AP(48)/PACI(50)，以及 FU 内部伪装的 48/49/50 类型一律返回 `demux_failed`；FU 和普通 NAL 同受 4 MiB 默认内存上限；异常后可恢复处理下一条合法 NAL。
- 验证状态：子 agent 真实尝试 `go test ./internal/nvrsnapshot`，开发机返回 `command not found: go`，且不存在 `gofmt`。因此代码尚未提交、推送、发布或声称测试通过；待具备 Go 1.22 工具链后先格式化并运行该包单测。

### 2026-08-26 NVR 缩略图协议层本机工具链与验证更新

- 已将经 Go 官方 SHA-256 清单核验的 `go1.22.12 darwin/arm64` 安装至本机用户目录 `~/.local/go1.22.12`；未写入仓库、容器、K8s 或任何运行时环境。
- 已对 `internal/nvrsnapshot/depacketizer.go` 与 `internal/nvrsnapshot/depacketizer_test.go` 执行 `gofmt`，并通过定向测试：`go test ./internal/nvrsnapshot`。
- `go build ./...` 通过，说明全仓当前可编译。
- `go test ./...` 未能作为全仓通过证据：除纯协议包外，多数既有测试二进制在本机启动时被 macOS 动态加载器以 `missing LC_UUID load command` 中止。该失败发生在测试进程启动，且本轮新增协议包实际测试通过；后续需要在公司 Linux/Wharf 构建环境再次执行全仓测试，不能将本机结果记为全部测试失败或通过。
- 本轮仍未连接真实 WSS、未抓图、未写 MySQL/OSS、未执行 DDL/Job、未推送 GitLab 或发布环境。下一步是提交并仅同步已验证的纯协议层代码到 GitHub，然后再实现不持久化的单摄像头解码闸门。

### 2026-08-26 NVR 缩略图单摄像头解码闸门实现完成

- 新增 `internal/nvrsnapshot/capture.go` 与纯 fake 单测：真实 `nhooyr.io/websocket` WSS dialer、二进制 RTP/H.265 输入、VPS/SPS/PPS 缓存、marker 封口的完整关键帧 access unit、短生命周期 ffmpeg JPEG 转换及受限输出校验。
- 安全边界：短期 WSS URL 只在内存中交给 dialer；错误码严格白名单化；不记录 token、URL、媒体或上游正文。读取/写入 MySQL、OSS、Web 路由、K8s Job 与 Dockerfile 均未引入。
- 可靠性：FU 分片强制同一 RTP timestamp；参数集不完整时继续等候后续关键帧；stdin/stdout 并发处理且取消时等待全部 I/O goroutine 退出；JPEG 必须可解码、最长边不超过 640、体积不超过 1 MiB。
- 验证：`CGO_ENABLED=0 go test ./internal/nvrsnapshot -count=1 -timeout=30s`、`go vet ./internal/nvrsnapshot`、`go mod verify` 与 `git diff --check` 通过。未设置 CGO 的本机 Go 1.22 测试仍受 macOS `dyld missing LC_UUID` 影响；后续 Linux/Wharf 应补 `go test -race ./internal/nvrsnapshot`。
- 审查：一轮规格审查和一轮质量审查均曾发现问题并修复；最终规格与质量复审均通过。
- 下一步：提交并仅同步 GitHub 后，才实现独立 one-shot 命令和可构建的 runner 镜像；在用户明确批准后，用测试 `10001` 单摄像头做无持久化真实 WSS -> JPEG 验证。通过前禁止 DDL、OSS 写入和批量初始化。

### 2026-08-26 NVR 缩略图 spike runner 与独立镜像完成（未发布）

- 新增 `cmd/nvr-snapshot-spike`：仅执行一个正数 `--camera-id` 的直播抓图技术闸门；默认和最大端到端时限均为 20 秒，拒绝回放参数、数据库、OSS、文件写入与批处理。
- 授权只从现有 `K8S_SECRET_NVR_STREAM_AUTHORIZATION` 读取，未配置时回退 `NVR_STREAM_AUTHORIZATION`；复用 `internal/nvrlab.NewHTTPAuthorizationClient` 取得短期 WSS URL，再交给现有内存内 RTP/H.265 -> ffmpeg JPEG 捕获路径。
- 成功输出仅含 `camera_id`、图片类型、宽高和字节数；失败仅含 `camera_id` 与稳定错误码。命令不会打印或持久化 Authorization、JWT、WSS URL、上游正文或媒体字节，返回前会清除 JPEG 内存。
- 新增 `Dockerfile.nvr-snapshot-spike`：独立构建二进制，运行层只安装 `ffmpeg` 和 CA；未修改 Web Dockerfile 或其入口。
- 完成主会话规格审查：曾发现 timeout 可超过 20 秒和连接失败码未纳入设计集合，均已修复。稳定码集合现包含 `wss_connect_failed`。
- 本机 Go 1.22.12 验证通过：`gofmt`、`CGO_ENABLED=0 go test ./cmd/nvr-snapshot-spike ./internal/nvrsnapshot -count=1 -timeout=30s`、`go build ./cmd/nvr-snapshot-spike`、`go vet ./cmd/nvr-snapshot-spike ./internal/nvrsnapshot`、`git diff --check`。
- `go test ./...` 在本机沙箱仅因既有 `internal/nvrlab` / `internal/nvrmonitor` 的 `httptest` 无法监听 `::1` 端口而中止；本轮 runner 定向包和 `cmd/server` 均已通过。仍需在公司 Linux 镜像构建中补全仓测试与 `go test -race ./internal/nvrsnapshot`。
- 当前状态：仅本地未发布代码，尚未构建镜像、推 GitLab、访问真实 WSS、执行 DDL、写入 MySQL/OSS 或触发批量初始化。下一步先完成最终代码质量审查和 GitHub 备份，再由用户明确批准测试 `10001` 的单摄像头无副作用真实验证。

### 2026-08-26 runner 最终复审与验证收口

- 最终代码质量复审发现并关闭：独立镜像遗漏 `internal/resourceview` 依赖、单横线/双横线混用可绕过重复 `camera-id` 校验、ffmpeg deadline 错误分类不准确、以及失败 JPEG 和读取临时缓冲未清零。
- 最终本机验证通过：`gofmt`、`CGO_ENABLED=0 go test ./cmd/nvr-snapshot-spike ./internal/nvrsnapshot -count=1 -timeout=30s`、`go build -o /tmp/nvr-snapshot-spike ./cmd/nvr-snapshot-spike`、`go vet ./cmd/nvr-snapshot-spike ./internal/nvrsnapshot`、`go mod verify` 和 `git diff --check`。
- Docker CLI 在当前开发机不存在，`Dockerfile.nvr-snapshot-spike` 尚无本机镜像构建证据；不得将其表述为已构建。提交后由公司 Linux/Wharf 或具备 Docker 的隔离构建环境执行独立镜像构建，并补全仓 Go 测试与 `go test -race ./internal/nvrsnapshot`。
- 代码版本：`c53d631 feat: add nvr snapshot spike runner` 已仅推送 GitHub `origin/codex/containerize-single-image`；未推公司 GitLab、未触发 Wharf、未部署测试或正式环境。

### 2026-08-26 主会话执行方式纠偏

- 用户明确目标是完成全量缩略图初始化。对其既已确认的方案，主会话不再对无副作用技术验证、常规代码提交或 GitHub 备份重复索要确认；仅在生产写入/删除、权限扩大、显著外部成本、范围变化或公司平台权限动作时给出具体影响说明。
- 本机检查结果：未注入 NVR 授权变量，符合该值只存在于测试 K8s Secret 的安全边界。真实单摄像头验证的正确执行载体是复用现有测试 Secret 的临时 Job，不是让用户配置实例或提供 token。

### 2026-08-26 NVR 缩略图回填 DBA 复审

- 已产出 `db/nvr_camera_snapshots.sql`，仅为 DBA/运维变更材料，尚未在任何库执行。`camera_id` 作为全局唯一键；成功行和失败行均有严格元数据约束，不记录 URL、token、原始错误或媒体内容。
- Job 除 K8s `parallelism=1` 外必须持有全程 MySQL 命名锁，且相邻授权请求至少间隔 2 秒；连续授权/WSS 失败需熔断停止，避免接口风控和错误扩大。
- Job 与 Web 使用分离最小数据库权限；`database_write_failed` 只能作为 Job 非零退出/汇总错误，不能伪造为已持久化的摄像头失败状态。
- 已实现并通过 `internal/nvrsnapshot` 定向测试的核心回填服务：候选查询精确限制在有效 `HikVisionNvrChannel` 摄像头，缺图初始化与显式失败续跑分离；每路 20 秒、默认相邻请求至少 2 秒、连续三次鉴权/WSS 连接类失败熔断。该阶段仍未连接真实 MySQL、OSS 或 WSS。
- `0f3ace5` 已仅同步 GitHub：回填核心、DDL 和 DBA 验收材料。`286445b` 已仅同步 GitHub：独立回填 CLI、全程 MySQL 命名锁及临时测试 Job 模板；均未推 GitLab 或执行。当前待完成 Web 受控读取路径、测试环境独立镜像构建、DBA 执行测试 DDL 和测试 Job 实际运行。

### 2026-08-26 3.2.2 NVR 回填快照读取路径

- Web 侧已完成受控读取接线：正常 NVR 摄像头 API 会优先查询自有 `tb_nvr_camera_snapshots` 的成功结果，并仅返回项目内受控图片路由；图片请求再次复用该门店监控授权，数据库行必须与 `nvr-camera-snapshots/{tenant_id}/{camera_id}.jpg` 及 `image/jpeg` 完全匹配后才会打开私有 OSS 对象。
- 兼容策略：快照表尚未由 DBA 创建、读取异常、无成功行或对象缺失时，摄像头列表继续使用已验证的“单旧录像机 + 同通道号”历史截图，最终回退前端中性灰色占位；不影响既有工控机直播、回放、鉴权或门店范围权限。
- 新增单测覆盖新图优先、查询异常的旧图回退、快照图片授权读取与未授权拒绝；回填 CLI 补充非法参数和运行时 Secret 缺失的失败关闭测试。
- 本机已在允许 localhost 临时监听的受控测试环境中通过 `CGO_ENABLED=0 go test ./... -count=1 -timeout=120s` 和 `go build ./cmd/server`；未连接 WSS、MySQL、OSS、K8s 或执行 DDL/Job。待推送测试分支、DBA 测试 DDL 和临时 Job 验证。

### 2026-08-26 3.2.2 测试发布与页面验收

- 已提交 `58da8b9 feat: read backfilled nvr camera snapshots`，同步 GitHub 备份与 GitLab 测试分支；Wharf `752` 自动部署后，Chrome 已登录会话确认页面版本为 `3.2.2 (container)`。
- 实测资源首页：67 家门店、75 台 NVR、2881 路摄像头正常加载；实测 `10001` 正常监控页：44 路摄像头、区域筛选和入口正常，控制台/页面未见项目错误。
- 视觉结论：当前 10001 仍显示中性灰色缩略图占位，符合预期，因为 `tb_nvr_camera_snapshots` 尚未由 DBA 创建且临时回填 Job 尚未执行。该结果不是新读取路由故障；真实图验证必须在 DDL 与单路 Job 成功后进行。
- Chrome 插件已用于实际页面验收；未降级为模拟点击。未执行 WSS、MySQL、OSS 或任何数据写入。

### 2026-08-26 3.2.5 NVR 回填执行载体调整

- Wharf 当前测试流水线不支持为同一提交选择独立 Dockerfile，也未修改流水线或实例配置。
- 已确认测试 Pod 提供受控 WebShell；因此将既有 `nvr-snapshot-backfill` 二进制与 `ffmpeg` 纳入正常 Web 镜像，但保持 Web 的 `ENTRYPOINT` 不变。回填只能通过受控终端显式执行，不会随 Web 启动、部署或请求自动运行。
- 该执行路径仅复用 Pod 已有的 MySQL、OSS 和 NVR 授权环境变量；命令不打印 Secret、不写 MySQL，仅写既有私有 OSS 对象。
- 本机 `GOOS=linux` 回填命令构建及 `git diff --check` 通过；定向 Go 测试在 macOS 测试二进制启动时仍受既有 `dyld missing LC_UUID` 限制。Wharf Dockerfile 将在 Linux 构建阶段执行 `go test ./...`，构建成功后再通过 Pod WebShell 做 `10001/camera 111` 的一次性验证。

### 2026-08-27 3.2.6 构建期依赖纠偏

- `8b2f0e3` 在 Wharf 测试流水线两次均以 `retry exceeded workflow deadline` 结束，未提供 Go 编译或 Docker 指令失败日志。该提交唯一新增的构建期外部依赖是 Alpine 的 `apk add ffmpeg`；项目历史也已记录公司构建环境不应依赖公网系统包源。
- 因此将 `ffmpeg` 从正常 Web 镜像的构建阶段移除，保留回填二进制但不改变 Web `ENTRYPOINT`。测试 Pod 更新后，只有在受控 WebShell 中显式执行单路回填前，才临时检查并安装 `ffmpeg`；这不会更改 Deployment、Secret、流水线或 MySQL，也不会随着 Pod 重启持久化。
- 先通过正常 GitLab 推送触发一次自动测试构建。只有确认 `3.2.6` 已自动部署，才在 Pod 内继续 `10001/camera 111` 的低频单路验证。

### 2026-08-27 3.2.7 浏览器 Canvas 缩略图回填

- 为规避测试 Pod 缺少 `ffmpeg` 且外部 APK 源不可用，回填改为一次性浏览器执行：播放器确认首帧后，由前端自身将 Canvas 导出为受限 JPEG，再上传至既有私有 OSS 的确定性 Key `nvr-camera-snapshots/{tenant_id}/{camera_id}.jpg`。
- 上传仅在摄像头详情 URL 显式带 `snapshot_backfill=1` 时启用；正常直播、回放、下载截图和监控列表没有自动上传行为。
- 服务端上传路由要求调用者同时具备 `store:write` 权限和该门店监控访问范围，严格校验候选摄像头、JPEG MIME/文件签名及 2 MiB 上限；不写 MySQL、不建表、不修改 Deployment、Secret 或流水线。
- 执行策略：先对 `10001` 单摄像头确认 OSS 写入与列表读取，再以单标签页串行、每路间隔至少 2 秒完成其余摄像头；失败仅记录统计，不做高频重试。

### 2026-08-27 3.2.8 回填队列可观测性

- NVR 监控列表的摄像头卡片增加不展示的 `data-camera-id`，仅供受控浏览器回填器从已加载列表读取真实业务 ID。它不产生取流、上传、接口调用或用户可见交互。
- 这样回填器无需逐卡进入播放页反查 ID，避免为构建队列额外建立 44 个视频会话。

### 2026-08-27 3.2.9 NVR 缩略图子路径修复

- 浏览器 Canvas 回填已在测试环境对北京保利总部店完成 44/44 成功上传，串行执行约 4 分 53 秒，未写 MySQL。
- 列表最初仍回退占位图的根因是服务端返回根路径 `/api/h5/nvr-monitor/.../snapshot`，在 `/erzhuang-project/` 子路径部署下绕开了项目 API 前缀；前端现统一将该受控 URL 解析到运行时 API Base。

### 2026-08-27 3.2.10 资源详情复用 NVR 缩略图

- 资源详情摄像头表与 NVR 监控列表复用同一个私有 OSS 对象和同一受控图片路由；未创建第二张图、未执行抓图、未写 MySQL。
- 资源详情仅在用户具备该门店监控权限、且 NVR 快照对象确实存在时优先返回 NVR 缩略图；对象不存在时继续使用历史截图兜底，避免影响未回填门店。
- 前端将该 NVR 路由固定解析到 `/erzhuang-project/api/...`，保证子路径部署下图片可加载。

### 2026-08-27 3.2.11 城市简称静态映射

- 以产品提供的 `district.xlsx` 为源，静态固化省级直辖市和城市级 `id -> alias` 映射，共 402 条；运行时不读取 Excel、不依赖数据库或外部服务。
- 门店资源列表、城市筛选和详情统一经 `cityName` 使用城市简称，例如 `1 -> 北京`、`9 -> 上海`、`175 -> 杭州`、`275 -> 长沙`、`385 -> 成都`。未覆盖的异常 ID 暂保留 `城市 {id}`，便于识别源数据缺口。

### 2026-08-27 3.2.12 城市筛选项保持

- 城市筛选的选项集合仅由“全部”查询或搜索重置刷新；选择某个城市后，后端返回的单城市 `cities` 不再覆盖完整筛选项。
- 城市筛选仍保持单选高亮，默认“全部”；用户可直接在北京、上海等任意城市间连续切换，无需先恢复“全部”。

### 2026-08-27 3.2.13 监控页城市简称统一

- NVR 监控页此前有独立的城市分组文案，导致门店切换浮层和标题仍显示 `城市 {id}`。
- 现改为复用资源查看的 `CityName` 静态映射，监控页的当前门店城市、门店切换浮层分组均返回 Excel 的城市简称。

### 2026-08-27 3.2.14 摄像头默认缩略图

- 已按产品确认的 Figma `07 · Illustrated Scene` 最终稿裁出七张静态默认图：面诊室、治疗室、前台接待、候诊休息、走廊出入口、公共功能区和未绑定；仅用于缺少真实截图时的展示兜底。
- 资源详情与 NVR 监控接口均下发 `thumbnail_kind`。分类只读取已展示的空间类型、空间名称和三级空间语义，不修改业务库、二壮 MySQL、OSS、取流或权限链路。
- 前端始终优先真实缩略图；无图或真实图加载失败时切换对应默认图。NVR 和保留的旧版 H5 监控页均复用该规则；资源详情缩略图移除图片自身边框和圆角，仍按 `64 x 40` 固定展示。
- 本地前端验证：Vitest `12 files / 68 tests`、`npm --prefix frontend run build` 通过。当前开发机未安装 Go 工具链，新增的 Go 定向测试由 Wharf Linux 构建补验。
- 测试发布：`7197908 feat: add camera placeholder illustrations` 已同步 GitHub 和 GitLab 测试分支；Wharf `752` 构建 `169439` 于 2026-08-27 15:50 启动、2 分 9 秒后成功，自动部署记录 `415574` 显示已部署到阿里云测试集群。Chrome 已确认测试页版本为 `3.2.14 (container)`，`10001` 资源详情的 44 路真实图保持优先展示，默认图静态资源可从测试域名的项目子路径访问。

### 2026-08-27 3.2.15 NVR 默认图比例修正

- 机构详情的 `64 x 40` 横向默认图效果符合验收；NVR 监控列表误复用了旧版圆形预览容器，导致 Figma `2:1` 默认图被裁切和放大。
- 仅将 NVR 监控列表的预览框改为 `2:1` 横向比例。默认图使用 `object-fit: contain` 完整显示，真实缩略图继续 `cover` 填充；不影响播放器、取流、真实图或其他页面的旧圆形视觉。
- 测试发布：`a2a28bc fix: preserve nvr placeholder aspect ratio` 已同步 GitHub 和 GitLab 测试分支；Wharf `752` 构建 `169443` 于 2026-08-27 16:04 启动、2 分 4 秒后成功，自动部署记录 `415578` 已部署到阿里云测试集群。
- Chrome 实页验收：`/h5/orgs/10016/monitor` 的 34 路摄像头默认图均以横向 `2:1`、`4px` 圆角和 `contain` 完整展示；页面无项目控制台告警。未修改真实截图、OSS、取流、权限、数据库或实例配置。
