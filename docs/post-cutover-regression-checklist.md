# MySQL/OSS 切换后线上回归清单

最后更新：2026-07-04

本文用于 MySQL + OSS 切换后的线上抽验、发布后回归和故障定位。它偏“项目负责人验收清单”，不是自动化测试脚本。

## 当前基线

- 公司入口：`https://lite.sy.soyoung.com/erzhuang-project/`
- 健康检查：`/erzhuang-project/health`
- 期望：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"mysql","asset_store":"oss"}
```
- 当前有效门店数：54。
- 第 55 家空 `external_org_id` 门店不迁移。

## 回归范围

每次涉及数据库、资产、权限、AI、萤石、H5 Monitor、门店详情或发布配置的变更后，至少按本清单做针对性抽验。

## 1. 基础健康

- [ ] `/health` 返回 200。
- [ ] `database=mysql`。
- [ ] `asset_store=oss`。
- [ ] 页面能打开，静态资源路径仍在 `/erzhuang-project/` 前缀下。
- [ ] `/api/auth/me` 返回当前登录用户和权限。
- [ ] 页面底部版本号与本次发布版本/commit 可对上。

## 2. 门店列表

- [ ] 门店列表返回 200。
- [ ] 总数为 54。
- [ ] 城市筛选可用。
- [ ] 名称搜索可用。
- [ ] 分页/列表排序无明显异常。
- [ ] 不出现空 `external_org_id` 的第 55 家门店。

建议抽验门店：

- `10030` 北京保利实验室门店。
- `10019` 新氧青春诊所(上海陆家嘴店)。
- `10081` 新氧青春诊所(杭州城北万象城店)。
- 最近迁移或刚修复的问题门店。

## 3. 门店详情与设计图

- [ ] 门店详情页打开正常。
- [ ] 设计图数据接口返回 200。
- [ ] 设计图预览图/缩略图能显示。
- [ ] 区域标注框能显示。
- [ ] 保存设计图不会触发 MySQL collation 错误。
- [ ] 手动新增/编辑/删除临时区域后可保存并重新读取。

注意：临时写入验收应使用测试门店或临时门店，完成后清理。

## 4. 录像机与通道

- [ ] 录像机列表显示正常。
- [ ] 有效通道数量与详情一致。
- [ ] 扫描录像机通道可返回 200。
- [ ] 删除临时录像机后详情页不再显示。
- [ ] 删除临时通道后列表刷新正确。
- [ ] 通道确认状态不会被未确认识别结果覆盖。

## 5. 通道截图与 AI 识别

- [ ] 单通道刷新截图返回 200。
- [ ] 截图能通过后端代理路径访问。
- [ ] 单通道识别返回 200。
- [ ] AI 结果只做预填，未自动确认。
- [ ] 非业务区域、入口、侧门、走廊等低置信结果需要人工复核。
- [ ] MiniMax/GPT 返回非 JSON 或 `<think>` 时不会导致整批流程不可恢复。
- [ ] 批量识别接口保持节流，不应一次高频请求大量萤石截图。

故障判断：

- 502/504 多发生在萤石抓图、AI 请求或 APISIX 超时链路。
- 如果单通道重试可成功，优先判断为外部接口波动或模型输出波动。
- 如果所有通道都失败，再查 provider、密钥、网络、OSS 和后端日志。

## 6. H5 Monitor

- [ ] `/api/h5/orgs/{external_org_id}/monitor` 返回 200。
- [ ] 门店名称、城市、通道分组正常。
- [ ] 直播播放可打开。
- [ ] 回放日期和片段选择可用。
- [ ] 移动端视口下播放器和门店切换不遮挡。
- [ ] 响应不暴露 app secret、access token、完整签名播放 URL 等敏感信息。

## 7. 权限与用户

- [ ] 未登录或无效 SSO token 时按预期拒绝。
- [ ] `/api/auth/me` 返回真实 SSO 用户，而不是错误回退本地管理员。
- [ ] viewer 不应执行写接口。
- [ ] editor 可执行门店空间写接口。
- [ ] admin/user manager 可访问用户管理和 AI 设置。
- [ ] disabled 用户不可访问。

## 8. 写接口验收

建议使用临时门店，完成后删除：

- [ ] 创建门店。
- [ ] 编辑门店基础信息。
- [ ] 保存设计图。
- [ ] 添加录像机。
- [ ] 删除录像机。
- [ ] 删除临时门店。
- [ ] 删除后门店列表和详情接口一致。

## 9. 资产与 OSS

- [ ] 设计图 PDF/预览图/缩略图能读。
- [ ] 通道截图能读。
- [ ] 新生成的截图写入 OSS 后可立即读取。
- [ ] 后端代理返回合适 content type。
- [ ] 前端不直连 OSS。
- [ ] 日志不暴露 OSS AK/SK 或完整签名 URL。

## 10. 发布后记录

每次回归完成后，记录到 `docs/codex-learning-state.md`：

- 日期和版本。
- commit。
- 验证人。
- 验证入口。
- 通过项。
- 失败项和处理结论。
- 是否需要回滚或补丁发布。

## 浏览器控制台常用片段

基础检查：

```js
async function read(name, url, options = {}) {
  const r = await fetch(url, {
    credentials: 'include',
    cache: 'no-store',
    headers: { 'Cache-Control': 'no-cache', ...(options.headers || {}) },
    ...options
  })
  const text = await r.text()
  let body
  try { body = text ? JSON.parse(text) : null } catch { body = text.slice(0, 1000) }
  console.log('\n==', name, '==')
  console.log('status=', r.status)
  console.log(body)
  return { name, status: r.status, body }
}

await read('health', '/erzhuang-project/health')
await read('auth me', '/erzhuang-project/api/auth/me')
await read('store list', '/erzhuang-project/api/store-space/stores?page=1&page_size=20')
await read('monitor 10030', '/erzhuang-project/api/h5/orgs/10030/monitor')
```

门店数量检查：

```js
const stores = await fetch('/erzhuang-project/api/store-space/stores?page=1&page_size=100', {
  credentials: 'include',
  cache: 'no-store',
  headers: { 'Cache-Control': 'no-cache' }
}).then(r => r.json())

console.log('total=', stores.total)
console.table((stores.items || []).map(s => ({
  id: s.id,
  city: s.city,
  name: s.name,
  external_org_id: s.external_org_id
})))
```
