# DBA 方案对照 MySQL 迁移验收文档差距清单

最后更新：2026-07-01

本文以 `docs/mysql-migration-acceptance-cases.md` 为反向约束，检查当前 DBA 方案、MySQL schema 草案和迁移准备工作的覆盖情况。本文只做差距分析和待确认清单，不连接数据库、不修改 schema、不导入历史数据、不发布。

## 1. 总体结论

当前 DBA 文档已经覆盖了 MySQL 迁移的大方向：14 张业务表分层、旧 designplan 表保留、治理表建议、资产映射、阶段 A/阶段 B 生命周期、金丝雀、接口兼容、数据对账和正式库初始化流程。

但距离 `docs/mysql-migration-acceptance-cases.md` 要求的“历史数据导入前门禁”还有明显差距。主要缺口不在理念，而在可执行物：

- 还没有固化的 `db/mysql_governance_schema_tb.sql`。
- 还没有 vnext 版业务 DDL 修订，`text not null`、索引、软删除、外部区域/床位预留等仍停留在建议。
- 还没有可重复的样本导入脚本、清理脚本、备份恢复脚本和验收 runner。
- Go MySQL repository、SSO/session、后端权限校验、审计写入、资产权限代理等实现尚未完成。
- 金丝雀门店 `externalOrgId=10030`、录像机 `GN0941203` 的真实链路还没有在 MySQL 测试库跑通。

因此当前状态只能进入“阶段 A 方案和脚本准备”，不能进入 Supabase/PostgreSQL 历史数据导入。

## 2. P0 验收差距

### 2.1 健康检查和路径兼容

验收要求：

- `/erzhuang-project/health` 返回 `database=mysql` 和明确 `asset_store`。
- `/erzhuang-project/`、`/erzhuang-project/api/store-space/*`、`/erzhuang-project/api/h5/*`、H5 页面路径继续可用。

当前覆盖：

- 文档已列入路径兼容门禁。
- 现有 Go 代码已有 base path 前缀剥离逻辑和 `/health` 响应字段。

缺口：

- 当前 app store 只有 `postgres` 和 memory；MySQL store 尚未实现，无法返回 `database=mysql`。
- 还没有 MySQL 模式下的端到端路由 smoke test。

结论：

- P0 实现缺口。历史数据导入前必须实现 MySQL store，并跑通路径 smoke test。

### 2.2 门店列表、详情、设计图、通道映射

验收要求：

- 门店列表分页、搜索、城市筛选、统计汇总。
- 门店详情基础信息、设计图 Tab、通道映射 Tab。
- 设计图标注读写、通道确认、`bed_label` 保存读取。

当前覆盖：

- `db/mysql_schema_tb.sql` 有核心业务表。
- `docs/mysql-testdb-schema-review.md` 已提出索引和字段修订建议。
- `docs/mysql-migration-acceptance-cases.md` 有 AC-010 到 AC-034。

缺口：

- MySQL repository 未实现，当前 SQL 仍是 PostgreSQL 方言。
- `tb_video_channels.area_note text not null` 无默认会卡当前插入路径。
- `tb_ezviz_accounts.app_secret_ciphertext/access_token_ciphertext text not null` 无默认会卡账号同步/创建。
- 还没有压力样本数据和页面 smoke 截图结论。

结论：

- P0 DDL 和实现缺口。必须先修 DDL 草案并实现 MySQL repository，再用假数据和金丝雀跑通。

### 2.3 H5 Monitor 真实链路

验收要求：

- 北京保利实验室门店 `externalOrgId=10030`、录像机 `GN0941203` 的 H5 Monitor 首页、实时视频、回放入口、通道详情可用或有可解释错误。

当前覆盖：

- 文档已明确金丝雀门店和录像机。
- 当前 PostgreSQL 代码已有 H5 Monitor repository 和路由。

缺口：

- MySQL H5 repository 未实现。
- H5 Monitor 权限过滤尚未落到后端统一权限模型。
- 多 Pod 并发限制是否用 Redis/DB 租约表仍待确认。

结论：

- P0 实现和待确认缺口。历史数据导入前至少要在 MySQL 小样本环境跑通 H5 首页和通道详情；播放失败可以接受，但必须是萤石或播放链路可解释错误，不是数据库错误。

### 2.4 后端权限和 SSO

验收要求：

- 未启用 SSO 时不影响现有测试流程。
- 启用后 admin 全量，viewer/operator 按范围过滤。
- 后端接口拒绝越权，不能只靠前端隐藏。

当前覆盖：

- `docs/mysql-dba-plan.md` 已设计 `tb_users`、`tb_roles`、`tb_user_roles`、`tb_user_store_scopes`、`tb_permissions`、`tb_role_permissions`、`tb_auth_sessions`。
- 文档已明确企业邮箱为第一版唯一标识。

缺口：

- 治理表 DDL 未固化到 `db/mysql_governance_schema_tb.sql`。
- 代码已实现 APISIX-SSO callback、`/api/auth/me` 和 `sy_sso_token` RS256 JWT 校验骨架，但尚未接入 `tb_users`、session、权限中间件和登录审计。
- APISIX-SSO payload 已明确包含 `data.mail`、`data.open_id`、`data.user_id`、`data.display`、`data.phone`、`data.username`、`data.login_way`。
- 权限 scope 用 `store_id` 还是 `external_org_id` 作为第一版主过滤键，需要主会话确认。

结论：

- P0 待确认和实现缺口。历史数据导入前，如果权限模块未启用，需要明确“未启用权限不阻断当前流程”的验收口径；若启用，则必须完成后端权限校验。

### 2.5 资产代理和敏感访问

验收要求：

- 图片/PDF 不存二进制。
- `/api/store-space/channel-snapshots/{name}` 代理路径保持不变。
- 旧路径归一化。
- 资产访问权限由后端执行。
- 公司文件服务切换可灰度。

当前覆盖：

- 当前资产策略和 `tb_asset_objects` 方案已写入 DBA 文档。
- 第一阶段继续 Supabase Storage 的口径明确。

缺口：

- `tb_asset_objects`、`tb_asset_access_logs` 还没有 DDL 文件。
- 旧路径归一化脚本未实现。
- 资产代理后端权限校验未实现。
- 公司文件服务是否支持自定义 logical key 未确认。

结论：

- P0/P1 混合缺口。第一阶段可继续 Supabase，但图片代理路径、缺图诊断、旧路径归一化和敏感访问不泄密必须 P0 验收；公司文件服务切换可作为后续 P1/正式化事项。

### 2.6 数据对账、备份恢复和回滚

验收要求：

- 行数、外键孤儿、JSON、状态、关键字段、自增值、权限范围、资产引用、AI 状态、日志可查。
- 导入失败可回滚。
- 历史导入前备份可恢复。

当前覆盖：

- 文档已有多组 SQL 对账清单和阶段 A/B 规则。

缺口：

- 未形成可执行脚本包。
- 未演练备份恢复。
- 未定义测试库阶段 A 清理脚本和阶段 B 迁移脚本命名规范。
- 未定义 baseline 报告产物格式。

结论：

- P0 脚本缺口。历史数据导入前必须至少完成备份、恢复、baseline、导入、校验的可重复流程。

### 2.7 正式交接包

验收要求：

- 运维正式初始化交接包包含 DDL/migration 版本、数据快照时间、导入步骤、校验 SQL、回滚方案、环境变量清单、密钥注入清单、风险和联系人。

当前覆盖：

- 文档已描述交接包方向。

缺口：

- 还没有实际交接包目录或 README。
- 正式库初始化方式待确认：整库镜像、DDL+导数脚本，还是公司迁移平台。
- 正式 MySQL 版本、`sql_mode`、时区、字符集是否与测试库一致待确认。

结论：

- P0 待确认和交付物缺口。可在历史数据导入后、正式初始化前补齐，但方案阶段应先形成模板。

## 3. P1 验收差距

### 3.1 AI 识别和模型切换

验收要求：

- AI provider 设置保留。
- 图片输入格式兼容。
- 非标准模型返回可恢复。
- 继续上次识别能跳过已确认/已成功通道。

当前覆盖：

- `tb_app_settings` 可存 AI provider。
- `recognition_result json` 字段已在业务表中。
- 文档已提示 JSON 合法性。

缺口：

- MySQL repository 中 `app_settings` upsert 未实现。
- AI 识别失败诊断是否落库、短周期日志如何查还未设计清楚。
- “继续上次识别”的 MySQL 查询和状态过滤还未验证。

结论：

- P1 实现缺口，但 AC-100/101/102/103 会阻断历史导入前完整验收，需要纳入 smoke test。

### 3.2 性能、并发和资源释放

验收要求：

- 门店列表性能基线。
- 详情 Tab 独立加载性能。
- H5 Monitor 并发限制可执行。
- 播放地址清理可追踪。

当前覆盖：

- DBA 文档提出索引建议。
- H5 当前存在进程内并发限制背景。

缺口：

- 没有 `EXPLAIN` 结果。
- 没有 P95 阈值。
- H5 并发限制正式方案待确认。
- 播放地址失效结果是否写审计/诊断日志未落地。

结论：

- P1/P0 待确认。性能可先记录基线，但 H5 并发正式方案需要主会话确认。

### 3.3 Excel 导出和验收截图材料

验收要求：

- 通道映射 Excel 内容完整且不越权。
- 迁移前后保留同页面同门店同通道截图或录屏结论。

当前覆盖：

- 文档有 Excel API smoke。

缺口：

- 没有验收截图模板。
- Excel 权限过滤未实现。
- Excel 内容字段对照 SQL 未形成。

结论：

- P1 缺口。历史导入前至少要用金丝雀导出一次 Excel 并留结果。

## 4. 当前方案与验收文档的冲突点

### 4.1 `tb_auth_sessions` vs 验收文档中的 auth API

背景：

- DBA 方案已有 `tb_auth_sessions`。
- 验收文档要求 `/_/auth/login`、`/_/auth/callback`、`/api/auth/me`、`/api/auth/logout`。

差距：

- 当前代码和 DDL 都还没有对应实现。

建议：

- 先由主会话确认 auth 路由命名是否采用验收文档口径，再固化到后端实现和测试。

### 4.2 权限 scope 主键口径

背景：

- DBA 方案支持 `store_id`、`external_org_id`、city、region。
- 验收文档的核心金丝雀以 `externalOrgId=10030` 表达。

差距：

- 第一版权限过滤到底以 `store_id` 还是 `external_org_id` 为主要授权单位尚未拍板。

推荐：

- 第一版同时支持 `store` 和 `external_org`，但后台授权配置优先用门店；H5 Monitor 校验时必须能从 `external_org_id` 反查授权门店。

需要确认：

- 主会话/产品负责人确认第一版授权管理 UI 和导入脚本用哪种 scope 作为主路径。

### 4.3 资产对象表是否第一阶段建表

背景：

- DBA 方案说 `tb_asset_objects` 可第一阶段建表但暂不启用。
- 验收文档 AC-113 要求公司文件服务切换可灰度。

差距：

- 如果第一阶段不建表，后续公司文件服务灰度会缺少映射基座。

推荐：

- 第一阶段就建 `tb_asset_objects` 和 `tb_asset_access_logs`，但只对新迁移或回填对象写少量数据，不阻断 Supabase 读取。

需要确认：

- 公司文件服务是否支持自定义 logical key；若不支持，`tb_asset_objects` 必须成为正式表。

### 4.4 测试库阶段 A 清理方式

背景：

- 阶段 A 可重建，阶段 B 不可随意清表。
- 验收文档要求阶段 A 结束前清理或隔离假数据。

差距：

- 还没有决定阶段 A 结束是“重建空库再导历史”，还是“按标记清理假数据”。

推荐：

- 如果测试库已被多轮试验污染，阶段 B 前重建 schema 更干净；如果无法重建，则所有假数据必须有 `canary_` 标识和清理脚本。

需要确认：

- 主会话确认阶段 A 到 B 切换时是否允许重建测试库。

## 5. 待确认清单

### 5.1 SSO payload 字段

背景：公司 APISIX-SSO 文档已明确 `sy_sso_token` 是 RS256 JWT，payload 包含 `data.mail`、`data.open_id`、`data.user_id`、`data.phone`、`data.username`、`data.display`、`data.login_way`、`exp`、`sub`。  
结论：第一版授权使用 `data.mail` 作为 `tb_users.email` 匹配主键，预留飞书 open id 和 user id。  
影响范围：`tb_users`、登录态查询、审计 user_email、权限导入。  
风险：如果真实联调发现个别账号缺少 `data.mail`，后端应拒绝登录，并由安全/SSO 侧修正字段下发，不回退到 username 主键。  
下一步：接入 `tb_users` 校验和登录审计。

### 5.2 MySQL 正式环境参数

背景：测试库为 MySQL 8.0.13、`sql_mode` 为空、时区 +08:00。  
可选方案：应用 session 设置严格模式；推动库级设置；两者都做。  
推荐方案：应用 session 设置 + 运维库级规范双保险。  
影响范围：迁移脚本、连接初始化、正式交接包。  
风险：静默截断、非法日期、时区偏移。  
需要确认：运维、主会话。  
确认后下一步：写入 DDL README 和 MySQL 连接初始化要求。

### 5.3 正式库初始化方式

背景：正式库不会给直连账号。  
可选方案：整库镜像；DDL+导数脚本；公司迁移平台。  
推荐方案：由运维确认公司标准方式，DBA 提供版本化 DDL、校验 SQL、baseline 报告。  
影响范围：交接包、停写窗口、回滚策略。  
风险：测试库和正式库结构/数据不一致。  
需要确认：运维、主会话。  
确认后下一步：生成正式交接包模板。

### 5.4 H5 Monitor 多 Pod 并发

背景：当前并发限制是进程内计数。  
可选方案：暂保留进程内；Redis；MySQL 租约表。  
推荐方案：正式环境至少设计 Redis 或 DB 租约表，测试阶段可先记录风险。  
影响范围：H5 live-url/playback-url、disable-url、审计和资源释放。  
风险：多 Pod 下并发限制失效，播放资源长期占用。  
需要确认：研发/运维/主会话。  
确认后下一步：决定是否新增租约表或引入 Redis。

### 5.5 阶段 A 到 B 的测试库处理

背景：测试库先用于折腾，后续承载真实历史数据。  
可选方案：重建测试库后导历史；清理假数据后导历史。  
推荐方案：若已污染，重建 schema 后导历史；否则按 canary 标识清理并校验。  
影响范围：历史数据导入、baseline、回滚。  
风险：假数据污染正式初始化来源。  
需要确认：主会话、产品负责人、DBA。  
确认后下一步：写清理/重建脚本和导入前备份流程。

## 6. 建议下一步

1. 先固化 `db/mysql_schema_tb_vnext.sql` 或修订原 `db/mysql_schema_tb.sql`，解决 `text not null`、索引、外部区域/床位、软删除、截图 logical key 语义。
2. 新增 `db/mysql_governance_schema_tb.sql`，覆盖用户、角色、权限、session、审计、资产对象和资产访问日志。
3. 新增 `docs/mysql-migration-decision-log.md`，记录所有待确认项的结论、确认人、时间和后续动作。
4. 设计阶段 A 假数据 seed 和清理脚本，不导入真实历史数据。
5. 实现 MySQL repository 前，先把 acceptance cases 拆成 P0 smoke runner 清单。
6. 主会话确认 SSO、正式库初始化方式、公司文件服务 key 能力、H5 多 Pod 并发策略。

## 7. 当前禁止事项

在上述 P0 gap 和待确认项处理前，不应执行：

- 不导入 Supabase/PostgreSQL 历史数据到 MySQL 测试库。
- 不把测试库结构直接作为正式 DDL 交付运维。
- 不清表或重建进入阶段 B 后的测试库。
- 不把 MySQL 密码、Supabase service role key、萤石密钥、SSO JWT/cookie 原文写入仓库或文档。
- 不发布到公司环境。
