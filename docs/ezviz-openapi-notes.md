# 萤石云 OpenAPI 接入注意事项

最后更新：2026-06-11

来源：用户与 OpenClaw 侧交流记录整理。

本文只记录接口规则和工程约束，不记录任何真实密钥。

## 1. 接口地址

本项目使用萤石开放平台 OpenAPI。

### 1.1 获取 accessToken

```text
POST https://open.ys7.com/api/lapp/token/get
Content-Type: application/x-www-form-urlencoded
```

请求参数：

```text
appKey=xxx
appSecret=xxx
```

成功返回后，从以下字段读取 token：

```text
data.accessToken
```

### 1.2 设备抓图

```text
POST https://open.ys7.com/api/lapp/device/capture
Content-Type: application/x-www-form-urlencoded
```

请求参数：

```text
accessToken=xxx
deviceSerial=设备序列号
channelNo=通道号
```

成功返回后，从以下字段读取图片 URL：

```text
data.picUrl
```

## 2. 请求格式

萤石接口要求使用：

```text
application/x-www-form-urlencoded
```

不要使用 JSON body。

Go 后端应使用 `url.Values` 编码请求体，并设置：

```text
Content-Type: application/x-www-form-urlencoded
```

## 3. 多账号 token 缓存

系统会维护多个萤石云账号。一个账号/key 只能管理有限门店，不能使用全局单 token。

token 必须按账号维度缓存：

```text
ezviz_account_id -> accessToken / expireAt
```

调用链：

```text
门店 + 通道
-> 查录像机和通道映射
-> 得到 deviceSerial / channelNo / ezviz_account_id
-> 查萤石账号密钥
-> 获取或复用该账号 token
-> 调用萤石接口
```

禁止：

- 按城市或区域推断 key。
- 使用全局默认 key。
- 映射缺少 `ezviz_account_id` 时兜底使用任意账号。

## 4. token 过期和刷新

accessToken 有有效期，按过往经验可先按 7 天处理，但不能假设长期有效。

萤石接口可能通过 JSON `code` 表示 token 过期，而不是 HTTP 错误。

常见 token 失效码：

| 错误码 | 含义 |
| --- | --- |
| 10002 | token 过期或无效 |
| 10014 | token 过期或无效 |

底层调用链必须支持自动续期：

1. 使用当前 token 调用接口。
2. 如果返回 `10002` 或 `10014`，丢弃当前账号缓存 token。
3. 使用该账号 appKey/appSecret 重新获取 accessToken。
4. 使用新 token 重试当前请求一次。
5. 如果仍失败，再返回错误。

不要只依赖定时刷新。

## 5. 错误码处理

常见错误码：

| 错误码 | 处理建议 |
| --- | --- |
| 200 | 成功 |
| 10002 | token 过期/无效，刷新 token 后重试一次 |
| 10014 | token 过期/无效，刷新 token 后重试一次 |
| 60012 | 未知错误，可能是通道不存在、设备未响应或账号无权限 |
| 20008 | 设备响应超时，可能设备离线、网络异常、通道无响应 |
| 9001 | 无录像片段，抓图场景可作为无有效画面或设备异常处理 |

要求：

- 所有错误都必须记录日志。
- 返回给调用方时要带结构化原因，不要只返回“失败”。
- token 过期类错误自动处理。
- 设备/通道类错误不要无限重试。

## 6. 重试和超时

所有 HTTP 请求必须设置 timeout。

建议：

- token 获取：最多重试 2 次，间隔 1 秒。
- 抓图接口普通失败：最多 1 次重试。
- token 过期：刷新 token 后额外重试 1 次。
- 设备超时：最多 1 次重试，不要长时间阻塞。
- HTTP 超时建议先设为 30 秒，后续按真实调用耗时调整。

## 7. 图片 URL 处理

萤石抓图成功后返回 `picUrl`。

本项目优先采用服务端下载图片方案：

```text
萤石 picUrl -> 服务端下载 -> 本地临时存储 -> 返回本系统图片地址
```

原因：

- 避免直接暴露萤石资源地址。
- 便于权限控制。
- 便于审计。
- 便于缓存缩略图和大图。

第一版保存：

- 最近一次缩略图。
- 最近一次大图。
- 大图一周后删除。

## 8. 安全要求

appSecret/accessToken 不得外泄。

要求：

- 不在接口响应中返回 appSecret/accessToken。
- 不在普通日志中打印完整 appSecret/accessToken。
- 不把 secret 写入错误堆栈。
- 不把 secret 提交到 GitHub。
- 日志如需记录 key，只能脱敏。

脱敏示例：

```text
appKey=675f****76a
appSecret=2e71****816
```

## 9. 审计日志

每次抓图请求都要记录审计日志。

建议字段：

| 字段 | 说明 |
| --- | --- |
| request_id | 请求唯一 ID |
| request_user | 请求用户，当前第一版可固定 admin |
| store_id | 门店 ID |
| store_name | 门店名称 |
| device_serial | 录像机设备编码 |
| channel_no | 通道号 |
| area_name | 区域名称 |
| ezviz_account_id | 使用的萤石账号 ID |
| result | 成功 / 失败 |
| error_code | 萤石错误码 |
| error_message | 错误信息 |
| captured_at | 抓图时间 |
| cost_ms | 接口耗时 |

审计日志不要记录 appSecret/accessToken。

## 10. 结构化错误

数据缺失或配置缺失时，返回明确错误码。

建议错误码：

| 缺失项 | 错误码 |
| --- | --- |
| 找不到门店 | STORE_NOT_FOUND |
| 找不到区域位置 | LOCATION_NOT_FOUND |
| 缺少录像机编号 | DEVICE_SERIAL_MISSING |
| 缺少通道号 | CHANNEL_NO_MISSING |
| 缺少萤石账号 ID | EZVIZ_ACCOUNT_ID_MISSING |
| 找不到账号配置 | EZVIZ_ACCOUNT_NOT_FOUND |
| 账号状态停用 | EZVIZ_ACCOUNT_DISABLED |
| 缺少 App Key | APP_KEY_MISSING |
| 缺少 App Secret | APP_SECRET_MISSING |

不要在数据不完整时默认用某个兜底 key。

## 11. 巡检建议

后续可提供巡检接口或脚本：

```text
POST /admin/ezviz/check
```

巡检内容：

- 每个启用账号能否成功获取 token。
- 每个门店至少一个通道能否抓图。
- 哪些通道超时。
- 哪些账号失效。
- 哪些门店没有配置账号 ID。
- 哪些通道缺设备序列号或通道号。

巡检结果可记录：

- 最近校验时间。
- 最近校验结果。
- 异常原因。
