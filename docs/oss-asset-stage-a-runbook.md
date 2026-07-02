# OSS 资产迁移 Stage A 执行手册

最后更新：2026-07-02

## 目标

把二壮项目里的设计图文件、设计图预览图、通道截图从历史存储迁移到公司 OSS，同时保持前端和接口路径稳定。Stage A 只做测试库和样本门店验证，不发布到公司线上，不改正式数据。

样本门店先使用 `external_org_id = 10030`。

## 分工

- 主会话：负责 OSS provider 代码、smoke 工具、发布决策和最终验收。
- DBA 专项：负责 MySQL DDL、迁移清单 SQL、校验 SQL、样本 dry-run 执行记录。
- 运维/安全：负责 K8s Secret、OSS 内网访问、正式环境发布窗口。

## 禁止事项

- 不把 OSS AK/SK 写入仓库、SQL、文档、日志、Dockerfile、前端变量或命令历史。
- 不直接把 OSS bucket 开公网读。
- 不在未完成样本验证前切换 `ASSET_STORE=oss`。
- 不在未确认回滚方案前删除历史 Supabase/local 对象。

## 当前交付物

- `internal/assets/oss.go`：OSS asset store provider。
- `cmd/oss-smoke/main.go`：OSS 连通性 smoke 工具，默认 dry-run。
- `db/oss_asset_schema_patch_tb.sql`：`tb_asset_objects` 迁移追踪字段补丁。
- `db/oss_asset_inventory_sql_tb.sql`：历史对象引用清单。
- `db/oss_asset_validation_sql_tb.sql`：迁移后校验 SQL。
- `docs/oss-dba-migration-plan.md`：DBA 方案说明。

## 执行顺序

1. 确认测试库结构已完成业务表和治理表初始化：
   - `db/mysql_schema_tb.sql`
   - `db/mysql_business_schema_patch_tb.sql`
   - `db/mysql_governance_schema_tb.sql`
2. 执行前检查 `tb_asset_objects` 当前字段和索引：
   - 使用 `db/oss_asset_schema_patch_tb.sql` 文件头里的 `information_schema` 查询。
3. 执行 `db/oss_asset_schema_patch_tb.sql`。
   - 该文件使用动态 SQL 和 `information_schema` 检查，适合测试库重复核验。
4. 执行迁移清单 dry-run：
   - `db/oss_asset_inventory_sql_tb.sql`
   - 先看样本门店 `10030` 的结果。
5. 用迁移程序或临时受控脚本复制样本门店对象到 OSS。
   - 第一阶段 `target_oss_key = logical_key`。
   - 成功后 upsert `tb_asset_objects`：
     - `storage_provider = 'oss'`
     - `bucket = 'sy-camera-erzhuang-project'`
     - `storage_key = logical_key`
     - `storage_key_hash = sha2(logical_key, 256)`
     - `migration_status = 'migrated'`
6. 执行 `db/oss_asset_validation_sql_tb.sql`。
7. 主会话通过后端代理路径验证图片/PDF 是否仍能显示。

## OSS Smoke 验证

默认命令只 dry-run，不写 OSS：

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/oss-smoke-check ./cmd/oss-smoke
/private/tmp/oss-smoke-check
```

真实写入验证必须由主会话或用户明确确认后执行，并且只允许通过环境变量注入密钥。不要把密钥拼进命令行。

```bash
ASSET_STORE=oss \
OSS_BUCKET=sy-camera-erzhuang-project \
OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com \
OSS_ACCESS_KEY_ID=<from secret> \
OSS_ACCESS_KEY_SECRET=<from secret> \
/private/tmp/oss-smoke-check --apply
```

本机当前 Go 产物执行可能出现 `dyld missing LC_UUID`，因此真实 smoke 更适合在公司容器、Linux 测试机或修复本机 Go runtime 后执行。

## Manifest 迁移工具

`cmd/asset-migrate` 用于读取 `db/oss_asset_inventory_sql_tb.sql` 导出的 CSV 清单，并把对象从源存储复制到目标 OSS。它默认 dry-run，不写目标存储；只有显式传 `--apply` 才会复制对象。

构建：

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/asset-migrate-check ./cmd/asset-migrate
```

样本 dry-run：

```bash
/private/tmp/asset-migrate-check \
  --manifest /path/to/oss-inventory-10030.csv \
  --external-org-id 10030 \
  --max-rows 20
```

源存储和目标存储使用独立前缀，避免迁移时无法表达“从旧存储读、写到 OSS”：

```text
SOURCE_ASSET_STORE=local|supabase|oss
SOURCE_UPLOAD_DIR=<local root>
SOURCE_SUPABASE_URL=<source supabase url>
SOURCE_SUPABASE_SERVICE_ROLE_KEY=<source secret>
SOURCE_SUPABASE_STORAGE_BUCKET=<source bucket>

TARGET_ASSET_STORE=oss
TARGET_OSS_BUCKET=sy-camera-erzhuang-project
TARGET_OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
TARGET_OSS_ACCESS_KEY_ID=<from secret>
TARGET_OSS_ACCESS_KEY_SECRET=<from secret>
```

真实复制样本门店时，先限制 `--external-org-id 10030` 和较小的 `--max-rows`。确认输出 CSV 中 `action=copied` 且没有 `failed` 后，再扩大范围。该工具当前只负责对象复制，不直接回写 MySQL；数据库状态回写在样本复制验证通过后再加。

## 验收口径

样本门店 `10030` 通过条件：

- inventory SQL 能列出设计图和通道截图引用。
- `http(s)` 临时 URL 被标记为 `skipped`，没有被误迁移。
- OSS smoke `--apply` 能完成写入、读取、删除。
- 样本对象复制到 OSS 后，`tb_asset_objects` 中记录完整：
  - `logical_key`
  - `storage_provider`
  - `bucket`
  - `storage_key`
  - `storage_key_hash`
  - `proxy_path`
  - `migration_status`
- 后端代理 URL 不变，前端图片/PDF 原位置可显示。
- `db/oss_asset_validation_sql_tb.sql` 不出现：
  - migrated 但 storage_key 为空。
  - migrated 但 proxy_path 为空。
  - 非预期 duplicate target key。
  - 样本门店 failed 记录。

## 停止条件

出现以下任一情况，立即停止迁移，不扩大范围：

- OSS smoke 失败，且错误不是明确的网络/权限配置问题。
- inventory 中同一 logical_key 映射到多个不同业务归属，且无法解释。
- 样本门店代理路径出现图片裂图或 PDF 无法打开。
- validation SQL 出现 migrated 记录缺少 `storage_key`、`bucket` 或 `proxy_path`。
- 迁移程序日志出现 AK/SK、签名串、Authorization 等敏感字段。

## 回滚策略

Stage A 不删除历史对象，不改前端路径，因此回滚优先级如下：

1. 运行时把 `ASSET_STORE` 从 `oss` 切回旧 provider。
2. 将样本 `tb_asset_objects.migration_status` 标记为 `failed` 或恢复为 `pending`。
3. 保留 OSS 已迁移对象，待确认后再清理，不急删。
4. 如果 DDL patch 需要回滚，先停写并备份，再由 DBA 生成反向 migration；不要手工临场 drop 字段。

## 待确认项

- 公司 K8s Secret 的变量名是否与当前代码一致。
- 迁移程序由主仓库新增 CLI 实现，还是 DBA 用一次性脚本执行。
- 全量迁移前是否需要按资产敏感级别分批：先设计图，再通道截图。

## 2026-07-02 测试库 Stage A 记录

已在 MySQL 测试库完成 Stage A 最小初始化，用于验证 OSS inventory 链路：

- 测试库初始状态没有 `external_org_id = 10030` 的门店，也没有任何设计图/截图资产引用。
- 初始 governance 表缺失，已执行治理表 schema 和 OSS asset schema patch。
- 执行 business patch 时发现测试库当前 schema 还缺两个线上代码已使用字段：
  - `tb_video_channels.bed_label`
  - `tb_stores.short_name`
- 已在测试库临时补齐上述字段后，Stage A seed 成功。
- 样本门店 `10030` inventory 导出 2 行，均为 `pending`，无 skipped。
- 2 行来自同一个截图对象的 `full_image_path` / `thumbnail_path` 双引用，`logical_key_hash` 相同，`logical_key_rank` 分别为 1 和 2。迁移工具应只复制 rank=1，rank=2 作为重复引用跳过。

待 DBA 专项修正：

- 将 `bed_label` 和 `short_name` 纳入正式 MySQL schema/patch 执行包，避免测试库重建后再次手工补字段。
- `db/oss_asset_inventory_sql_tb.sql` 当前依赖 `tb_channel_snapshots.snapshot_key`；若目标库尚未执行 business patch，会报字段不存在。执行顺序必须保持：business patch 先于 inventory。
