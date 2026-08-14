# MySQL 阶段 A 实操前执行计划

最后更新：2026-07-01

本文是阶段 A 进入测试库实操前的执行包草案。它只用于主会话 review 和执行前确认，不包含密码、连接串、access token、app secret、SSO token，也不代表已经允许连接或修改 MySQL 测试库。

## 1. 阶段 A 目标

阶段 A 只验证 MySQL schema、补丁、治理表、假数据、金丝雀链路、API smoke、页面 smoke 和只读校验 SQL。

阶段 A 不导入 Supabase/PostgreSQL 历史数据，不把测试库当前状态当正式环境基线，不把阶段 A 假数据当作历史数据来源。历史数据导入必须等阶段 A 门禁全部通过，并完成冻结、备份、回滚方案确认后再进入阶段 B。

当前 SSO 背景：

- APISIX-SSO 骨架已发布到公司环境 `2.23.0`。
- 默认 `SSO_ENABLED=false`，因此阶段 A MySQL 验收可以先在未启用 SSO 的情况下验证现有业务流程。
- `SSO_ENABLED=false` 时，`/api/auth/me` 返回本地 admin 兼容态，不查 `tb_users`。
- `SSO_ENABLED=true` 且无/无效 `sy_sso_token` 时，`/api/auth/me` 返回 401 和 `login_url`。
- `SSO_ENABLED=true` 且 token 有效时，`/api/auth/me` 返回 SSO 用户字段；正式权限上线后再接 `tb_users`、角色和机构 scope。

## 2. 前置条件

执行任何连接、DDL、DML 或启动 MySQL 模式后端之前，必须由主会话确认：

| 前置项 | 成功标准 | 未满足时处理 |
| --- | --- | --- |
| 主会话确认阶段 A 可开始 | 明确允许连接测试 MySQL 并执行阶段 A 草案 | 不连接数据库 |
| 测试库生命周期 | 明确当前测试库是否允许重建、清表、回滚到空 schema | 不执行破坏性 DDL/DML |
| 测试库连接方式 | 连接信息只通过运行时环境或交互式输入提供，不写入文件/日志 | 不写连接串和密码 |
| 当前代码 commit | 记录 `git rev-parse HEAD` 输出 | 不进入可复盘执行 |
| DDL 文件版本 | 记录以下文件的 commit/校验结果 | 先更新交付包 |
| 备份或可重建状态 | 阶段 A 当前数据可丢弃，或已有备份 | 不重建、不清理 |
| SSO 开关状态 | 明确 `SSO_ENABLED=false` 或已具备 SSO 联调用户 | 权限 smoke 标记为未启用但不跳过数据库 seed |
| 资产模式 | 明确第一阶段仍为 Supabase Storage 或本地 mock | 截图代理只做可解释失败验收 |

本轮 DDL/文档输入：

- `db/mysql_schema_tb.sql`
- `db/mysql_business_schema_patch_tb.sql`
- `db/mysql_governance_schema_tb.sql`
- `db/mysql_stage_a_seed_sample_tb.sql`
- `db/mysql_stage_a_cleanup_sample_tb.sql`
- `docs/mysql-validation-sql.md`
- `docs/mysql-migration-acceptance-cases.md`

## 3. 执行顺序

### 3.1 环境探针

草案 SQL：使用 `docs/mysql-validation-sql.md` 的“环境探针”和“表清单”。

成功标准：

- 能看到 MySQL version、`@@sql_mode`、`@@time_zone`、字符集和排序规则。
- 测试库目标 database 与主会话确认一致。
- 如果 `sql_mode` 为空或不严格，执行记录中必须标红，后续 migration session 显式设置严格模式。

失败处理：

- 连接失败：停止，检查网络、账号权限、白名单或连接方式。
- 目标库不一致：停止，不执行任何 DDL/DML。
- 正式库误连风险：立即停止并回报主会话。

禁止事项：

- 不在文档、脚本、命令历史中写密码。
- 不对正式库直连。

### 3.2 应用业务 schema

草案文件：`db/mysql_schema_tb.sql`。

成功标准：

- 14 张 `tb_` 表创建成功，至少包含当前业务核心表和旧 designplan 兼容表。
- `information_schema.tables` 表清单与预期一致。
- 失败时有明确 SQL 行号和错误原因。

失败处理：

- 空库执行失败：修订 DDL 草案，重新由主会话确认。
- 非空库执行冲突：先导出当前 schema，再决定重建或写 migration，不手工拍脑袋改表。

禁止事项：

- 未确认测试库可重建前，不执行 drop/truncate。
- 不把 `CHECK` 当成唯一业务校验，因为 MySQL 8.0.13 不能依赖它强制枚举。

### 3.3 应用 business patch

草案文件：`db/mysql_business_schema_patch_tb.sql`。

成功标准：

- `tb_ezviz_accounts` 的密文字段允许阶段 A 空值/NULL。
- `tb_video_channels.area_note` 支持默认空字符串。
- `external_area_id`、`external_bed_id`、`snapshot_key`、`snapshot_key_hash` 字段存在。
- 门店列表、H5 Monitor、最新截图、外部区域/床位查询相关索引存在。

失败处理：

- 字段或索引已存在：转换为按 `information_schema` 判断的版本化 migration。
- MySQL 版本不支持某个 DDL 写法：拆成兼容 8.0.13 的单条 migration。

禁止事项：

- 软删除 DDL 当前保持注释，未获产品/主会话确认前不落地。
- 注释中的 backfill/update 只作为策略，不在阶段 A 首次执行时直接跑。

### 3.4 应用 governance schema 和 seed 权限

草案文件：`db/mysql_governance_schema_tb.sql`。

成功标准：

- `tb_users`、`tb_roles`、`tb_permissions`、`tb_user_roles`、`tb_role_permissions`、`tb_user_store_scopes`、`tb_auth_sessions`、`tb_audit_logs`、`tb_asset_objects`、`tb_asset_access_logs` 创建成功。
- `admin/operator/viewer` 三个系统角色存在。
- 权限点和角色权限 seed 可重复执行，不产生重复数据。
- session 表只保存 `session_token_hash`，不保存原始 session token。

失败处理：

- FK 失败：确认业务表是否已创建，或按阶段 A 顺序重来。
- 权限 seed 冲突：检查角色/权限 code 是否被手工改过，先记录再修订。

禁止事项：

- 不写真实企业邮箱以外的敏感个人信息。
- 不写任何 SSO token、cookie、session 明文。

### 3.5 插入假数据和金丝雀

草案文件：`db/mysql_stage_a_seed_sample_tb.sql`。

成功标准：

- 金丝雀门店 `external_org_id=10030`、北京保利实验室、录像机 `GN0941203`、通道 1 存在。
- 普通样本门店覆盖设计图、区域、通道、截图路径、`bed_label`、`recognition_result`。
- 权限样本覆盖 admin、viewer 单机构、viewer 多机构、operator 单机构、禁用用户。
- 样本数据全部使用 `stage_a_` 或 `canary_` 标记，可定位、可清理。

失败处理：

- 唯一键冲突：检查阶段 A 是否已执行过 seed；若允许重建则重建，否则写清理脚本草案并交主会话确认。
- FK 失败：检查前置业务/治理表和 patch 是否完整。

禁止事项：

- 不写真实 token/app secret/access token。
- 不把阶段 A 样本合并进历史数据迁移脚本。

### 3.5.1 Seed 执行策略

首次空库：

- 可在业务 schema、business patch、governance schema 都完成后执行 seed 草案。
- seed 依赖 `external_area_id`、`external_bed_id`、`snapshot_key`、`snapshot_key_hash`、治理表和资产/审计表，前置 DDL 缺一不可。

重复试跑：

- 方案 A：先执行 `db/mysql_stage_a_cleanup_sample_tb.sql` 清理阶段 A 样本，再执行 seed。
- 方案 B：把 seed 改成幂等 migration，例如 `insert ignore` 或 `insert ... on duplicate key update`。
- 两个方案必须由主会话二选一确认；未确认前不重复执行 seed。

阶段 B 历史数据导入后：

- 禁止再执行 cleanup。
- 如果需要补样本，必须写新的 migration，避开真实数据 ID、真实门店和历史资产路径。

### 3.6 启动 MySQL 模式后端

执行方式由主会话确认。环境变量只允许通过运行时注入，不写入仓库。

成功标准：

- `/erzhuang-project/health` 或 `/health` 返回 200。
- 响应中 `database=mysql`。
- `asset_store` 能明确当前模式。
- 数据库连接失败时能明确失败，不出现页面空数据但健康状态正常。

失败处理：

- 后端启动失败：先看启动日志和数据库连接配置，不改数据库。
- health 不显示 MySQL：检查应用配置是否仍指向 PostgreSQL。

禁止事项：

- 不在命令行明文拼接密码。
- 不改公司发布配置，不发布公司环境。

### 3.7 API Smoke

详见第 6 节。成功标准是 P0 endpoint 全部返回预期状态码，核心字段存在；外部依赖如萤石未配置时允许可解释失败，但不能 500 或泄密。

### 3.8 页面 Smoke

成功标准：

- 门店列表分页、搜索、城市筛选、统计汇总可用。
- 门店详情基础信息、设计图标注 Tab、通道映射 Tab 可打开。
- 截图代理缺图时页面不崩溃，有可诊断信息。
- H5 Monitor 页面 `/h5/orgs/10030/monitor` 可打开；底层 API 可返回数据或可解释失败。
- SSO 未启用时现有流程不被登录拦截；SSO 启用后按权限范围过滤。

失败处理：

- 页面请求失败：记录 URL、状态码、响应体、后端日志 request id。
- 视觉或交互问题不影响 DB 导入判断时，可标为 P1/P2，但 P0 数据链路必须修复。

### 3.9 Validation SQL

草案文件：`docs/mysql-validation-sql.md`。

成功标准：

- 行数、主键范围、外键孤儿、JSON 合法性、自增值、截图路径、权限范围、金丝雀链路、日志可查性均可输出结果。
- P0 校验无阻断问题。
- 若出现需要 `alter table ... auto_increment` 的结果，只生成修复 SQL 文本，不直接执行。

失败处理：

- P0 失败：停止进入阶段 B，保留现场，整理待确认/待修复清单。
- P1/P2 失败：记录风险和修复优先级，由主会话决定是否阻断下一步。

## 4. 阶段 A 最小样本数据方案

### 4.1 推荐样本

| 样本 | 目的 | 最小数据 |
| --- | --- | --- |
| 北京保利实验室金丝雀 | H5 Monitor 真实链路 | `external_org_id=10030`、录像机 `GN0941203`、通道 1、截图路径 |
| 普通门店样本 | 后台业务全流程 | 门店、设计图、区域、标注、录像机、业务通道、非业务通道、`bed_label` |
| 权限用户 | SSO/权限数据库准备 | admin、viewer 单机构、viewer 多机构、operator 单机构、禁用用户 |

### 4.2 推荐插入顺序

1. `tb_stores`
2. `tb_store_areas`
3. `tb_store_design_plans`
4. `tb_design_plan_annotations`
5. `tb_ezviz_accounts`
6. `tb_video_recorders`
7. `tb_video_channels`
8. `tb_channel_snapshots`
9. `tb_asset_objects`
10. `tb_users`
11. `tb_user_roles`
12. `tb_user_store_scopes`
13. `tb_operation_logs`
14. `tb_audit_logs`
15. `tb_asset_access_logs`

### 4.3 最小字段集合

- 门店：`id`、`city`、`name`、`short_name`、`normalized_name`、`external_org_id`、状态字段。
- 区域：`id`、`store_id`、`area_type`、`area_number`、`display_name`、`external_area_id`。
- 设计图：`id`、`store_id`、`upload_id`、路径字段、`recognition_status`、`recognition_result`。
- 标注：`design_plan_id`、`area_id`、`box_x/y/width/height`、`status`。
- 录像机：`id`、`store_id`、`ezviz_account_id`、`device_code`、`status`、`effective_channel_count`。
- 通道：`id`、`recorder_id`、`channel_no`、`status`、`scene_type`、`area_type`、`area_number`、`bed_label`、`area_id`、`recognition_result`。
- 截图：`channel_id`、`snapshot_key`、`snapshot_key_hash`、`thumbnail_path`、`full_image_path`。
- 权限：`email`、`enabled`、角色、`scope_type`、`scope_key`、`store_id` 或 `external_org_id`。

## 5. 样本数据草案

样本 SQL 文件：`db/mysql_stage_a_seed_sample_tb.sql`。

执行前必须确认：

- business patch 已应用，否则 `snapshot_key`、`external_area_id`、`external_bed_id` 不存在。
- governance schema 已应用，否则权限、审计、资产表不存在。
- 测试库允许写入阶段 A 样本。
- 样本 ID 段不会与已有数据冲突；推荐先用空库或可重建库。

当前推荐 scope 口径：

- 阶段 A seed 同时保留 `store_id` 和 `external_org_id` 信息，但 `scope_type` 推荐先用 `external_org`，因为 H5 Monitor 入口天然使用 `externalOrgId`。
- 这不是最终拍板。若主会话确认第一版权限过滤以 `store_id` 为主，应修改 seed 和后端过滤逻辑。

## 6. API Smoke Checklist

P0 必须通过或给出可解释外部依赖失败。所有请求在公司路径前缀下也要验证一次，即 `/erzhuang-project` 前缀不能破。

| 优先级 | 方法 | Endpoint | 期望状态码 | 关键字段/判定 | 失败定位 |
| --- | --- | --- | --- | --- | --- |
| P0 | GET | `/health`、`/erzhuang-project/health` | 200 | `status=ok`、`database=mysql`、`asset_store` 明确 | 检查 DB 配置和 health handler |
| P0 | GET | `/api/auth/me` | 200 或 401 | `SSO_ENABLED=false`：200，`enabled=false`、`authenticated=true`、`user.email=local-admin@example.com`、`permissions=["admin"]`；`SSO_ENABLED=true` 且无/无效 `sy_sso_token`：401 + `login_url`；有效 token：200 + SSO 用户字段。正式权限上线后再验 `tb_users`/角色/scope | 检查 `SSO_ENABLED`、`sy_sso_token`、APISIX JWT、`SSO_EXPECTED_SUB`、后续权限表 |
| P0 | GET | `/api/store-space/stores?page=1&page_size=20` | 200 | `items`、`total`、`summary`、`cities` | 查门店表、分页 SQL、权限过滤 |
| P0 | GET | `/api/store-space/stores?q=保利` | 200 | 能定位 `external_org_id=10030` | 查搜索条件和 normalized_name |
| P0 | GET | `/api/store-space/stores?city=北京` | 200 | 城市筛选生效，不只筛当前页 | 查城市索引和 summary SQL |
| P0 | GET | `/api/store-space/stores/{storeId}` | 200 | `short_name`、`external_org_id`、状态字段 | 查主键、权限 scope |
| P0 | GET | `/api/store-space/stores/{storeId}/design-plan-data` | 200 | 设计图路径、区域、标注框 | 查设计图/标注 FK |
| P0 | PUT | `/api/store-space/stores/{storeId}/design-plan` | 200 | 保存后刷新标注仍存在 | 查 request body、事务、operation log |
| P0 | GET | `/api/store-space/stores/{storeId}/channel-data` | 200 | 录像机、通道、`bed_label`、截图路径 | 查 recorder/channel/snapshot |
| P0 | PUT | `/api/store-space/channels/{channelId}/confirmation` | 200 | `confirmed_business`、`bed_label` 保留 | 查 channel update 和 area_id |
| P0 | GET | `/api/store-space/channel-snapshots/{name}/diagnostics` | 200 | `asset_store`、`snapshot_key`、`exists`、错误可解释 | 查 AssetStore key 映射 |
| P0 | GET | `/api/store-space/channel-snapshots/{name}` | 200 或 404 | 有图返回图片；缺图返回可解释错误，不 500 | 查 Supabase/local asset 配置 |
| P0 | GET | `/h5/orgs/10030/monitor` | 200 | H5 页面可打开 | 查前端路由和 base path |
| P0 | GET | `/api/h5/orgs/10030/monitor` | 200 | 门店、通道列表、externalOrgId | 查 H5 repository |
| P0 | POST | `/api/h5/orgs/10030/monitor/channels/{channelId}/live-url` | 200 或可解释 4xx/502 | 成功有播放地址；失败有萤石/配置错误，不泄露密钥 | 查 Ezviz 配置和日志 |
| P0 | GET | `/api/h5/orgs/10030/monitor/channels/{channelId}/record-segments?date=YYYY-MM-DD` | 200 或可解释 4xx/502 | 片段列表或明确无片段 | 查时间参数和萤石接口 |
| P0 | POST | `/api/h5/orgs/10030/monitor/channels/{channelId}/playback-url` | 200 或可解释 4xx/502 | 成功有回放地址；失败不泄密 | 查时间窗口和并发限制 |
| P1 | POST | `/api/store-space/recorders/{recorderId}/scan-channels` | 200 或可解释设备错误 | 不破坏已有通道确认 | 查设备配置 |
| P1 | POST | `/api/store-space/recorders/{recorderId}/recognize-channels` | 200 或可解释 AI 错误 | 失败写入 `recognition_result` 错误 | 查 AI provider 和图片输入格式 |
| P1 | GET | `/api/store-space/stores/{storeId}/channel-mappings/export.xlsx` | 200 | Excel 可打开且不越权 | 查导出 SQL 和权限 |

## 7. 页面 Smoke Checklist

| 页面 | 操作 | 成功标准 | 失败定位 |
| --- | --- | --- | --- |
| 后台首页/门店列表 | 打开、分页、搜索“保利”、筛选“北京” | 样本门店可见，统计汇总正确 | 网络请求、列表 SQL、权限过滤 |
| 门店详情基础信息 | 打开金丝雀和普通门店 | `short_name`、`external_org_id`、状态展示正确 | 详情接口 |
| 设计图标注 Tab | 打开普通门店，保存一个标注 | 图片/空态可解释，标注保存刷新仍在 | 设计图接口、资产路径 |
| 通道映射 Tab | 查看通道 1 和普通门店通道 | 录像机、通道号、业务区域、`bed_label`、截图路径正确 | channel-data 接口 |
| 截图代理 | 打开有图/缺图样本 | 有图显示，缺图不崩溃 | diagnostics 接口 |
| H5 Monitor | 打开 `/h5/orgs/10030/monitor` | 页面可打开，通道列表来自 `10030` | H5 API、前端 base path |
| 权限 | viewer 单机构、viewer 多机构、operator、禁用用户 | 后端接口按范围过滤或拒绝 | `/api/auth/me`、scope 表、审计日志 |

## 8. 主会话必须确认的问题清单

| 问题 | 背景 | 可选方案 | 推荐方案 | 影响范围 | 风险 | 确认方 | 确认后下一步 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 阶段 A 是否允许重建测试库 | 阶段 A seed 可能多次试跑 | A. 可重建空库；B. 不重建，仅 migration；C. 先备份再重建 | 若尚无历史数据，推荐 A 或先备份后 A | DDL、seed、清理脚本 | 误删已有可用数据 | 主会话、用户 | 决定是否允许 drop/recreate 或只增量 |
| 阶段 A 假数据命名和清理规则 | 后续测试库会承载真实历史数据 | A. `stage_a_` 前缀；B. 独立 ID 段；C. 单独清理表记录 | `stage_a_` + 独立 ID 段 | 样本导入、清理、对账 | 假数据混入历史数据 | 主会话、用户 | 生成清理 SQL 草案 |
| 权限 scope 第一版过滤键 | H5 使用 externalOrgId，后台详情使用 storeId | A. `store_id`；B. `external_org_id`；C. 双写，查询统一转换 | 阶段 A 用 `external_org` 验证 H5，后端可双写兼容 | 所有权限查询和接口过滤 | 越权或误拒绝 | 主会话、研发 | 固化 repository 查询条件 |
| `SSO_EXPECTED_SUB` 上线值 | APISIX-SSO 即将联调 | A. 不校验；B. 校验固定值；C. 分环境配置 | 由安全/运维提供分环境配置 | 登录、`/api/auth/me`、审计 | 错配导致全员无法登录或校验失效 | 安全/SSO、运维、主会话 | 更新运行时配置，不写入仓库 |
| 时间字段口径 | PostgreSQL `timestamptz` 到 MySQL `datetime(3)` | A. 北京时间；B. UTC | 当前推荐北京时间，需主会话确认 | 迁移脚本、页面展示、对账 SQL | 系统性差 8 小时 | 主会话、研发、运维 | 固化导出/导入转换 |
| `tb_asset_objects` 第一阶段策略 | 文件服务未接入，资产仍在 Supabase | A. 建表不启用；B. 建表并为样本回填；C. 不建表 | 阶段 A 建表并样本回填，业务仍走现有路径 | 资产迁移、审计、未来文件服务 | 过早耦合未定文件服务 | 主会话、运维/文件服务负责人 | 明确 provider 切换和回填节奏 |

## 9. 当前禁止事项

- 不连接 MySQL，除非主会话明确授权。
- 不写真实密码、连接串、access token、app secret、SSO token。
- 不导入 Supabase/PostgreSQL 历史数据。
- 不发布公司环境。
- 不提交、不 push。
- 不把本文件中的 SQL 草案当成正式 migration 直接执行。

## 10. Review 与执行边界

可以直接 review 的内容：

- `docs/mysql-stage-a-execution-plan.md`
- `db/mysql_stage_a_seed_sample_tb.sql`
- `db/mysql_stage_a_cleanup_sample_tb.sql`
- `db/mysql_business_schema_patch_tb.sql`
- `db/mysql_governance_schema_tb.sql`
- `docs/mysql-validation-sql.md`

必须等主会话确认后才能执行的内容：

- 任何 MySQL 连接命令。
- `db/mysql_schema_tb.sql` 初始化业务表。
- `db/mysql_business_schema_patch_tb.sql` 中的 ALTER/INDEX。
- `db/mysql_governance_schema_tb.sql` 中的治理表 DDL 和 seed。
- `db/mysql_stage_a_seed_sample_tb.sql` 中的样本 DML。
- `db/mysql_stage_a_cleanup_sample_tb.sql` 中的样本清理 DML。
- `docs/mysql-validation-sql.md` 中生成的 `alter table ... auto_increment` 修复 SQL。
- 启动 MySQL 模式后端并访问公司测试环境。

禁止执行的内容：

- 历史数据导入。
- 正式库直连。
- 生产/公司环境发布。
- 未确认的清表、重建、drop、truncate。
- 写入真实密钥、token、设备验证码或长期签名 URL。

## 11. Stage A 执行记录模板

```markdown
# MySQL Stage A 执行记录

- 执行日期：
- 执行人：
- 代码 commit：
- DDL 文件版本或 hash：
  - db/mysql_schema_tb.sql：
  - db/mysql_business_schema_patch_tb.sql：
  - db/mysql_governance_schema_tb.sql：
  - db/mysql_stage_a_seed_sample_tb.sql：
  - db/mysql_stage_a_cleanup_sample_tb.sql：
- 测试库名：
- 是否确认阶段 A 沙箱：
- 是否允许重建/清理：

## 步骤结果

| 步骤 | 结果 | 备注 |
| --- | --- | --- |
| 环境探针 |  | version/sql_mode/time_zone/charset |
| 应用业务 schema |  |  |
| 应用 business patch |  |  |
| 应用 governance schema |  |  |
| 执行 cleanup |  | 首次空库可填未执行 |
| 执行 seed |  |  |
| 启动 MySQL 模式后端 |  |  |
| API smoke |  | P0/P1 摘要 |
| 页面 smoke |  |  |
| validation SQL |  | 行数、孤儿、JSON、auto_increment、权限、金丝雀 |

## 阻断问题

- P0：
- P1：
- 待确认：

## 结论

- 是否允许进入阶段 B：
- DBA 建议：
```

## 12. 阶段 A 出口材料

阶段 A 结束时，应交给主会话：

- 执行过的 DDL/migration 文件列表和顺序。
- 阶段 A seed 数据版本和样本 ID。
- API smoke 结果。
- 页面 smoke 结果。
- `docs/mysql-validation-sql.md` 输出摘要。
- P0/P1/P2 问题清单。
- 是否允许进入阶段 B 的 DBA 建议。
