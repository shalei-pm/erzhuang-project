# 二壮安全复核记录（2026-09-02）

## 范围与结论

本次复核对照了外部提供的 `high-risk-report.html`（测试日期：2026-08-28）与当前工作分支源码。报告中的历史验证结果不能直接代表当前线上或测试实例；本记录只说明当前源码可见的控制措施和缺口，未使用真实用户 Cookie、未调用生产接口，也未执行有副作用操作。

原报告的 V1（普通查看用户跨门店查看监控）和 V2（`store-space` 写操作无授权）在当前代码层面已经补上权限控制。顺序 ID 仍然存在，但在对象授权正确的前提下不构成 P0；UUID 化应作为后续防御加固，不应替代服务端授权。

本轮修复完成后，普通查看用户不能再访问旧版门店空间管理接口；截图映射导出仅限管理员且以审计记录成功为前置条件。低权限跨门店的运行时回归不纳入本次验收。

## 现有修复已覆盖的报告项

### SEC-2026-001：NVR 监控跨门店越权（原报告 V1）

- **状态：源码已修复。**
- **证据：** NVR 的门店列表、摄像头列表、截图和取流会话请求均先调用 `ensureCanViewStore`，见 `internal/nvrmonitor/handler.go:79`、`internal/nvrmonitor/handler.go:96`、`internal/nvrmonitor/handler.go:137`。
- **授权依据：** `internal/app/authz.go:232` 使用 `CanUserViewMonitorStore`；普通查看用户仅保留已授权的门店范围，门店列表也在 `internal/app/authz.go:251` 过滤。
- **说明：** 低权限跨门店运行时回归不纳入本次验收；服务端授权代码与单元测试覆盖仍保留。

### SEC-2026-002：旧版门店空间写操作越权（原报告 V2）

- **状态：源码已修复。**
- **证据：** `internal/storespace/handler.go:48` 至 `internal/storespace/handler.go:86` 将写路由统一包装为 `writeGuard`；应用层的 `storeWriteGuard` 强制 `store:write`，见 `internal/app/handler.go:274`。
- **权限模型：** 普通查看角色只拥有 `store:read`，不具备 `store:write`，见 `internal/app/auth_users.go:94`。
- **说明：** 低权限写操作运行时回归不纳入本次验收；服务端权限守卫与单元测试覆盖仍保留。

### SEC-2026-003：顺序 ID 枚举（原报告 V3）

- **状态：二期加固。**
- **说明：** 门店、摄像头和录像机仍使用顺序 ID。当前 NVR 服务会在资源读取前校验门店范围和摄像头归属，见 `internal/nvrmonitor/service.go:187`。因此应优先确保所有资源端点都做对象授权和统一错误处理，而不是先做全量 ID 重构。
- **后续：** 新增外部接口时优先使用不可枚举标识；对未授权和不存在资源统一响应，减少资源存在性探测。

## 本轮已修复的问题

### SEC-2026-004：旧版门店空间读取与导出未按门店范围授权

- **严重性：高（已修复）**
- **位置：** `internal/storespace/handler.go:68` 至 `internal/storespace/handler.go:72`、`internal/storespace/handler.go:143`、`internal/storespace/handler.go:160`、`internal/storespace/handler.go:177`、`internal/storespace/handler.go:284`。
- **证据：** 这些接口只需 `store:read`。处理器调用 `applyMonitorVisibility` 后仅将返回字段 `can_view_monitor` 设置为 `false`，仍会返回门店、设计图和通道数据；导出接口甚至不执行该可见性判断。
- **敏感数据：** 导出逻辑会把已保存的通道截图写入 Excel，见 `internal/storespace/channel_mapping_excel.go:24` 至 `internal/storespace/channel_mapping_excel.go:72`。
- **影响：** 普通查看用户拥有 `store:read`，见 `internal/app/auth_users.go:100`。若仍可进入或直接调用旧版 API，可按顺序门店 ID 读取门店布局/通道信息，或导出含截图的文件。
- **修复：** 旧版 `store-space` 全部读取与写入接口统一要求 `store:write`；`export.xlsx` 单独要求管理员才拥有的 `store:export`，并在导出前强制写入审计。普通查看用户只能使用 3.0 资源查看与按门店授权的 NVR 监控。
- **测试：** 增加普通查看账号对列表、详情、设计图、通道、截图和导出的拒绝回归；增加管理员导出审计回归。
- **后续：** 当前编辑和管理员具备全量旧版管理权限。若未来出现“按门店管理”角色，还需给旧版管理接口补对象级门店范围校验。

### SEC-2026-005：萤石云诊断取流可由客户端指定任意录像机序列号

- **严重性：高（已下线）**
- **位置：** `internal/storespace/service.go:96` 至 `internal/storespace/service.go:137`，路由见 `internal/storespace/handler.go:64`。
- **证据：** 虽然接口已要求 `store:write`，但请求体仍接受 `account_id`、`account_name`、`device_serial` 和 `channel_no`，服务端未验证该序列号是否属于本系统已登记的录像机或所属门店。
- **影响：** 知道同一萤石云账号下其他设备序列号的编辑者，可能获取未登记设备的直播地址。
- **修复：** 当前 3.0 监控不使用萤石云诊断取流，已删除该前端 Demo、`/api/store-space/diagnostics/ezviz/live-address` 路由和服务端参数处理；接口回归验证为 404。萤石云保留的账号/抓图能力不再向客户端签发此诊断直播地址。

### SEC-2026-006：HTTP 服务缺少连接与请求头超时上限

- **严重性：高（已修复，已完成编译验证）**
- **位置：** `cmd/server/main.go:100`。
- **证据：** `http.Server` 当前仅配置 `Addr` 和 `Handler`，没有 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout` 或 `MaxHeaderBytes`。
- **影响：** 恶意慢请求可能长期占用连接/协程。即使 APISIX 或 Ingress 提供防护，应用层仍应设置明确边界。
- **修复：** 增加 `ReadHeaderTimeout=10s`、`ReadTimeout=30s`、`WriteTimeout=60s`、`IdleTimeout=120s`、`MaxHeaderBytes=32KiB`，并为应用请求体设置 8MiB 上限。NVR 媒体流不经该 HTTP 服务，因此不受此变更影响。

## 次级加固与待运维确认项

### SEC-2026-007：健康检查暴露实现信息

- **严重性：低**
- **位置：** `internal/app/handler.go:427` 至 `internal/app/handler.go:441`。
- **状态：已修复。** 健康检查已仅返回 `status`，不再暴露版本、数据库或资产存储类型；路径与 HTTP 状态码保持不变，兼容 K8s 探针。

### SEC-2026-008：安全响应头、WAF 和限流未在应用代码中可见

- **严重性：待确认**
- **状态：部分修复，网关侧待确认。** 应用已统一下发最小 CSP（`base-uri`、`frame-ancestors`、`object-src`）、`X-Frame-Options`、`X-Content-Type-Options`、`Referrer-Policy`、`Permissions-Policy`，并对 API/健康检查返回 `Cache-Control: no-store`。该 CSP 不限制 `connect-src`，避免影响当前 NVR WebSocket/WASM 播放。
- **需运维确认：** APISIX/Ingress 实际响应头是否与应用一致、敏感接口和登录接口限流、网关请求体上限、WebSocket 连接限制与访问日志保留策略。

### SEC-2026-009：GET 登出存在低风险登出 CSRF

- **严重性：低**
- **位置：** `internal/app/auth.go:226` 至 `internal/app/auth.go:238`。
- **状态：已缓解。** 保留现有 SSO 联合退出跳转，但拒绝浏览器标记为 `Sec-Fetch-Site: cross-site` 的 GET 登出请求；本系统同源退出、POST 本地登出和空闲退出不受影响。旧浏览器未发送该请求头时仍允许，以保持公司兼容性，因此这是一层缓解而不是完全替代 POST-only 方案。

## 后续验证顺序

1. 发布测试环境后，检查健康检查只返回 `status`，并确认安全响应头、HTTP 超时配置和 8MiB 请求体限制已随镜像生效。
2. 验证 `POST /api/store-space/diagnostics/ezviz/live-address` 返回 404，且 `?tool=ezviz-live-demo` 不再出现诊断页面。
3. 由运维核对 APISIX/Ingress 限流、WAF 与 WebSocket 连接限制；这属于网关基础设施配置，不阻塞本次应用补丁验收。

## 验证限制

- 已使用临时官方 Go 1.22 工具链执行 `go build ./...`，全量编译通过。`go test ./...` 的测试二进制受当前 macOS/Codex 沙箱 `dyld` 限制而无法启动；该问题影响所有包，非项目测试断言失败。
- 未使用真实登录态、Cookie、JWT、数据库凭据或线上接口进行验证。
- 网关、APISIX、K8s Ingress 以及正式环境配置不在本次源码审查可见范围内，必须由运维或运行时响应复核。
