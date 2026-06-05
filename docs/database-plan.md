# Database Plan

最后更新：2026-06-05

## 当前结论

本项目当前数据库方案采用 Supabase PostgreSQL。

当前状态：已完成服务器接入和公网验证。

原因：

- 适合个人练习项目，创建快，运维负担低。
- 不需要在 2GB 腾讯云 Lighthouse 上额外安装和维护 MySQL。
- 后端仍然可以练习真实数据库连接、SQL、迁移、配置管理和发布验证。
- Supabase 底层是 PostgreSQL，工程实践上比本地文件型数据库更接近线上后端服务。

## 使用边界

当前阶段 Supabase 只用于个人练习项目：

- 不接入公司生产环境。
- 不导入公司业务数据。
- 不使用公司密钥、公司账号、公司代码。
- 不把数据库密码、连接串、API Key 提交到 GitHub。

## 连接方式

Go 后端只通过环境变量读取数据库配置。

建议环境变量：

```text
DATABASE_URL=postgres://...
```

本地开发时可以放在本机未提交的环境文件里；服务器发布时写入 systemd 的受控环境配置中。

`.gitignore` 应覆盖本地密钥文件，例如：

```text
.env
.env.*
```

## Supabase 连接选择

Supabase 支持 Direct connection 和 Pooler connection。

对本项目当前阶段：

- 优先选择 Supabase Dashboard 中提供的 pooled connection string。
- 如果腾讯云 Lighthouse 所在网络不能访问 direct IPv6 地址，就使用 Supabase pooler 的 IPv4 连接。
- 后端服务是常驻 Go 进程，后续需要在 Go 里配置合理的连接池参数。

## 后端接入计划

第一阶段：只做连接验证。

1. 已新增数据库配置读取逻辑：`DATABASE_URL`。
2. 已新增 `GET /health` 中的数据库状态字段：`database`。
3. 已通过服务器 systemd 环境变量验证连接。

第二阶段：把 `/api/tasks` 从内存假数据改成数据库读取。

1. 已建表：`tasks`。
2. 已写入少量练习数据。
3. 已将后端查询数据库返回任务列表。
4. 已保留内存 store，方便本地无数据库时开发和测试。

第三阶段：引入迁移机制。

候选方案：

- 简单 SQL 文件：适合当前学习阶段。
- goose / migrate：适合后续更真实的发布流程。

## 需要用户提供的信息

创建 Supabase 项目后，需要给 Codex 的信息只包括连接所需内容。

推荐提供：

- Project URL 或 project ref
- Database pooled connection string
- 数据库密码
- Supabase 项目所在 region

注意：

- 不要把这些信息发给前端专项会话。
- 不要提交到仓库。
- 如果需要我配置服务器，我会在操作前确认目标服务器和具体动作。

## 当前服务器配置

服务器通过 systemd 环境文件读取数据库连接串：

```text
/etc/erzhuang-project.env
```

文件权限：

```text
root root 600
```

systemd service 中配置：

```text
EnvironmentFile=/etc/erzhuang-project.env
```

该文件不进入 GitHub。

## 验证结果

公网健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

公网任务接口已返回 4 条数据库任务，其中包括：

```json
{"id":4,"title":"接入 Supabase PostgreSQL","done":false}
```

## 当前不采用的方案

暂不在 Lighthouse 上安装 MySQL。

原因：

- 2GB 内存可以跑 MySQL，但会增加内存、备份、权限、升级、安全和故障排查成本。
- 当前目标是练习 Codex + Go + GitHub + 部署闭环，托管 PostgreSQL 更适合先把主链路跑通。

暂不使用公司线上数据库。

原因：

- 即使是空白库，也会引入公司网络、权限、审计和边界问题。
- 等个人链路成熟后，再向研发申请更规范的公司环境权限更合适。
