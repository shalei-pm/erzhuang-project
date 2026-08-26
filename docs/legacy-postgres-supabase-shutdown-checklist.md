# 旧 PostgreSQL/Supabase 下线确认清单

最后更新：2026-07-06

本文用于规划旧 PostgreSQL/Supabase 数据源和 Supabase Storage 的归档、保留、删除和验证。它不是执行脚本，也不是授权删除指令。任何真实删除、断网、撤密钥、清 bucket 或清库动作，都必须经过产品负责人、公司安全/运维、相关研发/DBA 明确确认。

## 当前事实

- 公司运行时已切换为 MySQL + OSS。
- 最新健康检查口径：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"mysql","asset_store":"oss"}
```

- `2.30.23` 起，运行时代码已删除 PostgreSQL runtime、pgx 依赖、pg-mysql 迁移入口和旧库回滚连接。
- 当前有效业务门店口径为 54 家有 `external_org_id` 的门店。
- 第 55 家空 `external_org_id` 门店“新氧青春诊所(长沙北辰荟店)”不迁移。
- 用户已在 2026-07-06 删除旧 Supabase 数据库；删除后线上核心回归已通过。

## 下线目标

- 防止线上服务误连旧 PostgreSQL/Supabase。
- 防止敏感经营现场图片、PDF、截图继续长期保存在外部旧存储。
- 保留必要审计、问题追溯和合规证据。
- 明确责任人、时间点、验证方式和回退边界。

## 责任人与确认项

| 角色 | 需要确认 |
| --- | --- |
| 产品负责人 | 54 家门店口径、第 55 家不迁移、业务验收完成、旧系统只读期是否满足业务追溯 |
| 安全/合规 | 旧数据源保留期、删除方式、日志留存、数据出海风险是否解除 |
| 运维/K8s | 旧环境变量、旧 Secret、网络出站、bucket 权限、DNS/代理配置清理方式 |
| DBA/研发 | MySQL 数据完整性、迁移校验、旧表/旧库冻结、删除前备份和恢复方式 |
| 主会话 | 文档、验证清单、发布状态、风险边界和执行记录归档 |

## 删除前必须完成

- [x] 公司线上 `/health` 连续确认 `database=mysql`、`asset_store=oss`。
- [x] 门店列表总数确认 54，且业务接受 54 家有效机构门店口径。
- [x] 关键门店 H5 Monitor 可打开并播放/回放。
- [x] 门店后台读写验收通过：创建、编辑、删除临时门店、保存设计图、添加/删除录像机。
- [x] 萤石扫描、单通道截图刷新、单通道 AI 识别、通道确认流程抽验通过。
- [x] OSS 对象台账和代理访问抽验通过。
- [ ] PostgreSQL/Supabase 旧数据做只读备份或归档，归档位置和访问责任人明确。
- [ ] 旧数据源保留期明确，例如保留到某日期后再删除。
- [ ] 确认没有其他公司系统或脚本仍依赖旧 Supabase/PostgreSQL。
- [ ] 确认删除动作不会影响历史审计、问题追溯、财务/合规留存要求。
- [x] 删除后线上健康、只读、H5 Monitor 核心回归通过。
- [ ] 删除后资产/识别链路按需抽验。

## 技术核查

代码核查：

```sh
rg -n "postgres|PostgreSQL|pgx|DATABASE_URL|SUPABASE|supabase" cmd internal frontend scripts db docs README.md
```

判断规则：

- 运行时代码不应存在 PostgreSQL 连接路径。
- 文档中允许保留历史迁移记录，但必须标注“历史阶段”或“归档参考”。
- Supabase asset provider 如果仅作为历史兼容代码存在，不能在公司运行时配置中启用。

运行环境核查：

- [ ] K8s 不再配置旧 `DATABASE_URL` 作为业务数据库连接。
- [ ] K8s 不再配置 `ASSET_STORE=supabase` 作为公司运行时资产存储。
- [ ] 旧 Supabase service role key 不在当前 Pod 环境变量中。
- [ ] 旧 Supabase 出站访问不是当前业务必要路径。
- [ ] 公司发布分支和 Dockerfile 没有硬编码旧数据源。
- [ ] Supabase Dashboard / Logs 中最近访问来源已确认；不能只根据首页 Last 60 minutes 请求数判断公司运行时仍在访问。
- [ ] 完成至少一次 60-70 分钟静默窗口观察：关闭 Supabase 控制台、不运行旧脚本、不触发迁移/ops 后，请求数应归零或可解释为 Supabase 平台内部/控制台访问。

数据核查：

- [ ] MySQL `tb_stores` 与业务确认的 54 家门店一致。
- [ ] MySQL 录像机、通道、截图数量与迁移验收记录一致。
- [ ] 关键样本门店的设计图、区域、通道截图、识别结果可读。
- [ ] 旧 PostgreSQL 空 `external_org_id` 门店已记录不迁移原因。

## 建议下线顺序

1. 标记旧 PostgreSQL/Supabase 为只读，不允许再写。
2. 保留观察期，期间所有新写入都应只进入 MySQL/OSS。
3. 使用 Supabase Logs / `pg_stat_activity` 确认最近访问来源，不把控制台自访问误判为业务访问。
4. 移除公司运行环境中的旧数据库和旧 Storage 相关 Secret。
5. 做一次线上回归，确认无运行时依赖。
6. 归档旧数据源和旧对象存储。
7. 在安全/运维/产品确认后，执行删除或权限撤销。
8. 删除后再次执行线上回归，并把结果写入 `docs/codex-learning-state.md`。

## 禁止事项

- 禁止主会话单独执行删除旧库、清 bucket、撤销密钥等不可逆操作。
- 禁止把旧库重新作为“临时回滚路径”接回线上。
- 禁止在文档、命令、日志或截图中暴露 service role key、数据库密码、OSS AK/SK 或完整签名 URL。
- 禁止在未确认归档和保留期前删除旧数据。

## 完成标志

- 旧 PostgreSQL/Supabase 无公司运行时依赖。
- 旧数据源归档、删除或撤权动作有责任人和执行记录。
- 删除后线上回归通过。
- `docs/codex-learning-state.md`、`docs/decisions.md`、`work/current-plan.md` 均已更新。
