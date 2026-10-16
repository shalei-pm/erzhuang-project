# CRM 权限核心最终独立复核

日期：2026-10-16。审查目录：`/private/tmp/erzhuang-crm-institution-authority`，基于当前未提交实现。本结论为本次独立复核，不代表置顶 DBA 线程批准，也不代表正式发布批准。

## 结论

本次限定范围内未发现剩余 P1/P2 阻断问题；此前 nil revision 绕过、Memory 状态门禁及非 viewer 列表绕过已修复，可交主会话执行最终全量本地验证和测试环境验收。真实数据库验证未执行，列为测试发布门禁，不作为本轮代码修复交付阻塞。

- Memory 在同一写锁内重新计算当前 scope、角色、用户启用状态的 revision，所有 UpdateAuthUser 请求（包括 nil）均先比对，再修改。种子用户也不能以空 revision 绕过。nil 保留、显式空数组撤销；停用/缺失的既有授权可保留及撤销，撤销后不能重新添加不可用机构。
- Memory 所有角色先通过当前启用 CRM 候选及规范身份校验，viewer 再校验已保存 scope。仅为已有合成场景补入明确的启用候选 10001，不放行任意 orgID。NVR/H5 两个列表均逐机构鉴权并去掉空城市组。
- MySQL 更新入口强制非空 revision（`internal/app/mysql_store.go:675`）；随后锁用户，读取带 FOR UPDATE 的 scope，并按原始元组、当前角色、用户启用状态校验；nil 仅在校验后跳过 scope 写入（同文件 `setMySQLUserMonitorScopesWithAudit`；`internal/app/mysql_monitor_scopes.go:98`）。内部创建路径不携带旧 revision 与更新入口有明确区别。
- MySQL admin/editor 改查启用 CRM；viewer 的 scope 条件同时包含启用 CRM 门禁，旧授权按已存 external_key，新身份要求 resource_id 匹配，混合身份读取拒绝。未新增 DDL。
- Create/Update 的 Commit 错误包装为 errAuthUserCommitUnknown；handler 在失败审计分支前返回 503，提示刷新核对，未追加确定失败审计（`internal/app/mysql_store.go:661`、`:731`；`internal/app/users_handler.go:69`、`:113`）。这是静态分支复核；现有脚本测试验证了存储层 commit 错误分类。
- 已快速核对 `docs/releases/2026-10-16-crm-institution-authority-4.9.0.md`：包含权限编辑冻结、所有旧实例退出后恢复编辑、真实结构和其他写入者核验、双请求并发验收；产生新身份后不能直接退 4.8.5，撤销不可随整表旧快照复活，CRM-only 机构不能猜测旧映射。未发现本轮范围内遗漏的切换/回滚阻断项。

## 本地验证

新增回归在修复前出现预期失败（nil 旧 revision、空 revision、全角色停用/缺失与列表泄漏），修复后通过。针对当前合并状态执行：

```sh
CGO_ENABLED=0 GOCACHE=/tmp/erzhuang-go-cache GOPROXY=off /Users/sylar/erzhuang-project/.tools/go/bin/go test ./internal/app -run 'Test(CRM|MySQLMonitor|MemoryCRM|NVRMonitor|H5Monitor|MonitorAuthorizers|DigitalTwin|MonitorScreenshot|MonitorScopeAudit|UserMutationReturnsMonitorStoreScopes)' -count=1
```

结果：通过（2.761s）。包含本次 Memory/列表回归，以及主会话新增的 MySQL stale nil、CRM 启用门禁、混合身份、替换失败回滚、审计失败与 commit 不确定等 scripted SQL 测试。后者使用本地脚本驱动，未连接数据库。本会话未重跑全量 Go 测试、前端 build 或浏览器验收；由主会话在本次编辑结束后统一执行。

## 测试发布门禁与测试缺口

- 在测试 MySQL/InnoDB 上用两个真实连接验证：先撤销/降权/停用，再提交持旧 revision 的 nil 和非空数组更新，均冲突且不复活状态；核对实际事务隔离、空 scope 时的锁行为及所有写入者是否遵守用户锁顺序。
- 核对实际列类型、索引/约束、触发器与其他写入者；验证 ID 碰撞、CRM-only、停用/恢复/缺失、源查询失败及事务中途失败的持久化结果。scripted SQL 不能证明真实数据库的锁竞争或原子回滚效果。
- 建议补 handler 级 commit-unknown 回归，明确断言 Create/Update 返回 503 且失败审计调用次数为零；当前已静态核对分支并验证存储层错误分类，此项为覆盖缺口，不是已发现漏洞。
- 主会话完成全量 Go/build 与浏览器弹窗验收后再记录最终发布决定。本轮未连接真实库、未发布、未推送。

## 本会话变更

1. `internal/app/authz.go`
2. `internal/app/memory_store.go`
3. `internal/app/memory_crm_monitor_authority_test.go`（新增）
4. 本最终审查文档（按后续授权新增）。

MySQL、handler、SQL 测试和发布文档由主会话维护，本会话仅复核，未修改。

## 主会话收尾补充

独立复核结束后，主会话补充 TestCRMUserCommitUnknownReturns503WithoutFailureAudit，覆盖 Create/Update 两个 handler 返回 503、不追加失败审计及核对提示。最终 go test ./... 与 Go 服务构建通过；前端 23 个测试文件/151 项测试、生产构建通过；1440/1024/390 三种宽度的本地无头浏览器权限弹窗检查通过。以上为本地验证，不替代实际数据库、测试环境或正式环境验收。
