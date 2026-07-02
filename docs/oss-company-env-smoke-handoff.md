# OSS 公司环境 Smoke Handoff

最后更新：2026-07-02

## 目的

验证公司运行环境是否能访问 OSS 内网 endpoint，并验证对象存储账号对 bucket 具备最小读写删权限。该步骤通过后，才允许进入样本对象迁移；该步骤未通过前，不切换 `ASSET_STORE=oss`，不执行全量数据迁移。

## 当前阻塞

本机访问以下内网 endpoint 超时：

```text
sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
```

因此 smoke 必须在公司 K8s / 阿里云 VPC 内执行，或由运维确认可临时使用外网 endpoint。

## 需要注入的环境变量

密钥必须来自 K8s Secret 或受控环境变量，不要写入镜像、仓库、日志或命令历史。

```text
ASSET_STORE=oss
OSS_BUCKET=sy-camera-erzhuang-project
OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
OSS_ACCESS_KEY_ID=<from secret>
OSS_ACCESS_KEY_SECRET=<from secret>
```

## Smoke 命令

在公司环境构建或进入已构建容器后执行：

```bash
go build -o /tmp/oss-smoke-check ./cmd/oss-smoke
/tmp/oss-smoke-check --apply
```

成功输出应类似：

```text
OSS smoke apply ok; key=smoke-tests/... bytes=... content_type=...
```

成功标准：

- 能 PUT 一个 `smoke-tests/*.txt` 文本对象。
- 能 GET 回同一对象，内容一致。
- 能 DELETE 该对象。
- 输出不包含 AK/SK、Authorization、签名串。

失败时请回传：

- 命令输出。
- 当前使用的是内网 endpoint 还是外网 endpoint。
- 容器/Pod 是否在可访问 OSS 内网的 VPC/网络内。
- 不要回传 AK/SK。

## 样本迁移 Dry-Run

OSS smoke 通过后，才执行样本迁移 dry-run。dry-run 不写 OSS。

准备：

1. 从测试库导出 `external_org_id=10030` 的 inventory CSV。
2. 将 CSV 放入容器，例如 `/tmp/oss-inventory-10030.csv`。
3. 配置源存储和目标 OSS 的 env。源存储可能是 `local`、`supabase` 或后续实际来源。

命令：

```bash
go build -o /tmp/asset-migrate-check ./cmd/asset-migrate
/tmp/asset-migrate-check \
  --manifest /tmp/oss-inventory-10030.csv \
  --external-org-id 10030 \
  --max-rows 20
```

成功标准：

- 输出 CSV 中至少存在 `action=would_copy`。
- 重复引用应被标记为 skipped，例如 `duplicate_logical_key`。
- 不出现 `failed`。

## 样本迁移 Apply

仅当以下条件都满足，才允许执行样本 apply：

- OSS smoke `--apply` 通过。
- 样本 dry-run 输出符合预期。
- 源存储配置已确认可读取真实对象。
- 主会话确认可以写入 OSS 样本对象。

命令：

```bash
/tmp/asset-migrate-check \
  --manifest /tmp/oss-inventory-10030.csv \
  --external-org-id 10030 \
  --max-rows 20 \
  --apply
```

样本 apply 通过后，主会话再补数据库状态回写流程：`tb_asset_objects.storage_provider='oss'`、`bucket`、`storage_key`、`storage_key_hash`、`migration_status='migrated'`。

## 何时进入数据迁移

这里的数据迁移分三层：

1. **Stage A 样本迁移**：OSS smoke 通过后即可进入，只迁 `10030` 样本对象。
2. **Stage B 历史资产迁移**：样本迁移、代理访问、validation SQL 均通过后进入，迁设计图和截图对象。
3. **业务数据库历史迁移**：MySQL schema、权限、资产映射、回滚方案、冻结窗口都确认后进入，和 OSS 对象迁移分开排期。

当前状态：可以准备 Stage A 样本迁移，但还不能执行，因为 OSS 内网 smoke 尚未在公司环境通过。
