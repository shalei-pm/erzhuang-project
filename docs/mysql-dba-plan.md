# MySQL DBA 与权限数据模型方案

最后更新：2026-07-01

本文面向主会话验收和后续工程实施，用于把当前 Supabase/PostgreSQL 项目逐步迁移到公司 MySQL 正规环境。本文只覆盖方案、schema 方向、样本迁移和验证建议，不代表已经发布或连接任何公司环境。

## 0. DBA 专项长期背景基线

`erzhuang-project` 是新氧青春门店空间资源、摄像头监控、设计图标注后台，当前正在从个人练习项目逐步迁移到公司正规环境。公司没有专门 DBA，本专项需要承担项目 DBA 职责：表结构、索引、迁移、校验、权限数据模型、资产映射和回滚策略。

当前主业务运行在公司测试环境，后端 Go，前端 React/Vite。多次线上 health 记录显示 `database=postgres`、`asset_store=supabase`。当前业务数据库是 PostgreSQL/Supabase，未来迁到公司 MySQL；公司 MySQL 表名强制使用 `tb_` 前缀。测试 MySQL 库允许本地直连调试，正式库不会提供本地直连账号，正式环境只能通过 K8s Secret 等公司规范接入。

图片和资产第一阶段仍暂留 Supabase Storage，bucket 文档记录为 `design-plan-assets`。通道截图当前由数据库字段保存 `/api/store-space/channel-snapshots/{name}.jpg` 这类接口路径，后端内部映射到 AssetStore key `channel-snapshots/{name}.jpg`。设计图文件 key 主要是 `uploads/{upload_id}/original.pdf`、`uploads/{upload_id}/preview.png`、`uploads/{upload_id}/thumbnail.png`。摄像头截图属于敏感经营现场信息，不能长期放在外部 Supabase，后续必须迁到公司受控图片/文件服务，并纳入权限校验和审计。

公司 SSO 第一阶段采用公司推荐的 APISIX 网关 `security-sso` 插件，不自建 OAuth2 登录流程。APISIX-SSO 通过 `sy_sso_token` cookie 下发 RS256 JWT；payload 已明确包含 `data.mail`、`data.open_id`、`data.user_id`、`data.phone`、`data.username`、`data.display`、`data.login_way`、`exp`、`sub`。业务服务需要保留 `/_/auth/callback`、`/logout` 和 `/api/auth/me`，并校验 JWT 签名、过期时间和企业邮箱。详见 `docs/sso-demo-handoff.md`。

项目权限由本项目自己负责，SSO 只负责身份认证。第一版简化决策是使用企业邮箱作为 `tb_users` 的唯一登录标识；`username`、`display_name` 可能重名，不能作为权限主键。登录逻辑：后端从已验签的 `sy_sso_token` 中拿到 `data.mail` 后查 `tb_users.email`；用户存在且 enabled 才允许进入，不存在或 disabled 则拒绝。

业务权限要支持“全量机构、多机构、单机构”的范围差异。第一版角色先收敛为 `admin`、`operator`、`viewer`，并用 scope 约束可见机构/门店。权限维度需要覆盖页面可见、Tab 可见、具体操作按钮可见/可用；前端隐藏只作为体验，后端接口必须做真实权限校验。H5 Monitor、实时视频、录像回放、截图读取都属于敏感访问，需要权限和审计。

测试库曾探测到 MySQL 版本 `8.0.13`、`time_zone=+08:00`、`sql_mode` 为空。后续所有 schema 与迁移脚本都必须按这个风险基线设计。

### 0.1 需确认事项协作规则

DBA 专项在分析 MySQL 迁移、权限模型、资产存储、数据清理或 schema 调整时，如果遇到需要主会话、产品负责人、DBA 双向确认的事项，不直接蛮干实现。必须先整理成明确文档或待确认清单，并写清楚：

- 背景：为什么这个问题出现，当前约束是什么。
- 可选方案：至少列出可执行选项，而不是只给单一路径。
- 推荐方案：说明为什么推荐。
- 影响范围：涉及哪些表、接口、页面、迁移脚本、运维动作。
- 风险：数据丢失、权限绕过、回滚困难、正式环境不可用等风险。
- 需要谁确认：主会话、产品负责人、运维、安全/SSO、研发等。
- 确认后下一步：确认通过后具体做什么，未确认前禁止做什么。

该清单应先发给主会话复核，由主会话与用户确认后再继续推进。尤其要注意：测试库在阶段 A 可以用于 schema 调试；但历史数据导入后会成为公司测试环境的数据基座，不能随意清表或重建。正式环境会由运维基于确认后的测试库结构与数据初始化。

## 1. 当前判断

当前 `db/mysql_schema_tb.sql` 已有 14 张 `tb_` 前缀表，基本覆盖任务配置、门店、设计图、区域、录像机、通道、截图和操作日志。方向是对的，但它仍是业务数据表草案，缺少正式环境必须具备的用户、角色、机构范围、权限点、会话与审计模型。

第一阶段不建议把图片、PDF、通道截图二进制迁入 MySQL。数据库只保存 logical key 或路径，文件内容继续走 Supabase Storage，由 Go 后端代理读取。摄像头截图属于敏感信息，未来必须迁到公司受控文件服务，并纳入权限校验和审计日志。

## 2. 表分层结论

### 2.1 保留为核心业务表

这些表是 MySQL 第一阶段主路径，应保留并作为后端 MySQL repository 的主要读写对象：

- `tb_app_settings`：应用配置，例如 AI provider。
- `tb_stores`：门店主表，保留 `external_org_id` 作为公司组织/机构 ID 对接字段。
- `tb_store_areas`：门店业务区域，承接设计图识别、人工确认和通道映射。
- `tb_store_design_plans`：新门店模型下的设计图文件与识别结果。
- `tb_design_plan_annotations`：设计图标注框，关联 `tb_store_design_plans` 与 `tb_store_areas`。
- `tb_ezviz_accounts`：萤石账号展示与绑定引用。密钥仍应来自运行时 Secret，不建议把 app secret/access token 落业务库。
- `tb_video_recorders`：录像机。
- `tb_video_channels`：通道、识别结果、床位拆分 `bed_label`。
- `tb_channel_snapshots`：通道截图 logical key 与过期时间，不存图片二进制。
- `tb_operation_logs`：现有业务操作日志，可作为过渡表，但正式审计应新增更结构化的 `tb_audit_logs`。

### 2.2 旧表，仅用于兼容和迁移

这些表来自旧的“设计图门店”独立模块，不应继续扩展新功能：

- `tb_design_plan_stores`
- `tb_design_plan_store_areas`
- `tb_design_plan_operation_logs`

建议策略：

1. 如果测试 MySQL 已建表，保留到迁移完成，避免旧接口或导入脚本中断。当前 `cmd/server/main.go` 仍注入 `designplan.NewPostgresStore(db)` 并注册旧 `designplan` 路由，不能在代码改造前轻易删除旧表。
2. 样本迁移时如源数据仍在旧表，先导入旧表，再转换到 `tb_stores`、`tb_store_design_plans`、`tb_store_areas`、`tb_design_plan_annotations`。
3. 后端 MySQL 新实现不再面向旧表开发新接口。
4. 全量迁移验收完成后，进入只读冻结期；确认没有依赖后再由单独 migration 删除或归档。

### 2.3 练习/非正式表

- `tb_tasks` 是个人练习任务表。公司正式环境如果没有业务依赖，建议不进入正式库；如果为了 `/health` 或 demo 兼容需要保留，也应标注为非核心表。

### 2.4 必须新增的治理表

正式环境至少应新增：

- `tb_users`
- `tb_roles`
- `tb_user_roles`
- `tb_user_store_scopes`
- `tb_permissions`
- `tb_role_permissions`
- `tb_auth_sessions`
- `tb_audit_logs`
- `tb_asset_objects`，可第一阶段建表但只回填少量样本或暂不启用。
- `tb_asset_access_logs`，用于摄像头截图、设计图/PDF 等敏感资产访问审计。

### 2.5 MySQL schema 草案修订清单

建议下一版 `db/mysql_schema_tb.sql` 或新增 governance DDL 时处理：

- 给 `tb_stores` 增加软删除字段：`deleted_at`、`deleted_by`，正式环境避免物理删除直接级联清空门店资产索引。
- 给 `tb_store_areas` 预留 `external_area_id`，未来对接公司业务系统空间/房间对象。
- 给 `tb_video_channels` 预留 `external_area_id`、`external_bed_id`，或新增独立通道映射表，避免长期扩大 `area_type + area_number + bed_label` 这套临时文本映射。
- 给 `tb_channel_snapshots` 增加 `snapshot_key` 或统一要求 `thumbnail_path/full_image_path` 存 logical key，不存完整 API URL；兼容层可继续把 `/api/store-space/channel-snapshots/{name}.jpg` 转成 `channel-snapshots/{name}.jpg`。
- `tb_channel_snapshots` 最新截图查询索引建议调整为 `(channel_id, created_at, id)`，查询用 `order by created_at desc, id desc` 并实际看执行计划。
- `tb_operation_logs` 保留过渡用途，但新增 `tb_audit_logs` 承接正式审计；后续可以把业务摘要日志和安全审计日志拆开。
- `tb_ezviz_accounts` 的密文字段保留但默认不写敏感密钥；如果正式要求落库，必须先确定 KMS/加密、轮换和审计方案。
- 新增 `tb_users`、`tb_roles`、`tb_user_roles`、`tb_user_store_scopes`、`tb_permissions`、`tb_role_permissions`、`tb_auth_sessions`、`tb_audit_logs`、`tb_asset_objects`、`tb_asset_access_logs`。

## 3. 权限数据模型

### 3.1 用户表

第一版用户唯一身份使用企业邮箱，预留飞书、手机号、展示名、部门和公司 SSO subject。`username/display_name` 可能重名，不能作为权限主键。

```sql
create table tb_users (
  id bigint not null auto_increment,
  email varchar(255) not null,
  username varchar(255) not null default '',
  display_name varchar(255) not null default '',
  feishu_user_id varchar(255) not null default '',
  mobile varchar(64) not null default '',
  department varchar(255) not null default '',
  sso_subject varchar(255) not null default '',
  enabled tinyint(1) not null default 1,
  last_login_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_users_email (email),
  key idx_tb_users_feishu_user_id (feishu_user_id),
  key idx_tb_users_sso_subject (sso_subject)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

登录判断：`email` 命中且 `enabled=1` 才允许登录。离职、禁用、未授权用户都通过 `enabled=0` 或不存在来拒绝。后续如需要更细的生命周期，可再增加 `status`，但第一版避免同时维护两套状态。

### 3.2 角色与权限点

第一版角色先收敛为产品可解释的三类，不直接绑定页面实现细节：

- `admin`：系统管理员。
- `operator`：运营/项目执行人员，具备授权范围内的业务编辑能力。
- `viewer`：只读查看，仍受机构/门店范围约束。

权限点建议稳定命名，避免跟 UI 文案绑定：

- 页面/Tab：`store_space.view`、`store_space.design_plan.view`、`store_space.channels.view`、`h5_monitor.view`。
- 操作：`store.create`、`store.update`、`store.delete`、`design_plan.upload`、`design_plan.annotate`、`recorder.manage`、`channel.scan`、`channel.confirm`、`snapshot.refresh`。
- 敏感能力：`h5_monitor.play_live`、`h5_monitor.playback`、`asset.sensitive.view`、`audit.view`、`permission.manage`。

```sql
create table tb_roles (
  id bigint not null auto_increment,
  code varchar(64) not null,
  name varchar(128) not null,
  description varchar(512) not null default '',
  is_system tinyint(1) not null default 0,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_roles_code (code)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table tb_permissions (
  id bigint not null auto_increment,
  code varchar(128) not null,
  name varchar(128) not null,
  category varchar(64) not null,
  description varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  unique key uq_tb_permissions_code (code),
  key idx_tb_permissions_category (category)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table tb_role_permissions (
  role_id bigint not null,
  permission_id bigint not null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (role_id, permission_id),
  constraint fk_tb_role_permissions_role foreign key (role_id) references tb_roles(id),
  constraint fk_tb_role_permissions_permission foreign key (permission_id) references tb_permissions(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table tb_user_roles (
  user_id bigint not null,
  role_id bigint not null,
  created_at datetime(3) not null default current_timestamp(3),
  created_by bigint null,
  primary key (user_id, role_id),
  constraint fk_tb_user_roles_user foreign key (user_id) references tb_users(id),
  constraint fk_tb_user_roles_role foreign key (role_id) references tb_roles(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

### 3.3 机构/门店范围

SSO 只解决“你是谁”，本项目必须解决“你能看哪些机构/门店”。scope 至少支持 `all` 和 `store`，未来扩展 `city`、`region`、`external_org`。

```sql
create table tb_user_store_scopes (
  id bigint not null auto_increment,
  user_id bigint not null,
  store_id bigint null,
  external_org_id varchar(255) not null default '',
  city varchar(128) not null default '',
  region varchar(128) not null default '',
  scope_type varchar(32) not null default 'store',
  created_at datetime(3) not null default current_timestamp(3),
  created_by bigint null,
  primary key (id),
  unique key uq_tb_user_store_scopes_store (user_id, store_id),
  key idx_tb_user_store_scopes_external_org_id (external_org_id),
  constraint fk_tb_user_store_scopes_user foreign key (user_id) references tb_users(id),
  constraint fk_tb_user_store_scopes_store foreign key (store_id) references tb_stores(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

建议规则：

- `scope_type='all'`：全部门店，`store_id` 为空。
- `scope_type='store'`：指定门店，`store_id` 必填。
- `scope_type='external_org'`：按公司机构 ID 授权，`external_org_id` 必填。
- `scope_type='city'` / `scope_type='region'`：为未来城市/大区权限预留，第一版可不开放配置入口。
- 查询门店、H5 Monitor、设计图和通道时，统一走权限过滤，不能只靠前端隐藏入口。

### 3.4 Session 与审计日志

公司 SSO 通常会在网关注入用户信息或回调后给项目 session。项目侧建议只保存必要 session 元数据，不保存 SSO 原始 token。

```sql
create table tb_auth_sessions (
  id bigint not null auto_increment,
  session_id varchar(128) not null,
  user_id bigint not null,
  sso_subject varchar(255) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  expires_at datetime(3) not null,
  revoked_at datetime(3) null,
  primary key (id),
  unique key uq_tb_auth_sessions_session_id (session_id),
  key idx_tb_auth_sessions_user (user_id, created_at),
  key idx_tb_auth_sessions_expires_at (expires_at),
  constraint fk_tb_auth_sessions_user foreign key (user_id) references tb_users(id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table tb_audit_logs (
  id bigint not null auto_increment,
  user_id bigint null,
  user_email varchar(255) not null default '',
  action varchar(128) not null,
  entity_type varchar(64) not null,
  entity_id bigint null,
  store_id bigint null,
  external_org_id varchar(255) not null default '',
  ip_address varchar(64) not null default '',
  user_agent varchar(512) not null default '',
  request_id varchar(128) not null default '',
  result varchar(32) not null default 'success',
  detail_json json null,
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_audit_logs_user_time (user_id, created_at),
  key idx_tb_audit_logs_store_time (store_id, created_at),
  key idx_tb_audit_logs_action_time (action, created_at)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

审计最低要求：

- 记录登录、登出、权限变更、门店新增/编辑/删除、设计图上传/标注、录像机和通道变更。
- H5 Monitor 播放直播、查询录像、获取回放地址、刷新/查看截图必须记录。
- 日志中禁止写 app secret、access token、Supabase service role key、完整签名播放 URL。

### 3.5 SSO 接入对数据库和审计的影响清单

- `tb_users.email` 是授权主键；SSO payload 缺少 email 时，本项目不能稳定授权，需要安全/SSO 同学补字段或提供稳定映射。
- `tb_users.feishu_user_id`、`username`、`display_name`、`department` 只作为展示和后续扩展，不参与唯一授权判断。
- SSO 登录成功但本项目未授权，应写登录拒绝审计，原因记录为 `user_not_provisioned` 或 `user_disabled`。
- 每次登录成功更新 `tb_users.last_login_at`，并写 `tb_auth_sessions`。
- 权限变更必须写 `tb_audit_logs`，包括操作者、目标用户、角色变化、scope 变化。
- 后端每个敏感接口都要从 session 解析 user，再校验权限点和 scope；H5 Monitor 播放、回放、截图读取不能只依赖 URL 中的 `externalOrgId`。
- 前端可根据权限点控制页面/Tab/按钮显示，但后端拒绝才是最终安全边界。

## 4. 资产迁移数据模型

第一阶段继续 Supabase Storage，数据库保留路径字段：

- `tb_store_design_plans.original_pdf_path`
- `tb_store_design_plans.preview_image_path`
- `tb_store_design_plans.thumbnail_path`
- `tb_channel_snapshots.thumbnail_path`
- `tb_channel_snapshots.full_image_path`

为未来公司文件服务准备 `tb_asset_objects`，建议即使暂不启用，也先确定字段：

```sql
create table tb_asset_objects (
  id bigint not null auto_increment,
  logical_key varchar(1024) not null,
  logical_key_hash char(64) not null default '',
  storage_provider varchar(32) not null default 'supabase',
  bucket varchar(255) not null default '',
  file_id varchar(255) not null default '',
  object_url varchar(1024) not null default '',
  content_type varchar(128) not null default '',
  size_bytes bigint null,
  checksum_sha256 varchar(64) not null default '',
  sensitivity varchar(32) not null default 'internal',
  owner_entity_type varchar(64) not null default '',
  owner_entity_id bigint null,
  migrated_at datetime(3) null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uq_tb_asset_objects_logical_key_hash (logical_key_hash),
  key idx_tb_asset_objects_file_id (file_id),
  key idx_tb_asset_objects_owner (owner_entity_type, owner_entity_id),
  key idx_tb_asset_objects_sensitivity (sensitivity)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

`logical_key_hash` 建议存 `sha256(logical_key)`，避免在 MySQL 8.0.13 上对 `varchar(1024)` 做唯一索引时遇到索引长度和 row format 风险。`logical_key` 仍保留原文，便于排查和回放迁移。

敏感级别建议：`public`、`internal`、`sensitive`。通道截图和监控相关截图默认 `sensitive`，设计图/PDF 至少 `internal`，如包含门店布局、安全区域等也可标为 `sensitive`。

资产访问审计建议独立表：

```sql
create table tb_asset_access_logs (
  id bigint not null auto_increment,
  asset_id bigint null,
  logical_key varchar(1024) not null default '',
  user_id bigint null,
  action varchar(64) not null,
  result varchar(32) not null default 'success',
  ip_address varchar(64) not null default '',
  request_id varchar(128) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  primary key (id),
  key idx_tb_asset_access_logs_asset_time (asset_id, created_at),
  key idx_tb_asset_access_logs_user_time (user_id, created_at)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
```

读取策略建议：

1. 先按 logical key 查 `tb_asset_objects`。
2. 如果存在公司 `file_id`，走公司文件服务。
3. 如果不存在或读取失败，按配置回退 Supabase Storage。
4. 回退成功后可异步上传公司文件服务并回填映射。
5. 前端不拿真实文件服务地址，继续访问 Go 后端接口，由服务端完成鉴权、读取和审计。

## 5. 样本迁移方案

样本先选 1-2 家门店，不做全量切换。样本必须覆盖：

- 有 `external_org_id` 的门店。
- 有设计图 PDF、预览图、缩略图。
- 有设计图标注框。
- 有录像机、萤石账号绑定、通道、通道截图。
- 有 H5 Monitor 可播放通道。
- 有 AI 识别 JSON 结果。
- 有 `bed_label` 的治疗室/VIP 治疗室/美容室床位拆分。
- 有操作日志更好。

迁移步骤建议：

1. 冻结样本门店写入窗口，记录源库快照时间。
2. 从 PostgreSQL/Supabase 只读导出样本数据，按依赖顺序导出：门店、区域、设计图、标注、萤石账号、录像机、通道、截图、操作日志。
3. MySQL 导入前关闭应用写入，不建议全局关闭外键；如必须临时 `SET FOREIGN_KEY_CHECKS=0`，只能在一次性导入事务/会话内使用，并在导入后立即恢复和做孤儿检查。
4. 尽量保留原始 `id`，`insert` 时显式写入主键。
5. 导入后逐表修正 `auto_increment`，确保大于当前最大 `id`。
6. JSON 字段导入前用源库和脚本双重校验，空值用 `null`，不要写空字符串。
7. 时间字段统一按 UTC 或明确的 Asia/Shanghai 规则转换。建议 MySQL 存 `datetime(3)` 时用 UTC，并在应用层明确转换展示。
8. 图片路径字段保持 logical key 不变，例如 `uploads/{upload_id}/preview.png`、`channel-snapshots/{name}.jpg`。
9. 跑数据校验 SQL、应用只读接口验收，再开放样本写入。

导入后检查：

```sql
select count(*) from tb_stores;
select max(id) from tb_stores;
select count(*) from tb_store_areas where store_id not in (select id from tb_stores);
select count(*) from tb_store_design_plans where store_id not in (select id from tb_stores);
select count(*) from tb_design_plan_annotations a
left join tb_store_design_plans p on p.id = a.design_plan_id
left join tb_store_areas ar on ar.id = a.area_id
where p.id is null or ar.id is null;
select count(*) from tb_video_recorders where store_id not in (select id from tb_stores);
select count(*) from tb_video_channels where recorder_id not in (select id from tb_video_recorders);
select count(*) from tb_channel_snapshots where channel_id not in (select id from tb_video_channels);
```

`auto_increment` 修正应由迁移脚本根据实际最大值生成，例如：

```sql
select concat('alter table tb_stores auto_increment = ', coalesce(max(id), 0) + 1, ';') from tb_stores;
```

## 6. MySQL 8.0.13 风险清单

- `CHECK` 约束风险：MySQL 8.0.13 解析但不强制执行 `CHECK`。当前 DDL 中大量状态枚举、坐标范围依赖 `CHECK`，必须在 Go 层 validation、迁移脚本校验和测试中兜住。
- `sql_mode` 为空风险：测试库已探测到 `sql_mode` 为空，非法日期、截断字符串、隐式类型转换可能静默发生。建议应用连接设置严格 session `sql_mode`，同时推动测试库/正式库调整为至少 `STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO`。
- `datetime` 时区风险：测试库已探测到 `time_zone=+08:00`。MySQL `datetime` 不带时区，PostgreSQL `timestamptz` 迁移时最容易差 8 小时。必须明确“存 UTC、展示按 Asia/Shanghai”或公司统一规范；迁移脚本不能靠本机时区隐式转换。
- JSON 兼容风险：PostgreSQL `jsonb` 到 MySQL `json` 不保留完全相同的索引能力和比较语义。第一阶段只做存取，不做复杂 JSON 查询。
- 唯一索引与空值风险：MySQL unique index 允许多个 `null`。涉及可空字段唯一约束时必须确认预期。
- 字符集和排序风险：`utf8mb4_unicode_ci` 默认大小写不敏感，`normalized_name` 已规避一部分，但邮箱、编码类字段是否大小写敏感要单独确认。
- 外键删除风险：当前多处 `on delete cascade`，删除门店会级联删除区域、设计图、录像机、通道、截图记录。正式环境应把删除改为软删除或增加删除审批/审计，避免误删敏感数据索引。
- 索引方向风险：MySQL 8.0 支持降序索引，但当前 DDL 多数是普通 `(id, created_at)`。查询最新截图、日志倒序分页时建议用 `(channel_id, created_at, id)`，并用 `order by created_at desc, id desc` 验证执行计划。
- 文本索引长度风险：`varchar(1024)` 不适合频繁精确索引。`logical_key` 如要唯一，需确认公司 MySQL row format 和索引长度限制；必要时增加 `logical_key_hash char(64)` 唯一索引。
- 密钥落库风险：`tb_ezviz_accounts` 当前有 `app_secret_ciphertext`、`access_token_ciphertext` 字段，但现有决策是密钥来自运行时 Secret。正式环境如果要落库，必须先确定 KMS/加密方案、轮换、访问审计；否则字段保留空值，不写密钥。
- 多 Pod 并发风险：H5 Monitor 当前并发控制是进程内内存计数，多 Pod 下不准确。正式化需要 Redis 或 `tb_auth_sessions`/专门播放租约表。

## 7. 只读检查命令建议

如果后续需要连接测试 MySQL，只做只读检查时建议使用低权限账号，权限范围：

- `select` on 目标 schema。
- `show view` 可选。
- 不需要 `insert/update/delete/drop/alter`。

示例命令不要把密码写入命令、文件或日志：

```sh
mysql --host "$MYSQL_HOST" --port "$MYSQL_PORT" --user "$MYSQL_USER" --password --database "$MYSQL_DATABASE" --execute "select version(), @@sql_mode, @@time_zone, @@system_time_zone;"
```

结构检查：

```sql
select table_name
from information_schema.tables
where table_schema = database()
order by table_name;

select table_name, column_name, column_type, is_nullable, column_default
from information_schema.columns
where table_schema = database()
order by table_name, ordinal_position;
```

## 8. 主会话验收建议

验收时不需要先写 MySQL repository，可以先确认这些问题：

1. 是否接受“旧设计图表只迁移兼容，不继续扩展”的方向。
2. 是否接受第一阶段图片/PDF/截图继续 Supabase Storage，只迁数据库路径字段。
3. 公司正式权限是否以“SSO 身份 + 本项目角色 + 门店/机构范围 + 权限点”为边界。
4. 摄像头截图是否全部按敏感资产处理，并在访问时写 `tb_asset_access_logs` 或 `tb_audit_logs`。
5. 样本迁移门店名单是否覆盖 H5 Monitor、设计图、通道截图和 AI JSON。
6. 公司 MySQL 8.0.13 的 `sql_mode`、时区、字符集、外键策略是否能由运维确认。

通过上述验收后，下一步再拆实施：

1. 新增 `db/mysql_governance_schema_tb.sql`，放用户、角色、权限、资产映射和审计表。
2. 新增样本迁移脚本，只处理 1-2 家门店。
3. 新增 MySQL repository 或抽象 SQL 方言。
4. 在测试 MySQL 上只读校验和小样本写入验证。
