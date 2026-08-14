# MySQL 迁移前验收用例

最后更新：2026-07-01

本文是主会话负责人验收口径，用于约束 MySQL 测试库在导入 Supabase/PostgreSQL 历史数据之前必须完成的验收。它补充 `docs/mysql-dba-plan.md` 和 `docs/mysql-testdb-schema-review.md`，重点回答“迁移前要先证明什么，才能允许把历史数据搬进 MySQL 测试库”。

## 1. 验收目标

MySQL 测试库未来不是一次性沙箱。它会先用于 schema 和代码适配预演，随后承接 Supabase/PostgreSQL 中已经积累的历史数据，并成为公司测试环境的数据基座；正式环境后续会基于测试库确认后的结构和数据初始化。

因此历史数据导入前必须先证明：

- MySQL schema 能覆盖现有核心业务流程。
- Go 接口路径、前端后台、H5 Monitor、图片代理迁移后仍兼容。
- 权限和敏感数据访问边界可被后端执行，而不是只靠前端隐藏。
- 数据导入、校验、回滚、正式库初始化都有可重复流程。
- 金丝雀门店和假数据用例全部通过后，才允许历史数据导入。

## 1.1 待确认事项处理规则

迁移验收过程中如发现需要主会话、产品负责人、DBA、运维、安全/SSO 或研发共同确认的事项，不直接改 schema、不清表、不重建、不导入历史数据。先整理待确认清单，必须包含：

- 背景。
- 可选方案。
- 推荐方案。
- 影响范围。
- 风险。
- 需要谁确认。
- 确认后下一步。

清单交主会话复核，由主会话与用户确认后再继续。特别是测试库进入历史数据导入后，会成为公司测试环境数据基座；正式库也会基于确认后的测试库结构和数据初始化，因此所有破坏性或方向性动作都必须先确认。

## 2. 阶段门禁

### 2.1 阶段 A：schema 折腾期

允许：

- 修改、重建 MySQL 测试库表结构。
- 插入假门店、假录像机、假权限用户。
- 反复跑迁移脚本和回滚脚本。

要求：

- 每次 DDL 调整必须记录在 Git 文档或 migration 草案中。
- 不把阶段 A 的脏数据当成历史数据来源。
- 阶段 A 结束前，必须清理或隔离假数据，并冻结 schema 版本。

### 2.2 阶段 B：历史数据导入后

允许：

- 用 migration 脚本演进 schema。
- 用可审计脚本修复数据。
- 继续执行页面/API/数据对账验收。

禁止：

- 随意清表、重建表、手工改历史数据。
- 在没有备份和回滚方案时跑破坏性脚本。
- 把测试库当成可丢弃沙箱。

## 3. 验收数据集

### 3.1 真实金丝雀门店

优先使用：

- 门店：北京保利实验室门店
- 机构 ID：`10030`
- 录像机：`GN0941203`

该门店用于验证 H5 Monitor、录像机、通道、实时视频/回放入口、截图路径等真实链路。

### 3.2 补充假数据

至少准备这些假数据，覆盖真实门店不一定具备的边界：

- `canary_empty_design`：有门店、无设计图、无录像机。
- `canary_design_only`：有设计图、标注区域、无录像机。
- `canary_channels_no_snapshot`：有录像机和通道，但无截图记录。
- `canary_business_bed`：有治疗室/VIP 治疗室/美容室通道，包含 `bed_label`。
- `canary_non_business`：有前台、候诊、通道、弱电室等非业务区域。
- `canary_json_result`：通道和设计图包含合法 `recognition_result` JSON。
- `canary_missing_asset`：数据库有截图路径，但底层文件不存在，用于验证缺图降级和诊断。

### 3.3 权限假用户

第一版身份使用企业邮箱作为唯一标识，预留飞书 user_id。

- `admin@example.com`：admin，全量机构。
- `viewer.single@example.com`：viewer，只能访问机构 `10030`。
- `viewer.multi@example.com`：viewer，可访问 `10030` 和另一家样本门店。
- `viewer.none@example.com`：viewer，无机构范围。
- `operator.store@example.com`：operator，可维护 `10030`。
- `disabled@example.com`：已禁用用户。
- `unknown@example.com`：SSO 登录成功但用户表不存在。

### 3.4 迁移压力和异常数据

除金丝雀门店外，阶段 A 至少准备一组可删除的压力/异常数据，避免只用一两条样本把问题藏起来：

- `canary_many_channels`：单门店 2 台录像机、60+ 通道，模拟真实大门店。
- `canary_many_stores`：至少 3 个城市、每城多页门店，用于验证城市筛选、分页和统计汇总。
- `canary_ai_failure`：截图存在但 AI 返回非标准 JSON、带 `<think>` 前缀、空结果、超时和 400 错误。
- `canary_asset_forbidden`：资产存在但当前用户无权限访问。
- `canary_old_path`：历史数据中保存旧 API 路径，验证迁移归一化。
- `canary_duplicate_external_org`：重复或空 `external_org_id`，用于验证导入阻断。

## 4. Given/When/Then 验收用例

### 4.1 健康检查和路径兼容

**AC-001 健康检查识别 MySQL**

Given 应用连接 MySQL 测试库且启动成功  
When 访问 `/erzhuang-project/health`  
Then 返回 HTTP 200  
And `status` 为 `ok`  
And `database` 为 `mysql`  
And `asset_store` 返回当前资产模式

**AC-002 公司路径前缀兼容**

Given 应用部署在 `/erzhuang-project/` 前缀下  
When 访问 `/erzhuang-project/`  
Then 返回前端首页  
And 静态资源路径可加载  
And 浏览器控制台无明显运行时报错

**AC-003 旧兼容路径不被破坏**

Given 代码仍兼容 `/erzhuang/` 旧路径  
When 访问 `/erzhuang/health` 或 `/erzhuang/api/tasks`  
Then 返回兼容响应  
And 不影响 `/erzhuang-project/` 主路径

### 4.2 门店列表和详情

**AC-010 门店列表数据一致**

Given MySQL 测试库已导入金丝雀门店  
When admin 访问门店列表第一页  
Then 返回门店列表、分页、统计汇总和城市选项  
And 金丝雀门店可以通过搜索定位  
And 列表统计与数据对账 SQL 一致

**AC-011 城市筛选覆盖全量数据**

Given 多页门店中存在同一城市门店  
When 选择该城市筛选  
Then 返回所有符合城市条件的门店  
And 不是只筛当前页数据

**AC-012 门店详情 tab 独立加载**

Given 金丝雀门店包含设计图和通道数据  
When 打开门店详情  
Then 基础信息正确展示  
When 切换到设计图标注 Tab  
Then 设计图和标注区域正常展示  
When 切换到通道映射 Tab  
Then 录像机、通道、确认状态和截图路径正常展示

**AC-013 机构简称和外部机构 ID 保留**

Given 门店已有 `short_name` 和 `external_org_id`  
When 打开详情和编辑弹窗  
Then 两个字段正确展示  
When 保存后刷新页面  
Then 两个字段仍保留

### 4.3 设计图和标注

**AC-020 设计图资产路径兼容**

Given 门店设计图文件仍存储在 Supabase Storage  
When 打开设计图标注 Tab  
Then 预览图可通过 Go 后端路径展示  
And 前端不直接暴露 Supabase service role key 或内部密钥

**AC-021 标注框读写一致**

Given 门店已有标注区域  
When 修改一个非关键标注字段并保存  
Then 接口返回成功  
And 刷新后标注数量、坐标、区域类型、编号仍一致

**AC-022 缺失设计图可解释**

Given 假门店没有设计图  
When 打开设计图标注 Tab  
Then 页面显示可理解的空态或上传入口  
And 不出现空白页或前端异常

### 4.4 通道映射和截图

**AC-030 通道数据保留**

Given 金丝雀门店存在录像机和通道  
When 打开通道映射 Tab  
Then 录像机编号、通道号、通道状态、业务区域、编号/备注、`bed_label` 正确展示

**AC-031 床位拆分保存**

Given operator 对指定门店有维护权限  
When 将治疗室/VIP 治疗室/美容室通道保存为带 `bed_label` 的确认结果  
Then 接口返回成功  
And 刷新后 `bed_label` 保留  
And `area_type + area_number + bed_label` 展示符合产品规则

**AC-032 非业务区域保存**

Given 通道识别为前台、候诊、通道或弱电室  
When 确认为非业务区域  
Then 状态保存为 `confirmed_non_business`  
And 不要求填写业务区域编号或床位

**AC-033 图片代理路径兼容**

Given 通道截图路径为 `/api/store-space/channel-snapshots/{name}.jpg`  
When 前端请求 `/erzhuang-project/api/store-space/channel-snapshots/{name}.jpg`  
Then 后端通过 AssetStore 读取文件  
And 返回图片 Content-Type  
And 前端不需要知道底层是 Supabase 还是公司文件服务

**AC-034 缺图诊断可用**

Given 数据库存在截图路径但底层文件不存在  
When 请求 `/api/store-space/channel-snapshots/{name}/diagnostics`  
Then 返回 `asset_store`、`snapshot_key`、`exists=false` 和可解释错误  
And 页面不因缺图崩溃

### 4.5 H5 Monitor

**AC-040 H5 Monitor 首页可打开**

Given 金丝雀门店有 `external_org_id=10030` 且存在确认通道  
When 访问 `/erzhuang-project/h5/orgs/10030/monitor`  
Then 页面打开成功  
And 通道列表按该机构数据展示  
And 非授权机构不混入列表

**AC-041 实时视频取流走到可解释状态**

Given 通道可用于萤石取流  
When 点击实时视频  
Then 后端请求播放地址  
And 成功时返回播放地址并可播放  
And 失败时展示萤石错误码或可解释诊断  
And 不泄露 access token、app secret、设备验证码

**AC-042 回放入口和片段查询可用**

Given 通道有本地录像或萤石接口可返回片段  
When 选择日期并查询回放片段  
Then 返回片段列表或明确无片段状态  
When 点击片段播放  
Then 播放器走到可播放或可解释错误状态

**AC-043 播放地址失效流程可用**

Given 实时或回放播放地址已获取  
When 关闭详情、切换 Tab 或超时停止播放  
Then 后端调用失效流程或记录无法失效原因  
And 不长期占用直播/回放资源

### 4.6 SSO 和权限

**AC-050 未启用 SSO 不影响现有流程**

Given `SSO_ENABLED` 未启用  
When 打开后台和 H5 Monitor  
Then 当前测试流程不被登录页阻断  
And 可继续完成门店、设计图、通道、H5 Monitor 验收

**AC-051 admin 全量访问**

Given SSO 启用且 `admin@example.com` 登录成功  
When 访问门店列表、门店详情、H5 Monitor 和用户管理入口  
Then 可看到全部授权功能和全部机构  
And 后端接口直接调用也返回授权数据

**AC-052 viewer 单机构访问**

Given `viewer.single@example.com` 只授权机构 `10030`  
When 访问门店列表  
Then 只返回 `10030` 对应门店  
When 访问其他机构详情或 H5 Monitor API  
Then 返回 403 或无权限错误  
And 不返回其他机构数据

**AC-053 viewer 无机构范围**

Given `viewer.none@example.com` 登录成功但没有任何 scope  
When 访问门店列表  
Then 返回空列表和可理解空态  
When 直接访问任意门店详情或 H5 Monitor API  
Then 返回 403

**AC-054 禁用和未知用户拒绝**

Given SSO 返回 `disabled@example.com` 或 `unknown@example.com`  
When 用户完成 SSO 回调  
Then 系统拒绝进入后台  
And 展示“未授权或账号已停用”  
And 写入登录拒绝审计日志

**AC-055 前端隐藏不等于后端放行**

Given viewer 用户前端看不到编辑按钮  
When 直接调用编辑、删除、识别、刷新截图等接口  
Then 后端返回 403  
And 不产生业务数据变更

### 4.7 操作日志和敏感审计

**AC-060 敏感查看写审计**

Given 登录用户访问实时视频、录像回放或截图代理  
When 请求成功或失败  
Then 写入审计日志  
And 日志包含 user/email、action、store_id 或 external_org_id、channel_id、result、request_id  
And 不记录完整播放 URL、access token、app secret、设备验证码

**AC-061 权限变更写审计**

Given admin 修改用户角色或机构范围  
When 保存成功  
Then 写入审计日志  
And 可追溯操作者、目标用户、变更前后范围

### 4.8 数据导入和一致性

**AC-070 主键和外键保留**

Given 从 Supabase/PostgreSQL 导出样本数据  
When 导入 MySQL 测试库  
Then 核心表原始 `id` 尽量保留  
And 外键无孤儿记录  
And `auto_increment > max(id)`

**AC-071 JSON 合法**

Given 源库存在 `recognition_result`  
When 导入 MySQL  
Then 非空 JSON 均满足 `json_valid`  
And 空字符串被转换为 `NULL`  
And 前端读取后不报错

**AC-072 时间口径一致**

Given 源库使用 `timestamptz`，MySQL 使用 `datetime(3)`  
When 比对迁移前后更新时间、确认时间、日志时间  
Then 差异符合约定时区转换  
And 不出现系统性相差 8 小时的问题

**AC-073 历史数据导入前后金丝雀行为一致**

Given 金丝雀门店在小样本阶段验收通过  
When 全量历史数据导入 MySQL 测试库后  
Then 同一套 API smoke、页面验收和数据对账再次通过  
And 金丝雀门店行为与导入前一致

### 4.9 回滚和运营连续性

**AC-080 导入失败可回滚**

Given 历史数据导入过程中任一关键校验失败  
When 触发回滚流程  
Then MySQL 测试库可恢复到导入前备份点  
And Supabase/PostgreSQL 仍可作为只读或原运行数据源  
And 失败原因被记录

**AC-081 停写窗口明确**

Given 准备执行历史数据全量导入  
When 进入停写窗口  
Then 运营后台写操作被暂停或明确公告  
And 导入完成并校验通过后再恢复写入  
And 停写开始和结束时间被记录

**AC-082 正式库初始化前测试库冻结**

Given MySQL 测试库已经承载历史数据并通过验收  
When 准备交给运维初始化正式库  
Then schema 版本、数据快照时间、迁移脚本版本被记录  
And 在正式库初始化完成前，测试库禁止随意 schema 变更和手工数据修复

### 4.10 数据库配置和密钥边界

**AC-090 MySQL 连接配置来自运行时环境**

Given 应用切换到 MySQL 测试库  
When 检查仓库、前端构建产物、Dockerfile 和文档  
Then 不存在 MySQL 密码、Supabase service role key、萤石 app secret、MiniMax/OpenAI key、SSO JWT/cookie 原文明文  
And 应用只能通过运行时环境变量或 K8s Secret 获取这些配置

**AC-091 严格模式、字符集和时区可观测**

Given 应用连接 MySQL  
When 访问 `/health` 或只读诊断接口  
Then 能看到数据库类型、MySQL 版本、连接时区、字符集、资产存储模式  
And 迁移脚本显式设置或校验 `sql_mode`  
And 如果严格模式、时区或字符集不符合约定，迁移流程阻断

**AC-092 数据库连接失败有可解释降级**

Given MySQL 配置错误或数据库不可达  
When 应用启动或访问 `/health`  
Then 健康检查返回非健康状态或启动失败  
And 日志能定位到数据库连接问题  
And 不进入“页面可打开但数据静默为空”的状态

### 4.11 AI 识别和模型切换链路

**AC-100 AI provider 设置保留**

Given 当前后台支持 OpenAI/MiniMax 识别模型切换  
When 从 PostgreSQL 迁到 MySQL  
Then 默认识别模型配置保留  
And 在机构详情页切换识别模型后刷新页面仍保留  
And 不需要改代码或重发镜像才能切换默认模型

**AC-101 通道识别图片输入格式兼容**

Given 通道截图已经通过后端代理存储为 logical key  
When 调用 OpenAI 或 MiniMax 识别  
Then 发送给模型的图片参数是 `http(s)://` 或 `data:image/...;base64` 合法格式  
And 不把内部 `/api/store-space/channel-snapshots/...` 相对路径直接发给外部模型

**AC-102 AI 非标准返回可恢复**

Given 模型返回带 `<think>` 前缀、Markdown 代码块、空文本或非 JSON 内容  
When 识别流程解析失败  
Then 通道被标记为可解释的识别失败  
And 失败原因、provider、request_id、耗时写入短周期诊断日志  
And 不影响同一录像机剩余通道继续识别

**AC-103 继续上次识别**

Given 一台录像机 60+ 通道中部分通道已经识别成功，部分失败  
When 运营再次点击识别  
Then 默认跳过已确认或已成功识别且未解锁的通道  
And 仅继续处理失败、待识别或已解锁通道  
And 最终结果能区分成功数、失败数和跳过数

### 4.12 资产存储和公司文件服务预留

**AC-110 数据库不存图片二进制**

Given 通道截图、设计图 PDF、预览图和缩略图已经存在  
When 检查 MySQL 数据  
Then 数据库只保存 logical key、文件 ID 或代理路径  
And 不保存图片/PDF 二进制大字段  
And 不保存外部长期可公开访问的签名 URL

**AC-111 旧截图路径归一化**

Given 源库中同时存在 `/api/store-space/channel-snapshots/{name}` 和 `channel-snapshots/{name}` 两种写法  
When 导入 MySQL  
Then 统一归一化为约定 logical key  
And 前端仍可通过 `/api/store-space/channel-snapshots/{name}` 访问  
And Excel 导出、H5 Monitor 缩略图、诊断接口都不需要改前端路径

**AC-112 资产权限由后端代理执行**

Given viewer 只授权机构 `10030`  
When 请求未授权机构的设计图、截图或 PDF 代理路径  
Then 后端返回 403  
And 不返回图片内容  
And 写入资产访问拒绝日志

**AC-113 公司文件服务切换可灰度**

Given 第一阶段资产仍在 Supabase Storage  
When 后续接入公司文件服务  
Then 可以通过配置切换 AssetStore provider  
And 同一代理路径保持不变  
And `tb_asset_objects` 或等价映射能记录旧 key、新 file_id、content_type、size、checksum、迁移状态

### 4.13 性能、并发和资源释放

**AC-120 门店列表性能基线**

Given MySQL 测试库至少包含阶段 A 压力样本和历史导入后的真实数据量  
When 访问门店列表、城市筛选、搜索和详情接口  
Then P95 响应时间满足验收阈值  
And 慢查询能通过索引或查询计划解释  
And 不出现城市筛选只筛当前页的问题

**AC-121 详情 Tab 独立加载性能**

Given 门店包含设计图、标注、2 台录像机和 60+ 通道  
When 首次进入详情页  
Then 首屏不等待所有 Tab 数据全量加载完成  
When 切换到对应 Tab  
Then 只加载该 Tab 必要数据  
And 不重复拉取已经缓存且未变更的数据

**AC-122 H5 Monitor 并发限制可执行**

Given 普通用户最多 15 路、管理员额外最多 5 路的播放资源策略  
When 多用户同时打开实时视频或回放  
Then 后端按用户角色和资源类型执行限制  
And 超限时返回可理解错误  
And 关闭播放、切换 Tab、页面卸载、超时确认未继续观看时释放资源

**AC-123 播放地址清理可追踪**

Given 萤石失效播放地址接口返回成功、失败或超时  
When 播放会话结束  
Then 系统记录失效结果和原因  
And 失败不会阻塞用户返回列表  
And 可通过短周期日志追踪是否存在长期占用风险

### 4.14 导出、报表和人工验收材料

**AC-130 通道映射 Excel 内容完整**

Given 金丝雀门店有业务通道、非业务通道、床位拆分和截图  
When 导出通道映射 Excel  
Then 文件可打开  
And 包含门店、录像机、通道号、区域类型、编号/备注、床位拆分、确认状态、截图或缺图说明  
And 不包含未授权门店数据

**AC-131 迁移前后截图验收材料可对比**

Given PostgreSQL/Supabase 当前环境已经生成金丝雀门店验收截图  
When MySQL 小样本和历史导入后分别验收  
Then 保留同一页面、同一门店、同一通道的对比截图或录屏结论  
And 能证明页面主要信息、图片和 H5 Monitor 入口没有退化

### 4.15 脚本幂等、备份和正式交接

**AC-140 schema 初始化脚本幂等**

Given 空 MySQL 测试库  
When 连续执行 schema 初始化脚本两次  
Then 第二次执行不破坏已有结构和种子数据  
And 不产生重复角色、权限点或默认配置

**AC-141 样本导入脚本可重复演练**

Given 阶段 A 假数据已经导入过一次  
When 清理假数据后再次执行样本导入  
Then 导入结果一致  
And 脚本输出导入行数、跳过行数、失败行数和校验摘要

**AC-142 历史导入前备份可恢复**

Given 准备导入 Supabase/PostgreSQL 历史数据  
When 执行导入前备份  
Then 备份文件、生成时间、schema 版本、代码 commit、执行人和恢复命令被记录  
When 在隔离库或测试库演练恢复  
Then 恢复后的行数和关键校验与备份点一致

**AC-143 运维正式初始化交接包完整**

Given 测试库历史数据验收通过  
When 准备交给运维初始化正式环境  
Then 交接包包含 DDL/migration 版本、数据快照时间、导入步骤、校验 SQL、回滚方案、环境变量清单、密钥注入清单、已知风险和联系人  
And 没有把任何真实密钥写进交接文档

## 5. API Smoke Test 清单

Smoke test 应在三种状态下各跑一遍：

1. PostgreSQL/Supabase 当前环境，作为基线。
2. MySQL 小样本环境，作为适配验证。
3. MySQL 历史数据导入后，作为迁移验证。

执行优先级：

- P0：阻断迁移和上线，必须自动化或半自动化验证。
- P1：阻断历史导入，但可用手工页面验收补足。
- P2：不阻断导入，但必须记录风险和补测计划。

### 5.1 基础路径

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/erzhuang-project/health` | 200，`status=ok`，数据库和资产模式明确 |
| GET | `/erzhuang-project/` | 200，前端首页 |
| GET | `/erzhuang-project/assets/*` | 200，静态资源可加载 |

### 5.2 Store Space

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/api/store-space/stores?page=1&page_size=20` | 200，含 `items/total/summary/cities` |
| GET | `/api/store-space/stores?q=保利` | 200，可定位金丝雀 |
| GET | `/api/store-space/stores?city=北京` | 200，城市筛选生效 |
| POST | `/api/store-space/stores/check-duplicate` | 200，重复判断稳定 |
| GET | `/api/store-space/stores/{storeId}` | 200，基础详情 |
| GET | `/api/store-space/stores/{storeId}/design-plan-data` | 200，设计图数据 |
| GET | `/api/store-space/stores/{storeId}/channel-data` | 200，通道数据 |
| PATCH | `/api/store-space/stores/{storeId}` | 200，非破坏字段保存 |
| PUT | `/api/store-space/stores/{storeId}/design-plan` | 200，标注保存 |
| POST | `/api/store-space/recorders/{recorderId}/scan-channels` | 200 或可解释设备错误 |
| POST | `/api/store-space/recorders/{recorderId}/recognize-channels` | 200，识别结果落库或可解释失败 |
| POST | `/api/store-space/channels/{channelId}/snapshot` | 200，返回截图路径或可解释失败 |
| PUT | `/api/store-space/channels/{channelId}/confirmation` | 200，确认结果保存 |
| GET | `/api/store-space/stores/{storeId}/channel-mappings/export.xlsx` | 200，Excel 可下载 |

破坏性接口只允许在假门店上跑：

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| DELETE | `/api/store-space/channels/{fakeChannelId}` | 204，且审计记录存在 |
| DELETE | `/api/store-space/recorders/{fakeRecorderId}` | 204，且不影响真实门店 |
| DELETE | `/api/store-space/stores/{fakeStoreId}` | 204 或软删除成功 |

### 5.3 图片代理

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/api/store-space/channel-snapshots/{name}` | 200，图片 Content-Type |
| GET | `/api/store-space/channel-snapshots/{name}/diagnostics` | 200，含 asset_store/snapshot_key/exists |
| GET | `/api/store-space/channel-snapshots/{missingName}/diagnostics` | 200，exists=false，可解释 |

### 5.4 H5 Monitor

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/api/h5/orgs/10030/monitor` | 200，返回授权通道列表 |
| POST | `/api/h5/orgs/10030/monitor/channels/{channelId}/live-url` | 200 或可解释萤石错误 |
| GET | `/api/h5/orgs/10030/monitor/channels/{channelId}/record-segments?date=YYYY-MM-DD` | 200，片段数组或无数据 |
| POST | `/api/h5/orgs/10030/monitor/channels/{channelId}/playback-url` | 200 或可解释错误 |
| POST | `/api/h5/orgs/10030/monitor/channels/{channelId}/disable-url` | 200，成功或可解释失败 |

### 5.5 Auth 和权限

当前 SSO 口径采用公司推荐的 APISIX 网关 `security-sso` 插件：业务系统保留 `/_/auth/callback`、`/logout` 和 `/api/auth/me`，并通过 `/erzhuang-project` 和 `/erzhuang` 前缀兼容公司路径。项目不自建 OAuth2 登录页作为主流程。

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| GET | `/_/auth/callback` | 由 APISIX SSO 插件处理登录回调，业务侧可跳回首页 |
| GET | `/logout` | 由 APISIX SSO 插件/业务侧完成登出 |
| GET | `/api/auth/me` | 200，返回当前用户和权限；未登录返回 401 |
| POST | `/api/auth/logout` | 204 或 200，session 失效 |
| GET | 未授权门店详情 | 403 |
| POST | viewer 调用编辑/识别/删除接口 | 403，无数据变更 |

### 5.6 AI 和诊断

| 方法 | 路径 | 期望 |
| --- | --- | --- |
| POST | `/api/store-space/recorders/{recorderId}/probe-recognize-channel` | 200 或可解释识别失败，图片输入格式合法 |
| POST | `/api/store-space/channels/{channelId}/recognize` | 200 或可解释识别失败，不影响其他通道 |
| GET | `/api/store-space/diagnostics/ezviz/live-address` | 200 或可解释萤石错误，不泄露 token |
| GET | 短周期诊断日志入口 | 能按 request_id 或门店/录像机定位失败原因 |

### 5.7 执行矩阵

| 验收组 | PostgreSQL 基线 | MySQL 小样本 | MySQL 历史导入后 | 优先级 |
| --- | --- | --- | --- | --- |
| 健康检查和路径 | 必跑 | 必跑 | 必跑 | P0 |
| 门店列表/详情 | 必跑 | 必跑 | 必跑 | P0 |
| 城市筛选/搜索/分页 | 必跑 | 必跑 | 必跑 | P0 |
| 设计图和标注 | 必跑 | 必跑 | 必跑 | P0 |
| 通道映射和截图代理 | 必跑 | 必跑 | 必跑 | P0 |
| H5 Monitor 实时/回放 | 必跑 | 必跑 | 必跑 | P0 |
| 后端权限越权 | 可用 mock | 必跑 | 必跑 | P0 |
| SSO 回调/session | 可用 mock | 必跑 | 必跑 | P0 |
| AI 识别和模型切换 | 必跑 | 必跑 | 抽样跑 | P1 |
| Excel 导出 | 必跑 | 必跑 | 抽样跑 | P1 |
| 性能/慢查询 | 记录基线 | 必跑 | 必跑 | P1 |
| 备份/恢复演练 | 不适用 | 必跑 | 必跑 | P0 |
| 正式交接包 | 不适用 | 草案 | 必须完成 | P0 |

## 6. 页面手工验收清单

### 6.1 后台管理

- 打开后台首页，不出现登录循环或白屏。
- 登录欢迎页启用时，SSO 登录按钮居中、文案明确、错误态可理解。
- 门店列表可搜索“保利”，可按城市筛选，可翻页。
- 门店详情基础信息准确。
- 设计图标注 Tab 能显示图、框、区域卡片。
- 通道映射 Tab 能显示录像机、通道、最近截图、业务区域和床位。
- H5 Monitor 入口只在授权门店显示。
- 编辑/保存后刷新页面，数据仍在。
- 权限不足时，入口隐藏；直接访问 URL 时显示无权限，而不是空白。

### 6.2 H5 Monitor

- 手机和 PC 均可打开金丝雀门店 H5 Monitor。
- 通道列表名称展示符合“区域 + 编号/备注 + 床位”规则。
- 实时视频和回放入口均可走到成功或可解释失败。
- 播放器控制条、滑块、全屏/横屏、截图等既有体验不因数据库切换退化。
- 关闭播放、切换 Tab、返回列表时，播放地址失效或记录无法失效原因。

### 6.3 缺图和资产异常

- 有图时正常展示。
- 缺图时不裂成不可理解状态。
- 诊断入口能给出 asset_store、snapshot_key、exists。
- 不向前端暴露 Supabase service role key、公司文件服务 token、萤石 accessToken。

## 7. 数据对账 SQL

### 7.1 行数和最大 ID

```sql
select 'tb_stores' table_name, count(*) row_count, max(id) max_id from tb_stores
union all select 'tb_store_areas', count(*), max(id) from tb_store_areas
union all select 'tb_store_design_plans', count(*), max(id) from tb_store_design_plans
union all select 'tb_design_plan_annotations', count(*), max(id) from tb_design_plan_annotations
union all select 'tb_ezviz_accounts', count(*), max(id) from tb_ezviz_accounts
union all select 'tb_video_recorders', count(*), max(id) from tb_video_recorders
union all select 'tb_video_channels', count(*), max(id) from tb_video_channels
union all select 'tb_channel_snapshots', count(*), max(id) from tb_channel_snapshots
union all select 'tb_operation_logs', count(*), max(id) from tb_operation_logs;
```

### 7.2 外键孤儿

```sql
select count(*) as orphan_store_areas
from tb_store_areas a
left join tb_stores s on s.id = a.store_id
where s.id is null;

select count(*) as orphan_design_plans
from tb_store_design_plans p
left join tb_stores s on s.id = p.store_id
where s.id is null;

select count(*) as orphan_annotations
from tb_design_plan_annotations a
left join tb_store_design_plans p on p.id = a.design_plan_id
left join tb_store_areas ar on ar.id = a.area_id
where p.id is null or ar.id is null;

select count(*) as orphan_recorders
from tb_video_recorders r
left join tb_stores s on s.id = r.store_id
where s.id is null;

select count(*) as orphan_channels
from tb_video_channels c
left join tb_video_recorders r on r.id = c.recorder_id
where r.id is null;

select count(*) as orphan_snapshots
from tb_channel_snapshots cs
left join tb_video_channels c on c.id = cs.channel_id
where c.id is null;
```

### 7.3 JSON、状态和关键字段

```sql
select count(*) as invalid_channel_json
from tb_video_channels
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as invalid_design_json
from tb_store_design_plans
where recognition_result is not null
  and json_valid(recognition_result) = 0;

select count(*) as missing_external_org
from tb_stores
where trim(external_org_id) = '';

select count(*) as confirmed_business_without_area
from tb_video_channels
where status = 'confirmed_business'
  and (area_type is null or area_number is null);

select count(*) as bed_label_count
from tb_video_channels
where trim(bed_label) <> '';
```

### 7.4 自增值

```sql
select table_name, auto_increment
from information_schema.tables
where table_schema = database()
  and table_name in (
    'tb_stores',
    'tb_store_areas',
    'tb_store_design_plans',
    'tb_design_plan_annotations',
    'tb_ezviz_accounts',
    'tb_video_recorders',
    'tb_video_channels',
    'tb_channel_snapshots',
    'tb_operation_logs'
  )
order by table_name;
```

每张表必须满足 `auto_increment > max(id)`。

### 7.5 权限范围

```sql
select u.email, r.code as role_code, s.scope_type, s.store_id, s.external_org_id
from tb_users u
left join tb_user_roles ur on ur.user_id = u.id
left join tb_roles r on r.id = ur.role_id
left join tb_user_store_scopes s on s.user_id = u.id
order by u.email, r.code, s.scope_type, s.store_id;
```

要求：

- admin 至少有全量范围或被代码识别为全量。
- viewer.none 没有 store/all scope。
- viewer.single 仅有 `10030` 对应门店范围。
- 权限表不存在重复脏数据。

### 7.6 机构 ID、截图路径和资产引用

```sql
select external_org_id, count(*) as cnt
from tb_stores
where trim(external_org_id) <> ''
group by external_org_id
having count(*) > 1;

select count(*) as api_path_snapshot_count
from tb_channel_snapshots
where thumbnail_path like '/api/%'
   or full_image_path like '/api/%';

select count(*) as remote_signed_url_count
from tb_channel_snapshots
where thumbnail_path like 'http%'
   or full_image_path like 'http%';

select count(*) as missing_snapshot_path
from tb_channel_snapshots
where trim(thumbnail_path) = ''
  and trim(full_image_path) = '';
```

要求：

- `external_org_id` 不应重复；确实重复时必须有业务解释和处理方案。
- 迁移后的截图路径应统一为 logical key 或明确兼容格式，不能混存签名 URL。
- 缺失截图路径允许存在，但页面和诊断接口必须能解释。

### 7.7 AI 识别和失败重试状态

```sql
select status, count(*) as cnt
from tb_video_channels
group by status
order by status;

select count(*) as failed_without_reason
from tb_video_channels
where status = 'recognition_failed'
  and (
    recognition_result is null
    or json_extract(recognition_result, '$.error') is null
  );

select recorder_id, count(*) as pending_or_failed_count
from tb_video_channels
where status in ('pending_recognition', 'recognition_failed')
group by recorder_id
having count(*) > 0
order by pending_or_failed_count desc;
```

要求：

- 识别失败必须有可解释原因，不能只留下失败状态。
- “继续上次识别”应能基于状态筛出剩余通道。
- 已确认通道不能被批量识别意外覆盖，除非用户先解锁。

### 7.8 审计和日志最低可查性

```sql
select action, entity_type, count(*) as cnt
from tb_operation_logs
group by action, entity_type
order by cnt desc;

select count(*) as logs_without_store_or_entity
from tb_operation_logs
where store_id is null
  and entity_id is null;
```

要求：

- 核心操作至少能按门店、录像机、通道或用户动作追踪。
- 诊断日志允许短周期保存，但必须能按 request_id、门店、录像机或通道定位最近失败。
- 安全审计日志和短周期诊断日志要区分保存策略。

## 8. 可重建与必须保留数据

### 8.1 必须保留

- 门店主数据、城市、简称、`external_org_id`。
- 设计图文件路径、识别结果、标注框。
- 业务区域、区域编号、备注、床位拆分。
- 萤石账号名称和录像机绑定关系。
- 通道列表、确认状态、业务/非业务判断、AI 识别 JSON。
- 操作日志和后续审计日志。
- 用户、角色、机构范围授权。

### 8.2 可重建或弱保留

- 通道截图文件，可通过刷新/识别重新生成。
- `tb_channel_snapshots` 历史多版本记录，可只保留最新或按策略清理。
- 过期播放地址、回放地址。
- 前端缓存和临时诊断日志。

### 8.3 不应迁移到业务库

- Supabase service role key。
- 萤石 app secret、access token 明文。
- SSO JWT/cookie 原文。
- 完整签名播放 URL 长期落库。

## 9. 通过/失败判定

### 9.1 允许进入历史数据导入

必须同时满足：

- 所有 P0 用例通过：健康、路径、门店列表、详情、设计图、通道、H5 Monitor、图片代理、权限越权、数据对账。
- 金丝雀真实门店 `10030` 通过页面和 API 验收。
- 假数据边界用例通过。
- 数据对账 SQL 无孤儿、无非法 JSON、无主键自增风险。
- 回滚脚本或恢复方案已演练。
- schema 版本、代码 commit、迁移脚本版本已记录。

### 9.2 必须阻断历史数据导入

任一情况出现即阻断：

- `/health` 或主路径不可用。
- 门店列表/详情/H5 Monitor 核心路径不可用。
- 后端权限可被直接接口绕过。
- 数据导入产生外键孤儿或非法 JSON。
- 图片代理路径变更导致前端需要大改或旧路径全部不可用。
- 敏感密钥、播放 URL、设备验证码泄露到前端或日志。
- 无法回滚或无法恢复测试库到导入前状态。

## 10. 待确认问题

- APISIX-SSO 真实联调时 `sy_sso_token` payload 是否按文档稳定包含 `data.mail` 和 `data.user_id`；如果个别账号缺字段，后端拒绝登录并推动安全/SSO 侧修正。
- 公司文件服务是否支持自定义 logical key；如果不支持，`tb_asset_objects` 必须成为正式表。
- MySQL 正式库版本、`sql_mode`、时区、字符集是否和测试库一致。
- 正式库初始化由运维采用整库镜像、DDL+导数脚本，还是公司内部迁移平台。
- H5 Monitor 多 Pod 并发限制是否需要 Redis/DB 租约表，而不是进程内计数。
