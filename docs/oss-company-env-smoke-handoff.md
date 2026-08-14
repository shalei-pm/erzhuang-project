# OSS 公司环境 Smoke Handoff

最后更新：2026-07-02

## 目的

验证公司运行环境是否能访问 OSS 内网 endpoint，并验证对象存储账号对 bucket 具备最小读写删权限。该步骤通过后，才允许进入样本对象迁移；该步骤未通过前，不切换 `ASSET_STORE=oss`，不执行全量数据迁移。

## 当前状态

本机访问以下内网 endpoint 超时：

```text
sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
```

因此 smoke 必须在公司 K8s / 阿里云 VPC 内执行，或由运维确认可临时使用外网 endpoint。

2026-07-02 已通过应用内受控入口完成公司 Pod smoke：

```json
{
  "ok": true,
  "key": "smoke-tests/20260702T121916Z-715bab6dab3c.txt",
  "bytes": 48,
  "content_type": "text/plain; charset=utf-8"
}
```

结论：公司运行时 Pod 可以访问 OSS 内网 endpoint，运行时变量已注入，bucket 权限支持 PUT/GET/DELETE。可以进入 Stage A 样本对象迁移准备，但仍不能切换全局 `ASSET_STORE=oss`。

## 需要注入的环境变量

密钥必须来自 K8s Secret 或受控环境变量，不要写入镜像、仓库、日志或命令历史。

```text
ASSET_STORE=oss
OSS_BUCKET=sy-camera-erzhuang-project
OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
OSS_ACCESS_KEY_ID=<from secret>
OSS_ACCESS_KEY_SECRET=<from secret>
```

如果通过 GitLab CI/CD Variables 注入到公司 K8s 运行时，当前公司链路通常需要使用 `K8S_SECRET_` 前缀：

```text
K8S_SECRET_ASSET_STORE=oss
K8S_SECRET_OSS_BUCKET=sy-camera-erzhuang-project
K8S_SECRET_OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
K8S_SECRET_OSS_ACCESS_KEY_ID=<from secret>
K8S_SECRET_OSS_ACCESS_KEY_SECRET=<from secret>
K8S_SECRET_OPS_ENABLED=true
```

## Smoke 方式 A：应用内受控入口

推荐优先使用该方式，因为请求会在公司已部署应用 Pod 内执行，验证的是实际运行环境到 OSS 内网 endpoint 的访问能力。

入口：

```text
POST /api/admin/ops/oss-smoke
```

保护规则：

- `OPS_ENABLED=true` 时才开放；未开启时返回 404。
- 需要管理员权限，即 `user:manage`。
- 只写入、读取并删除一个 `smoke-tests/*.txt` 文本对象。
- 返回结果会脱敏，不返回 AK/SK、Authorization、Signature、StringToSign。
- 不迁移业务图片，不写数据库。

触发方式：

1. 发布包含该入口的版本到公司环境。
2. 在 GitLab Variables 配置上面的 `K8S_SECRET_*` 变量。
3. 用管理员账号登录项目。
4. 通过已登录浏览器上下文或受控接口客户端发起 `POST` 请求。

成功返回应类似：

```json
{
  "ok": true,
  "key": "smoke-tests/...",
  "bytes": 48,
  "content_type": "text/plain; charset=utf-8"
}
```

失败时请回传接口返回 JSON、当前使用 endpoint 类型、变量是否已注入运行环境。不要回传 AK/SK。

## Smoke 方式 B：容器内命令

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

### 方式 A：应用内受控入口

推荐优先使用该方式。请求在公司应用 Pod 内执行，复用已验证通过的运行环境；接口只迁用户提交的 inventory CSV，不查询数据库、不写数据库。

入口：

```text
POST /api/admin/ops/asset-migrate
```

保护规则：

- `OPS_ENABLED=true` 或 `K8S_SECRET_OPS_ENABLED=true` 时才开放。
- 需要管理员权限，即 `user:manage`。
- 请求体最多 2MB。
- 默认 `external_org_id=10030`，默认 `max_rows=20`。
- `apply=true` 目前只允许 `external_org_id=10030`。
- dry-run 不写 OSS。
- apply 只复制对象到 OSS，并返回待人工审查的 `result_sql`；接口不直接写 MySQL。

请求示例：

```js
fetch('/erzhuang-project/api/admin/ops/asset-migrate', {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    manifest_csv: '<inventory csv text>',
    external_org_id: '10030',
    max_rows: 20,
    apply: false
  })
}).then(async r => ({ status: r.status, body: await r.json() })).then(console.log)
```

dry-run 成功后，才允许把 `apply` 改为 `true` 并附加批次号：

```js
fetch('/erzhuang-project/api/admin/ops/asset-migrate', {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    manifest_csv: '<inventory csv text>',
    external_org_id: '10030',
    max_rows: 20,
    apply: true,
    batch_id: 'stage-a-10030-oss-smoke'
  })
}).then(async r => ({ status: r.status, body: await r.json() })).then(console.log)
```

返回中的 `result_sql` 只会更新已存在的 `tb_asset_objects` 记录。执行前必须先确认样本 logical key 已有 pending 记录，否则 SQL 可能影响 0 行。

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
  --apply \
  --result-sql /tmp/oss-inventory-10030-result.sql \
  --batch-id stage-a-10030-oss-smoke
```

样本 apply 通过后，回传：

- `asset-migrate` 输出 CSV。
- `/tmp/oss-inventory-10030-result.sql`。

主会话审查 `result.sql` 后，再决定是否执行数据库状态回写。该 SQL 只会对 `action=copied` 的对象生成 `migration_status='migrated'` 更新，不会把 skipped/failed 行标记成功。

## 何时进入数据迁移

这里的数据迁移分三层：

1. **Stage A 样本迁移**：OSS smoke 通过后即可进入，只迁 `10030` 样本对象。
2. **Stage B 历史资产迁移**：样本迁移、代理访问、validation SQL 均通过后进入，迁设计图和截图对象。
3. **业务数据库历史迁移**：MySQL schema、权限、资产映射、回滚方案、冻结窗口都确认后进入，和 OSS 对象迁移分开排期。

当前状态：OSS 内网 smoke 已在公司环境通过，可以准备 Stage A 样本迁移。下一步是导出 `external_org_id=10030` 的 inventory CSV，先通过应用内受控入口 dry-run，再做样本 apply。
