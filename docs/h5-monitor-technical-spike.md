# H5 Monitor Technical Spike

最后更新：2026-06-24

本文记录“门店监控 H5 查看端”的阶段性技术讨论。当前不是最终 PRD，也不是实施计划；目标是先确认萤石云直播/回放接口能力是否足够支撑，再决定页面和后端实现细节。

## 1. 模块定位

本模块是现有门店空间资源系统的一个新大版本方向，核心目标是让少量授权人员通过移动端 H5 查看单门店摄像头。

入口预期：

- 后台门店详情页右上角增加“监控二维码”入口。
- 二维码链接使用业务 ID：`/h5/orgs/{externalOrgId}/monitor`。
- `externalOrgId` 即新氧机构 ID；没有新氧机构 ID 的门店不生成二维码。
- 二维码本身不承担安全职责，最终由飞书扫码、飞书登录和权限校验控制访问。

## 2. 已确认产品边界

H5 是纯查看端，不做维护。

本期考虑：

- H5 门店监控首页。
- H5 单摄像头详情页。
- 单通道直播查看。
- 单通道回放查看。
- 回放按日期查询录像片段，点击片段播放。
- 首页只展示已确认、有效通道。
- 后台门店详情页展示门店级监控二维码。
- 预留飞书登录和权限校验入口，但登录授权由后续专人实现。

明确不做：

- 多画面同屏。
- 云台控制。
- 倍速播放。
- 截图按钮。
- 下载录像。
- 通道维护、编辑、确认。
- 摄像头分享能力。
- 复杂时间轴拖动。

## 3. 路由原则

门店级入口：

```text
/h5/orgs/{externalOrgId}/monitor
```

摄像头详情可以有真实路由，方便刷新、返回和内部跳转：

```text
/h5/orgs/{externalOrgId}/monitor/channels/{channelId}
```

但详情页不主动提供分享能力：

- 不展示详情链接。
- 不提供复制链接。
- 不提供详情二维码。
- 不提供分享按钮。

如果用户手动复制详情 URL，后续仍必须经过飞书登录、门店权限和通道归属校验。

## 4. 首页数据规则

H5 首页只展示可真实查看的通道：

- `is_active = true`
- 通道状态为：
  - `confirmed_business`
  - `confirmed_non_business`
- 有录像机设备编码。
- 有通道号。
- 归属于当前 `externalOrgId` 对应门店。

不展示：

- 待确认通道。
- 识别失败但未确认通道。
- 已失效通道。
- 未扫描出来的通道。

分组顺序：

1. 面诊室
2. 治疗室
3. 生美
4. 其他区域

排序建议：

- 面诊室、治疗室、生美按编号升序。
- 其他区域按备注或通道号排序。

首页不自动拉直播流，只展示列表和最近截图。点击进入详情后再获取直播地址。

## 5. 最近截图刷新规则

直播开始时触发最近截图刷新：

- 进入详情并开始直播时，后端异步触发一次该通道抓图。
- 抓图成功后更新通道最近截图。
- 抓图失败不阻塞直播。
- 同一通道 60 秒内最多刷新一次最近截图，避免频控。
- 播放回放不更新最近截图，避免历史画面覆盖当前实时截图。

## 6. 现有技术基础

当前项目已经具备：

- 多萤石账号运行时配置：`EZVIZ_ACCOUNTS_JSON`。
- accessToken 获取、缓存、过期刷新。
- 录像机通道扫描：`POST /api/lapp/device/camera/list`。
- 单通道抓图：`POST /api/lapp/device/capture`。
- 门店、录像机、通道、区域映射主数据。
- `external_org_id` 字段，可作为 H5 业务入口 ID。

相关代码位置：

- 萤石 OpenAPI client：`internal/ezviz/client.go`
- 萤石 scanner adapter：`internal/storespace/ezviz_scanner.go`
- 门店/录像机/通道模型：`internal/storespace/models.go`
- 门店空间服务：`internal/storespace/service.go`
- 前端 API adapter：`frontend/src/api.ts`

已确认接口记录：

- `docs/ezviz-openapi-notes.md`

## 7. 待确认萤石云能力

本模块能否落地，核心取决于萤石云开放平台是否稳定支持以下能力。

### 7.1 已补充：按时间查询录像文件

用户提供的萤石云文档片段确认存在“根据时间获取存储文件信息”接口，可作为 H5 回放片段列表的数据来源候选。

接口：

```text
POST https://open.ys7.com/api/lapp/video/by/time
Content-Type: application/x-www-form-urlencoded
```

请求参数：

```text
accessToken
deviceSerial
channelNo
startTime
endTime
recType
version
pageSize
```

子账户 token 最小权限：

```text
无
```

关键字段理解：

- `deviceSerial`：设备序列号，字母需大写。
- `channelNo`：通道号，非必填，默认 1；本项目应显式传当前通道号。
- `startTime` / `endTime`：毫秒时间戳；不传时 `startTime` 默认当天 0 点，`endTime` 默认当前时间。
- `recType`：
  - `0`：系统自动选择。
  - `1`：云存储。
  - `2`：本地录像。
- `version=2.0`：可返回分页结构；`recType=1` 时传 2.0 会返回分页结构；`recType=2` 时需同时传 2.0 且 `pageSize` 不为空才会返回分页结构。
- `pageSize`：
  - 云存储范围 `1-1000`。
  - 本地录像范围 `1-500`。

返回结构有两种形态：

1. 老结构：`data` 为文件数组。
2. 分页结构：`data.files` 为文件数组，并返回：
   - `isAll`
   - `nextFileTime`

对本项目的产品价值：

- 这个接口可以支撑“选择日期 -> 查询当天录像片段 -> 点击片段播放”的回放列表。
- 本地录像 `recType=2` 返回的字段中，`startTime`、`endTime`、`channelNo` 足够形成片段列表。
- 本地录像返回中云存储相关字段会是 `null` 或 `0`，第一版不要依赖 `fileId`、`fileIndex`、`coverPic`。
- 云存储 `recType=1` 可能额外返回 `fileId`、`fileIndex`、`coverPic` 等字段。
- `nextFileTime` 可以作为下一次查询的 `startTime`，用于分页拉取剩余录像文件。

待实测点：

- `recType=0` 是否能同时覆盖云存储和本地录像，还是需要分别查 `1` 和 `2`。
- 我们的测试录像机是否有本地录像/云录像权限。
- 返回分页结构时，是否需要循环用 `nextFileTime` 拉完一天数据。
- `coverPic` 是否稳定可用于片段封面；第一版可不依赖。
- 文档中 HTTP 请求报文示例写成 `POST /api/lapp/alarm/video`，但请求地址写的是 `/api/lapp/video/by/time`；以真实探针验证为准，不能直接相信示例路径。
- 错误码包括 token 过期、appKey 异常、设备不存在、通道不存在、未开通萤石服务、非开发者账号无权限等；探针需要保留脱敏错误响应，方便区分账号/设备/通道/权限问题。

### 7.2 已补充：设备本地录像统一查询接口

用户提供的萤石云文档片段确认存在“设备本地录像统一查询接口”。该接口是回放片段列表的另一个重要候选，尤其适合本项目“录像机/NVR + 多通道”的场景。

接口：

```text
GET https://open.ys7.com/api/v3/device/local/video/unify/query
```

请求参数：

Header：

```text
accessToken
deviceSerial
localIndex
```

Query：

```text
recordType
startTime
endTime
isQueryByNvr
location
pageSize
```

关键字段理解：

- `accessToken`：支持托管、子账号、设备小权限 token，权限为 `Replay`。
- `deviceSerial`：设备序列号。
- `localIndex`：通道号，Header 参数；本项目应显式传当前通道号。
- `recordType`：
  - `1`：定时录像。
  - `2`：事件录像。
  - `3`：智能-车。
  - `4`：智能-人形。
  - `5`：自动浓缩录像。
  - 不填默认查询所有类型。
- `startTime` / `endTime`：秒级时间戳，开始结束时间必须在同一天。
- `isQueryByNvr`：
  - `0`：不反查 NVR，默认。
  - `1`：反查 NVR。
- `location`：
  - `1`：本地录像检索，默认。
  - `2`：CVR 中心录像检索。
- `pageSize`：默认 50，最大 500。

返回结构：

```json
{
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  },
  "data": {
    "records": [
      {
        "startTime": 1731945592,
        "endTime": 1731949200,
        "type": "ALARM",
        "size": ""
      }
    ],
    "fromNvr": true,
    "deviceSerial": "J79401957",
    "localIndex": "1",
    "hasMore": true,
    "nextFileTime": 1732007943
  }
}
```

对本项目的产品价值：

- 该接口专门面向设备本地录像，字段更轻、更适合做 H5 回放片段列表。
- `isQueryByNvr=1` 允许反查 NVR 录像，这与本项目“门店录像机管理多个摄像头通道”的实际形态更贴近。
- 返回的 `records[].startTime/endTime/type/size` 足以支撑第一版“按日期列出可回放片段”。
- 返回 `fromNvr`、`deviceSerial`、`localIndex`，可以帮助识别录像实际来自关联 NVR 还是入参设备。

当前判断：

- 回放片段查询建议同时 Spike 两个接口：
  - A：`/api/lapp/video/by/time`，兼容云存储和本地录像。
  - B：`/api/v3/device/local/video/unify/query`，更聚焦本地录像/NVR。
- 如果 B 接口对当前录像机稳定可用，H5 第一版回放片段列表优先使用 B。
- A 接口可作为兼容云存储或旧设备的备选。

待实测点：

- 当前四个区域账号是否具备 `Replay` 权限。
- `isQueryByNvr=1` 对海康录像机是否必须开启，是否能查到更多通道录像。
- `startTime/endTime` 秒级时间戳与前一接口毫秒时间戳不同，封装时要避免混用。
- `nextFileTime` 分页时应作为下一页 `startTime`，还是另有规则；需要按真实返回验证。
- 设备离线时是否还能查询本地/NVR 历史录像。
- 该接口返回的片段时间能否直接传给 `/api/lapp/v2/live/address/get` 的 `startTime/stopTime` 获取回放播放地址。
- 错误码包括无权限、通道不存在、设备不存在、网络异常、设备不在线、设备响应超时、设备不支持等；探针需要记录脱敏错误响应。

### 7.3 已补充：创建直播流

用户提供的萤石云文档片段确认存在“创建直播流”接口。该接口看起来用于创建直播流资源并返回 `streamId`，不是直接返回 H5 可播放 URL。

接口：

```text
POST https://open.ys7.com/api/service/media/streammanage/stream
Content-Type: application/x-www-form-urlencoded
```

请求参数分布和现有 `/api/lapp/*` 接口不同：

Header：

```text
accessToken
deviceSerial
Content-Type
```

Body：

```text
localIndex
accessType
startTime
endTime
```

关键字段理解：

- `accessToken`：放在 header，不是 form body；支持 at token、托管、子账号 token，需要 Real 权限。
- `deviceSerial`：放在 header。
- `localIndex`：设备通道号，默认 1；本项目应显式传 `video_channels.channel_no`。
- `accessType`：接入类型，`1=设备接入`，默认 1。
- `startTime` / `endTime`：格式为 `yyyy-MM-dd HH:mm:ss`。
- 文档说明开始和结束时间跨度最多 7 天，且结束时间不能小于等于当前时间。

返回结构：

```json
{
  "data": {
    "streamId": "787305182210818048"
  },
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  }
}
```

注意：

- 该接口属于 `/api/service/media/streammanage/*` 新接口族，返回结构是 `meta.code`，不同于当前项目已接入的 `/api/lapp/*` 的 `code/msg/data`。
- 当前无法仅凭此接口完成 H5 播放，因为它只返回 `streamId`。
- 还需要继续找到并验证“根据 streamId 获取直播流地址/播放地址/播放信息”的接口。
- `startTime` / `endTime` 可能表示直播流资源的有效期，而不一定是录像回放时间；需要探针验证。

对本项目的产品价值：

- 这个接口可能是直播播放链路的第一步。
- 如果后续接口能用 `streamId` 换取 HLS/FLV/ezopen 播放地址，则可支撑详情页直播 Tab。
- 由于它要求 Real 权限，需要确认当前萤石账号是否已开通该权限。

待实测点：

- 当前四个区域账号是否具备 Real 权限。
- `startTime` / `endTime` 应如何传入直播场景：例如 `now` 到 `now+N小时`，还是固定有效期窗口。
- 同一通道反复创建直播流是否会复用 streamId，还是每次新建。
- 是否需要主动删除/关闭直播流，避免资源泄露或计费问题。
- 是否有频控或并发限制。
- 后续播放地址接口返回的协议类型和有效期。

### 7.4 已补充：修改直播流

用户提供的萤石云文档片段确认存在“修改直播流”接口，用于修改已有 `streamId` 的有效时间。

接口：

```text
PUT https://open.ys7.com/api/service/media/streammanage/stream
```

请求参数：

Header：

```text
accessToken
```

Query：

```text
streamId
startTime
endTime
```

返回结构：

```json
{
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  }
}
```

对本项目的产品价值：

- 说明 `streamId` 是可持续管理的直播流资源。
- H5 直播可能不需要每次都创建新直播流，可以考虑保存或缓存 `streamId`，过期前续期。
- 但第一版应先以简单可靠为主：直播开始时获取可播放能力，是否缓存/复用 `streamId` 需要 Spike 后决定。

文档矛盾点：

- 参数描述写“开始时间和结束时间跨度最多 720 天”。
- 错误码说明中写“startTime 和 endTime 跨度最多为 7 天”。
- 以真实接口探针结果为准，不能按文字直接实现。

待实测点：

- `PUT` 请求参数到底放 query，还是 form body 也可接受。
- `streamId` 不存在、过期、无权限时错误结构。
- 修改直播流是否影响已经获取的播放地址。
- `streamId` 是否需要落库，还是只做短期内存缓存。
- 是否存在删除直播流接口；如果没有，过期策略更重要。

### 7.5 已补充：变更直播流状态

用户提供的萤石云文档片段确认存在“变更直播流状态”接口，用于启用或禁用已有 `streamId`。

接口：

```text
PUT https://open.ys7.com/api/service/media/streammanage/stream/status
```

请求参数：

Header：

```text
accessToken
```

Query：

```text
streamId
status
```

关键字段理解：

- `streamId`：由“创建直播流”接口生成的直播流 ID。
- `status`：
  - `0`：禁用。
  - `1`：启用。

返回结构：

```json
{
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  }
}
```

对本项目的产品价值：

- 进一步证明直播流是一个可管理资源，不只是一次性播放地址。
- 如果后续选择缓存/复用 `streamId`，该接口可用于直播流启停控制。
- 如果萤石侧对直播流资源有并发、计费或有效期限制，禁用接口可能是资源回收的一环。

当前判断：

- 第一版 H5 不建议先引入复杂直播流状态管理，除非探针证明必须启用/禁用才能正常播放。
- 更稳妥的第一版策略仍是：进入直播详情时按需获取可播放能力，离开页面不强依赖状态切换，避免因为状态同步失败影响用户观看。

待实测点：

- 创建直播流后默认是否已经启用。
- 禁用直播流后，已经获取的播放 URL 是否立即失效。
- 重新启用后，原播放 URL 是否恢复可用，还是必须重新获取播放 URL。
- 禁用接口是否有调用频控或权限要求。
- 如果不主动禁用，直播流是否会在 `endTime` 后自动失效。

### 7.6 已补充：删除直播流

用户提供的萤石云文档片段确认存在“删除直播流”接口，用于删除已有 `streamId`。

接口：

```text
DELETE https://open.ys7.com/api/service/media/streammanage/stream
```

请求参数：

Header：

```text
accessToken
```

Query：

```text
streamId
```

返回结构：

```json
{
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  }
}
```

对本项目的产品价值：

- 说明萤石直播流资源可以显式回收，不一定只能等待 `endTime` 过期。
- 如果直播流创建会占用资源、并发额度或产生计费，后端应考虑在不再需要时删除直播流。
- 如果第一版采用“每次进入直播详情创建临时直播流”的策略，删除接口可以作为退出页面、过期清理任务或后台兜底清理的候选能力。

当前判断：

- 第一版 H5 不能强依赖前端页面关闭时一定能调用删除接口，因为移动端浏览器、飞书容器、网络切换都可能导致退出事件不可靠。
- 更稳妥的策略是：直播流设置较短有效期；后端记录必要的 `streamId` 元信息；用定时清理或下次请求时清理过期资源。
- 如果后续播放接口返回的是短期播放 URL，而不是长期直播流资源，第一版也可以先不落库 `streamId`，只在单次请求链路中创建并返回播放信息。

待实测点：

- 删除直播流后，已经获取的播放 URL 是否立即失效。
- 删除后同一通道重新创建直播流是否稳定成功。
- 删除不存在或已过期的 `streamId` 时返回 `404` 是否可以视为幂等成功。
- 删除接口是否有权限、频控或并发限制。
- 如果不删除，仅依赖 `endTime`，萤石侧是否自动释放资源。

### 7.7 已补充：查询直播流推流记录列表

用户提供的萤石云文档片段确认存在“查询直播流推流记录列表”接口，用于查询某个 `streamId` 在最近 30 天内的推流记录。

接口：

```text
GET https://open.ys7.com/api/service/media/streammanage/stream/record/list
```

请求参数：

Header：

```text
accessToken
```

Query：

```text
streamId
startTime
endTime
pageStart
pageSize
```

关键字段理解：

- `streamId`：直播流 ID，文档类型写为 `long`，但前面创建接口返回示例是字符串；实现时应按字符串传递并以真实返回为准。
- `startTime` / `endTime`：查询窗口必须在最近 30 天内，且不能晚于当前时间。
- `pageStart`：页码，默认 0。
- `pageSize`：页大小，默认 50，最大 100。

返回结构：

```json
{
  "data": {
    "recordList": [
      {
        "client": "127.0.0.1:8080",
        "startTime": "2024-12-11 12:00:00",
        "endTime": "2024-12-11 12:00:00"
      }
    ]
  },
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": {
      "total": 7,
      "pageSize": 100,
      "pageStart": 1,
      "hasMore": true
    }
  }
}
```

对本项目的产品价值：

- 它不是 H5 播放主链路接口，不能直接生成播放地址，也不能替代直播播放能力。
- 它适合用于后端排障和运营观测：例如判断某个直播流是否真的发生过推流、推流何时开始/结束、是否存在推流中断。
- 如果后续用户反馈“页面打不开直播”，该接口可能帮助区分是播放端问题、直播流未推流、还是设备/萤石侧异常。

当前判断：

- 第一版 H5 不需要在页面展示推流记录。
- 播放探针可以把该接口作为可选验证项：创建直播流并获取播放能力后，查询推流记录，看萤石侧是否记录到推流行为。
- 如果后续要做后台诊断面板，可以把这个接口纳入“直播诊断详情”，但不进入当前主流程。

待实测点：

- 创建直播流后，不播放时是否已有推流记录。
- H5 开始播放后，推流记录是否实时产生，还是延迟写入。
- `client` 字段是否能识别播放来源，还是仅内部地址。
- 推流记录是否能帮助判断播放失败原因。
- 当前账号是否具备查询该接口的权限。

### 7.8 已补充：直播流推流状态变更 Webhook

用户提供的萤石云文档片段确认存在“直播流推流状态变更 webhook 消息”，消息类型为：

```text
ys.stream.manage
```

前提条件：

- 需要开通 webhook 消息推送。
- 需要在“云信令-消息推送”产品中新建该消息类型。
- 文档说明开发者接收该类型需要联系小助手在后台手动开通。

消息 Header：

```text
type
userId
messageTime
deviceId
channelNo
```

消息 Body：

```text
version
type
streamId
client
status
reason
timestamp
```

关键字段理解：

- `streamId`：直播流 ID。
- `status`：
  - `1`：开始播放。
  - `2`：结束播放。
  - `3`：开始推流。
  - `4`：结束推流。
  - `101`：启用流。
  - `102`：禁用流。
- `reason`：结束推流原因，包括流 ID 不存在、流禁用、不在可用时间区间、用户设备关系验证失败、通道隐藏、4G 无限流量卡、其他错误等。

对本项目的产品价值：

- 它不是获取播放地址的接口，也不是 H5 第一版必须依赖的能力。
- 它适合后续做直播健康观测和排障：例如记录某次播放是否真的开始、是否结束、结束原因是什么。
- 如果后续要做“直播状态实时提示”或“播放失败诊断”，该 webhook 比前端轮询更可靠。

当前判断：

- 第一版 H5 不建议依赖该 webhook 才能播放，因为开通链路额外复杂，还需要萤石后台人工开通。
- 后端设计时可以预留 webhook 接收接口，但不进入第一版必做范围。
- 若公司后续希望统计监控使用情况或排查播放失败，该能力值得进入 P1/P2。

待实测点：

- 当前萤石账号是否能开通 `ys.stream.manage` 消息。
- 开始播放、结束播放、开始推流、结束推流的消息触发顺序和延迟。
- `reason` 是否稳定返回，是否足以定位播放失败原因。
- Webhook 验签、重试、幂等机制是什么。
- 消息里的 `deviceId` / `channelNo` 是否足够和本项目 `recorders` / `video_channels` 对齐。

### 7.9 已补充：查询直播流列表

用户提供的萤石云文档片段确认存在“查询直播流列表”接口，用于按条件查询已创建的直播流资源。

接口：

```text
POST https://open.ys7.com/api/service/media/streammanage/stream/list
```

注意：文档标题写“查询直播流列表（GET）”，但请求方式和 curl 示例均为 `POST`。第一版探针应按 `POST` 验证，必要时再测试 `GET` 是否可用。

请求参数：

Header：

```text
accessToken
deviceSerial
Content-Type
```

Body：

```text
streamId
pageStart
pageSize
accessType
status
```

关键字段理解：

- `deviceSerial`：header 中的筛选条件，非必填；本项目按录像机查询时应传。
- `streamId`：可筛选具体流。
- `accessType`：
  - `1`：设备接入。
  - `2`：RTMP 接入。
- `status`：
  - `0`：已禁用。
  - `1`：已启用。
- `pageStart` / `pageSize`：分页参数，`pageSize` 最大 100。

返回字段中对本项目较有价值的字段：

```text
streamId
deviceSerial
channelNo
deviceName
status
clientNum
bitrate
bandwidth
latestPubTime
startTime
endTime
playStatus
```

对本项目的产品价值：

- 该接口可能支持“先查是否已有同设备同通道的可用直播流，再决定是否创建新流”的策略。
- `clientNum`、`bitrate`、`bandwidth`、`latestPubTime`、`playStatus` 适合用于后续直播诊断和后台观测。
- 如果创建直播流有频控、资源上限或计费，该接口能帮助减少重复创建。

当前判断：

- 第一版 H5 可以先不做复杂复用策略，但技术 Spike 必须验证是否能按 `deviceSerial + channelNo + status=1` 查到既有流。
- 如果查询结果稳定，推荐后端封装为 `ensureLiveStream`：
  1. 查询已有启用且未过期的直播流。
  2. 有则复用或续期。
  3. 没有则创建。
  4. 返回后续播放地址接口需要的标识。
- 如果查询接口不稳定或权限受限，第一版可退回“每次进入详情创建短有效期直播流”的简单策略。

待实测点：

- 请求方法到底只支持 `POST`，还是 `GET` 也可用。
- `deviceSerial` 放 header 是否真能筛选设备。
- 是否可以按通道号筛选；文档未提供 `channelNo` 入参，可能需要查出后本地过滤。
- `streamId` 返回类型到底是字符串还是数字；实现应统一按字符串处理。
- `startTime` / `endTime` 是否表示直播流有效期。
- `playStatus` 是否代表“历史是否有用户观看过”，还是当前播放状态。
- 该接口是否能查到其他会话或历史创建的直播流。

### 7.10 已补充：获取播放地址

用户提供的萤石云文档片段确认存在“获取播放地址”接口。这个接口是当前 H5 监控模块最关键的候选接口，因为它可以直接通过设备序列号和通道号获取播放地址。

接口：

```text
POST https://open.ys7.com/api/lapp/v2/live/address/get
Content-Type: application/x-www-form-urlencoded
```

请求参数：

```text
accessToken
deviceSerial
channelNo
protocol
code
expireTime
type
quality
startTime
stopTime
supportH265
containerFormat
mute
playbackSpeed
gbchannel
diserr
```

关键字段理解：

- `deviceSerial`：设备序列号，最多 50 个字符。
- `channelNo`：通道号，默认 1；本项目必须显式传当前通道号。
- `protocol`：
  - `1`：ezopen。
  - `2`：hls。
  - `3`：rtmp。
  - `4`：flv。
- `expireTime`：播放地址有效期，单位秒；针对 hls/rtmp/flv，范围为 30 秒到 720 天。
- `type`：
  - `1`：预览，也就是直播，默认值。
  - `2`：本地录像回放。
  - `3`：云存储录像回放。
- `quality`：预览清晰度，`1=高清主码流`，`2=流畅子码流`；仅直播预览生效。
- `startTime` / `stopTime`：本地录像或云存储录像回放时间，格式 `yyyy-MM-dd HH:mm:ss`。
- `mute`：服务端静音，仅 RTMP、HTTP-FLV、HLS 生效。
- `playbackSpeed`：回放倍速，仅 `protocol=4` 且 `type=2/3` 时可用；本期不做倍速。

返回结构：

```json
{
  "msg": "Operation succeeded",
  "code": "200",
  "data": {
    "id": "254708522214232064",
    "url": "https://open.ys7.com/v3/openlive/C78957921_1_1.m3u8?...",
    "expireTime": "2020-12-03 20:41:13"
  }
}
```

权限要求：

```text
"Permission": "Real,Replay"
"Resource": "dev:序列号"
```

对本项目的产品价值：

- 这是目前最适合作为 H5 第一版直播播放主链路的接口。
- 它不要求先创建 `streamId`，可以直接基于现有 `recorders.device_serial + video_channels.channel_no` 获取播放地址。
- 同一个接口也可能支撑回放播放地址：`type=2` 本地录像回放，`type=3` 云存储录像回放，并配合 `startTime` / `stopTime`。
- 它和前面“根据时间获取存储文件信息”接口可以组合成回放链路：
  1. 查询某天录像片段。
  2. 用户选择片段。
  3. 调用本接口获取该片段的回放播放地址。

当前判断：

- 第一版 H5 应优先 Spike 这个接口，而不是先接入 `/api/service/media/streammanage/*` 的直播流管理链路。
- 推荐直播协议优先测试 `protocol=2 hls`，因为移动端 H5 对 HLS 兼容性通常最好；同时测试 `protocol=4 flv` 是否需要额外播放器。
- 第一版直播播放地址可以设置较短有效期，例如 10-30 分钟；播放页在过期或失败时重新请求。
- 回放第一版可优先测试 `protocol=2 hls` 是否支持；如果文档所说“回放仅支持 rtmp、ezopen、flv 协议”属实，则 H5 回放可能需要 FLV 播放器或萤石官方播放器。
- `streamId` 管理接口暂时降级为可选的云直播资源管理/诊断能力，除非探针证明该播放地址接口无法满足直播或回放。

待实测点：

- 当前四个区域账号是否具备 `Real` 和 `Replay` 权限。
- 直播 `type=1 + protocol=2 hls` 是否能在移动端 H5 直接播放。
- 直播 `quality=2` 是否明显降低延迟和带宽，适合作为默认。
- `expireTime` 最短和最长是否符合文档，过期后是否需要重新获取 URL。
- 回放 `type=2/3` 是否真的不支持 HLS；如果只支持 FLV/ezopen，H5 播放方案要单独确认。
- `code` 加密密码是否在当前设备上必需；如果设备开启视频加密，是否需要存储或配置。
- 设备离线时能否获取历史回放地址。
- `supportH265` / `containerFormat` 对当前设备是否有影响；第一版可先 `supportH265=0`。
- 错误结构中是否存在文档提到的 `ret/status/exception`，示例里没有，需要按真实返回解析。

2026-06-24 华北测试录像机初步探针：

- 测试账号：华北。
- 测试录像机：`GN0941203`。
- 接口：`/api/lapp/v2/live/address/get`。
- 首次使用文件内旧 accessToken 返回 `10002 accessToken过期或参数异常`。
- 使用 appKey/appSecret 刷新 token 成功，token 接口返回 `200`。
- 刷新 token 后再次请求播放地址返回 `60019 加密已开启`。
- 结论：A 方案接口链路可达，token 刷新逻辑必要；当前设备需要传 `code` 视频加密密码/验证码后才能继续验证播放地址。

2026-06-24 华北另一台录像机播放探针：

- 测试账号：华北。
- 测试录像机：`GH6808095`。
- 通道：1。
- 接口：`/api/lapp/v2/live/address/get`。
- 参数：`protocol=2`、`type=1`、`quality=2`、`expireTime=600`、`supportH265=0`。
- 返回：`200 操作成功`。
- 播放地址路径形态：`/v3/openlive/GH6808095_1_2.m3u8`。
- 浏览器测试页可加载 HLS，触发 `loadedmetadata` 和 `playing`，视频尺寸 `512x288`。
- 实际视频画面为萤石提示图：“视频编码类型非 H264，请检查设备的视频编码类型，并将视频编码类型修改为 H264”。
- 结论：A 方案已验证可获取 HLS 且浏览器可播放；当前卡点转为设备/码流编码兼容问题。后续应优先验证 `quality=1/2`、编码格式查询接口、或 EZUIKit/ezuikit-flv 对 H265 的支持。

### 7.11 已补充：失效播放地址

用户提供的萤石云文档片段确认存在“失效播放地址”接口，用于让已获取的播放地址失效。

接口：

```text
POST https://open.ys7.com/api/lapp/v2/live/address/disable
Content-Type: application/x-www-form-urlencoded
```

请求参数：

```text
accessToken
deviceSerial
channelNo
urlId
```

关键字段理解：

- `deviceSerial`：设备序列号。
- `channelNo`：通道号，默认 1；本项目应显式传。
- `urlId`：播放地址 ID，来自获取播放地址接口返回的 `data.id`，或播放 URL 中的 `id` 参数。

返回结构：

```json
{
  "msg": "Operation succeeded",
  "code": "200"
}
```

权限要求：

```text
"Permission": "Get"
"Resource": "dev:序列号"
```

对本项目的产品价值：

- 如果 H5 第一版采用 `/api/lapp/v2/live/address/get` 获取播放 URL，那么该接口就是对应的播放地址回收能力。
- 它比“删除直播流”更贴近第一版主链路，因为第一版可能完全不创建 `streamId`。
- 如果播放地址有效期设置较长，后台可以用该接口主动失效不再使用的地址。

当前判断：

- 第一版仍建议以较短 `expireTime` 为主要安全边界，避免强依赖移动端退出页面时一定能调用失效接口。
- 前端可以在明确的“返回列表/关闭播放”动作上触发后端失效，但不能把它作为唯一清理机制。
- 后端可以记录最近一次 `urlId`、通道、过期时间，用于排障和必要时主动失效；是否落库取决于探针后播放地址有效期策略。

待实测点：

- 失效直播地址后，正在播放的 H5 是否立即断流。
- 失效回放地址后，回放是否立即停止。
- 同一个 `urlId` 重复失效时返回什么，是否可以按幂等处理。
- `Permission=Get` 是否足够，当前四个区域账号是否具备。
- 失效接口是否影响同一通道其他正在播放的地址。

### 7.12 已补充：获取直播流播放地址

用户提供的萤石云文档片段确认存在“获取直播流播放地址”接口。该接口用于通过 `streamId` 获取直播流播放地址，是前面 `/api/service/media/streammanage/*` 直播流管理链路的播放出口。

接口：

```text
POST https://open.ys7.com/api/service/media/streammanage/stream/address
Content-Type: application/x-www-form-urlencoded
```

注意：文档标题写“获取直播流播放地址（GET）”，但请求方式和 curl 示例均为 `POST`。第一版探针应按 `POST` 验证。

请求参数：

Header：

```text
accessToken
Content-Type
```

Body：

```text
streamId
protocol
quality
supportH265
mute
type
expireTime
```

关键字段理解：

- `streamId`：直播流 ID，来自“创建直播流”接口，文档类型写 `long`，实现中应统一按字符串处理。
- `protocol`：
  - `1`：HLS。
  - `2`：RTMP。
  - `3`：FLV。
- `quality`：
  - `1`：高清主码流，默认。
  - `2`：流畅子码流。
- `supportH265`：是否支持 H265。
- `mute`：是否静音。
- `type`：
  - `1`：播放地址，默认。
  - `2`：推流地址，仅 RTMP 接入直播流生效。
- `expireTime`：过期时间，文档说明对 RTMP 接入直播流生效，默认 730 天，最大 730 天。

返回结构：

```json
{
  "data": {
    "address": "rtmp://rtmp02open.ys7.com:1935/v3/openlive/FR1229491_1_1?sid=787305182210818048..."
  },
  "meta": {
    "code": 200,
    "message": "操作成功",
    "moreInfo": null
  }
}
```

对本项目的产品价值：

- 它补齐了 streamId 直播流管理链路：创建直播流 -> 获取直播流播放地址 -> 可选查询/修改/禁用/删除。
- 如果直接接口 `/api/lapp/v2/live/address/get` 不能满足某些直播场景，该接口可以作为备选直播链路。
- 该链路可能更适合未来做直播资源管理、推流记录、Webhook 状态观测和诊断。

当前判断：

- H5 第一版仍优先 Spike `/api/lapp/v2/live/address/get`，因为它直接按设备和通道取地址，链路更短。
- 同时应把 streamId 链路作为 B 方案探针：
  1. 创建直播流。
  2. 用 `streamId` 获取 HLS 播放地址。
  3. H5 试播。
  4. 删除直播流。
- 如果 A 方案直接取地址可用，第一版不需要引入 streamId 资源状态管理。
- 如果 A 方案在权限、稳定性、回放兼容性上受限，再评估 B 方案。

待实测点：

- `protocol=1 HLS` 返回的地址是否能在移动端 H5 播放。
- `quality=2` 是否可降低延迟/带宽。
- `expireTime` 对设备接入直播流是否生效，还是只对 RTMP 接入生效。
- 同一 `streamId` 多次获取地址是否返回同一个地址，还是每次新地址。
- 删除或禁用 `streamId` 后，已获取的播放地址是否立即失效。
- B 方案相比 A 方案是否有更高延迟、更多权限要求或更高失败率。

### 7.13 已补充：视频编码格式切换

用户提供的萤石云文档片段确认存在“视频编码格式切换”接口，用于切换设备通道的编码格式。

接口：

```text
PUT https://open.ys7.com/api/v3/device/video/encodeType
Content-Type: application/x-www-form-urlencoded
```

请求参数：

Header：

```text
accessToken
deviceSerial
localIndex
Content-Type
```

Body：

```text
encodeType
streamType
```

关键字段理解：

- `localIndex`：资源/通道号，默认 1；本项目如调用必须显式传当前通道号。
- `encodeType`：`H264` 或 `H265`。
- `streamType`：
  - `1`：主码流。
  - `2`：子码流。
- 能力集校验：`support_video_encode_switch_disable=0` 表示支持切换。
- 支持托管及子账号，权限为 `Config`，设备通道级鉴权。

对本项目的产品价值：

- 该接口可能解决“H5 播放器不支持 H265 导致无法播放”的兼容问题。
- 但它是设备配置变更接口，会影响门店真实监控设备，不属于 H5 查看端的常规播放链路。

当前判断：

- 第一版 H5 不应主动调用该接口，也不应在用户查看直播时自动切换门店设备编码。
- 优先在获取播放地址时使用 `supportH265=0`，并选择更适合 H5 的协议和子码流。
- 如果实测发现某些设备只能返回 H265 且 H5 无法播放，应作为单独的运维/设备配置问题处理，而不是在播放流程里隐式修改。
- 该接口可记录为后续运维工具或诊断建议，不进入第一版 H5 产品范围。

待实测点：

- 当前设备是否存在 H265 导致 H5 无法播放的问题。
- 获取播放地址时 `supportH265=0` 是否能稳定返回 H264 可播放地址。
- 如果确需切换，设备是否支持该能力集，切换是否影响录像机主码流/子码流配置。
- 该接口是否需要额外高风险确权，错误码 `60058` 表示设备存在高风险需要确权。

### 7.14 已补充：编码格式查询

用户提供的萤石云文档片段确认存在“编码格式查询”接口，用于查询设备通道主码流或子码流的视频编码格式。

接口：

```text
GET https://open.ys7.com/api/v3/das/device/video/encode
Content-Type: application/x-www-form-urlencoded
```

请求参数：

Header：

```text
Content-Type
accessToken
deviceSerial
channelNo
```

Request：

```text
streamType
```

关键字段理解：

- `channelNo`：通道号，默认 1；本项目应显式传当前通道号。
- `streamType`：
  - `1`：主码流。
  - `2`：子码流。
- `data.videoCode`：
  - `0`：私有 H264。
  - `1`：标准 H264。
  - `2`：标准 MPEG4。
  - `3`：标准 MPEG2。
  - `4`：MJPEG。
  - `5`：标准 H265。
  - `6`：SMART264。
  - `7`：SMART265。

权限要求：

- 支持托管设备，需要授予托管权限 `CONFIG`。

对本项目的产品价值：

- 这是只读诊断接口，比“编码格式切换”安全。
- 它可以帮助判断 H5 播放失败是否与 H265 编码有关。
- 技术探针可以查询主码流和子码流编码，用于解释为什么某些设备 HLS/FLV 无法播放。

当前判断：

- 该接口可进入技术 Spike 的诊断项，但不作为 H5 第一版页面必需接口。
- 如果播放失败且返回地址本身正常，后端可用该接口辅助判断是否是 H265 兼容问题。
- 即使查询发现 H265，第一版也不自动切换编码；仍优先尝试 `quality=2` 子码流、`supportH265=0` 或播放器兼容方案。

待实测点：

- 当前四个区域账号是否具备 `CONFIG` 或托管权限，能否调用查询接口。
- 主码流和子码流是否存在不同编码，例如主码流 H265、子码流 H264。
- `supportH265=0` 获取播放地址时，萤石是否会自动选择 H264 码流。
- 查询接口的 `channelNo` 和其他 v3 接口的 `localIndex` 是否一致映射。

### 7.15 已补充：UIKit 拼接播放地址方案

用户提供的萤石社区博客说明了一种无需自研播放器的极简播放方式：使用萤石官方 `console/jssdk` 页面，并通过 URL 参数传入 `accessToken` 和 `ezopen://` 播放地址。

Web 页面模板：

```text
https://open.ys7.com/console/jssdk/pc.html?accessToken={accessToken}&url=ezopen://{verifyCode@}open.ys7.com/{deviceSerial}/{channelNo}{quality}{playType}.live&themeId={themeId}
```

H5 页面模板：

```text
https://open.ys7.com/console/jssdk/mobile.html?accessToken={accessToken}&url=ezopen://{verifyCode@}open.ys7.com/{deviceSerial}/{channelNo}{quality}{playType}.live&themeId={themeId}
```

回放模板：

```text
https://open.ys7.com/console/jssdk/mobile.html?accessToken={accessToken}&url=ezopen://{verifyCode@}open.ys7.com/{deviceSerial}/{channelNo}{quality}.rec?begin={yyyyMMddHHmmss}&end={yyyyMMddHHmmss}&themeId={themeId}
```

关键参数理解：

- `accessToken`：萤石 accessToken。
- `verifyCode@`：设备验证码；加密设备需要，不加密设备可不传。
- `deviceSerial`：设备序列号。
- `channelNo`：录像机接入时为通道号；摄像头直接接入时通常为 1。
- `quality`：
  - `.hd`：高清，取主码流。
  - 不传：流畅，取子码流。
- `.live`：直播预览。
- `.rec`：录像回放。
- `begin/end`：回放开始和结束时间，格式为 `yyyyMMddHHmmss`。
- `themeId`：如 `pcLive`、`pcRec`、`security`、`simple`。

注意：用户粘贴的博客中 H5 回放模板文字写成 `.live`，但示例使用 `.rec?begin=...&end=...`。判断应以示例和播放语义为准，回放应使用 `.rec`。

对本项目的产品价值：

- 这是最快验证 H5 能否播放的方案，可以绕过自研播放器集成成本。
- 如果直接 HLS/FLV 播放兼容性不稳定，UIKit 方案可作为第一版备选播放器承载方式。
- 该方案也能帮助判断问题在“取流/权限”还是“我们自己的播放器实现”。

风险和边界：

- URL 中会携带 `accessToken`，甚至可能携带设备验证码，不适合直接长期暴露。
- 如果使用该方案，后端应返回短期可用的跳转 URL 或内部代理入口，不能把长期 token 直接写在前端代码里。
- 使用官方页面会降低 UI 可控性，H5 详情页体验可能受萤石 UIKit 限制。
- 飞书容器内 iframe 或跳转到外部页面的兼容性需要实测。

当前判断：

- 第一版技术探针应把 UIKit 方案作为 C 方案：
  - A：`/api/lapp/v2/live/address/get` 返回 HLS/FLV，项目自有页面播放。
  - B：`streamId` 链路获取 HLS/FLV，项目自有页面播放。
  - C：萤石 `mobile.html + ezopen://` 官方 UIKit 播放。
- 推荐先验证 A 方案；如果 H5 播放器兼容性或回放能力受阻，再验证 C 方案。
- C 方案更适合作为“先跑通业务闭环”的兜底方案，而不是长期最优体验。

待实测点：

- 飞书内置浏览器能否打开 `open.ys7.com/console/jssdk/mobile.html` 并正常播放。
- H5 UIKit 对直播和回放的主题、返回、全屏、声音控制支持程度。
- 加密设备是否必须传设备验证码；验证码是否可以安全配置在后端。
- accessToken 暴露在 URL 中的安全风险是否可接受，是否需要短期 token 或后端跳转页包装。
- `.hd` 主码流和默认子码流在移动端播放体验差异。

### 7.16 已补充：Ezviz-OpenBiz GitHub 播放器资料

用户提供萤石开放平台 GitHub 组织：

```text
https://github.com/Ezviz-OpenBiz
```

该组织下与本项目 H5 直播/回放相关度最高的仓库：

1. `EZUIKit-JavaScript-npm`
   - npm 版本轻应用播放器，适配主流前端框架和自定义 UI。
   - README 明确支持低延时预览、云存储回放、SD 卡回放。
   - 初始化参数包括 `id`、`accessToken`、`url`、`width`、`height`、`template`、`staticPath` 等。
   - 支持 `ezopen://open.ys7.com/{deviceSerial}/{channelNo}.live` 和 `.rec` 形式。
   - README 提醒：hls/flv 在该包里后续不再维护，flv 建议使用 `ezuikit-flv`，hls 建议使用 `@ezuikit/player-hls`。
2. `EZUIKit-flv`
   - 开源纯 H5 FLV 播放器。
   - 支持 H264、H265、AAC、2K、多实例、手机浏览器、硬解失败自动切 wasm 软解。
   - npm 包名为 `ezuikit-flv`。
   - 需要自行部署 `decoder.js` 和 `decoder.wasm` 静态资源；仓库说明暂不提供 CDN 地址。
3. `EZUIKit-JavaScript`
   - 旧版/传统 UI 组件仓库。
   - 文档说明支持 `ezopen`、HLS、RTMP、FLV、m3u8 等地址播放。
   - 需要配置 `accessToken` 和解码器路径。

对本项目的产品价值：

- 如果浏览器原生 `<video>` 无法播放萤石返回的 HLS/FLV，播放器集成可以作为下一步。
- 如果设备返回 H265 或回放只支持 FLV/ezopen，`ezuikit-flv` 是比原生 video 更现实的技术选项。
- `EZUIKit-JavaScript-npm` 更适合完整接入萤石能力，例如预览、回放、截图、OSDTime、录制、对讲等；但第一版暂不需要这些重能力。

当前判断：

- 第一版验证顺序调整为：
  1. 先验证 A 方案接口是否返回播放 URL。
  2. 如果是 HLS 且浏览器原生可播，继续用原生 `<video>`，实现最轻。
  3. 如果 HLS 原生不可播或回放需要 FLV/ezopen，再评估接入 `EZUIKit-JavaScript-npm` 或 `ezuikit-flv`。
  4. 如果希望最快业务闭环，也可用萤石官方 `mobile.html` 跳转页作为临时方案。
- 不建议一开始就引入完整 EZUIKit，因为会增加静态解码资源、播放器生命周期、移动端兼容和安全处理复杂度。

待实测点：

- 返回 HLS URL 时，飞书内置浏览器和主流移动浏览器是否能原生播放。
- 返回 FLV URL 时，`ezuikit-flv` 在飞书 H5 中是否稳定，decoder 静态资源加载是否受限制。
- `EZUIKit-JavaScript-npm` 对加密设备错误码、验证码传参、回放 `.rec` 是否更友好。
- 解码库资源应托管在公司静态资源路径还是应用自身 `/assets` 下。
- accessToken 是否必须进入前端播放器；如果必须，必须使用短期 token 或短期播放会话，不能暴露长期 token。

### 7.17 已补充：海康 ISAPI 取流路线

用户补充：公司研发还有另一种取流方式，使用海康 ISAPI 取流。

这条路线与前面萤石云方案不同：

- 萤石云方案依赖 `open.ys7.com`、萤石账号、accessToken、Real/Replay 权限、设备验证码等。
- 海康 ISAPI 方案通常直接面向海康设备/NVR/DVR，依赖设备网络可达、设备账号、ISAPI 能力、RTSP/HTTP 流或播放代理。

对本项目的潜在价值：

- 可能绕过萤石云播放地址接口、账号权限、云端频控、加密设备 `60019` 等问题。
- 如果门店录像机在公司网络或专线/VPN 内可访问，ISAPI 可能更接近设备原生能力。
- 对回放、通道列表、设备状态等能力，ISAPI 也可能比萤石云开放平台更完整。

主要风险：

- H5 用户通常无法直接访问门店内网设备，必须由后端或边缘服务代理。
- 不能把录像机公网地址、设备账号密码、RTSP/ISAPI 凭据暴露给前端。
- 如果要给 H5 播放，后端可能需要做流代理、协议转换或转码，例如 RTSP -> HLS/FLV/WebRTC。
- 运维复杂度更高：门店网络、端口映射、NAT、防火墙、设备账号、证书、并发和带宽都要纳入设计。
- 安全风险高于萤石云托管方案，需要严格权限、审计、限流和隔离。

当前判断：

- ISAPI 应作为 D 方案进入技术调研，但不应在缺少研发现有实现细节时直接替代萤石云方案。
- 若公司研发已有稳定 ISAPI 取流服务或网关，优先复用该服务，而不是在本项目里重新直连门店录像机。
- 如果只是设备级接口能力，没有现成服务，则第一版 H5 仍优先验证萤石云 A 方案；ISAPI 作为后续备选或公司内网部署增强。

需要向研发确认的问题：

- 他们的 ISAPI 方案是直接调设备，还是已有后端网关/流媒体服务。
- 输入是什么：门店 ID、录像机序列号、通道号，还是设备 IP、端口、账号。
- 输出是什么：RTSP、HLS、FLV、WebRTC、截图，还是仅设备信息。
- 是否支持回放，回放按什么时间格式和通道标识查询。
- H5 是否能直接播放输出，还是需要播放器/转码。
- 设备凭据如何保存，是否已有密钥管理和审计。
- 门店网络如何打通，是否依赖 VPN/专线/公网映射。
- 并发、延迟、带宽和失败重试策略。

### 7.18 已补充：研发参考项目 nvr-proxy 评估

用户提供研发参考项目：

```text
/Users/sylar/Downloads/nvr-proxy
```

该项目是一个 Go 版海康 NVR WebSocket 视频播放 Demo，核心定位是“后端代理 NVR 设备流，前端通过 WebSocket 接收并播放”。

项目关键结构：

- `main.go`
  - 注册 `/ws/nvr` WebSocket 代理入口。
- `internal/nvrclient/client.go`
  - 调海康 ISAPI 登录能力。
  - 调 `/ISAPI/Security/sessionLogin/capabilities` 获取 challenge/salt/sessionID。
  - 按海康规则对密码做 SHA256 challenge 加密。
  - 调 `/ISAPI/Security/sessionLogin` 登录。
  - 调 `/ISAPI/Security/adminAccesses` 获取设备端口。
  - 调 `/ISAPI/Security/token?format=json` 获取播放 token。
  - 维护 WebSession Cookie，token 失效时可重新登录。
- `internal/handler/nvr_handler.go`
  - 接收浏览器 WebSocket 请求。
  - 后端获取 NVR token。
  - 连接 NVR WebSocket 端口。
  - 将浏览器 WebSocket 与 NVR WebSocket 做双向代理。
  - 首次收到设备 JSON 后发送 `realplay` 或 `playback` 命令。
- `internal/handler/picture_handler.go`
  - 通过同一 NVR WebSocket 链路抓取视频帧。
  - 解析 RTP payload 和 H265 NALU。
  - 用 ffmpeg 从 H265 帧中提取 JPEG。

该项目的播放链路不是“返回 HLS/FLV URL”，而是：

```text
浏览器 WebSocket
-> Go 代理服务
-> NVR WebSocket 端口
-> 后端转发二进制视频流
-> 前端播放器/JSMpeg/Canvas 或自定义解码
```

直播命令形态：

```json
{
  "sequence": 0,
  "cmd": "realplay",
  "url": "live://soyoung:dev/{channelId}/{streamType}"
}
```

回放命令形态：

```json
{
  "sequence": 0,
  "cmd": "playback",
  "startTime": "2026-06-24T10:00:00Z",
  "endTime": "2026-06-24T10:30:00Z",
  "url": "live://soyoung:dev/{channelId}/{streamType}"
}
```

对本项目的价值：

- 证明公司已有“后端代理设备/NVR WebSocket 流”的可行样例。
- 证明 ISAPI 路线可以支持直播和回放，不局限于截图。
- 代码里已经处理了海康 session 登录、token 获取、token 失效重登、NVR WebSocket 连接、首帧截图等关键技术点。
- 对“最近截图刷新”也有启发：可通过 NVR WebSocket 抓帧并用 ffmpeg 提 JPEG，而不依赖萤石抓图接口。

重要边界：

- 参考项目是 Demo，不是可直接进生产的服务。
- 项目配置中存在硬编码 NVR 地址、端口、用户名和密码；本项目不能照搬这种方式，必须使用密钥管理或受控配置。
- 项目输出的是 WebSocket 视频流，不是 HLS/FLV 播放地址；如果要接入我们的移动 H5，需要选择前端解码播放器，或增加服务端转 HLS/FLV/WebRTC。
- 项目依赖 NVR WebSocket 端口和网络可达；公司 K8s 服务是否能访问门店 NVR/FRP 地址需要确认。
- 截图依赖 ffmpeg，当前公司容器已有 poppler，但是否有 ffmpeg 需要另行确认。
- WebSocket 长连接会带来并发、带宽、连接清理和权限控制问题，不能和普通 HTTP API 等同处理。

当前判断：

- `nvr-proxy` 对我们非常有参考价值，但更适合作为独立“取流服务/媒体代理服务”的原型，而不是直接揉进当前空间资源 Go 服务。
- 如果公司研发能提供稳定的 NVR WebSocket 代理服务，本项目 H5 可只对接一个内部播放 API，避免自己维护设备账号和 ISAPI 细节。
- 如果没有现成服务，第一版不建议直接在当前项目里实现完整 ISAPI 代理；可以先做 Spike，验证一个门店、一台 NVR、一个通道的直播 WebSocket 和截图。
- 萤石云 A 方案仍适合互联网/云端快速验证；ISAPI D 方案更适合公司内网、设备原生和长期可控播放能力。

建议下一步向研发确认：

- 这个 `nvr-proxy` 是否只是 Demo，还是已有线上服务版本。
- 设备接入方式是否统一走 FRP/VPN/专线，K8s 服务是否能访问。
- 真实生产中的 NVR 账号密码如何管理。
- 前端是否已有 JSMpeg/EZUIKit/其他播放器方案。
- WebSocket 输出编码到底是 MPEG1、H264 还是 H265；当前代码截图部分明显处理 H265。
- 是否有办法把该 WebSocket 流转成 HLS/FLV/WebRTC，供移动 H5 更稳定播放。

需要真实账号和测试录像机验证：

1. A 方案：用 `/api/lapp/v2/live/address/get` 获取直播播放地址。
2. A 方案：H5 是否能直接播放返回的 HLS/FLV/ezopen 地址。
3. B 方案：创建 `streamId` 后，用 `/api/service/media/streammanage/stream/address` 获取直播播放地址。
4. B 方案：H5 是否能直接播放返回的 HLS/FLV 地址。
5. C 方案：萤石 UIKit `mobile.html + ezopen://` 是否能在飞书 H5 中播放。
6. D 方案：如果研发已有 ISAPI 服务，验证其输入、输出和 H5 可播放性。
7. 是否必须使用萤石官方 JS/H5 播放器。
8. 回放片段 A 方案：验证 `/api/lapp/video/by/time`。
9. 回放片段 B 方案：验证 `/api/v3/device/local/video/unify/query`，尤其是 `isQueryByNvr=1`。
10. 用 `/api/lapp/v2/live/address/get` 获取某个录像片段或时间段的回放播放地址。
11. 回放地址在 H5 是否能播放。
12. 播放地址有效期和刷新方式。
13. 失效播放地址后，直播/回放是否立即中断，以及是否可幂等调用。
14. 离线设备是否仍可查历史录像。
15. 无录像、无权限、设备离线、通道不存在时的错误码和返回结构。
16. 播放开始时同步触发抓图是否会触发接口频控。
17. 播放失败或 H265 可疑时，编码格式查询是否能辅助定位。

## 8. 建议先做的技术 Spike

先不要直接开发完整页面。建议先做一个萤石播放探针，验证接口能力。

探针输入：

```text
accountName
deviceSerial
channelNo
date
startTime / endTime（如接口需要）
```

探针输出：

- accessToken 是否获取成功。
- `/api/lapp/v2/live/address/get` 直播地址接口原始脱敏响应。
- 直播播放地址类型和有效期。
- `/api/service/media/streammanage/stream` 创建直播流原始脱敏响应。
- `/api/service/media/streammanage/stream/address` 直播流播放地址原始脱敏响应。
- 萤石 UIKit H5 拼接 URL 的脱敏样例和飞书容器播放结果。
- `/api/lapp/video/by/time` 录像片段查询原始脱敏响应。
- `/api/v3/device/local/video/unify/query` 本地/NVR 录像片段查询原始脱敏响应。
- `/api/lapp/v2/live/address/get` 回放地址接口原始脱敏响应。
- 回放播放地址类型和有效期。
- `/api/lapp/v2/live/address/disable` 失效地址接口原始脱敏响应。
- `/api/v3/das/device/video/encode` 编码格式查询原始脱敏响应。
- 关键错误码和错误消息。

探针原则：

- 不输出 appSecret。
- 不输出 accessToken。
- 不把真实播放 URL 写入仓库。
- 只在本地或受控环境执行。
- 保存脱敏样例，用于后续固化字段解析。

## 9. 预期后端接口草案

如果 Spike 通过，后端可新增 H5 专用 API：

```text
GET  /api/h5/orgs/{externalOrgId}/monitor
GET  /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}
POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url
GET  /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments?date=YYYY-MM-DD
POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url
```

接口必须校验：

- `externalOrgId` 存在。
- 用户有该机构访问权限（后续接飞书授权）。
- `channelId` 属于该机构。
- 通道有效且已确认。
- 萤石账号配置存在。


## 10. 当前结论（2026-06-25 验证后更新）

从系统主数据和当前工程基础看，本模块具备落地基础。

萤石云播放链路已通过实测验证，关键结论如下：

- **直播地址获取**：`POST /api/lapp/v2/live/address/get` 稳定可用，`protocol=4`（FLV）可获取播放 URL。
- **回放地址获取**：同一接口 `type=2` + `startTime/stopTime` 可获取回放 FLV URL，实测 `AZ3988334` 通道 1 回放可播。
- **回放片段查询**：`GET /api/v3/device/local/video/unify/query` 可用，返回录像片段列表。
- **H265 视频**：`ezuikit-flv@2.1.1` 支持 H265 硬解 + wasm 软解兜底，实测 `1920x1080 H265` 可播。
- **声音方案**：原始流音频编码为 G.711（FLV audio codec idx 7），`ezuikit-flv` 不支持。开启萤石云 AAC 转码接口后，音频变为 `AAC / mp4a.40.2 / 16000Hz`，播放器可正常解码。
- **错误码和频控**：萤石接口错误码通过 `code` 字段返回，token 过期（10002/10014）需自动刷新重试。
- **地址过期**：播放 URL 有 `expireTime`，过期后返回 404，需重新获取。

仍需后续确认的点：

- 多门店、多通道并发播放时的萤石云路数限制和频控策略。
- 飞书内置浏览器对 `ezuikit-flv` wasm 解码的兼容性。
- AAC 转码接口的计费政策（文档标注限时免费，需确认正式收费时间）。

## 11. 播放器选型与声音方案（2026-06-25 确定）

### 11.1 播放器选型

对比了萤石官方 GitHub 两个播放器：

| 维度 | ezuikit-flv (EZUIKit-flv) | EZUIKit-JavaScript-npm |
|---|---|---|
| 定位 | 纯 H5 FLV 播放器 | 完整萤石播放器体系 |
| AAC 音频 | 支持，实测通过 | 支持，有 openSound/closeSound |
| G.711 原始音频 | 不支持 | 未明确，大概率不支持 |
| H265 | 支持，硬解+wasm 兜底 | 支持 |
| 回放 | 支持 FLV 回放 | 自带回放时间轴/片段 UI |
| 体积 | index.js 1.9MB + wasm 1.1MB | 更大，含完整 UI |
| 安全 | 只暴露播放 URL | 需前端持有 accessToken |
| 对讲/云台/截图 | 不支持 | 支持 |
| 维护状态 | 活跃 2.1.1 | 活跃，但 hls/flv 不再维护 |

**结论：第一版选用 `ezuikit-flv`。**

理由：

1. 当前产品边界不需要对讲、云台、截图等重能力。
2. 只暴露播放 URL 不暴露 accessToken，安全风险更低。
3. FLV 直播和回放均已实测通过。
4. 后续如需完整萤石能力再评估切换。

### 11.2 声音方案

**根因**：萤石录像机原始音频编码为 G.711（FLV codec idx 7），`ezuikit-flv` 只支持 AAC，原始流无声音。

**解法**：调用萤石云 AAC 转码接口，将设备音频转码为 AAC。

接口：

```text
POST https://open.ys7.com/api/service/media/aac/transfer?enable=1
Header: accessToken / deviceSerial / localIndex
```

关键注意：

- `accessToken`、`deviceSerial`、`localIndex` 必须放在 **Header** 中，不能放 body。
- `enable=1` 放在 query string 中。
- 接口返回 `meta.code=200` 表示成功。
- 文档标注限时免费，正式收费前会通知。

验证结果：

| 状态 | 开启前 | 开启后 |
|---|---|---|
| hasAudio | false | true |
| audioCodec | null | mp4a.40.2 (AAC) |
| audioSampleRate | null | 16000 |
| 播放器报错 | Unsupported audio codec idx: 7 | 无 |

### 11.3 产品交互方案

- 默认 `muted=true` 静音自动播放（绕过浏览器自动播放限制）。
- 取流时 `mute=0`（不服务端静音，保证音频轨存在）。
- 页面显著显示"点击开启声音"按钮。
- 用户点击后取消静音，同一次 H5 会话内记住选择。
- 后端在首次播放前确保该通道 AAC 转码已开启，成功后入库标记。

### 11.4 并发限制

萤石云限制：

- 标清（256Kbps）：同时 20 路。
- 高清（512Kbps）：同时 10 路。

产品规划：

- 普通用户：最多 15 路同时播放。
- 管理员：额外 5 路（共 20 路）。
- 后端需做并发路数计数和限制。
- 播放结束或离开页面时主动失效播放地址。

### 11.5 后端待开发能力

```text
ensureChannelAacTransfer(deviceSerial, channelNo)
  - 首次播放前调用萤石 AAC 转码接口
  - 成功后入库标记，不重复调用
  - 失败时返回明确错误，不阻塞视频播放

getLiveUrl(deviceSerial, channelNo)
  - protocol=4, type=1, supportH265=1, mute=0
  - 返回带有效期的 FLV 播放 URL

getPlaybackUrl(deviceSerial, channelNo, startTime, stopTime)
  - protocol=4, type=2, supportH265=1, mute=0
  - 返回带有效期的回放 FLV URL

getRecordSegments(deviceSerial, channelNo, date)
  - 调用 /api/v3/device/local/video/unify/query
  - 返回当天录像片段列表

concurrencyControl(userId, externalOrgId)
  - 按用户计数当前活跃播放路数
  - 普通用户上限 15，管理员上限 20
  - 离开页面或主动失效时释放计数
```

## 12. 移动端 H5 直播调通结论（2026-06-29）

### 12.1 验证结果

公司线上试点门店移动端复测通过：实时视频可以在 iPhone 微信 H5 内显示。

已跑通链路：

```text
萤石云 live/address/get
-> protocol=4 FLV URL
-> ezuikit-flv@2.1.1
-> mobile wasm decode
-> canvas render
-> iPhone 微信 H5 可见画面
```

这说明第一版 H5 监控查看端可以继续沿用 `ezuikit-flv`，暂不需要因为移动端黑屏立刻切到萤石完整 UIKit 或公司 ISAPI 代理。

### 12.2 关键播放器配置

移动端不要走 HLS/native video，也不要依赖 MSE/WebCodecs：

```ts
{
  autoPlay: true,
  muted: true,
  isLive: true,
  autoWasm: true,
  useMSE: false,
  useWCS: false,
  forceNoOffscreen: true,
  hasVideo: true,
  hasAudio: true,
  keepScreenOn: true,
  wasmDecodeErrorReplay: true,
  wasmDecodeAudioSyncVideo: true,
  scaleMode: 2,
  videoBuffer: 1,
  themeData: null,
  mutedShowAutoReload: false
}
```

桌面端仍可保留 MSE 路径，移动端使用 `mobile-wasm`。

### 12.3 诊断经验

黑屏排查必须分层，不要把“流连接成功”误判为“画面成功”：

| 阶段 | 可信信号 | 说明 |
|---|---|---|
| 播放地址返回 | H5 API 返回 FLV URL | 只说明后端取流地址成功 |
| decoder 资源加载 | `decoder.js` 200、`decoder.wasm` 200 且 MIME 为 `application/wasm` | 排除 wasm 资源部署问题 |
| 流连接成功 | `streamSuccess` | 只说明 FLV 流连接成功，不代表画面可见 |
| 视频解析/渲染 | `videoFrame`、`firstFrameDisplay`、`playToRenderTimes` | 才能视为首帧/画面渲染成功 |

页面级诊断必须放在播放器黑框外。播放器内部 canvas/video/第三方 DOM 可能遮挡自定义错误层，导致手机端看起来没有任何提示。

### 12.4 后续产品化建议

- 试点期保留状态卡或折叠式诊断入口，便于现场截图定位。
- 回放播放也复用同一套 `stream-connected` 与 `first-frame-ready` 拆分逻辑。
- 如果扩门店后个别通道黑屏，优先看状态卡事件停在哪一层，再决定是账号/取流、decoder、编码、播放器渲染还是设备端问题。
- 不建议为了移动端播放统一关闭录像机 H265；当前已经证明 H265 + wasm 软解路径可跑通。
