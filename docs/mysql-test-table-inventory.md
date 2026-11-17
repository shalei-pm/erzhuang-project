# 测试环境 MySQL 表清单

最后更新：2026-09-01  
用途：记录二壮项目测试库表的来源、归属、DDL 和正式环境同步状态。

## 状态口径

- **已记录/需复核**：仓库或历史交接记录表明已执行过，正式同步前仍以目标库 `information_schema` 为准。
- **应用幂等确保**：应用运行时会在缺失时确保表存在；正式环境应改为运维预建并做结构核对。
- **DDL 待执行**：仓库有 DDL，但没有将其当作测试库已创建的证据。
- **业务同步表**：由运维从业务库同步，二壮只读，不归二壮创建和维护。
- **废止未创建**：历史方案已废止，不应执行或同步。

## 二壮基础业务表

以下 14 张表由 `db/mysql_schema_tb.sql` 定义，历史交接记录标记为测试库已创建。正式环境同步前仍需逐表核对结构和字段补丁：

| 表名 | 用途 | 来源/备注 | 正式环境 |
| --- | --- | --- | --- |
| `tb_tasks` | 任务示例 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_app_settings` | 应用设置 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_design_plan_stores` | 设计图门店 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_design_plan_store_areas` | 设计图区域 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_design_plan_operation_logs` | 设计图操作日志 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_stores` | 二壮门店 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_store_areas` | 二壮空间区域 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_store_design_plans` | 门店设计图 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_design_plan_annotations` | 设计图标注 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_ezviz_accounts` | 萤石账号 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_video_recorders` | 录像机 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_video_channels` | 视频通道 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_channel_snapshots` | 通道截图 | `mysql_schema_tb.sql` | 待运维核对 |
| `tb_operation_logs` | 过渡操作日志 | `mysql_schema_tb.sql` | 待运维核对 |

已知字段补丁：`tb_stores.short_name`、`tb_video_channels.bed_label`。它们是字段变更，不是新增表。

## 治理与权限表

这些表由 `db/mysql_governance_schema_tb.sql` 定义，历史 Stage A 记录表明治理表已在测试库执行过；当前清单将其标为“已记录/需复核”，正式同步前统一核对。

| 表名 | 用途 | 正式环境 |
| --- | --- | --- |
| `tb_users` | SSO 用户 | 待运维核对 |
| `tb_roles` | 角色 | 待运维核对 |
| `tb_permissions` | 权限 | 待运维核对 |
| `tb_user_roles` | 用户角色关系 | 待运维核对 |
| `tb_role_permissions` | 角色权限关系 | 待运维核对 |
| `tb_user_store_scopes` | 用户门店范围 | 待运维核对 |
| `tb_user_resource_scopes` | 通用资源范围 | 待运维核对，应用有幂等确保逻辑 |
| `tb_auth_sessions` | 登录会话与空闲超时 | 本需求验收后同步 |
| `tb_audit_logs` | 安全审计日志 | 待运维核对 |
| `tb_asset_objects` | 资产对象台账 | 待运维核对 |
| `tb_asset_access_logs` | 资产访问审计 | 待运维核对 |

`tb_audit_logs.actor_display_name` 是后续增加的字段，不是新增表；测试环境已执行过字段补充，正式环境需单独提交字段 DDL。

## 破坏性 migration 集成测试门禁

`TestMySQLAuthSessionMigrationIntegration` 会清理并重建 `tb_auth_sessions`，默认跳过。只有同时满足以下条件才允许继续：

- `MYSQL_TEST_DSN_ALLOW_MIGRATION=1`
- `MYSQL_TEST_DSN` 解析后主机必须精确为 `polar-dev.rwlb.rds.aliyuncs.com:3306`
- `MYSQL_TEST_MIGRATION_DATABASE` 必须与 DSN 解析出的库名完全一致，且库名以 `_migration_test` 结尾
- `MYSQL_TEST_MIGRATION_INSTANCE_ID` 必须提供，格式为 `@@hostname:@@port`

测试连接建立后会先查询 `@@hostname` 和 `@@port`，只有与 `MYSQL_TEST_MIGRATION_INSTANCE_ID` 完全匹配，才会执行任何 DROP。缺少配置、主机/端口不符、实例身份不符或查询失败都会在破坏性操作前跳过。本文不保存 DSN、密码、Token 或实例真实值。

真实 MySQL 的并发、过期、撤销和不存在会话状态验证属于隔离迁移库的环境验证项；未配置并通过上述门禁前，不使用共享库补测。

## 业务同步表

以下表由运维从业务侧同步到 `db_pm_erzhuang`，不是二壮新增表，代码只读：

- `tb_crm_admin_tenant`
- `tb_crm_iot_device`
- `tb_crm_consulting_room`
- `tb_crm_iot_area_device_relation`

其结构和数据由业务/运维负责，二壮不得在启动时创建、修改或写入。

## 明确未创建的表

- `tb_nvr_camera_snapshots`：NVR 缩略图回填的数据库方案已废止，当前缩略图只写既有私有 OSS 确定性对象，不需要测试或正式环境建表。

## 正式环境同步前核对 SQL

运维或 DBA 应在目标库执行只读核对，确认表和关键字段：

```sql
select table_name
from information_schema.tables
where table_schema = database()
  and table_name in (
    'tb_auth_sessions', 'tb_audit_logs', 'tb_users',
    'tb_user_resource_scopes', 'tb_asset_objects', 'tb_asset_access_logs'
  )
order by table_name;

select table_name, column_name, column_type, is_nullable, column_default
from information_schema.columns
where table_schema = database()
  and table_name = 'tb_auth_sessions'
order by ordinal_position;
```

本清单不保存账号、密码、Token、连接串或 OSS 密钥。正式环境执行状态由运维补充，不由代码发布自动推断。
