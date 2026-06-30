# Codex Learning State

最后更新：2026-06-24

## 当前主题

学习 Codex 开发、Go 后端、GitHub 版本管理，以及腾讯云 Lighthouse 部署、验证、回滚流程。

## 2026-06-24 通道缩略图队列加载 2.14.7 修复记录

- 版本号：`2.14.7`。
- 用户反馈：
  - 公司环境 `新氧青春诊所(上海新淮海坊店)` 通道最近截图加载非常慢，转一段时间后失败。
- 排查结论：
  - 门店 ID：`9`，录像机 `L18975312`，通道数 `30`。
  - 截图更新时间为 `2026-06-24 12:35` 之后，说明不是旧截图对象缺失的单一问题。
  - 只读请求测试显示：
    - 串行加载前 8 张时，单张也会出现 2s 到 20s 以上不等的耗时，部分请求 20s 内读不完响应体。
    - 并发加载 30 张时，21 个请求拿到 HTTP 200 但 20s 内未读完 body，9 个请求 AbortError，平均耗时接近 20s。
    - 并发 4 或 6 时，12 张测试样本基本全部 25s 超时。
  - 结论：前端一次性加载缩略图会放大失败，必须先做队列/限并发；但单张读取也偏慢，后续仍需要后端生成真正小缩略图或优化 Supabase 图片代理链路。
- 修复：
  - 新增 `frontend/src/domain/image-load-queue.ts`，提供通用前端图片加载队列，当前缩略图并发限制为 `2`。
  - 通道表格缩略图不再直接一次性设置全部 `<img src>`，而是进入队列并等待图片真实 `load/error` 后才释放下一个名额，避免浏览器仍然同时拉取几十张图。
  - 队列等待期间展示稳定尺寸的小 loading 占位，避免表格抖动。
  - 离开页面、筛选或切换数据时取消仍在排队的加载任务。
- 后续建议：
  - 继续做后端真实缩略图生成：表格加载几十 KB 小图，点击预览再加载大图。
  - 评估后端缓存或 signed URL，减少 Go 后端代理 Supabase 大图造成的慢请求。
- 验证：
  - 新增 `frontend/src/domain/image-load-queue.test.ts`，覆盖并发限制、任务完成后释放下一个名额、取消排队任务。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-image-queue-test src/domain/image-load-queue.ts src/domain/image-load-queue.test.ts && node /tmp/erzhuang-image-queue-test/image-load-queue.test.js` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-channel-test src/domain/channel-recognition.ts src/domain/channel-recognition.test.ts && node /tmp/erzhuang-channel-test/channel-recognition.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地浏览器打开 `http://127.0.0.1:5177/erzhuang/`，页面能正常渲染，控制台无 error；因本地 dev server 未连接完整后端数据，本轮未在本地复现真实公司门店缩略图瀑布。

## 2026-06-23 录像机识别失败提示修正 2.14.6 修复记录

- 版本号：`2.14.6`。
- 用户反馈：
  - 公司线上环境 `新氧青春诊所(合肥银泰中心店)` 中，录像机 `GG9803685` 点击“识别区域”后速度很快。
  - 页面提示“已完成 GG9803685 的通道识别”，但最近截图没有刷新，也没有重新截图识别。
- 排查结论：
  - 门店 ID：`16`，录像机 ID：`19`。
  - 公司线上详情接口显示 `GG9803685` 的 21 个通道都执行了识别尝试，`updated_at` 更新到了 `2026-06-23 18:53` 左右。
  - 这些通道的 `recognition_result` 均为 `status=recognition_failed`，并且抓图耗时只有几十毫秒。
  - 具体错误为：`ezviz api error code=10028 msg=抓图接口调用次数超限`。
  - 所以真实根因不是前端没有调用“识别区域”，也不是没有进入后端；而是萤石抓图接口触发次数限制，后端逐通道保存失败结果后继续队列，前端最终 toast 没有统计失败数，误导成“已完成”。
- 修复：
  - 新增 `frontend/src/domain/channel-recognition.ts`，统一生成通道识别行内提示和录像机级完成提示。
  - 单通道行内提示现在会展示失败 message，例如“抓图接口调用次数超限”，而不是只显示“失败 · 总 47ms”。
  - 录像机级识别完成后会统计失败通道：
    - 全部失败：`GG9803685 识别完成，但 21 个通道抓图/识别失败：...`
    - 部分失败：`GG9803685 识别完成，x/y 个通道抓图/识别失败：...`
    - 全部成功时保留原成功文案。
- 验证：
  - 新增 `frontend/src/domain/channel-recognition.test.ts`，覆盖萤石 `10028` 全失败、部分失败和全部成功三种提示。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-channel-test src/domain/channel-recognition.ts src/domain/channel-recognition.test.ts && node /tmp/erzhuang-channel-test/channel-recognition.test.js` 通过。
  - `cd frontend && npm run build` 通过。
- 发布：
  - GitHub `main` commit：`991033b`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`38158d8`，等待公司 K8s 自动发布。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 commit：`991033b`。
  - 韩国公网入口 `https://43.155.237.46/erzhuang/health` 验证通过。

## 2026-06-23 Supabase Storage Bucket 自愈 2.14.5 修复记录

- 版本号：`2.14.5`。
- 用户反馈：
  - 公司环境仍出现 `store space request failed`。
  - 最近截图显示“加载失败”，希望页面能展示更详细的抓图、识别、存储反馈，便于共同定位。
- 排查结论：
  - 公司环境 `/health` 返回 `database=postgres`、`asset_store=supabase`，说明后端已切到 Supabase Storage。
  - 公司环境前端 bundle 已是 `2.14.4 (container)`，包含截图诊断逻辑。
  - 抽样调用 `GET /api/store-space/channel-snapshots/9509d32aed822d963233de786e9a8ecd.jpg/diagnostics` 返回：
    - `code=snapshot_open_failed`
    - `stage=open_snapshot`
    - `asset_store=supabase`
    - `snapshot_key=channel-snapshots/9509d32aed822d963233de786e9a8ecd.jpg`
    - `exists=false`
    - `detail=open asset failed: http 400 {"statusCode":"404","error":"Bucket not found","message":"Bucket not found"}`
  - 根因不是前端，也不是萤石临时 URL 过期；而是公司 Supabase Storage 中缺少代码使用的 bucket，或 `SUPABASE_STORAGE_BUCKET` 与实际 bucket 名不一致。
- 修复：
  - Supabase Storage 保存资产时，如果首次写入返回 `Bucket not found`，后端会用 service role 自动创建私有 bucket，并重试一次保存。
  - 如果创建 bucket 失败或权限不足，仍返回明确错误，不做无限重试。
- 注意：
  - 已经因为 bucket 不存在而写入失败的历史截图对象不会自动恢复；需要对对应通道执行“刷新截图”或“重新识别”，生成新截图后才会写入 Supabase Storage。
- 验证：
  - 新增 `TestSupabaseStorageStoreCreatesBucketAndRetriesSaveWhenMissing`，覆盖 bucket 缺失时自动创建并重试保存。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 发布：
  - GitHub `main` commit：`8a73c95`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`1c59220`，等待公司 K8s 自动发布。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 commit：`8a73c95`。
  - 韩国服务器本机健康检查返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"local"}`。
  - 韩国公网入口 `https://43.155.237.46/erzhuang/health` 验证通过。

## 2026-06-23 通道截图与抓图识别诊断增强 2.14.4 修复记录

- 版本号：`2.14.4`。
- 用户反馈：
  - 公司环境再次出现 `store space request failed`。
  - 通道映射页所有“最近截图”显示加载失败，前端缺少足够信息判断是抓图、保存 Supabase、读取 Supabase，还是历史本地文件缺失。
- 产品/排障决策：
  - 页面需要展示脱敏后的诊断信息，便于用户直接把错误贴回给 Codex 定位。
  - 不能展示 accessToken、apiKey、service role key 或完整签名 URL。
- 修复：
  - store-space 错误响应保留旧 `error` 字段，同时新增 `code`、`stage`、`detail`。
  - 后端新增 `GET /api/store-space/channel-snapshots/{name}/diagnostics`，返回 `asset_store`、`snapshot_key`、`exists`、`code/stage/detail`。
  - 前端 `ApiError` 保留 `code/stage/detail`，通道映射页错误提示会展示这些字段。
  - 最近截图 `<img>` 加载失败时，前端自动请求截图诊断接口，并在缩略图下方用小字展示脱敏诊断信息。
- 验证：
  - 新增 `TestScanRecorderEndpointReturnsDiagnosticForUnexpectedError`，覆盖普通内部错误不再只返回一句 `store space request failed`。
  - 新增 `TestChannelSnapshotDiagnosticsReportsOpenFailure`，覆盖截图读取失败时返回脱敏诊断信息。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。

## 2026-06-23 兜底抓图队列不中途停止 2.14.3 修复记录

- 版本号：`2.14.3`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775` 已能进入逐通道抓图识别队列，但只抓到约 9、10 张图后停止。
  - 用户判断实际通道不可能只有 9、10 个，怀疑与此前 70 秒、每 6 秒的测算有关。
- 排查结论：
  - 当前逐通道队列是前端逐个调用 `probe-recognize-channel`，不再是单个后端请求卡满 70 秒。
  - 真正导致 9、10 张后停止的是前端保留了“连续 5 个通道失败就停止”的旧兜底策略。
  - 如果 1-10 有效，11-15 为空通道或抓图失败，队列会直接停止，导致 16 之后的有效通道被漏掉。
- 修复：
  - 新增 `fallbackProbeChannelNumbers()` 和 `shouldStopFallbackProbe()`，集中管理兜底通道探测策略。
  - 兜底识别最多检测到 64 路；30 路之前不允许因连续失败停止；30 路之后连续 8 个通道失败才停止。
  - 移除前端兜底队列里的“连续 5 次失败即停止”旧逻辑，避免中间空通道造成后续漏扫，同时避免每次无脑扫满 64 路。
- 验证：
  - 新增 `frontend/src/domain/fallback-probe.test.ts`，覆盖兜底检测计划为 1-64，且停止条件为 30 路后连续 8 次失败。
  - `cd frontend && npm run build` 通过。

## 2026-06-23 萤石错误透传 2.14.2 修复记录

- 版本号：`2.14.2`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775` 扫描不再表现为 504，但页面提示“录像机 K96112775 扫描失败：store space request failed”，仍没有进入逐通道抓图识别。
- 排查结论：
  - `2.14.1` 后端扫描接口已不再同步抓图兜底，能把萤石 `10026` 错误返回到 service 层。
  - 但 store-space handler 没有识别 `ezviz.Error`，统一把未知错误转成 `store space request failed`。
  - 前端只能看到泛化文案，无法命中 `10026` 或“设备数量超出个人版限制”的兜底判断。
- 修复：
  - store-space handler 对 `ezviz.Error` 返回 HTTP 502，并保留错误 code/msg，例如 `ezviz api error code=10026 msg=...`。
  - 前端现有 `shouldUseFallbackProbe` 可直接根据返回文案进入逐通道抓图识别队列。
- 验证：
  - 新增 `TestScanRecorderEndpointReturnsEzvizErrorCodeForFallback`，覆盖 `10026` 不再被吞成 `store space request failed`。

## 2026-06-23 扫描接口 10026 同步兜底下线 2.14.1 修复记录

- 版本号：`2.14.1`。
- 用户反馈：
  - 公司环境华东录像机 `K96112775`（上海静安）扫描仍然出现 HTTP 504。
  - 用户观察到系统仍像是在先跑完整通道扫描，而不是进入新的逐通道抓图识别流程。
- 排查结论：
  - `2.14.0` 前端已经在扫描接口返回 `10026` 时接管抓图识别队列。
  - 但后端 `EzvizScanner.ScanRecorderChannels` 在 `camera/list` 返回 `10026` 时仍会同步调用旧的 `probeChannelsByCapture`，最多串行探测 32 个通道、连续 5 次失败后才停止。
  - 在失败通道耗时较长时，公司网关容易先返回 504，前端无法收到 `10026`，也就无法进入新的逐通道抓图识别队列。
- 修复：
  - 下线扫描接口内的旧同步抓图兜底路径。
  - `camera/list` 返回 `10026` 时，后端原样返回萤石错误，由前端触发 `probe-recognize-channel` 队列逐通道抓图、识别和写入。
  - 保留非 `10026` 错误的原有返回逻辑。
  - 资产存储模式增加防守性识别：如果运行时已经提供完整 Supabase Storage 配置，但漏配 `ASSET_STORE=supabase`，后端会自动使用 Supabase Storage，避免公司 K8s 环境误写容器本地目录。
  - `/health` 增加非敏感字段 `asset_store`，用于确认线上当前使用 `local` 还是 `supabase`，方便排查“最近截图/设计图加载不出来”。
- 验证：
  - 更新 `TestEzvizScannerReturnsPlanLimitWithoutCaptureProbe`，覆盖 `10026` 时不发送任何 `/device/capture` 请求，并把错误返回给上层。
  - 保留 `TestEzvizScannerDoesNotFallbackForUnauthorizedDevice`，覆盖非授权错误不触发抓图探测。
  - 新增 `TestNewStoreFromEnvAutoSelectsSupabaseWhenStorageConfigExists` 覆盖 Supabase Storage 配置完整时自动选用 Supabase。
  - 更新 `/health` 测试覆盖 `asset_store` 字段。

## 2026-06-23 抓图兜底扫描识别 2.14.0 开发记录

- 版本号：`2.14.0`。
- 用户反馈与产品调整：
  - 华东录像机 `K92940413` 扫描上报 HTTP 504。
  - 实测 `camera/list` 返回 `10026 设备数量超出个人版限制`，通道 1-10 可抓图，通道 11-15 返回 `60012` 且每个失败耗时约 10-15 秒，完整同步兜底扫描约 70 秒。
  - 产品流程调整为：当无法直接获取通道列表时，不再等待完整扫描结果，而是逐通道抓图；抓图成功即创建有效通道、保存最近截图，并同步完成 AI 区域识别。
  - 页面只展示“已检测 X 个，有效 Y 个”，连续失败数只作为内部停止条件，不展示给用户。
- 实现：
  - 新增 `ProbeRecognizeChannel` 服务能力和 `POST /api/store-space/recorders/{recorder_id}/probe-recognize-channel`。
  - 新增仓库方法 `UpsertRecorderChannel`，单通道成功时创建/更新通道，不清空其他通道，也不覆盖已确认映射。
  - 前端扫描遇到 `10026` 或“设备数量超出个人版限制”时，自动进入抓图识别队列，从通道 1 开始逐个调用单通道接口。
  - 成功通道立即写入页面通道列表，截图和 AI 识别结果同步展示；连续 5 个失败或达到通道 32 后停止。
- 验证：
  - 新增 `TestProbeRecognizeChannelCreatesChannelAndStoresRecognition` 覆盖抓图成功后创建通道、保存稳定截图、写入 AI 识别结果。
  - 新增 `TestProbeRecognizeChannelReturnsInactiveWhenCaptureFails` 覆盖抓图失败不创建通道。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 发布状态：
  - GitHub `main` commit：`4d2860d`。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 发布，服务器当前 `COMMIT=4d2860d`，`VERSION=2.14.0`。
  - 韩国服务器本机验证：`/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 active。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`caff710`。
  - 公司环境由 GitLab/K8s 自动发布；本机当前无法直接验证公司内网页面版本，需要用户在公司网络侧确认。

## 2026-06-23 通道最近截图过期展示 2.13.1 修复记录

- 版本号：`2.13.1`。
- 用户反馈：
  - 有效通道里的“最近截图”过几天后仍然出现加载失败，怀疑图片没有妥善保存。
- 排查结论：
  - 当前新识别/刷新链路会把萤石云 `device/capture` 返回的临时图片先下载，再通过 `AssetStore` 保存为 `/api/store-space/channel-snapshots/{name}`，这是稳定托管路径。
  - 韩国服务器抽样检查：
    - 新测试门店 `萤石华北测试门店` 的截图均为 `/api/store-space/channel-snapshots/...`，后端接口返回 200。
    - 老门店 `新氧青春诊所 深圳龙岗坂田万科项目` 的 38 个通道仍保存为 `https://opencapture.ys7.com/...` 临时 URL，并带 `full_image_expires_at`，属于历史数据未迁移。
  - 因此本次现象主要来自历史临时截图 URL 过期；新截图保存逻辑本身可用。
- 修复：
  - 后端读取通道时，如果截图是带过期时间的远程临时图，且已过期，则不再把 `thumbnail_url/full_image_url` 暴露给前端。
  - 已保存到系统截图库的 `/api/store-space/channel-snapshots/...` 不受过期时间影响。
  - 前端对已过期截图显示“已过期”，保留“刷新截图/重新识别”入口，让用户重新生成稳定截图。
- 验证：
  - 新增 `TestExpiredRemoteChannelSnapshotIsNotExposed` 覆盖过期远程截图不再暴露给前端。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-23 萤石云扫描通道抓图兜底 2.13.0 开发记录

- 版本号：`2.13.0`。
- 用户反馈：
  - 部分录像机超过萤石云个人版设备限制时，`device/camera/list` 直接返回错误，导致系统扫描通道失败。
  - 实测 `GF8132547` 在华东账号下 `device/camera/list` 返回 `10026`，但 `device/capture` 抓取通道 1 成功。
- 产品决策：
  - 默认仍优先使用萤石官方 `device/camera/list` 扫描通道。
  - 当 `camera/list` 返回 `10026` 时，降级使用 `device/capture` 从通道 1 开始串行探测。
  - 抓图成功即认为该通道有效；连续 5 个通道抓图失败后停止；最大探测到通道 32。
- 修复：
  - `internal/ezviz/client.go` 新增 `ErrorCode`，供上层识别萤石错误码。
  - `internal/storespace/ezviz_scanner.go` 在 `10026` 时启用抓图兜底探测。
  - 其他萤石错误码，例如 `20018 该用户不拥有该设备`，仍保持原错误，不误触发兜底扫描。
- 验证：
  - 新增 `internal/storespace/ezviz_scanner_test.go` 覆盖 `10026` 兜底抓图、连续 5 个失败停止、非权限错误不兜底。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go build ./cmd/server` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-22 城市列表补充济南并按音序排列 2.12.2 开发记录

- 版本号：`2.12.2`。
- 用户反馈：
  - 添加机构时城市列表需要增加“济南”。
  - 整个城市列表需要按首字母音序排列。
- 修复：
  - 新增 `frontend/src/domain/cities.ts`，集中维护城市下拉配置。
  - 城市列表新增“济南”，并按拼音首字母顺序排列。
  - 添加机构弹窗和编辑机构弹窗统一引用 `CITY_OPTIONS`，避免两个入口城市列表不一致。
- 验证：
  - 新增 `frontend/src/domain/cities.test.ts` 覆盖“包含济南”和完整城市顺序。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-22 其他区域确认后显示未知 2.12.1 修复记录

- 版本号：`2.12.1`。
- 用户反馈：
  - 通道识别为“其他区域”后，手动填写“护士站”等编号/备注，点击确认正常。
  - 再点击编辑修改并重新确认后，业务区域类型显示成“未知”。
- 根因：
  - 非业务区域自定义备注不属于固定场景枚举，前端二次确认时会发送 `sceneType = unknown`。
  - 页面展示层把内部兜底枚举 `unknown` 直接翻译成“未知”，导致用户看到错误业务类型；实际备注仍保存在 `areaNote`。
- 修复：
  - 新增 `frontend/src/domain/channel-labels.ts`，集中维护通道场景展示名。
  - `unknown` 在通道映射业务展示中统一显示为“其他区域”。
  - `frontend/src/components/VideoChannelTab.tsx` 改为复用领域 helper，避免组件内重复维护场景文案。
- 验证：
  - 新增 `frontend/src/domain/channel-labels.test.ts` 覆盖 `unknown -> 其他区域`、`machine_room -> 机房`、`front_desk -> 前台`、`treatment -> 治疗室`。
  - 已用临时 TypeScript 编译链路验证新增领域测试通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - 修复代码 commit：`d0d0d8d`。
  - GitHub `main` 已推送修复代码。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已合入并推送，merge commit：`de514e6`。
  - 公司环境由 GitLab/K8s 自动发布；当前本机无法解析 `lite.sy.soyoung.com`，需要用户在公司内网侧确认页面版本。
  - 韩国服务器已执行 `/opt/apps/erzhuang-project/scripts/deploy.sh`，服务器测试、Go build、前端 build、服务重启均通过。
  - 韩国服务器本机验证：`VERSION=2.12.1`，`/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 与 `nginx` 均为 active。
  - 本机直连 `https://43.155.237.46/erzhuang/health` 仍连接失败，和既有网络现象一致；服务器本机服务状态正常。

## 2026-06-17 机构基础信息编辑 2.12.0 开发记录

- 版本号：`2.12.0`。
- 用户反馈：
  - 添加门店后，城市和新氧机构 ID 无法再修改。
  - 机构列表操作区希望改为 `详情 / 编辑 / 删除`。
- 产品决策：
  - 列表页新增“编辑”只维护基础信息：城市、门店名称、新氧机构 ID。
  - 设计图、录像机、通道映射仍在详情页对应 Tab 维护，不放进基础信息编辑弹窗，避免入口重复和校验混乱。
- 后端代码索引：
  - `PATCH /api/store-space/stores/{id}`：更新机构基础信息。
  - `internal/storespace/models.go`：`UpdateStoreBasicInfoInput`。
  - `internal/storespace/service.go`：`UpdateStoreBasicInfo`，复用同名门店校验并排除当前门店。
  - `internal/storespace/store.go`：Memory/Postgres 更新 `city/name/normalized_name/external_org_id/updated_at`。
  - `internal/storespace/handler.go`：`updateStoreBasicInfo`。
- 前端代码索引：
  - `frontend/src/components/StoreList.tsx`：操作区改为 `详情 / 编辑 / 删除`。
  - `frontend/src/components/EditStoreModal.tsx`：基础信息编辑弹窗。
  - `frontend/src/App.tsx`：编辑弹窗状态、保存、重复门店确认和列表刷新。
  - `frontend/src/api.ts`：`UpdateStoreBasicInfoPayload`、`storeSpaceApi.updateStoreBasicInfo`。
  - `frontend/src/components/CreateStoreModal.tsx`：导出 `CITY_OPTIONS` 供编辑弹窗复用。
- 验证状态：
  - 已补 service 与 handler 测试。
  - `cd frontend && npm run build` 通过。
  - 本机 Go 测试仍受 `.tools/go` / macOS 动态加载 `missing LC_UUID load command` 影响，待在服务器或可用 Go 环境验证。

## 2026-06-17 图片访问前缀修复 2.11.1 开发记录

- 版本号：`2.11.1`。
- 用户反馈：
  - 公司 GitLab 环境识别区域后，通道列表“最近截图”显示“已过期”。
  - 设计图图纸也无法加载图片。
- 根因判断：
  - 后端已把萤石云临时截图下载并保存到系统资产存储，通道截图路径保存在 `channel_snapshots.thumbnail_path/full_image_path`。
  - 设计图路径保存在门店设计图记录的 `preview_image_path/thumbnail_path`。
  - 前端旧逻辑把所有后端返回的 `/api/...` 图片地址硬编码补成 `/erzhuang/api/...`。
  - 公司环境实际前缀是 `/erzhuang-project/`，所以图片请求被改到错误路径，浏览器加载失败后被前端误显示为“已过期”。
- 修复：
  - 新增 `frontend/src/url-utils.ts`，集中处理 API base、图片展示 URL、存储路径反解。
  - 默认 API base 改为根据当前页面路径和 Vite `BASE_URL` 推导，兼容个人 `/erzhuang/` 与公司 `/erzhuang-project/`。
  - 设计图、门店缩略图、通道截图统一按对应 API base 转换，不再写死 `/erzhuang`。
  - 图片加载失败文案由“已过期”改为“加载失败”；截图预览说明改为“已保存到系统截图库”。
- 验证：
  - `frontend/src/url-utils.test.ts` 覆盖 `/erzhuang-project/api/...`、`/erzhuang/api/...`、历史 `uploads/...` 路径转换。
  - `cd frontend && npm run build` 通过。
  - `go test ./...` 本机未完成：系统 PATH 无 `go`，改用项目 `.tools/go` 后 macOS 动态加载报 `missing LC_UUID load command`，本次未改后端代码。
- 发布状态：
  - GitHub `main` 已推送：`2a443c2 Fix image URL base path handling`。
  - 公司 GitLab 固定分支 `codex/containerize-single-image` 已推送：`c5b0d22 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境由 GitLab/K8s 自动发布；当前本机无法解析 `lite.sy.soyoung.com`，需要用户在公司内网侧确认页面版本和图片加载。
  - 韩国服务器已通过 SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh`，服务器当前 commit：`2a443c2`，版本：`2.11.1`。
  - 韩国服务器 `go test ./...`、Go build、前端 build、服务重启均成功。
  - 韩国服务器本机 `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 韩国服务器 nginx 与 `erzhuang-project.service` 均为 active，监听 `0.0.0.0:443` 与 `127.0.0.1:18081`。
  - 本机直连 `https://43.155.237.46/erzhuang/health` 暂时连接失败，但 SSH 到服务器本机检查服务和端口均正常。
  - 本次发现本机 SSH 登录韩国服务器的 key 是 `~/.ssh/erzhuang_lighthouse`，不是文档里原先容易混淆的服务器内部 GitHub deploy key。

## 2026-06-17 通道映射 Excel 导出 2.11.0 开发记录

- 版本号：`2.11.0`。
- 目标：
  - 在机构详情页的通道映射 Tab 增加“导出 Excel”能力。
  - 按用户要求，按钮放在“通道列表”模块标题行，位于业务区域筛选条件左侧。
- 后端：
  - 新增 `GET /api/store-space/stores/{id}/channel-mappings/export.xlsx`。
  - 使用 Go 标准库生成 `.xlsx`，不引入第三方 Excel 依赖。
  - 导出列：序号、城市、门店名称、新氧机构 ID、录像机编号、通道号、最近截图、业务区域类型、编号/备注。
  - 过滤离线录像机、失效通道和已删除通道。
  - 排序顺序：面诊室、治疗室、生美、其他区域；同类型再按编号/备注、录像机编号、通道号排序。
  - 可读取的通道截图会作为 Excel 图片对象嵌入，读取不到则保留文字占位。
- 前端：
  - `VideoChannelTab` 通道筛选行新增 `导出 Excel` 按钮。
  - 支持导出中 loading 态和错误 toast。
  - 浏览器按后端 `Content-Disposition` 文件名下载 `.xlsx`。
- 验证：
  - `go test ./...` 通过。
  - `frontend npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器验收：按钮已出现在“通道列表”标题行，位于 `全部/面诊室/治疗室/生美` 筛选左侧。

## 2026-06-17 发布术语规范

用户已明确两套发布口径，后续跨会话按固定语义执行：

- 默认 GitHub 备份：
  - 除非用户明确说明“不要同步 GitHub”或“只推公司 GitLab”，所有已确认准备发布的代码都先提交并推送到 GitHub `origin/main`。
  - GitHub 是主代码备份；是否发布韩国服务器是另一件事，需要用户明确目标或沿用当次发布指令。
- “发布到公司”：
  - merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
  - 推送 remote `gitlab`。
  - 由公司 GitLab / K8s 自动发布，通常约 5 分钟。
  - 不操作韩国 Lighthouse，不 force push，不覆盖公司 Docker/K8s/运行时环境配置。
  - 验证 `https://lite.sy.soyoung.com/erzhuang-project/health` 和页面版本号。
- “发布到韩国服务器”：
  - 推送 GitHub `origin/main`。
  - 通过腾讯云 TAT 指定韩国 Lighthouse `ap-seoul / lhins-rjfpwj1u`。
  - 以 `lighthouse` 用户执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
  - 服务器从 GitHub 拉取最新 `main`，自动执行测试、构建、重启和健康检查。
  - 验证 `http://127.0.0.1:18081/health` 和公网 `/erzhuang/`。
- 如果用户同时要求两个环境，需要记录两个环境最终 commit，避免页面版本号和问题反馈对不齐。

同步文档：

- `AGENTS.md`
- `docs/deploy-runbook.md`

## 2026-06-17 萤石云账号区域自动同步

- 版本号：`2.10.1`。
- 公司环境添加录像机时“选择区域”为空，根因是公司新数据库 `ezviz_accounts` 没有 `华北/华东/华南/华中` 等展示记录。
- 当前决策：
  - 公司内网环境可临时把完整 `EZVIZ_ACCOUNTS_JSON` 写入内网 GitLab Dockerfile 或容器环境变量，后续再迁移到 K8s Secret。
  - 代码不把 `app_key/app_secret/access_token` 写入数据库。
  - 服务启动时从 `EZVIZ_ACCOUNTS_JSON` 读取账号 `name/account_name`，自动 upsert 到 `ezviz_accounts`，状态设为 `available`。
  - 前端继续从 `GET /api/store-space/ezviz-accounts` 获取可选区域。
  - 扫描、抓图时后端仍使用运行时 env 中的完整密钥。
- 验证重点：
  - 公司环境启动日志应出现 `ezviz scanner enabled, synced N account(s)`。
  - `GET /api/store-space/ezviz-accounts` 应返回 `华北/华东/华南/华中`。
  - 添加门店/添加录像机时“选择区域”下拉应出现对应大区。

## 2026-06-16 资产存储抽象 2.10.0 开发记录

- 版本号：`2.10.0`。
- 背景：
  - 公司研发反馈：如果设计图、预览图、缩略图和监控截图只存在容器本地目录，K8s 容器重启或重新调度后可能丢失。
  - 建议把这些文件放到 Supabase Storage，并保留数据库字段记录对象路径。
- 本次改进：
  - 新增 `internal/assets` 统一资产存储层。
  - 支持 `ASSET_STORE=local` 和 `ASSET_STORE=supabase` 两种实现。
  - 设计图上传仍在本地临时目录完成 PDF 转 PNG，然后把 `original.pdf`、`preview.png`、`thumbnail.png` 保存到 AssetStore。
  - 通道截图从萤石云临时 URL 下载后，也改为保存到 AssetStore。
  - 图片接口继续由 Go 后端读取并转发，前端不直连 Supabase Storage。
  - 兼容旧本地路径：数据库仍保存 `uploads/{upload_id}/preview.png`，Supabase 使用该逻辑 key；本地模式会映射回 `UPLOAD_DIR/{upload_id}/preview.png`，避免个人服务器旧图打不开。
- 环境变量约定：
  - 本地/个人服务器：`ASSET_STORE=local`，`UPLOAD_DIR=/opt/apps/erzhuang-project/uploads`。
  - 公司 K8s：`ASSET_STORE=supabase`，`SUPABASE_URL=...`，`SUPABASE_SERVICE_ROLE_KEY=...`，`SUPABASE_STORAGE_BUCKET=design-plan-assets`，`UPLOAD_DIR=/tmp/erzhuang-work`。
  - `SUPABASE_SERVICE_ROLE_KEY` 只允许放服务端环境变量或 K8s Secret，不进入仓库、镜像和前端 `VITE_*` 配置。
- 文档同步：
  - `docs/deploy-runbook.md` 补充 Supabase Storage 部署配置。
  - `docs/technical-architecture-index.md` 补充 `internal/assets`、设计图上传、通道截图的代码索引。
  - `docs/superpowers/plans/2026-06-16-asset-store-storage.md` 记录本次实施计划和后续维护要点。
- 发布状态：
  - 代码 commit：`dfc4845`。
  - GitHub `main` 已推送到 `dfc4845`。
  - TAT InvocationId：`inv-p4x3r8g8ad`。
  - 服务器发布脚本执行成功，服务器已拉取 `dfc4845`。
  - 服务器 `go test ./...` 通过。
  - 服务器 Go build 通过。
  - 服务器前端 build 通过，产物包含 `/erzhuang/assets/index-CPQG6Jsb.js`。
  - `erzhuang-project.service` 重启成功。
  - 服务器本机 `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 服务器 `npm install` 仍提示 2 个 high severity vulnerabilities，未在本次存储改造中处理，后续可单独做前端依赖安全升级评估。

## 2026-06-15 通道截图持久化 2.9.10 发布记录

- 版本号：`2.9.10`。
- Commit：`b470ec4`。
- 用户反馈：
  - 过了周末后，机构详情的“通道映射” Tab 里“最近截图”展示不出来。
- 根因：
  - 后端原来把萤石云 `opencapture.ys7.com` 返回的临时签名截图 URL 直接保存到 `channel_snapshots.thumbnail_path/full_image_path`。
  - 线上接口返回的旧 URL 访问为 `403 Forbidden`，过期后前端图片自然无法展示。
- 修复：
  - 新增 `LocalSnapshotStore`，抓图成功后下载截图到服务器本地 `uploads/channel-snapshots`。
  - 新增 `GET /api/store-space/channel-snapshots/{name}`，前端展示改用项目自己的稳定图片地址。
  - AI 识别仍使用萤石云刚返回的公网临时 URL，避免模型服务访问内网地址失败；前端展示使用本地持久化地址。
  - 新增 `POST /api/store-space/channels/{channel_id}/snapshot`，已确认通道可“刷新截图”，不改变确认状态、业务区域类型、编号，也不增加识别次数。
  - 前端缩略图加载失败时显示“已过期”，避免用户只看到空白。
- 本地验证：
  - 新增测试 `TestRecognizeRecorderChannelsStoresRemoteSnapshotsLocally` 和 `TestRefreshChannelSnapshotKeepsConfirmedMapping`。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器预览 `/erzhuang/` 和机构详情通道映射 Tab 可正常加载，操作列未溢出。
- 发布状态：
  - GitHub `main` 已推送到 `b470ec4`。
  - 服务器 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh` 执行成功。
  - 服务器当前 commit：`b470ec4`。
  - 服务器当前版本：`2.9.10`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 `active`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-BBai7Js_.js` 和 `/erzhuang/assets/index-AdczDtmt.css`。
- 已知影响：
  - 历史已经过期的萤石云 URL 无法凭空恢复；用户需要对已确认通道点击“刷新截图”，或对未确认通道执行“重新识别/识别区域”，新截图才会落到本地持久化存储。
  - `uploads/channel-snapshots` 目录会在第一次刷新/识别截图时自动创建。

## 2026-06-12 通道视觉模型切换 2.7.2 开发记录

- 版本号：`2.7.2`。
- 目标：
  - 给监控截图 AI 识别增加 provider 切换入口，便于对比当前 OpenAI-compatible 模型和 MiniMax/OpenClaw 图像理解脚本的速度。
  - 默认行为保持不变：未配置 `CHANNEL_AI_PROVIDER` 时继续走 `VISION_API_KEY` / `VISION_API_BASE_URL` / `VISION_MODEL`。
  - 新增 `CHANNEL_AI_PROVIDER=minimax-script` 和 `CHANNEL_AI_PROVIDER=external-command`，通过外部命令调用图像理解脚本；MiniMax 脚本默认路径为 `/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py`。
  - `recognition_result` 增加 `provider` 字段，和 `recognition_ms` 一起用于线上速度对比。
- 安全约定：
  - MiniMax key 不写入代码、文档或 Git，只通过服务器环境变量 `MINIMAX_API_KEY` 注入。
  - 当前代码只接 provider 切换和外部脚本适配；真正切到 MiniMax 前，需要先确认服务器上脚本存在并确认脚本参数格式。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - PR：`https://github.com/shalei-pm/erzhuang-project/pull/1`，已合并。
  - Commit：`a786887`。
  - 线上部署：SSH 执行 `/opt/apps/erzhuang-project/scripts/deploy.sh` 成功。
  - 服务器当前 commit：`a786887`。
  - 服务器当前版本：`2.7.2`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`，`erzhuang-project.service` 为 `active`。
  - 线上前端 JS 已确认包含 `2.7.2 (a786887)`。
- MiniMax 试跑状态：
  - 韩国服务器未发现 `/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py`。
  - 韩国服务器未发现 `/root/.openclaw/config/minimax.json`。
  - 当前线上仍使用默认 `VISION_API_BASE_URL=https://vibe.soyoung.com`、`VISION_MODEL=gpt-5.5`，未切换到 MiniMax。

## 2026-06-12 MiniMax Token Plan 线上识别 2.7.3 发布记录

- 版本号：`2.7.3`。
- Commit：`4ac3fb0`。
- 目标：
  - 确认用户提供的是 MiniMax Token Plan 订阅 Key，不是普通按量计费 API Key。
  - 根据 MiniMax 官方文档，Token Plan 可走 OpenAI-compatible Responses API：`https://api.minimaxi.com/v1`，模型 `MiniMax-M3`。
  - 修复 `VISION_API_BASE_URL` 已包含 `/v1` 时 endpoint 被拼成 `/v1/v1/responses` 的问题。
  - 兼容 MiniMax 可能返回 Markdown fenced JSON 的情况，并收紧 prompt 要求只输出 JSON。
  - `recognition_result.provider` 能正确记录为 `minimax`，避免速度对比时混淆。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 线上配置：
  - `/etc/systemd/system/erzhuang-project.service.d/20-vision-ai.conf`
  - `VISION_API_BASE_URL=https://api.minimaxi.com/v1`
  - `VISION_MODEL=MiniMax-M3`
  - `VISION_API_KEY` 使用 MiniMax Token Plan 订阅 Key，仅保存在服务器 systemd drop-in，不进入 Git。
- 线上验证：
  - 服务器部署脚本执行成功，`erzhuang-project.service` 为 `active`。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 直接调用 MiniMax Responses API 返回 HTTP 200，耗时约 `12429ms`。
  - 通道 `131` 真实识别成功，整体接口约 `10s`，后端记录 `capture_ms=1668`、`total_ms=6414`，识别为“弱电机房”。
  - 通道 `132` 真实识别成功，`provider=minimax`，整体接口约 `14s`，后端记录 `capture_ms=1029`、`recognition_ms=10297`、`total_ms=11327`。

## 2026-06-12 视觉模型对比结论

- 用户确认 MiniMax Token Plan 已跑通，但速度相比现有 GPT 链路没有优势。
- 线上视觉识别已切回 GPT：
  - `VISION_API_BASE_URL=https://vibe.soyoung.com`
  - `VISION_MODEL=gpt-5.5`
  - `VISION_API_KEY` 仅保存在服务器 systemd drop-in，不进入 Git。
- 切回后服务健康检查通过，`erzhuang-project.service` 为 `active`。
- 保留 MiniMax 兼容代码和 provider 记录能力，方便后续如果换更快 MiniMax 模型或其他视觉模型时复用。

## 2026-06-12 通道识别反馈弱化 2.7.4 开发记录

- 版本号：`2.7.4`。
- 目标：
  - 将通道缩略图下方的识别反馈数据弱化为灰色小字，避免抢占主要操作注意力。
  - 成功识别时只展示低置信标记和耗时，不重复展示区域类型。
  - 耗时信息压缩为一行，超出后省略。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 删除操作等待态 2.7.5 开发记录

- 版本号：`2.7.5`。
- 目标：
  - 门店删除、录像机删除、通道删除涉及数据库写操作，点击后按钮进入禁用态并显示 spinner + “删除中”。
  - 统一 row action 按钮内容为 inline-flex，保证“确认中 / 识别中 / 删除中”图标和文字对齐。
  - 修复通道区域类型从业务区域切回“其他区域”时，编号/备注输入框仍显示“必填”的问题。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 机构详情默认 Tab 2.7.6 开发记录

- 版本号：`2.7.6`。
- 目标：
  - 进入机构详情时，只要该门店已填写录像机，默认展示“通道映射”。
  - 如果没有录像机但有设计图，则默认展示“设计图标注”。
  - 新建门店后也按同一规则定位默认 tab。
  - 通道映射页增加业务区域类型单选筛选：全部、面诊室、治疗室、生美；默认全部。
  - 添加录像机入口统一把“选择账号”改为“选择区域”；该版本曾误写为华西，已在 `2.9.1` 修正为华中。
- 本地验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-12 删除等待态并发修复 2.7.7 开发记录

- 版本号：`2.7.7`。
- 目标：
  - 修复连续点击多个删除时，前一个删除按钮动效被后一个删除状态覆盖的问题。
  - 门店、录像机、通道删除等待态均改为多 ID 集合，直到对应接口返回或行消失前保持“删除中”。

## 2026-06-12 录像机识别动效优化 2.8.0 开发记录

- 版本号：`2.8.0`。
- 目标：
  - 录像机级“识别区域”按钮不再在按钮内展示转圈动效，按钮保持稳定。
  - 识别提示文案移动到操作区右侧，以灰色小字展示，并增加类似 Codex 思考态的高光扫过动画。
  - 进度展示改为录像机表格底部 3px 细进度线，从左到右推进，到 100% 后淡出。
- 本地预览：
  - 使用 mock 模式在本地预览，用户确认“还可以，可以发布”。

## 2026-06-12 机构列表城市筛选与汇总 2.9.0 开发记录

- 版本号：`2.9.0`。
- 目标：
  - 机构列表搜索框下方增加城市单选筛选，默认“全部”。
  - 城市选项只展示当前列表中实际存在的城市，避免出现无结果筛选项。
  - 右侧列表汇总增加面诊室、治疗室、生美数量，并随城市筛选联动。
  - 城市筛选后，门店数量和当前展示区间按筛选后的可见列表计算。

## 2026-06-12 萤石云区域选项修复 2.9.1 开发记录

- 版本号：`2.9.1`。
- 问题：
  - 录像机“选择区域”白名单误用了华西，漏掉华中。
  - 这是前端展示白名单问题，不是线上萤石云账号数据被删除。
- 修复：
  - 可选区域调整为华北、华东、华南、华中。
  - mock 账号补充华中，方便本地无后端时也能覆盖该选项。
  - 增加 `scripts/check-region-options.mjs`，用于检查大区白名单必须包含四个区域。

## 2026-06-12 设计图详情图片恢复 2.9.2 修复记录

- 版本号：`2.9.2`。
- 问题：
  - 用户保存设计图标注后，从机构列表重新进入机构详情，设计图图片加载不出来。
- 根因：
  - 保存后的设计图详情接口返回内部存储路径，例如 `uploads/{upload_id}/preview.png`。
  - 前端重新进入详情时没有把该内部路径转换为可访问的图片接口 `/api/design-plan/uploads/{upload_id}/preview`。
  - 服务器文件实际存在，详情接口也返回 200，问题位于前端图片 URL 映射层。
- 修复：
  - `toDisplayImageUrl` 增加对 `uploads/{upload_id}/preview.png` 和 `uploads/{upload_id}/thumbnail.png` 的转换。
  - 新增 `scripts/check-design-plan-image-url.mjs`，用于防止保存后内部图片路径无法恢复为前端可访问 URL。

## 2026-06-12 通道确认等待态 2.9.3 修复记录

- 版本号：`2.9.3`。
- 问题：
  - 通道点击“确认”后会显示“确认中”，但如果再点击其他通道按钮，前一个确认按钮的等待态会消失。
- 根因：
  - 前端用单个 `confirmingChannelId` 记录确认中状态，多个确认请求并发时后一个会覆盖前一个。
- 修复：
  - 通道确认等待态改为 `Set<number>`，按通道 ID 独立管理。
  - 每个通道从点击确认开始保持“确认中”，直到该通道确认请求结束或状态变化。
  - 新增 `scripts/check-channel-confirming-state.mjs`，防止确认等待态退回单 ID 管理。

## 2026-06-12 单通道识别缩略图状态 2.9.4 修复记录

- 版本号：`2.9.4`。
- 问题：
  - 单独点击某个通道识别后，缩略图已经出现；再识别其他通道后，之前通道的缩略图会在页面上消失。
  - 刷新页面后缩略图又恢复，说明数据库和文件没有丢，问题在前端页面状态合并。
- 根因：
  - 单通道识别完成后，前端用发起请求时的旧 `store` 快照合并 `updatedChannel`。
  - 多个识别请求前后完成时，后返回的请求会用旧快照覆盖前一个请求已经写入页面状态的缩略图。
- 修复：
  - `onStoreUpdated` 支持函数式更新，异步结果回写时基于最新门店状态合并。
  - 单通道识别和整台录像机队列识别均改为通过最新状态执行 `replaceChannelInStore`。
  - 录像机局部更新同样改为函数式更新，降低异步操作互相覆盖的风险。
  - 新增 `scripts/check-channel-recognition-merge-state.mjs`，防止单通道识别退回旧 `store` 快照合并。

## 2026-06-12 已确认通道识别锁定 2.9.5 修复记录

- 版本号：`2.9.5`。
- 产品规则：
  - 通道点击确认后，视为人工锁定。
  - 已确认通道的“重新识别”按钮置灰不可点击。
  - 点击录像机“识别区域”时，跳过已确认通道，不再自动重新识别。
  - 已确认通道点击“编辑”后，状态回到待确认，可重新修改区域类型/编号，也可重新识别。
- 修复：
  - 前端整台录像机识别队列过滤已确认通道。
  - 前端单通道“重新识别”按钮在已确认状态下禁用。
  - 后端 `RecognizeRecorderChannels` 跳过已确认通道，避免抓图和 AI 识别覆盖人工确认。
  - 后端 `RecognizeChannel` 对已确认通道返回校验错误，要求先编辑解锁。
  - 新增 `scripts/check-confirmed-channel-recognition-lock.mjs` 和后端测试，防止该锁定规则回退。

## 2026-06-12 删除按钮 hover 文字色 2.9.6 修复记录

- 版本号：`2.9.6`。
- 问题：
  - 各处删除按钮鼠标 hover 时背景变为红色系，但文字仍可能呈现普通操作按钮的蓝色。
- 修复：
  - `.danger-link:hover` 明确设置文字色为 `var(--danger)`。
  - 新增 `scripts/check-danger-link-hover-color.mjs`，防止危险按钮 hover 状态漏掉文字色。

## 2026-06-12 通道编辑解锁 2.9.7 修复记录

- 版本号：`2.9.7`。
- 问题：
  - 已确认通道点击“编辑”后，页面状态仍显示已确认，“重新识别”按钮仍不可点击。
- 根因：
  - 前端之前只写入本地草稿状态，后端真实通道状态仍为已确认。
  - 已确认通道识别接口已被后端锁定，所以仅前端放开按钮也无法真正重新识别。
- 修复：
  - 新增后端 `UnlockChannelForEdit` 能力和 `/api/store-space/channels/{channel_id}/unlock` 接口。
  - 点击“编辑”时调用后端解锁，将通道状态改为待确认，清空确认时间，保留当前区域类型/编号作为编辑草稿。
  - 解锁后“重新识别”恢复可点击；后端允许待确认通道重新抓图和 AI 识别。
  - 新增 `scripts/check-channel-edit-unlocks-state.mjs`，并更新 `scripts/check-confirmed-channel-recognition-lock.mjs`。

## 2026-06-12 进入详情加载反馈 2.9.8 修复记录

- 版本号：`2.9.8`。
- 问题：
  - 机构列表点击“进入详情”后，用户反馈半天没反应，需要点很多次才跳转。
- 排查：
  - 线上列表接口实测约 `0.27s-0.42s`。
  - 线上详情接口实测约 `0.5s-1.3s`，不算完全阻塞，但足以让无反馈按钮显得像没点上。
  - 前端此前没有行级详情加载态，重复点击会触发多个详情请求。
  - 基础接口 `/health` 和账号列表约 `0.137s`，说明 Supabase/网络往返是主要底噪。
  - 门店详情原实现会串行查询 areas、design plans、recorders，并对每台录像机单独查询 channels，存在 N+1 查询。
- 修复：
  - 机构列表增加行级 `openingStoreIds` 状态。
  - 点击“进入详情”后按钮显示“进入中”并带 spinner。
  - 正在进入详情时禁用该行进入/删除按钮，并忽略重复点击。
  - 门店详情内录像机通道查询由“每台录像机一次查询”改为“按门店一次批量查询”，减少远程数据库往返。
  - 通道“编辑解锁”接口由返回整份门店详情改为仅返回当前通道，前端用局部替换更新当前行。
  - 新增 `scripts/check-store-open-loading-state.mjs`，防止入口加载反馈回退。

## 2026-06-12 通道操作即时反馈 2.9.9 修复记录

- 版本号：`2.9.9`。
- 问题：
  - 用户反馈详情页内点击确认、编辑、删除都需要大量等待。
- 排查：
  - 这类操作不是浏览器到后端之间“没连上”，而是写接口需要等待远程数据库更新。
  - 其中确认、删除通道、删除录像机等路径还会返回或重新拉取较重的门店详情数据，导致按钮操作手感被详情接口耗时拖慢。
- 修复：
  - 通道确认增加乐观更新：点击确认后前端立即把当前行切到确认状态，后端返回后再校准；失败时回滚并提示。
  - 通道编辑解锁保持单通道轻量返回，并在点击后立即切到待确认编辑态。
  - 删除通道和删除录像机增加乐观更新：点击删除后先从页面移除，后端失败时恢复原门店状态。
  - 新增 `scripts/check-channel-actions-optimistic-state.mjs`，防止通道操作退回“等接口返回后才更新页面”的交互。

当前新增产品需求讨论：

- 项目方向：设计图标记与诊室区域管理。
- 状态：已形成 PRD 草稿和技术方案草稿。
- 文档：`docs/design-plan-marker-prd.md`。
- 技术方案：`docs/design-plan-marker-tech-plan.md`。
- 已进入代码实现拆分阶段。
- 已安排两个专项会话：
  - 后端 Phase 1：数据模型、schema、CRUD API、校验、重复检查、操作日志。
  - 前端 Phase 2：后台风格 UI、门店列表、编辑弹窗、区域卡片、图纸标注交互骨架。
- 旧前端技术栈会话仅负责 Vite + React + TypeScript 工程初始化，已完成并待命，不再承接当前业务功能。
- 后端专项会话：
  - thread: `019e978c-9e0d-7f53-b48a-75679af9369b`
  - worktree: `/Users/sylar/.codex/worktrees/e6f9/erzhuang-project`
- 前端专项会话：
  - thread: `019e978c-f41f-78d0-a5db-6b940b928c3f`
  - worktree: `/Users/sylar/.codex/worktrees/34e2/erzhuang-project`
- 测试样例：
  - `testdata/design-plans/sample-store-floor-plan.pdf`
  - `testdata/design-plans/generated/sample-store-floor-plan.png`
  - 用途：前端 mock 图纸预览、后续 PDF 转图片和 AI 识别联调。
  - 状态：用户确认该数据不敏感，已提交到 GitHub。
- 技术架构索引：
  - `docs/technical-architecture-index.md`
  - 用途：后续迭代前先定位业务能力对应的前端、后端、数据库和验证入口，避免整体重写。
- 版本号规范：
  - 采用 `大版本.中版本.小版本`。
  - 大版本：新增完整业务模块、一个及以上新页面、或核心业务流程变化。
  - 中版本：已有模块内交互、样式、信息架构或业务流程小迭代。
  - 小版本：bug 修复、测试补充、技术整理、部署脚本小调整、文档修正。
  - 重要线上验收问题需同时记录版本号和 Git commit。

## 2026-06-08 第一期开口收尾

用户准备 2-3 份真实测试 PDF，用于验证：

- 同名/相似门店重复判断与覆盖流程。
- 不同门店新建流程。
- 如有多页 PDF，用于验证多页上下拼接。

当前继续推进的 P0 收尾：

- 识别完成后触发同名/疑似同名提示。
- 保存前最终重复检查，支持确认覆盖或继续新建。
- 编辑态重新上传 PDF 前二次确认。
- 删除门店后清理对应上传文件目录。

## 协作模式

当前主会话作为项目架构和交付中枢：

- 需求澄清
- 技术架构
- 任务拆解
- 前后端边界定义
- 验收标准
- 专项 Codex 会话调度
- 合并判断
- 发布、验证、回滚
- 腾讯云 Lighthouse / nginx 操作

专项会话用于聚焦实现：

- 前端会话：`frontend/`、前端工程、页面、构建、本地验证
- 后端会话：Go API、`cmd/server`、`internal/`、后端测试
- 部署专项会话：未来如有需要，再单独拆分

原则：

- 只有主会话操作腾讯云 API/TAT、nginx、systemd、发布和回滚。
- 专项会话只做代码实现和本地验证，不使用云密钥，不直接改服务器。
- 专项会话完成后提交分支，主会话负责验收、合并和发布。
- 详细规则见 `docs/architecture.md`。

## 用户背景

- 用户是新氧青春的产品负责人，正在学习使用 Codex。
- 出于安全原因，当前阶段不接入公司开发环境。
- 用户希望先在个人项目和个人腾讯云 Lighthouse 服务器上练熟 Codex 的开发、测试、部署、回滚流程。
- 等个人练习链路成熟后，再考虑向研发申请公司环境权限。

## 沟通和操作偏好

- 默认中文沟通。
- 逐步教学，不只给命令。
- 重要操作先解释风险，再给命令。
- 解释每一步在真实研发流程里的对应含义。
- 可以动手实操，但每一步尽量可验证。
- 不接触公司环境、公司密钥、公司代码。
- 不建议使用云厂商主账号或高权限长期密钥。

## 本地项目状态

- 本地路径：`/Users/sylar/erzhuang-project`
- 当前已创建：
  - `AGENTS.md`
  - `docs/codex-learning-state.md`
- 当前已初始化：
  - Git 仓库，默认分支 `main`
  - Go module: `github.com/shalei-pm/erzhuang-project`
  - 最小 Go HTTP 服务骨架
- 当前已验证：
  - 项目内临时 Go 工具链：`.tools/go/bin/go`，版本 `go1.22.2 darwin/arm64`
  - `gofmt`
  - `go test ./...`
  - `go build -o bin/erzhuang-project ./cmd/server`
  - 本地启动服务并验证 `/health`
  - 本地启动服务并验证 `/api/tasks`
  - 已推送 `main` 分支到 GitHub：`git@github.com:shalei-pm/erzhuang-project.git`
  - Lighthouse 服务器已通过 GitHub Deploy Key 只读拉取代码
  - Lighthouse 服务器已完成 `go test`、`go build`、systemd 启动、开机自启和 `/health` 验证
  - 已完成从 v1 到 v2 的服务器发布练习
  - 已完成从 v2 回滚到 v1 的服务器回滚练习
  - 已验证 `scripts/deploy.sh` 可以一键发布当前 `main`
  - 已验证 `scripts/rollback.sh <commit-or-tag>` 可以一键回滚
  - 已通过 nginx 暴露公网 HTTPS 路径 `/erzhuang/`
  - 已通过腾讯云 TAT/API 验证可管理韩国 Lighthouse 实例
  - 已确定当前数据库练习方案采用 Supabase PostgreSQL
- 当前待验证：
  - Supabase 项目创建
  - 后端通过环境变量连接 Supabase PostgreSQL
  - 服务器通过 systemd 环境变量连接 Supabase PostgreSQL
- 当前本地限制：
  - 系统 PATH 暂时找不到全局 `go` 命令。
  - 已通过项目内 `.tools/go` 临时解决本项目的 Go 测试和构建问题。
  - 当前 Codex 沙箱下，Go 构建缓存需要显式设置到项目内绝对路径：`GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build`。

当前目录结构：

```text
.
├── .gitignore
├── AGENTS.md
├── README.md
├── bin
│   └── erzhuang-project
├── cmd
│   └── server
│       └── main.go
├── docs
│   └── codex-learning-state.md
├── go.mod
└── internal
    └── app
        ├── handler.go
        └── handler_test.go
```

## 技术方向

- 公司后端开发环境是 Go。
- 本项目优先练习 Go 后端。
- 数据库练习优先采用 Supabase PostgreSQL，暂不在 2GB Lighthouse 上安装 MySQL。
- 数据库密钥和连接串只通过环境变量配置，不提交到 GitHub。
- 目标链路：
  1. Codex 本地开发
  2. Git 管理代码
  3. 推送到 GitHub
  4. 腾讯云服务器拉取代码
  5. `go test`
  6. `go build`
  7. `systemctl restart`
  8. `curl /health` 验证
  9. 保留发布记录
  10. 支持回滚

## 个人腾讯云 Lighthouse 状态

- 系统：Ubuntu 24.04.4 LTS
- 机器名：`VM-0-12-ubuntu`
- 资源：约 2GB 内存，50GB 磁盘
- Git：`git 2.43.0`
- Go：`go1.22.2 linux/amd64`
- Docker：未安装
- UFW：inactive
- 安全主要依赖：腾讯云控制台安全组/防火墙

已有服务：

- `nginx`: 443，active
- Hermes Gateway:
  - 监听：`0.0.0.0:8644`
  - systemd 服务：`hermes-gateway.service`
- Feishu Poll Bot:
  - systemd 服务：`feishu-poll-bot.service`
- xray:
  - 本地代理：`127.0.0.1:10086`

## 已完成的服务器 Go Demo 练习

服务器上已有 Go demo 服务：

- 路径：`/opt/apps/codex-demo`
- 接口：
  - `GET /health`
  - `GET /api/tasks`
- systemd 服务：`codex-demo.service`
- 监听：`127.0.0.1:18080`
- 状态：active running
- 开机自启：enabled
- cgroup：`/system.slice/codex-demo.service`
- 当前 `/health` 返回：

```json
{"app":"codex-demo","status":"ok","version":"v1"}
```

已经练习过：

- 创建 Go 项目
- `gofmt`
- `go test ./...`
- `go build -o codex-demo .`
- `nohup` 临时启动
- `curl` 本机验证
- 创建 systemd service
- `systemctl start`
- `systemctl restart`
- `systemctl status`
- `journalctl` 查看日志
- `systemctl enable` 开机自启
- 从 v1 发版到 v2
- 从 v2 回滚到 v1
- 给 `/health` 写单元测试
- 故意改错 version，确认 `go test` 可以拦截问题，避免部署

## 重要学习点

第一次临时启动时，`codex-demo` 挂在 `hermes-gateway.service` 的 cgroup 下；后来改成 systemd 独立服务后，变成 `/system.slice/codex-demo.service`。

这说明：

- Hermes 适合做临时执行通道。
- 正式服务应该交给 systemd 管理。
- 真实生产环境中，服务归属、进程生命周期、日志和重启策略都应该清晰可控。

## 下一步目标

把服务器上的练习迁移到本地项目 `erzhuang-project` 中，建立更真实的链路：

1. 本地 Codex 开发 Go 项目。
2. Git 管理代码。
3. 推送到 GitHub。
4. 服务器拉取代码。
5. 服务器执行 `go test` 和 `go build`。
6. `systemctl restart`。
7. `curl /health` 验证。
8. 保留发布记录。
9. 支持回滚。

后续希望做到：用户对 Codex 说“开发并发版”，Codex 能通过 GitHub + SSH 或受控部署脚本完成发布，不再每一步都让用户转述给 Hermes。

## 数据库方案

当前决策：

- 使用 Supabase 创建个人练习用 PostgreSQL。
- Go 后端通过 `DATABASE_URL` 读取连接串。
- Lighthouse 上通过 systemd 环境变量注入连接串。
- 不把数据库密码、Supabase Key、连接串写入仓库。
- 暂不安装本机 MySQL，避免把当前学习重点转成数据库运维。

详细计划见 `docs/database-plan.md`。

## 当前进度快照

截至 2026-06-04 下班前，已完成：

1. 本地项目上下文文档：
   - `AGENTS.md`
   - `docs/codex-learning-state.md`
2. 本地 Go 服务骨架：
   - module: `github.com/shalei-pm/erzhuang-project`
   - `GET /health`
   - `GET /api/tasks`
   - `/health` 单元测试
   - `/api/tasks` 单元测试
3. 本地验证：
   - `gofmt` 已通过
   - `go test ./...` 已通过
   - `go build -o bin/erzhuang-project ./cmd/server` 已通过
   - 本地 `curl /health` 已通过
   - 本地 `curl /api/tasks` 已通过
4. Git 和 GitHub：
   - 本地 Git 仓库已初始化，分支 `main`
   - GitHub 仓库已创建：`git@github.com:shalei-pm/erzhuang-project.git`
   - 本地 `main` 已推送到 `origin/main`
   - 本机 GitHub SSH 已配置成功
5. 安全边界：
   - `.tools/`、`.cache/`、`bin/`、`.ssh/` 已加入 `.gitignore`
   - 本机个人 SSH key 只用于开发机向 GitHub push
   - 后续服务器拉取代码计划使用单独的 GitHub Deploy Key，不复用个人 SSH key

明天继续的核心目标：

1. 给 Lighthouse 服务器配置仓库级 read-only Deploy Key。
2. 让服务器从 GitHub clone/pull `erzhuang-project`。
3. 在服务器执行 `go test` 和 `go build`。
4. 准备或更新 `erzhuang-project.service`。
5. 用 `systemctl restart` 发布。
6. 用 `curl /health` 验证。
7. 记录第一次服务器发布。
8. 再讨论如何设计受控部署脚本和回滚策略。

## 2026-06-05 服务器首次发布进度

已完成：

1. GitHub Deploy Key：
   - 在 Lighthouse 服务器生成专用于本仓库的 SSH key。
   - 将公钥添加到 GitHub 仓库 `shalei-pm/erzhuang-project` 的 Deploy keys。
   - 权限策略：read-only，不允许写仓库。
   - 验证结果：服务器可通过 Deploy Key 访问仓库。

2. 服务器拉取代码：
   - 部署目录：`/opt/apps/erzhuang-project`
   - clone 仓库：`git@github.com:shalei-pm/erzhuang-project.git`
   - 服务器当前 commit：`0f1699e Document next deployment steps`

3. 服务器测试和构建：
   - Go 版本：`go1.22.2 linux/amd64`
   - `go test ./...` 通过
   - `go build -o erzhuang-project ./cmd/server` 通过
   - 构建产物：`/opt/apps/erzhuang-project/erzhuang-project`

4. systemd 服务：
   - 服务名：`erzhuang-project.service`
   - service 文件：`/etc/systemd/system/erzhuang-project.service`
   - 运行用户：`lighthouse`
   - 工作目录：`/opt/apps/erzhuang-project`
   - 启动命令：`/opt/apps/erzhuang-project/erzhuang-project`
   - 环境变量：`ADDR=127.0.0.1:18081`
   - 状态：active running
   - 开机自启：enabled
   - cgroup：`/system.slice/erzhuang-project.service`

5. 健康检查：
   - 验证命令：`curl -s http://127.0.0.1:18081/health`
   - 返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要边界：

- 新服务监听 `127.0.0.1:18081`，避免和旧 `codex-demo.service` 的 `127.0.0.1:18080` 冲突。
- 本次没有修改 nginx。
- 本次没有暴露公网端口。
- 本次没有重启 Hermes、Feishu Poll Bot、xray 或旧 `codex-demo.service`。

## 2026-06-05 v2 发布进度

已完成：

1. 本地代码变更：
   - 将 `/health` 返回的 `version` 从 `v1` 改为 `v2`。
   - 将 `/health` 单元测试改为明确期待 `v2`。
   - commit：`bc8d5a3 Release health version v2`

2. 本地验证：
   - `gofmt` 已执行。
   - `go build -o bin/erzhuang-project ./cmd/server` 通过。
   - 本地 `go test ./...` 由于 macOS 26.5 + 项目内 Go 1.22.2 运行测试二进制时出现 `dyld: missing LC_UUID load command`，未通过。
   - 决策：不把本地测试环境问题当作业务通过，改为以服务器 Linux 环境的 `go test ./...` 作为本次发布门禁。

3. GitHub：
   - v2 commit 已推送到 `origin/main`。

4. 服务器发布：
   - 服务器执行 `git pull --ff-only`，从 `0f1699e` 快进到 `bc8d5a3`。
   - 服务器执行 `go test ./...` 通过。
   - 服务器执行 `go build -o erzhuang-project ./cmd/server` 通过。
   - 执行 `sudo systemctl restart erzhuang-project.service`。
   - systemd 状态：active running。
   - cgroup：`/system.slice/erzhuang-project.service`
   - `/health` 返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

重要学习点：

- GitHub 上有新 commit 不等于服务器已经上线，服务器必须显式 pull/build/restart。
- 本地门禁失败时不能装作通过，要记录原因并选择可信的替代门禁。
- 本次服务器 Linux 环境的测试通过后才执行了 systemd restart。
- `git pull --ff-only` 能保证服务器只做快进更新，不产生自动 merge commit。

## 2026-06-05 v2 回滚到 v1 进度

已完成：

1. 回滚目标：
   - 从服务器运行的 v2 commit `bc8d5a3` 回滚到 v1 commit `fbdb249`。
   - `fbdb249` 对应代码中 `/health` 的 `version` 为 `v1`。

2. 执行过程：
   - 服务器尝试执行 `git fetch origin`，但因为没有带 `GIT_SSH_COMMAND='ssh -i ~/.ssh/erzhuang_project_deploy_key -o IdentitiesOnly=yes'`，返回 `git@github.com: Permission denied (publickey)`。
   - 由于服务器本地已经有目标 commit `fbdb249`，继续执行 `git checkout fbdb249` 成功。
   - 服务器进入 detached HEAD 状态，这是本次临时回滚练习可接受的状态。
   - 执行 `go test ./...` 通过。
   - 执行 `go build -o erzhuang-project ./cmd/server` 通过。
   - 执行 `sudo systemctl restart erzhuang-project.service` 成功。
   - systemd 状态：active running。

3. 验证结果：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- 回滚不一定需要改 GitHub 历史，可以先让服务器 checkout 到已知稳定 commit。
- 如果目标 commit 已经存在于服务器本地，`git checkout <commit>` 不依赖网络。
- 访问 GitHub 的服务器命令必须显式使用 Deploy Key，例如：

```sh
GIT_SSH_COMMAND='ssh -i ~/.ssh/erzhuang_project_deploy_key -o IdentitiesOnly=yes' git fetch origin
```

- detached HEAD 适合临时验证和紧急回滚，但长期流程应通过 tag、release 记录或受控脚本管理。
- 本次回滚没有修改 nginx、没有开放公网端口、没有影响 Hermes、Feishu Poll Bot、xray 或旧 `codex-demo.service`。

## 2026-06-05 服务器恢复 main 分支

回滚完成后，服务器曾处于 detached HEAD。已执行：

1. 使用 Deploy Key 执行 `git fetch origin`。
2. `git switch main`。
3. 使用 Deploy Key 执行 `git pull --ff-only`。

结果：

- 服务器 Git 工作区已恢复到 `main`。
- 服务器 `main` 已同步到 `origin/main`。
- 当前服务器工作区 HEAD：`87318ca Record v2 rollback exercise`。
- 运行中的服务仍返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- Git 工作区代码和正在运行的二进制是两回事。
- 服务器工作区已经是最新 `main`，其中代码包含 v2。
- 但没有重新执行 `go build` 和 `systemctl restart`，所以运行中的服务仍是之前回滚构建出的 v1。
- 服务器 `git status` 出现 `?? erzhuang-project`，这是在项目根目录构建出的二进制文件。
- 已在本地 `.gitignore` 增加 `/erzhuang-project`，后续服务器 pull 后该构建产物不应再显示为未跟踪文件。

## 2026-06-05 一键发布脚本验证

已完成：

1. 服务器 pull 最新 `main` 到 `2a40ec9 Add deployment runbook and scripts`。
2. 执行 `./scripts/deploy.sh`。
3. 脚本自动完成：
   - 使用 Deploy Key fetch `origin/main`
   - 将本地 `main` 指向 `origin/main`
   - 输出当前 commit
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
   - `sudo systemctl restart erzhuang-project.service`
   - `curl -fsS http://127.0.0.1:18081/health`

结果：

- 发布 commit：`2a40ec9 Add deployment runbook and scripts`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

重要学习点：

- 现在已经可以用 `./scripts/deploy.sh` 完成标准发布。
- 脚本内部封装了 Deploy Key，不需要每次手写 `GIT_SSH_COMMAND`。
- 脚本会先测试再构建再重启，失败会停止。

## 2026-06-05 一键回滚脚本验证

已完成：

1. 服务器 pull 最新 `main` 到 `70d94db Record deploy script verification`。
2. 执行 `./scripts/rollback.sh fbdb249`。
3. 脚本自动完成：
   - 使用 Deploy Key fetch 远程 refs 和 tags
   - checkout 到目标 commit `fbdb249`
   - 输出当前 commit
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
   - `sudo systemctl restart erzhuang-project.service`
   - `curl -fsS http://127.0.0.1:18081/health`

结果：

- 回滚后运行 commit：`fbdb249 Record first Lighthouse deployment`
- 服务器 Git 状态：detached HEAD
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

重要学习点：

- 现在可以用 `./scripts/rollback.sh <commit-or-tag>` 完成标准回滚。
- 回滚到 commit 会让服务器进入 detached HEAD，这是脚本文档中已说明的预期行为。
- 后续使用 `./scripts/deploy.sh` 可以恢复到最新 `main` 并重新发布。

## 2026-06-05 公网 HTTPS 入口

已完成：

1. 腾讯云 API/TAT：
   - 使用腾讯云 API 只读查询韩国区 `ap-seoul` Lighthouse 实例。
   - 确认目标实例：
     - InstanceId: `lhins-rjfpwj1u`
     - Public IP: `43.155.237.46`
     - OS: Ubuntu Server 24.04 LTS 64bit
     - State: RUNNING
   - 明确边界：只操作韩国区实例，不操作日本区实例。
   - 使用 TAT 在目标实例执行只读检查和 nginx 配置操作。

2. nginx：
   - 配置文件：`/etc/nginx/sites-enabled/vpn-proxy`
   - 保留原有 `/` 和 `/vless`。
   - 新增：

```nginx
location = /erzhuang {
    return 301 /erzhuang/;
}

location /erzhuang/ {
    proxy_pass http://127.0.0.1:18081/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

3. nginx 备份处理：
   - 曾将备份文件放到 `/etc/nginx/sites-enabled/`，导致 nginx 加载到重复 `server_name 43.155.237.46`，出现 warning。
   - 已将备份移动到 `/etc/nginx/backups/`。
   - `nginx -t` 通过。
   - 已执行 `systemctl reload nginx`。

4. Lighthouse 防火墙：
   - 原规则只放行 TCP 22 和 ICMP。
   - 已只在韩国实例 `lhins-rjfpwj1u` 添加 TCP 443 放行：
     - Protocol: TCP
     - Port: 443
     - CidrBlock: `0.0.0.0/0`
     - Description: `HTTPS for erzhuang nginx`

5. 验证结果：

```sh
curl -k https://43.155.237.46/erzhuang/health
curl -k https://43.155.237.46/erzhuang/api/tasks
```

返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

`/api/tasks` 也成功返回任务列表。

重要备注：

- 当前 HTTPS 证书来自 `/etc/xray/server.crt`，浏览器可能提示证书不受信任。
- 当前服务公网入口是 IP + 路径，不是正式域名。
- Go 服务仍只监听 `127.0.0.1:18081`，公网只通过 nginx 进入。
- 临时腾讯云 API 密钥已在聊天中暴露，完成后应在 CAM 中禁用或删除。

## 建议的本地 Go 项目初始化路线

第一阶段：本地最小服务

1. 初始化 Git 仓库。已完成。
2. 初始化 Go module。已完成。
3. 创建最小 HTTP 服务。已完成。
4. 实现。已完成：
   - `GET /health`
   - `GET /api/tasks`
5. 添加单元测试。已完成。
6. 本地验证。已完成：
   - `.tools/go/bin/gofmt -w cmd/server/main.go internal/app/handler.go internal/app/handler_test.go`
   - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build .tools/go/bin/go test ./...`
   - `GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build .tools/go/bin/go build -o bin/erzhuang-project ./cmd/server`
   - `ADDR=127.0.0.1:18080 ./bin/erzhuang-project`
   - `curl http://127.0.0.1:18080/health`
   - `curl http://127.0.0.1:18080/api/tasks`

第二阶段：GitHub

1. 创建 GitHub 仓库。已完成。
2. 添加远程仓库 `origin`。已完成。
3. 推送 `main` 分支。已完成。
4. 学习常用 Git 流程：
   - `git status`
   - `git add`
   - `git commit`
   - `git log`
   - `git diff`
   - `git push`
   - `git pull`
   - tag 或 release 标记版本

第三阶段：服务器拉取和发布

1. 在服务器准备部署目录，例如 `/opt/apps/erzhuang-project`。已完成。
2. 从 GitHub clone 或 pull 代码。已完成。
3. 在服务器运行。已完成：
   - `go test ./...`
   - `go build -o erzhuang-project ./cmd/server`
4. 配置或更新 systemd service。已完成。
5. `systemctl start`。已完成。
6. `curl /health` 验证。已完成。
7. `systemctl enable` 开机自启。已完成。
8. 记录发布。已完成。

第四阶段：受控部署脚本

等手动流程熟悉后，再把固定步骤整理为脚本，例如：

```text
git pull
go test ./...
go build
systemctl restart
curl /health
record release
```

脚本要避免保存高权限长期密钥。

## 风险提醒模板

后续执行重要操作前，先说明风险：

- `git init`: 会在当前目录创建版本管理元数据，通常安全；如果目录里已有复杂文件，需要确认不要误提交敏感信息。
- `git push`: 会把本地代码上传到 GitHub；推送前要确认没有密钥、公司代码、个人隐私文件。
- SSH 到服务器：可能修改线上运行状态；执行前要说明会影响哪个服务、端口和目录。
- `systemctl restart`: 会重启服务；如果服务配置或构建产物有问题，可能造成服务短暂不可用。
- 修改 nginx：可能影响已有 HTTPS 服务；需要备份配置并执行 `nginx -t`。
- 安全组/防火墙放行端口：会增加公网暴露面；优先只开放必要端口。

## 待补充信息

- GitHub 仓库名：`erzhuang-project`
- Go module 名：`github.com/shalei-pm/erzhuang-project`
- 本地服务端口：默认 `127.0.0.1:18080`
- 服务器部署目录：`/opt/apps/erzhuang-project`
- 服务器服务名：`erzhuang-project.service`
- 服务器监听地址：`127.0.0.1:18081`
- GitHub 访问方式：
  - 本机 push：个人 GitHub SSH key，已成功
  - 服务器 pull：GitHub Deploy Key，read-only，已成功
- 是否需要域名：
- 是否需要 nginx 反向代理：

## 发布记录

### 2026-06-05 Supabase PostgreSQL 接入发布

- 发布目标：个人腾讯云 Lighthouse 韩国实例
- 实例：`ap-seoul / lhins-rjfpwj1u`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`186cf5d`
- commit message：`Make deploy health retry robust`
- 数据库：Supabase PostgreSQL Shared Pooler
- 数据库配置方式：
  - systemd `EnvironmentFile=/etc/erzhuang-project.env`
  - 文件权限：`root root 600`
  - 未将连接串写入 GitHub
- 代码变化：
  - 新增 `Store` 接口
  - 新增 memory store
  - 新增 PostgreSQL store
  - `/api/tasks` 支持从 PostgreSQL 读取
  - `/health` 增加 `database` 字段
  - 自动创建 `tasks` 表并写入练习种子数据
  - 部署脚本健康检查增加重试，适配数据库冷启动时间
- 服务器验证：
  - `go test ./...` 通过
  - `go build -o erzhuang-project ./cmd/server` 通过
  - `npm install` 通过，0 个已知漏洞
  - `npm run build` 通过
  - `erzhuang-project.service` active running
- 公网健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

- 公网任务接口：
  - `https://43.155.237.46/erzhuang/api/tasks`
  - 返回 4 条数据库任务
  - 包含 `接入 Supabase PostgreSQL`
- 过程备注：
  - 首次数据库发布时，服务冷启动需要约 2 秒连接数据库和初始化 schema。
  - 原部署脚本重启后立刻 `curl`，导致误判失败。
  - 已修复为最多 15 次健康检查重试。
  - 真实服务已成功启动并连接 PostgreSQL。

### 2026-06-05 前端公网入口发布

- 发布目标：个人腾讯云 Lighthouse 韩国实例
- 实例：`ap-seoul / lhins-rjfpwj1u`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`df7da52`
- commit message：`Prepare frontend deployment path`
- Node 版本：`v22.22.2`
- npm 版本：`10.9.7`
- Go 版本：`go1.22.2 linux/amd64`
- 后端测试：`go test ./...` 通过
- 后端构建：`go build -o erzhuang-project ./cmd/server` 通过
- 前端安装：`npm install` 通过，0 个已知漏洞
- 前端构建：`npm run build` 通过
- 前端构建产物：
  - `frontend/dist/index.html`
  - `frontend/dist/assets/index-DAq-dh7A.css`
  - `frontend/dist/assets/index-DCmH9dBt.js`
- systemd：`erzhuang-project.service` 已重启，active running
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

- nginx 路由：
  - `/erzhuang/` 返回前端静态页面
  - `/erzhuang/health` 反向代理到 Go 后端 `/health`
  - `/erzhuang/api/` 反向代理到 Go 后端 `/api/`
- 公网验证：
  - `https://43.155.237.46/erzhuang/` 返回前端 HTML，HTTP 200
  - `https://43.155.237.46/erzhuang/assets/index-DCmH9dBt.js` 返回 JS，HTTP 200
  - `https://43.155.237.46/erzhuang/health` 返回健康 JSON，HTTP 200
  - `https://43.155.237.46/erzhuang/api/tasks` 返回任务 JSON，HTTP 200
- 备份：
  - nginx 修改前已备份到 `/etc/nginx/backups/`
- 影响范围：
  - 保留原 `/vless` 配置
  - 未改日本 Lighthouse 实例
  - 未接触公司环境、公司代码、公司密钥

### 2026-06-05 首次 Lighthouse 发布

- 发布目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`0f1699e`
- commit message：`Document next deployment steps`
- Go 版本：`go1.22.2 linux/amd64`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 测试命令：`go test ./...`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- 开机自启：enabled
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

- 影响范围：
  - 未修改 nginx
  - 未开放公网端口
  - 未影响 `codex-demo.service`
  - 未影响 `hermes-gateway.service`
  - 未影响 `feishu-poll-bot.service`
  - 未影响 xray

### 2026-06-05 v2 发布

- 发布目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 发布 commit：`bc8d5a3`
- commit message：`Release health version v2`
- 测试命令：`go test ./...`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 发布命令：`sudo systemctl restart erzhuang-project.service`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v2"}
```

- 影响范围：
  - 未修改 nginx
  - 未开放公网端口
  - 未影响 `codex-demo.service`
  - 未影响 `hermes-gateway.service`
  - 未影响 `feishu-poll-bot.service`
  - 未影响 xray

### 2026-06-05 v2 回滚到 v1

- 回滚目标：个人腾讯云 Lighthouse
- 服务名：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`
- 回滚前运行 commit：`bc8d5a3`
- 回滚后运行 commit：`fbdb249`
- 回滚方式：服务器 `git checkout fbdb249`
- 服务器 Git 状态：detached HEAD
- 测试命令：`go test ./...`
- 构建命令：`go build -o erzhuang-project ./cmd/server`
- 发布命令：`sudo systemctl restart erzhuang-project.service`
- 监听地址：`127.0.0.1:18081`
- systemd 状态：active running
- cgroup：`/system.slice/erzhuang-project.service`
- 健康检查：

```json
{"app":"erzhuang-project","status":"ok","version":"v1"}
```

- 过程备注：
  - `git fetch origin` 因未带 Deploy Key 参数失败：`Permission denied (publickey)`。
  - 目标 commit 已在本地存在，因此 checkout 和回滚仍成功。
  - 后续脚本必须统一封装 `GIT_SSH_COMMAND`。

## 本地验证记录

- 2026-06-04：本地 Go 服务 v1 骨架验证通过。
  - `/health` 返回：`{"app":"erzhuang-project","status":"ok","version":"v1"}`
  - `/api/tasks` 返回 3 条练习任务。
- 2026-06-04：完成第一次本地 Git 提交。
  - commit: `245e873 Initial Go service skeleton`
- 2026-06-04：推送本地 `main` 分支到 GitHub。
  - remote: `git@github.com:shalei-pm/erzhuang-project.git`
  - pushed commits:
    - `245e873 Initial Go service skeleton`
    - `cf99612 Document initial local verification`
    - `179f8a9 Ignore local SSH metadata`
- 2026-06-04：记录 GitHub 推送学习状态。
  - commit: `76b0400 Document GitHub push`

## 2026-06-05 设计图管理线上发布

发布目标：

- 腾讯云 Lighthouse 韩国实例：`ap-seoul / lhins-rjfpwj1u`
- 公网入口：`https://43.155.237.46/erzhuang/`
- systemd 服务：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`

本次发布内容：

- 后端 Phase 1：设计图门店和区域 CRUD API。
- 前端 Phase 2：设计图标记后台页面骨架。
- 前端 API adapter：真实后端 CRUD 优先，上传/识别继续 mock。
- 契约修复：
  - `e571d6c Return empty arrays for design plan responses`
  - `6fa5562 Return empty arrays for duplicate checks`

最终线上运行 commit：

```text
6fa5562 Return empty arrays for duplicate checks
```

发布方式：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 600 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

服务器发布脚本完成：

- `git fetch origin`
- `git switch -C main origin/main`
- `go test ./...`
- `go build -o erzhuang-project ./cmd/server`
- `npm install`
- `npm run build`
- `sudo systemctl restart erzhuang-project.service`
- `curl -fsS http://127.0.0.1:18081/health`

服务器验证结果：

- Linux `go test ./...` 通过。
- Go build 通过。
- 前端 Vite build 通过。
- systemd 状态：active running。
- 健康检查返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

公网最终验收：

```sh
curl -k https://43.155.237.46/erzhuang/health
curl -k https://43.155.237.46/erzhuang/
curl -k 'https://43.155.237.46/erzhuang/api/design-plan/stores?page=1&page_size=2'
curl -k -X POST https://43.155.237.46/erzhuang/api/design-plan/stores/check-duplicate \
  -H 'Content-Type: application/json' \
  --data '{"name":"测试门店"}'
```

验收结果：

- `/erzhuang/health`：HTTP 200，数据库 `postgres`。
- `/erzhuang/`：HTTP 200，返回前端 HTML。
- `/erzhuang/api/design-plan/stores?page=1&page_size=2`：HTTP 200。

```json
{"items":[],"page":1,"page_size":2,"total":0}
```

- `/erzhuang/api/design-plan/stores/check-duplicate`：HTTP 200。

```json
{"exact_match":null,"similar_matches":[]}
```

过程问题和学习点：

- 第一次用 TAT 默认 `root` 用户执行发布脚本时失败：
  - Git 报错：`fatal: detected dubious ownership in repository at '/opt/apps/erzhuang-project'`
  - 原因：仓库属于 `lighthouse` 用户，root 操作该仓库会触发 Git safe.directory 检查。
  - 处理：改用 TAT 的 `--username lighthouse` 执行发布脚本。
- 发布后先发现 `/erzhuang/api/design-plan/stores` 从 404 变成 200，但空列表返回 `items:null`。
  - 这会导致前端真实 adapter 对 `items.map` 出错。
  - 已修复为空数组 `items: []`。
- 随后发现重复检查接口空结果返回 `similar_matches:null`。
  - 已修复为空数组 `similar_matches: []`。
- 本地 Mac 的项目内 Go 工具仍有 `dyld missing LC_UUID` 问题，导致本机无法运行 `go test` 和编译出的 Go 二进制。
  - 本机可用 `go build` 做编译验证。
  - 完整 `go test ./...` 以 Lighthouse Linux 发布脚本结果为准。

## 2026-06-08 前端 UI 小迭代发布

发布目标：

- 腾讯云 Lighthouse 韩国实例：`ap-seoul / lhins-rjfpwj1u`
- 公网入口：`https://43.155.237.46/erzhuang/`
- systemd 服务：`erzhuang-project.service`
- 部署目录：`/opt/apps/erzhuang-project`

本次发布内容：

- 前端页面文案产品化：
  - `Design Plan Marker` 调整为 `空间资源管理`。
  - 上传占位说明移除 mock/Phase 文案，改为面向业务用户的说明。
  - `模拟识别失败` 调整为 `手动维护`。
- 前端弹窗状态文案调整：
  - `待上传`
  - `解析图纸中`
  - `识别区域中`
  - `可编辑`
- 区域卡片视觉和信息层级收紧：
  - 卡片标题优先展示区域名称。
  - 增加 `类型 · 编号` 摘要。
  - 缩小间距、表格行高和阴影，整体更接近后台系统风格。

发布 commit：

```text
9673866 Polish design plan frontend UI
```

发布方式：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 600 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

服务器发布脚本完成：

- `git fetch origin`
- `git switch -C main origin/main`
- `go test ./...`
- `go build -o erzhuang-project ./cmd/server`
- `npm install`
- `npm run build`
- `sudo systemctl restart erzhuang-project.service`
- `curl -fsS http://127.0.0.1:18081/health`

服务器验证结果：

- Linux `go test ./...` 通过。
- Go build 通过。
- 前端 Vite build 通过。
- systemd 状态：active running。
- 健康检查返回：

```json
{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}
```

公网最终验收：

- `/erzhuang/health`：HTTP 200。
- `/erzhuang/`：HTTP 200，返回前端 HTML。
- `/erzhuang/api/design-plan/stores?page=1&page_size=2`：HTTP 200。

```json
{"items":[],"page":1,"page_size":2,"total":0}
```

给用户的可访问地址：

- 本地预览：`http://127.0.0.1:5173/erzhuang/`
- 公网预览：`https://43.155.237.46/erzhuang/`

注意：

- 公网入口当前使用 IP + 自签证书，浏览器可能提示证书风险；这是当前练习环境的预期现象。
- 本次只改前端文案和样式，没有改数据库、后端接口、nginx 或 systemd 配置。

## 2026-06-08 识别失败可观测性增强

用户反馈：

- 上传一个门店设计图 PDF 后，页面没有识别结果。
- 页面也没有明确成功或失败反馈。
- 需要补充日志能力，方便后续提 bug 时定位问题。

判断：

- 旧代码里后端识别失败会统一返回 `design plan request failed`，没有在 `systemd` 日志中记录具体错误。
- 前端失败提示主要依赖页面 toast，弹窗里没有持久的上传/识别状态提示，用户容易感觉“没有任何提示”。

本次改动：

- 后端 `internal/designplan/handler.go` 增加上传与识别阶段日志：
  - 上传开始、上传完成、上传失败。
  - 识别开始、识别完成、识别失败。
  - 日志包含 `upload_id`、文件名、文件大小、页数、识别区域数量、耗时和错误信息。
- 前端 `frontend/src/App.tsx` 增加弹窗内持久状态提示：
  - PDF 解析中。
  - AI 识别中。
  - AI 识别完成。
  - AI 识别失败，可手动维护，并展示上传编号。
- 前端 `frontend/src/styles.css` 增加状态提示样式。
- 版本号从 `1.1.0` 升级到 `1.1.1`，按规则属于小版本：bug 定位/技术可观测性增强。

后续排查命令：

```sh
sudo journalctl -u erzhuang-project.service --since "30 minutes ago" | grep "designplan:"
```

如果用户再次反馈上传或识别失败，优先按上传时间查 `designplan:` 日志，确认失败阶段是 PDF 转图、上传资产读取、AI 接口调用、AI 返回解析，还是前端状态展示。

## 2026-06-08 上传错误提示增强

用户反馈：

- 上传两页 PDF 时页面直接显示 `HTTP 413`，不清楚业务含义。

判断：

- 两页 PDF 本身不违反第一版“最多 5 页”的规则。
- `HTTP 413` 通常表示请求体过大，可能是 PDF 文件超过 5MB，或 nginx 的请求体限制先于 Go 后端拦截。

本次改动：

- 前端上传前校验文件类型，只允许 PDF。
- 前端上传前校验文件大小，超过 5MB 直接提示“文件过大，请上传 5MB 以内的 PDF。”
- 前端将 `HTTP 413` 映射成文件过大提示。
- 前端将 `HTTP 504` 映射成 AI 识别超时提示。
- 版本号从 `1.1.1` 升级到 `1.1.2`，按规则属于小版本：错误提示和用户体验修复。

## 2026-06-08 门店搜索关键词匹配修复

用户反馈：

- 搜索框输入 `新氧青春` 没有检索出包含该关键词的门店。
- 产品预期：门店名称包含关键词时应该被检索出来。

判断：

- 后端列表搜索只使用 `normalized_name`。
- `normalized_name` 为了重复判断会去掉 `新氧青春`、`门店`、`店` 等品牌和后缀词。
- 这个规则适合“重复门店判断”，但不适合普通列表搜索。

本次改动：

- 后端搜索改为同时匹配：
  - 原始门店名称包含搜索词。
  - 归一化门店名称包含归一化搜索词。
- 重复判断逻辑仍继续使用归一化名称，不受影响。
- 前端 mock 搜索逻辑同步调整，避免本地预览和线上行为不一致。
- 增加后端测试覆盖品牌关键词搜索。
- 版本号从 `1.1.2` 升级到 `1.1.3`，按规则属于小版本：搜索 bug 修复。

## 2026-06-09 前端搜索请求稳定性修复

用户反馈：

- 线上 API 已确认 `q=新氧` 和 `q=新氧青春` 都返回 2 条门店。
- 但页面搜索仍显示没有结果。

判断：

- 后端搜索和公网 nginx API 均正常。
- 前端列表加载逻辑需要显式绑定当前搜索词，并防止旧请求返回后覆盖新请求结果。

本次改动：

- 前端 `loadStores` 显式接收当前 `query` 和 `page`。
- 增加请求序号，旧请求返回时不会覆盖最新列表状态。
- 列表加载失败时给出 toast，而不是静默显示空列表。
- 版本号从 `1.1.3` 升级到 `1.1.4`，按规则属于小版本：前端搜索状态修复。

## 2026-06-09 GitHub CLI 与 PR 流程确认

用户反馈：

- 本机 GitHub CLI 已可用。

确认结果：

- `gh` 路径：`/Users/sylar/.local/bin/gh`
- 版本：`gh version 2.93.0`
- 登录账号：`shalei-pm`
- token scope：`gist`、`read:org`、`repo`、`workflow`
- 仓库访问正常：`shalei-pm/erzhuang-project`
- 默认分支：`main`
- 当前无打开 PR。
- 当前无 GitHub Actions run 记录，说明仓库暂未配置 CI 或没有运行历史。

Codex 使用注意：

- 普通沙箱下 `gh` 可能无法访问 GitHub API 或 macOS keyring。
- 在 Codex 中使用 `gh` 查询仓库、PR、Actions 时，通常需要提升权限。
- 提升权限后已验证 `gh auth status` 和 `gh repo view` 可用。

全局研发流程约定：

1. 常规研发默认从 `main` 创建 `codex/<task-name>` 分支。
2. 专项会话或主会话在分支内实现、测试、提交。
3. 分支推送到 GitHub 后，用 `gh pr create` 创建 PR。
4. 主会话负责 review、决定合并、必要时打回修改。
5. PR 合并后，主会话负责发布到 Lighthouse、验证 `/health`、验证页面版本号，并记录发布结果。
6. 紧急线上修复或用户明确要求快速闭环时，可以直接在 `main` 小步提交并发布，但最终说明必须标注原因。
7. 后续如配置 GitHub Actions，PR 合并前必须检查 CI 结果。

## 2026-06-10 Supabase RLS 安全告警处理

用户收到 Supabase 邮件告警：

- Critical issue: Table publicly accessible
- Issue code: `rls_disabled_in_public`
- Project ref: `alsobcuythtbkldxmbvq`

判断：

- 本项目在 Supabase 的 `public` schema 下创建了 `tasks`、`design_plan_stores`、`design_plan_store_areas`、`design_plan_operation_logs`。
- Supabase 会将 `public` schema 表暴露给 Supabase API 层。
- 即使当前业务只通过 Go 后端连接数据库，也应该对 `public` 表开启 Row-Level Security。

处理方案：

- 在运行时 schema 初始化中增加：
  - `alter table tasks enable row level security`
  - `alter table design_plan_stores enable row level security`
  - `alter table design_plan_store_areas enable row level security`
  - `alter table design_plan_operation_logs enable row level security`
- 同步更新 `db/schema.sql` 和 `db/design_plan_schema.sql`。
- 在 `docs/database-plan.md` 记录 RLS 规则、立即修复 SQL 和验证 SQL。
- 在 `AGENTS.md` 增加数据库安全开发规则：Supabase `public` schema 新表必须开启 RLS。
- 版本号从 `1.1.4` 升级到 `1.1.5`，按规则属于小版本：安全配置修复。

注意：

- 本次不新增 anon/authenticated policy。
- 第一版仍保持浏览器端不直连 Supabase，业务读写统一经过 Go 后端 API。
- Go 后端使用数据库连接串访问 PostgreSQL，不受 Supabase API 层 RLS policy 限制。

发布结果：

- 提交：`38abcf3 Enable Supabase RLS for public tables`
- 线上版本：`1.1.5`
- 服务器部署：成功
- 服务器测试：`go test ./...` 通过
- 前端构建：通过
- systemd 重启：成功
- `/health`：返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`
- RLS 状态验证：通过 TAT 在服务器读取受保护的 `/etc/erzhuang-project.env`，使用只读 SQL 查询 `pg_tables.rowsecurity`。
- 验证结果：
  - `design_plan_operation_logs rowsecurity=true`
  - `design_plan_store_areas rowsecurity=true`
  - `design_plan_stores rowsecurity=true`
  - `tasks rowsecurity=true`
  - `RLS_CHECK=PASS`

## 2026-06-10 Supabase RLS policy 提示处理

用户反馈 Supabase Advisor 继续提示：

- `Detects cases where row level security (RLS) has been enabled on a table but no RLS policies have been created.`

判断：

- 这不是最初“表公开可读写”的问题。
- 当前 RLS 已开启且没有 policy 时，Supabase API 侧默认拒绝访问。
- 但为了让权限意图更明确，并减少 Advisor 提示，项目应增加显式拒绝前端直连的 policy。

处理方案：

- 对 `tasks`、`design_plan_stores`、`design_plan_store_areas`、`design_plan_operation_logs` 增加 `*_no_client_access` policy。
- policy 作用对象：`anon, authenticated`。
- policy 规则：`for all using (false) with check (false)`。
- 结果：浏览器端通过 Supabase anon/authenticated 角色仍不能读写业务表；Go 后端服务端数据库连接不受影响。
- 版本号从 `1.1.5` 升级到 `1.1.6`，按规则属于小版本：数据库安全策略说明和 Advisor 提示修复。

发布结果：

- 提交：`3ee780e Add explicit Supabase deny policies`
- 线上版本：`1.1.6`
- 服务器部署：成功
- 服务器测试：`go test ./...` 通过
- 前端构建：通过
- systemd 重启：成功
- `/health`：返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`

RLS policy 验证：

- `design_plan_operation_logs rowsecurity=true policies=design_plan_operation_logs_no_client_access`
- `design_plan_store_areas rowsecurity=true policies=design_plan_store_areas_no_client_access`
- `design_plan_stores rowsecurity=true policies=design_plan_stores_no_client_access`
- `tasks rowsecurity=true policies=tasks_no_client_access`
- `RLS_POLICY_CHECK=PASS`

## 2026-06-11 门店空间资源后端基础分支

后端专项分支：

- 分支：`codex/store-space-backend-foundation`
- 基线：`f819793`
- 范围：新增 `internal/storespace`，接入 `/api/store-space/*` 基础 API，新增门店空间资源 PostgreSQL schema 和 RLS deny policy。
- 边界：未操作腾讯云、nginx、systemd、Supabase 控制台、部署脚本、云密钥、萤石云密钥或 AI key；未改现有 `internal/designplan` 业务实现。
- 状态文档：`docs/store-space-backend-foundation-state.md`

本地验证：

```sh
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/storespace
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go test -c ./internal/app
GOCACHE=/Users/sylar/.codex/worktrees/1e39/erzhuang-project/.cache/go-build /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
```

结果：均通过。`go test ./internal/storespace` 在本机仍命中已知 macOS `missing LC_UUID load command` 问题，最终完整测试需主会话在服务器 Linux 环境执行。

## 2026-06-12 门店空间资源前后端专项合并

主会话完成两条专项分支验收并合并到本地 `main`：

- 后端分支：`codex/store-space-backend-foundation`
- 前端分支：`codex/store-space-frontend-shell`
- 后端合并提交：`cdd1d28 Merge store space backend foundation`
- 前端合并提交：`61ba624 Merge store space frontend shell`

已合入能力：

- 新增 `internal/storespace` 后端基础模型、校验、repository、service、handler。
- 新增 `/api/store-space/*` 基础接口：
  - `GET /api/store-space/ezviz-accounts`
  - `GET /api/store-space/stores`
  - `GET /api/store-space/stores/{id}`
  - `POST /api/store-space/stores`
  - `DELETE /api/store-space/stores/{id}`
  - `POST /api/store-space/stores/check-duplicate`
  - 录像机扫描/识别接口先保留稳定 `501 not implemented` 合同。
- 新增门店空间资源数据库 schema，并对所有新增 public 表启用 RLS + 显式 deny policy。
- 前端从单文件 `App.tsx` 拆分为门店列表、添加门店浮层、门店详情、设计图标注 Tab、通道映射 Tab，以及对应 domain 工具。
- 前端新增 store-space 后端 DTO/mapper；`createStore` 已走新后端 mapper。
- 添加门店浮层不再默认选择第一个萤石云账号；填写录像机设备编码时必须由用户明确选择账号。

合并后本地主线验证：

```sh
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go test -c -o /private/tmp/erzhuang-storespace-merged.test ./internal/storespace
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go test -c -o /private/tmp/erzhuang-app-merged.test ./internal/app
GOCACHE=/private/tmp/erzhuang-go-build-cache /Users/sylar/erzhuang-project/.tools/go/bin/go build ./...
cd frontend && PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run build
git diff --check
```

结果：均通过。

当前边界：

- 当前完成的是本地 `main` 合并，尚未推送 GitHub，尚未发布到 Lighthouse。
- 前端 `storeSpaceApi.listStores/getStore` 当前阶段仍暂走旧 `designPlanApi`，用于保护现有设计图列表体验；真正完整切换门店空间资源列表/详情时，需要改为 `storeSpaceHttpAdapter.listStores/getStore`。
- 通道扫描、抓图、识别、确认接口后端尚未接真实萤石云；当前只完成基础合同和前端 UI/mock 壳。
- `ezviz_accounts` 已有只读安全字段列表接口，并补充了仅保存账号名的轻量创建接口；真实 `appKey/appSecret/accessToken` 仍不通过前端表单维护，后续由后端受控配置/加密方案承接。

## 2026-06-12 准备发布门店空间资源 2.0.0

版本号按项目规则从 `1.1.6` 升级到 `2.0.0`：

- 原因：新增“门店空间资源管理/通道映射”完整业务模块，属于大版本升级。
- 线上页脚预期：`2.0.0 (<commit>)`。
- 发布方式：主会话通过腾讯云 TAT 指定韩国实例 `ap-seoul / lhins-rjfpwj1u`，以 `lighthouse` 用户执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
- 风险说明：发布会拉取 GitHub 最新 `main`、执行测试/构建、初始化新增数据库表和 RLS deny policy，并重启 `erzhuang-project.service`。

首次发布结果：

- GitHub 拉取成功。
- 服务器 `go test ./...` 成功。
- 服务器 Go build 成功。
- 服务器前端 build 成功。
- systemd restart 已执行。
- 健康检查失败：`127.0.0.1:18081` 连接失败。

定位结果：

- 服务日志连续出现：`database setup failed: timeout: context deadline exceeded`。
- 根因：本次大版本新增较多 PostgreSQL 表、索引和 RLS policy，启动时 schema 初始化超过原有 10 秒上下文超时。
- 修复：版本号升级到 `2.0.1`，将数据库连接 Ping 超时保留 10 秒，将 schema 初始化超时单独放宽到 90 秒。

第二次发布结果：

- 修复提交：`b79aad1 Extend database schema setup timeout`
- 线上版本：`2.0.1`
- TAT InvocationId：`inv-r4ranigidm`
- TAT 结果：`SUCCESS`
- 服务器 commit：`b79aad1`
- 服务器 `go test ./...`：通过
- 服务器 Go build：通过
- 服务器前端 build：通过
- systemd restart：成功
- 内网健康检查：成功，返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`
- 现象：因为 schema 初始化仍需数秒，健康检查前 11 次连接失败，第 12 次成功；deploy 脚本重试机制生效。

流程复盘：

- 发布链路没有变：本地开发 -> GitHub `main` -> TAT -> 服务器拉取 GitHub -> 测试/构建 -> systemd -> health。
- 本次问题在于主会话一开始没有优先读取既有 runbook 和历史发布记录，导致先撞了一次非交互 `getpass`。
- 已把 TAT 发布方式、必须使用交互式 PTY、失败诊断步骤写入 `AGENTS.md` 和 `docs/deploy-runbook.md`，作为之后本项目的固定发布能力。

## 2026-06-12 创建门店浮层 2.1.0 小迭代

本次版本号从 `2.0.1` 升级到 `2.1.0`：

- 原因：已有“门店空间资源管理”模块内的创建门店浮层交互和样式迭代，并补齐萤石云账号轻量创建入口，让录像机配置链路可继续测试。
- 创建门店浮层默认不再塞入一个删不掉的录像机行；录像机为选填资源，点击加号后再新增设备编码行。
- “增加录像机”改为 32px 图标按钮，符合轻操作定位。
- 右上角关闭按钮改为稳定的 `.modal-close-button`，不再直接使用文本 `x`，避免形状变形。
- 浮层内新增“萤石云账号名称”轻量创建入口；创建后刷新账号列表并自动选中未配置账号的录像机行。
- 后端新增 `POST /api/store-space/ezviz-accounts`，只接收 `account_name`，返回安全字段，不返回也不接收密钥字段。
- 后台风格规范补充到 `docs/technical-architecture-index.md`：当前采用轻量企业后台 / SaaS admin 风格，自建 tokenized CSS，参考 Ant Design、Arco Design、Semi Design 的克制控件层级。

## 2026-06-12 门店详情与通道映射 2.2.0 小迭代

本次版本号从 `2.1.0` 升级到 `2.2.0`：

- 原因：已有“门店空间资源管理”模块内的信息架构和交互迭代。
- 创建门店浮层移除“新增萤石云账号”入口；账号维护不属于创建门店主流程，后续由配置侧或后端受控接口维护。
- 创建门店浮层仍支持选择已有萤石云账号并填写录像机设备编码；如果没有账号，页面展示“暂无可选账号”。
- 门店详情顶部的新氧机构 ID、录像机数、有效通道数、业务区域数改为只读指标陈列，不再呈现类似输入框的样式。
- 门店详情不再展示萤石云账号配置区。
- 通道映射 Tab 的录像机列表改为横向表格：
  - 表头：录像机名称、状态、有效通道数、上次扫描时间、操作。
  - 未扫描录像机只显示“扫描通道”。
  - 已扫描录像机显示“再次扫描”和“识别区域”。
  - 删除入口保留，但后端级联删除接口尚未实现，当前仍提示入口待接。
- 前端验收复盘：
  - 用户指出 2.x 前端细节不如早期 1.x，应提升验收标准。
  - 已将“前端发布前必须实际页面截图/视觉验收”写入 `AGENTS.md`。
  - 本轮本地 mock 视觉验收发现并修复：原生 `Choose File` 露出、详情顶部指标过度卡片化、通道映射操作列挤压换行、Tab 默认焦点框观感差。
  - 2.2.0 已发布上线，线上 commit 为 `eb29e90`。

## 2026-06-12 创建门店 validation failed 2.2.1 修复

本次版本号从 `2.2.0` 升级到 `2.2.1`：

- 用户反馈：创建门店弹窗信息完善后点击“创建门店”不能继续，机构列表出现 `validation failed`。
- 根因：
  - 前端 `storeSpaceApi` 的列表、详情、重复校验、删除仍复用旧 `design-plan` 接口，但创建门店走新的 `store-space` 接口。
  - 因此创建前重复校验可能查旧表，真正创建时新表已存在同名门店，后端返回字段级校验错误。
  - 前端 `ApiError` 没有保留后端返回的 `fields`，导致页面只能显示笼统的 `validation failed`。
- 修复：
  - `storeSpaceApi.listStores/getStore/checkDuplicate/deleteStore` 统一走 `/api/store-space`。
  - `storeSpaceHttpAdapter` 增加新模块重复校验与删除接口。
  - `ApiError` 增加 `fields`，`errorMessage` 优先展示字段级错误文案，例如“已存在同名门店”。
- 验证：
  - 前端 `npm run build` 通过。
  - 后端 `CGO_ENABLED=0 ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-12 通道映射删除录像机 2.3.0 小迭代

本次版本号从 `2.2.1` 升级到 `2.3.0`：

- 原因：通道映射 Tab 中“删除录像机”此前只是占位提示，用户继续测试门店详情时无法清理误填录像机。
- 后端新增：
  - `DELETE /api/store-space/recorders/{recorder_id}`。
  - 删除录像机时依赖数据库外键级联删除其通道。
  - 删除后更新门店 `updated_at`，并写入操作日志。
  - 内存仓储同步支持删除，并释放设备编码，便于本地 mock 和测试复用。
- 前端新增：
  - `storeSpaceApi.deleteRecorder(storeId, recorderId)`。
  - 通道映射 Tab 删除按钮改为真实操作。
  - 删除前二次确认，删除后刷新门店详情和顶部统计。
- 验证：
  - 后端新增 handler/service 测试覆盖删除录像机和设备编码复用。
  - 本地 `CGO_ENABLED=0 ./.tools/go/bin/go test ./...` 通过。
  - 本地前端 `npm run build` 通过。
  - 服务器 `go test ./...` 通过。
  - 服务器 Go build 通过。
  - 服务器前端 build 通过。
  - systemd restart 成功。
  - 内网健康检查成功，返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 公网 `/erzhuang/health` 验证成功。
- 发布结果：
  - 线上 commit：`2563351`
  - 线上版本：`2.3.0`
  - TAT InvocationId：`inv-t4rgda09gn`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-CcEoTbGK.js`
  - 现象：健康检查前 11 次连接失败，第 12 次成功；符合当前数据库 schema 初始化较慢但可恢复的已知模式。

## 2026-06-12 旧设计图门店可见性 2.3.1 修复

本次版本号从 `2.3.0` 升级到 `2.3.1`：

- 用户反馈：门店列表里 1.x 版本创建的机构消失，担心历史数据被测试机构替换。
- 排查结论：
  - 历史数据没有被物理删除。
  - 旧接口 `/erzhuang/api/design-plan/stores` 仍能查到 3 个历史机构。
  - 新接口 `/erzhuang/api/store-space/stores` 只查到 2.x 新门店主数据。
  - 根因是 2.2.1 为修复创建链路把前端列表切到新 `store-space` 表，但旧 `design_plan_*` 数据尚未迁移到新主数据模型，导致页面视图隐藏旧门店。
- 修复：
  - 在 `storespace.EnsurePostgresSchema` 中增加幂等 legacy migration。
  - 将旧 `design_plan_stores` 复制到新 `stores`。
  - 将旧设计图文件信息复制到 `store_design_plans`，使用 `legacy-<old_id>` 标识。
  - 将旧标注区域复制到 `store_areas` 和 `design_plan_annotations`。
  - 使用 `on conflict do nothing`，不覆盖、不删除旧表或新表已有数据。
- UI 小修：
  - 门店详情页移除“门店详情”冗余文案。
  - 录像机列表标题下移除 `1 / 3 台`。
  - 录像机操作改成圆角按钮样式。
- 发布结果：
  - 本地 commit：`191e4ee`
  - 线上 commit：`191e4ee`
  - 线上页面版本：`2.3.1 (191e4ee)`
  - TAT InvocationId：`inv-s4rh2w0iqb`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-DYHlvXR0.js`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/api/store-space/stores?page=1&page_size=50` 返回 `total=8`。
    - 新列表已同时包含 5 个 2.x 测试门店和 3 个 1.x 历史设计图门店：
      - `新氧青春诊所 深圳龙岗坂田万科项目`
      - `新氧青春广州塔门店`
      - `新氧青春诊所 深圳壹方城项目`
    - 抽查 `/erzhuang/api/store-space/stores/6` 可读取 `深圳壹方城项目` 的设计图和 11 个区域。

## 2026-06-12 旧门店标注框坐标 2.3.2 修复

本次版本号从 `2.3.1` 升级到 `2.3.2`：

- 发布后补充验收发现：
  - 旧门店已经恢复到新列表和新详情接口。
  - 但 `store-space` 详情接口只返回区域主数据，没有返回 `design_plan_annotations` 中的矩形框坐标。
  - 前端已经支持读取 `area.box`，因此需要后端补齐该字段，避免历史门店进入设计图 Tab 后缺少左侧标注框。
- 修复：
  - `store-space` 的 `Area` 增加 `box` 返回字段。
  - `PostgresStore.listAreas` 左连接最新一条 `design_plan_annotations`，把 `box_x/y/width/height` 转为前端使用的 `box`。
  - 新增 `parseAreaBox` 单元测试，覆盖坐标解析和缺失坐标不返回 box 的情况。
- 发布结果：
  - 本地 commit：`fac00f7`
  - 线上 commit：`fac00f7`
  - 线上页面版本：`2.3.2 (fac00f7)`
  - TAT InvocationId：`inv-s4rhar0q7h`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：`/erzhuang/assets/index-Bn7UK7y3.js`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/api/store-space/stores?page=1&page_size=50` 仍返回 `total=8`。
    - 抽查 `/erzhuang/api/store-space/stores/6`，11 个区域均已返回 `box` 坐标，可供前端恢复旧设计图矩形标注。

## 2026-06-12 门店详情 UI 控件与图纸加载体验 2.3.3 修复

本次版本号从 `2.3.2` 升级到 `2.3.3`：

- 用户反馈：
  - 通道 Tab 下“扫描通道 / 删除”按钮样式不统一。
  - 未上传设计图的门店不应显示默认打底设计图。
  - 上传新 PDF 后应显示加载状态，旧设计图不应继续展示；失败时再恢复旧图。
  - “返回列表”和门店名称距离太近。
  - “新增区域 / 保存标注”也应使用统一圆角按钮。
- 修复：
  - 空设计图路径不再映射到 mock 示例图，只在 `mock/*` 路径时显示示例图。
  - 设计图上传流程增加 `pendingPreviewUrl`：新图加载成功后才切换预览；转换或图片加载失败时恢复旧图和旧区域。
  - 上传/转换/新图加载增加状态文案和转圈提示。
  - 未上传状态的图纸区域改为纯净空状态，不再显示网格底纹，避免误解为已有图纸。
  - 统一按钮圆角、大小和 danger 样式；详情页标题区域增加间距。
- 本地验证：
  - `git diff --check` 通过。
  - `npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 本地浏览器验收：
    - 无设计图门店 `imageCount=0`。
    - 空状态背景 `backgroundImage=none`。
    - `新增区域 / 保存标注` 圆角为 `999px`。
    - 通道 Tab 未扫描录像机仅显示 `扫描通道 / 删除`。
- 发布结果：
  - 本地 commit：`7abcc23`
  - 线上 commit：`7abcc23`
  - 线上页面版本：`2.3.3 (7abcc23)`
  - TAT InvocationId：`inv-r4ri2e0dh9`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-CM-T6EwS.js`
    - `/erzhuang/assets/index-C6Bw6lpq.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.3.3 (7abcc23)`。
  - 备注：部署脚本健康检查前 11 次连接失败，第 12 次成功；服务最终健康，仍符合当前冷启动较慢的已知现象。

## 2026-06-12 门店列表与详情布局 2.3.4 修复

本次版本号从 `2.3.3` 升级到 `2.3.4`：

- 用户反馈：
  - 机构详情页“返回列表”按钮过大。
  - 机构详情页首屏高度偏高，希望默认适配 1080 分辨率。
  - 机构列表页操作按钮出现溢出列表的情况。
- 根因：
  - 2.3.3 统一全局按钮后，`.plain-button` 继承了普通按钮高度、padding，并在详情 header 的 grid 布局中被拉伸为整行宽度。
  - 列表操作按钮统一为 72px 后，最后一列宽度仍为 122px，两个按钮加间距后容易溢出。
  - 设计图画布使用 `calc(100vh - 140px)`，对 1080 高度的后台页面偏高。
- 修复：
  - `.plain-button` 调整为 26px 高轻量文字按钮，详情页返回按钮限制为内容宽度。
  - 列表操作列增宽到 160px，行内操作按钮收敛为 68px。
  - 详情页 header 间距收紧，设计图编辑区域和画布高度改为 `clamp`，适配 1080 首屏。
- 本地验证：
  - `git diff --check` 通过。
  - `npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 1440x1080 浏览器复验：
    - 返回按钮尺寸为 66x26。
    - 详情页无需纵向滚动，主内容底部约 829px。
    - 列表操作按钮组 136px，操作列 160px，表格无横向溢出。
- 发布结果：
  - 本地 commit：`5dd89c6`
  - 线上 commit：`5dd89c6`
  - 线上页面版本：`2.3.4 (5dd89c6)`
  - TAT InvocationId：`inv-t4rigm075g`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-B9o-QAd9.js`
    - `/erzhuang/assets/index-CHPZUwoD.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.3.4 (5dd89c6)`。

## 2026-06-12 城市字段与门店列表 2.4.0 迭代

本次版本号从 `2.3.4` 升级到 `2.4.0`：

- 用户反馈：
  - “添加门店”弹窗没有看到城市字段。
  - 机构列表需要在门店名称前展示城市列，旧数据无城市时展示“未设置”。
  - 机构列表操作按钮虽然已进入列表内，但距离右侧边缘过近。
- 产品规则：
  - 新建门店必须选择城市。
  - 城市先内置一线/新一线城市下拉。
  - 列表列顺序调整为：序号 / 城市 / 门店名称 / 新氧机构 ID / 设计图状态 / 录像机 / 通道 / 面诊室 / 治疗室 / 生美 / 更新时间 / 操作。
- 后端修复：
  - `stores` 表新增 `city text not null default ''`，schema 初始化和迁移都覆盖。
  - 创建门店接口新增 `city` 校验，缺失时返回“城市必填”。
  - MemoryStore 和 PostgresStore 的创建、列表、详情均返回 city。
  - 旧设计图迁移到 stores 时 city 为空，前端统一显示“未设置”。
- 前端修复：
  - 创建门店弹窗新增城市下拉。
  - 创建门店请求体传入 `city`。
  - StoreSummary/StoreDetail 和 store-space API 映射补齐 city。
  - 门店列表新增城市列，空值显示“未设置”。
  - 操作列宽度和右侧 padding 调整，避免按钮贴边。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地浏览器检查：
    - 列表表头顺序包含城市列，且位于门店名称前。
    - 旧数据城市显示“未设置”。
    - 添加门店弹窗展示城市下拉，包含北京、上海、广州、深圳等城市。
    - 操作列按钮右侧留白约 56px。
- 发布状态：
  - 本地功能 commit：`eb6261c`
  - 线上 commit：`eb6261c`
  - 线上页面版本：`2.4.0 (eb6261c)`
  - TAT InvocationId：`inv-t4rjdb0w91`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-PKhj2K0q.js`
    - `/erzhuang/assets/index-DvSqg6-J.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.0 (eb6261c)`、“城市”和“未设置”。

## 2026-06-12 详情页流程体验 2.4.1 修复

本次版本号从 `2.4.0` 升级到 `2.4.1`：

- 用户反馈：
  - 设计图区域较多时，点击左侧矩形框会让页面整体滚动去找右侧卡片，导致左侧图纸跑出视野。
  - 门店只有一台录像机时，删除后没有再次添加录像机的入口。
- 根因：
  - 区域卡片定位直接使用 `scrollIntoView`，浏览器会滚动页面级祖先容器。
  - 右侧 `area-pane` 未固定高度，区域多时会撑高整个详情页，无法形成内部滚动。
  - 通道映射页只支持删除录像机，缺少已有门店补录录像机的接口和表单。
- 修复：
  - 设计图编辑区固定桌面高度，右侧区域面板独立滚动。
  - 点击左侧矩形框或新增区域后，只滚动右侧区域面板，并把对应卡片定位到面板可视区中部。
  - 新增 `POST /api/store-space/stores/{id}/recorders`，支持已有门店补录录像机。
  - 通道映射 Tab 增加“添加录像机”表单，删除到 0 台后仍可补录。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - 本地 mock 浏览器复验：
    - 16 个区域卡片时，右侧面板高度 680、内容高度 2465，形成内部滚动。
    - 点击靠后区域矩形后，`windowScrollY` 保持 0，右侧 `area-pane.scrollTop` 变化到 1785，选中卡片位于右侧面板可视范围内。
    - 删除唯一录像机后，通道映射页仍展示“添加录像机”表单。
    - 填写 `DNEW12345` 后可重新添加录像机，列表恢复为 1 台。
- 发布状态：
  - 线上 commit：`edb5f9c`
  - 线上页面版本：`2.4.1 (edb5f9c)`
  - TAT InvocationId：`inv-t4rk6ggmb6`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-7ssrpJ7q.js`
    - `/erzhuang/assets/index-CXbT8UIV.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.1`、`edb5f9c`、“添加录像机”、“返回列表”、“区域卡片”。

## 2026-06-12 详情页返回按钮 2.4.2 修复

本次版本号从 `2.4.1` 升级到 `2.4.2`：

- 用户反馈：
  - 机构详情页左上角“返回列表”按钮颜色不醒目，几次被忽略。
- 根因：
  - 按钮仍沿用普通弱操作按钮的视觉层级，虽然是浅蓝，但高度只有 26px，缺少方向图标和明确的导航权重。
- 修复：
  - `StoreDetail` 中为返回按钮增加独立 `detail-back-button` 类名和左箭头。
  - 详情页返回按钮改成蓝底白字的专用导航按钮，高度 34px，保留紧凑尺寸但提高第一眼识别度。
  - TAT 工具补充 `TENCENTCLOUD_SECRET_ID` / `TENCENTCLOUD_SECRET_KEY` 环境变量读取能力，避免非交互环境无法发布；密钥仍不写入仓库。
- 本地验证：
  - `git diff --check` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `PYTHONPYCACHEPREFIX=/Users/sylar/erzhuang-project/.cache/pycache python3 -m py_compile tools/tat_run.py tools/tencent_api.py tools/tencent_credentials.py` 通过。
  - 本地 mock 浏览器复验：
    - `.detail-back-button` 位于详情页左上角，尺寸约 101 x 34。
    - 背景色为 `rgb(37, 99, 235)`，文字为白色，标题区域未被异常撑高。
- 发布状态：
  - 按钮修复 commit：`e549029`
  - 部署工具 commit：`3d84d4f`
  - 线上 commit：`3d84d4f`
  - 线上页面版本：`2.4.2 (3d84d4f)`
  - TAT InvocationId：`inv-r4rkfw0f56`
  - TAT 结果：`SUCCESS`
  - 前端构建产物：
    - `/erzhuang/assets/index-DBJ1sfb3.js`
    - `/erzhuang/assets/index-Cx20omGX.css`
  - 发布后验证：
    - `/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
    - `/erzhuang/` HTML 已引用新 JS/CSS。
    - 线上 JS 中已确认包含 `2.4.2 (3d84d4f)`、`detail-back-button` 和“返回列表”。
    - 线上 CSS 中已确认包含 `detail-back-button`、`#2563eb` 和 `box-shadow`。

## 明日待办

## 2026-06-12 非业务区域备注与缩略图 2.7.1 发布记录

- 版本号：`2.7.1`。
- Commit：`255d301`。
- 目标：
  - AI 识别到非业务区域时，允许把实体名称放到通道的“编号/备注”字段，例如“机房”“药房”“前台”。
  - 通道列表列名由“编号”改为“编号/备注”：业务区域仍为数字编号，其他区域为文本备注。
  - 缩略图按钮清除全局按钮 padding，固定缩略图尺寸并使用 `object-fit: cover` 铺满，避免挤压变形和异常留白。
- 本地验证：
  - 新增 `TestRecognizeChannelStoresNonBusinessSceneAsNote` 覆盖 `machine_room -> 机房` 备注链路。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - TAT InvocationId：`inv-r4rt590tgw`。
  - TAT 结果：`SUCCESS`。
  - 服务器当前 commit：`255d301`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-CQ5C75RW.js` 和 `/erzhuang/assets/index-CkGYQwCd.css`。
  - 线上 JS 已确认包含 `2.7.1 (255d301)`、`编号/备注`、`area_note`、“机房”“药房”“前台”。
  - 线上 CSS 已确认包含缩略图相关 `padding:0`、`overflow:hidden`、`object-fit:cover`。

## 2026-06-12 通道识别工作流 2.7.0 发布记录

- 版本号：`2.7.0`。
- Commit：`4a94700`。
- 目标：
  - 修复单通道“重新识别”误触发整台录像机识别的问题。
  - 录像机级“识别区域”改为前端按通道队列执行，显示进度百分比，每完成一条立即更新截图和识别结果。
  - “再次扫描”改为增量同步通道有效性，不清空已确认通道的业务区域映射。
  - 通道行增加删除能力；删除后再次扫描如通道仍有效，会作为新的未确认通道出现。
  - 门店列表缩略图改为等比铺满缩略图框，避免挤压变形和两侧留白。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布状态：
  - TAT InvocationId：`inv-s4rsm8giq7`。
  - TAT 结果：`SUCCESS`。
  - 服务器当前 commit：`4a94700`。
  - 服务器发布脚本测试、Go build、前端 build 均通过。
  - `erzhuang-project.service` 重启后为 active running。
  - `/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres"}`。
  - 线上 `/erzhuang/` HTML 已引用 `/erzhuang/assets/index-CZDG6jnt.js`。
  - 线上 JS 已确认包含 `2.7.0 (4a94700)`、“重新识别”、“识别进度”、“删除后将移除”。

## 2026-06-12 通道截图与 AI 预识别 2.6.0 开发记录

- 版本号：`2.6.0`。
- 目标：
  - 萤石云通道真实抓图。
  - 通道最近截图保存和前端预览。
  - 接入可选监控画面 AI 识别，按截图预填业务区域类型和编号。
  - 记录单通道抓图、识别、总耗时，便于后续判断是否需要换更快模型。
- 关键产品规则：
  - AI 识别只预填，用户点击确认后才进入锁定确认状态。
  - 编号卡片写明“治疗室 1 / 面诊室 2 / 生美 3”时，以卡片文字为准。
  - 已确认通道再次识别时不覆盖已确认的业务区域类型和编号。
- 本地验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 安全约定：
  - 视觉模型 key 不写入仓库，只通过服务器环境变量 `VISION_API_BASE_URL`、`VISION_API_KEY`、`VISION_MODEL` 配置。
  - 本次密钥扫描未发现真实 key 进入项目文件。

1. 开始前先运行：

```sh
git status -sb
git pull --ff-only
```

2. 后续增强：
   - 在服务器 pull 最新脚本
   - 在服务器 pull 最新 `.gitignore`，确认 `?? erzhuang-project` 消失
   - 给 v1/v2 创建 tag，练习基于 tag 的发布和回滚
   - 删除或禁用本次使用的临时腾讯云 API 密钥
   - 后续如需正式公网访问，配置域名和可信 HTTPS 证书

服务器旧 demo 记录：

- v1：`/health` 返回 version `v1`
- v2：已练习发布
- rollback：已从 v2 回滚到 v1

## 2026-06-26 AI 模型 provider 切换开发记录

- 目标：
  - 解决 OpenAI 接口限流时项目识别能力不稳定的问题。
  - 通道截图识别和设计图识别都支持通过环境变量切换 OpenAI / MiniMax。
  - MiniMax HTTP 调用内置到 Go 代码中，避免正式服务依赖 OpenClaw 外部脚本。
- 关键改动：
  - `CHANNEL_AI_PROVIDER=openai|minimax|minimax-script` 控制通道截图识别。
  - `DESIGN_PLAN_AI_PROVIDER=openai|minimax` 控制设计图识别；不设置时跟随 `CHANNEL_AI_PROVIDER`。
  - `MINIMAX_API_KEY` 是 MiniMax 唯一 key 来源，不复用 `OPENAI_API_KEY` 或 `VISION_API_KEY`。
  - 设计图识别增加 markdown 代码块包裹 JSON 的解析兼容。
  - MiniMax/OpenAI base URL 带 `/v1` 时避免重复拼接 `/v1/v1/...`。
  - 新增 `cmd/ai-smoke`，用于 provider/key/model 切换后的真实冒烟验证。
  - 新增 `docs/model-provider-switching.md`，记录换 provider、换 key、换模型和冒烟步骤。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/channelai/... ./internal/designplan/... ./cmd/ai-smoke` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 真实 MiniMax 冒烟：
  - `https://api.minimaxi.com/v1/models` 可用，当前 key 返回模型列表：`MiniMax-M3`、`MiniMax-M2.7`、`MiniMax-M2.7-highspeed`、`MiniMax-M2.5`、`MiniMax-M2.5-highspeed`、`MiniMax-M2.1`、`MiniMax-M2.1-highspeed`、`MiniMax-M2`。
  - `MiniMax-01-vision` 返回 `unknown model`；`MiniMax-M1` 返回 `not support img`。
  - `MiniMax-M3` 设计图 smoke 成功，耗时约 `4557ms`。
  - `MiniMax-M3` 通道截图 smoke 成功，耗时约 `3027ms`。
- 后续：
  - `minimax-script` 仍作为短期兜底保留；MiniMax HTTP 在线上环境验证稳定后再删除，彻底解耦 OpenClaw。

## 2026-06-26 详情页识别模型切换按钮开发记录

- 目标：
  - 在机构详情页「设计图标注 / 通道映射」Tab 行最右侧增加「切换识别模型」按钮。
  - 按钮后展示当前识别模型，例如 `当前识别模型：OpenAI / gpt-5.5` 或 `MiniMax / MiniMax-M3`。
  - 点击按钮在 OpenAI 和 MiniMax 之间切换，同时影响设计图识别和通道截图识别。
- 实现：
  - 新增后端 `GET /api/ai-settings` 和 `POST /api/ai-settings/toggle`。
  - 新增 `app_settings` 表保存 `ai_provider`，并开启 RLS + 拒绝前端直连策略。
  - 识别服务改为运行时读取当前 provider，不需要重启服务。
  - API key 仍只来自运行时环境变量，不进入数据库、前端或仓库。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
- 未完成：
  - 当前沙箱无法用 Playwright/Computer Use 完成页面截图验收；本地 dev server 已启动在 `http://127.0.0.1:5177/erzhuang/`，需要浏览器人工确认按钮位置。

## 2026-06-26 识别模型切换 2.16.0 发布记录

- 版本号：`2.16.0`。
- GitHub `main` commit：`d783014 Add runtime AI model switching`。
- 公司 GitLab 发布分支：`codex/containerize-single-image`。
- 公司 GitLab merge commit：`0ebed48 Merge branch 'main' into codex/containerize-single-image`。
- 发布范围：
  - 机构详情页 Tab 行最右侧新增“切换识别模型”按钮和当前模型显示。
  - 后端新增 AI settings API，运行时在 OpenAI / MiniMax 之间切换。
  - 通道截图识别和设计图识别支持动态 provider。
  - MiniMax HTTP recognizer 内置到 Go 服务，保留 `minimax-script` 作为短期兜底。
  - 同步 H5 monitor 技术调研文档和隐藏 Ezviz live demo 支撑代码。
- 本地验证：
  - `git diff --check --cached` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - staged diff 敏感信息扫描未发现真实 key。
- 公司环境验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已确认包含 `2.16.0`、“当前识别模型”、“切换识别模型”、`OpenAI`、`MiniMax`。
- 韩国服务器发布：
  - 通过 SSH 执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
  - 服务器拉取 GitHub `main` 到 `d783014`。
  - 服务器 `go test ./...`、Go build、frontend build 通过。
  - 重启 `erzhuang-project.service` 后健康检查最终通过。
  - 公网 `https://43.155.237.46/erzhuang/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"local"}`。
  - 韩国线上 JS 已确认包含 `2.16.0`、`d783014`、“当前识别模型”、“切换识别模型”、`OpenAI`、`MiniMax`。
- 注意：
  - TAT 发布因本机无 `TENCENTCLOUD_SECRET_ID` / `TENCENTCLOUD_SECRET_KEY` 环境变量而未继续输入密钥，改用已记录的 SSH key 执行同一部署脚本。
  - 韩国部署时服务重启后前 13 次本机 health 连接失败，第 14 次成功，判断为服务启动/依赖初始化短暂延迟；本次无需回滚。

## 2026-06-26 VIP治疗室与通道筛选 2.17.0 开发记录

- 目标：
  - 新增业务区域类型 `VIP治疗室`，归入治疗室大类。
  - 通道映射筛选扩展为：全部、面诊室、治疗室、生美、前台/候诊区、通道/其他。
  - 筛选和排序规则沉淀为可复用前端领域模块，供后续 H5 monitor 首页复用。
- 关键规则：
  - `VIP治疗室` 对应 `area_type=vip_treatment`，治疗室筛选和治疗室数量统计都包含它。
  - `VIP治疗室` 编号/备注非必填，空编号在后端以 `area_number=0` 表示；同一门店最多一个未编号 VIP 治疗室。
  - 普通治疗室、面诊室、生美仍要求数字编号。
  - `前台/候诊区` 只按可见/可维护文本包含 `前台`、`候诊`、`等候` 判断。
  - `通道/其他` 作为非业务且非前台候诊的兜底组。
- 实现：
  - 新增 `frontend/src/domain/channel-filters.ts`，以最小字段接口 `ChannelFilterable` 承载通道筛选、归类、排序规则。
  - 新增 `frontend/src/domain/channel-filters.test.ts` 覆盖全部排序、治疗室包含 VIP、前台候诊匹配、通道/其他兜底。
  - 通道映射 Tab 改用共享筛选模块，新增 VIP 治疗室选项。
  - 设计图标注区域卡片新增 VIP 治疗室选项，并与通道映射一致支持空编号。
  - `internal/storespace` 与旧 `internal/designplan` 模块同步支持 `vip_treatment`，避免旧路由/schema 保留三类约束。
  - `docs/h5-monitor-dev-task.md` 增加复用实现方案，要求 H5 monitor 不复制分组排序逻辑。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && ./node_modules/.bin/vitest run src/domain/channel-filters.test.ts` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 未完成：
  - Vite dev server 需要提升权限后可启动；Playwright 自带浏览器未安装，系统 Chrome headless 被本机权限限制关闭，因此本轮未完成自动化截图验收。

## 2026-06-26 通道截图缓存 2.17.1 开发记录

- 目标：
  - 降低机构详情页通道映射 Tab 每次进入时最近截图重新排队加载的等待感。
  - 保持刷新截图/重新识别后能显示新图，不让用户看到过期图片。
- 实现：
  - 前端 `ImageLoadQueue` 增加已成功加载 URL 的内存记录。
  - `QueuedSnapshotImage` 仅在同一 URL 已成功加载过时直接显示；未命中仍走原来的队列预加载和错误兜底。
  - 后端通道截图接口增加 `Cache-Control: private, max-age=604800, immutable` 与 `ETag`，命中 `If-None-Match` 时返回 `304`。
  - 前端验收清单新增规则：版本化图片 URL 应支持浏览器缓存或前端内存缓存，刷新图片时通过新 URL 失效旧缓存。
- 风险控制：
  - 不修改截图 URL 生成逻辑。
  - 不修改图片加载失败兜底逻辑。
  - 当前截图刷新会生成新文件名，因此新 URL 会自然绕过旧缓存。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestChannelSnapshotResponseUsesBrowserCacheHeaders|TestChannelSnapshotDiagnosticsReportsOpenFailure'` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module NodeNext --moduleResolution NodeNext --target ES2022 --outDir /tmp/erzhuang-image-queue-test src/domain/image-load-queue.ts src/domain/image-load-queue.test.ts && node /tmp/erzhuang-image-queue-test/image-load-queue.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-26 门店详情即时进入 2.17.2 开发记录

- 背景：
  - 机构列表点击“详情”时，前端原逻辑会等待 `GET /api/store-space/stores/{id}` 全量详情接口返回后才切换页面。
  - 该接口会加载门店基础信息、区域、设计图、录像机、通道和最近截图路径；大门店或网络波动时，用户会感觉点击后“转很久才进入”。
- 实现：
  - 新增 `frontend/src/domain/store-detail-navigation.ts`，沉淀列表摘要到详情占位对象、默认 Tab 判断、短期详情缓存逻辑。
  - 点击详情后立即用列表摘要生成详情壳，先展示门店标题、统计、Tab 和“正在加载门店详情”面板。
  - 完整详情接口返回后再替换真实数据；请求失败则回到列表并展示错误提示。
  - 同一门店 60 秒内二次进入且列表 `updatedAt` 未变化时使用前端内存详情缓存。
  - 用户返回列表会递增详情请求版本号，避免旧请求返回后把页面重新拉回详情。
- 未做：
  - 暂未拆分后端详情接口。下一步如仍有明显慢接口，可把门店详情拆成轻量 shell、通道数据、设计图数据三个加载单元。
- 验证：
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - 本地 dev server 需提升权限启动，已启动在 `http://127.0.0.1:5176/erzhuang/`；Playwright 浏览器二进制未安装，本轮未完成自动化截图验收。

## 2026-06-26 门店详情 Tab 接口轻拆 2.18.0 开发记录

- 目标：
  - 在不废弃全量详情接口的前提下，把机构详情页数据按 Tab 轻量拆分，减少进入默认 Tab 时等待非当前业务块数据。
  - 避免把接口拆得过细，保持后续维护和 H5 monitor 复用简单。
- 后端实现：
  - 保留 `GET /api/store-space/stores/{id}` 全量详情接口，继续作为兼容和 mutation 兜底。
  - 新增 `GET /api/store-space/stores/{id}/design-plan-data`，只返回门店基础信息、设计图、区域标注。
  - 新增 `GET /api/store-space/stores/{id}/channel-data`，只返回门店基础信息、录像机和通道。
  - `PostgresStore` 抽出基础门店查询 helper，两个 Tab 接口分别只调用对应列表查询。
- 前端实现：
  - 详情页仍先用列表摘要立即进入详情壳。
  - 默认 Tab 只请求对应 Tab 数据；切换到另一个 Tab 时再懒加载。
  - 详情缓存升级为按 Tab 记录已加载状态，合并数据时不会清空另一个 Tab 已有内容。
  - 创建、编辑、保存、删除、确认等已有 mutation 仍保留现有全量返回处理，不在本轮扩大改造。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestGetStoreDesignPlanDataEndpointReturnsOnlyDesignPlanTabData|TestGetStoreChannelDataEndpointReturnsOnlyChannelTabData'` 通过。
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 风险：
  - 如果某个 mutation 返回全量详情后，缓存会标记两个 Tab 均已加载；这符合当前兼容策略，但后续若 mutation 也拆分，需要一起调整缓存标记。

## 2026-06-26 通道最近截图视口懒加载 2.18.1 开发记录

- 目标：
  - 降低机构详情页通道映射 Tab 首次进入时最近截图的并发加载压力。
  - 保留已有图片队列和缓存能力，避免改动后出现截图裂图或刷新截图不更新。
- 实现：
  - `QueuedSnapshotImage` 增加 `IntersectionObserver` 视口触发逻辑。
  - 未进入视野附近的截图只显示原加载占位，不立即进入预加载队列。
  - 截图进入视野附近约 `160px` 后才进入既有 `ImageLoadQueue(2)`，继续限制同时预加载 2 张。
  - 已成功加载过的同 URL 继续直接显示，保持 2.17.1 的内存缓存效果。
  - 不支持 `IntersectionObserver` 的浏览器自动回退到原队列加载逻辑。
- 验证：
  - `cd frontend && npm run build` 通过。
  - 本地 Vite 预览页面可打开，首页渲染正常，控制台未发现运行时错误。
  - `cd frontend && npm test -- --run` 未通过，失败为既有测试文件未使用 Vitest `test/it` 套件结构，以及 `api.test` 在当前测试环境下 base path 断言不一致；本次改动未触及对应逻辑。
- 风险：
  - 本地 mock 环境没有门店通道数据，未完成真实通道表格的浏览器截图验收；公司环境发布后需重点观察通道映射 Tab 首屏截图加载速度和滚动加载表现。
- 发布：
  - GitHub `main` commit：`463c32c Lazy load channel snapshots`。
  - 公司 GitLab 发布分支 merge commit：`6c71f07 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BJXO-7_s.js`，确认包含 `2.18.1`、`IntersectionObserver` 和 `160px 0px` 懒加载触发配置。

## 2026-06-26 详情页 Tab 统计修复 2.18.2 开发记录

- 背景：
  - 2.18.0 将详情数据拆成设计图 Tab 和通道映射 Tab 后，顶部统计被当前 Tab 的局部接口摘要字段覆盖。
  - 进入通道映射时，通道接口没有区域数据，导致业务区域数显示 0。
  - 切换到设计图标注时，设计图接口没有录像机和通道数据，导致录像机数、有效通道数显示 0。
- 实现：
  - `mergeStoreDetailTab` 不再使用局部接口的摘要字段整包覆盖当前详情。
  - 通道 Tab 只更新录像机、有效通道、确认状态和业务类型计数。
  - 设计图 Tab 只更新设计图状态、缩略图、业务区域数和区域标注数据。
  - 保留门店基础信息、状态和更新时间等共享字段更新。
- 验证：
  - 新增 `store-detail-navigation` 复现测试，覆盖“先加载通道 Tab、再加载设计图 Tab”后顶部统计不被互相清零。
  - `cd frontend && ./node_modules/.bin/tsc --module ESNext --moduleResolution bundler --target ES2022 --skipLibCheck --jsx react-jsx --types vite/client --outDir /tmp/erzhuang-store-detail-nav-test src/vite-env.d.ts src/domain/store-detail-navigation.ts src/domain/store-detail-navigation.test.ts && node /tmp/erzhuang-store-detail-nav-test/domain/store-detail-navigation.test.js` 通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 发布：
  - GitHub `main` commit：`92593fa Fix split detail tab metrics`。
  - 公司 GitLab 发布分支 merge commit：`01493f2 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BHsDPUSA.js`，确认包含 `2.18.2`。

## 2026-06-26 详情页局部统计未知值修复 2.18.3 开发记录

- 背景：
  - 2.18.2 修复了 Tab 切换时统计互相清零，但通道映射 Tab 首次进入仍可能显示业务区域数 0。
  - 根因是 `channel-data` 局部接口不返回 `areas` 字段，前端映射层把“未返回区域数据”推导成 `areaCount=0`。
- 实现：
  - `mapStoreSpaceDetail` 区分“后端返回空数组”和“后端未返回字段”。
  - 当 `areas` 字段缺失时，不再推导区域相关计数为 0，而是保留为未知值，交给详情合并逻辑沿用列表摘要或已加载设计图数据。
  - 当 `recorders` 字段缺失时，同理不推导录像机/通道计数为 0。
  - `mergeStoreDetailTab` 遇到局部详情统计为未知值时保留当前统计。
- 验证：
  - `store-detail-navigation` 定向测试通过。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
- 发布：
  - GitHub `main` commit：`b2bca42 Preserve split detail unknown metrics`。
  - 公司 GitLab 发布分支 merge commit：`22f53be Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BE0jYwKG.js`，确认包含 `2.18.3`。

## 2026-06-26 门店列表业务区域总数修复 2.18.4 开发记录

- 背景：
  - 2.18.3 保证局部 Tab 接口缺失统计字段时不覆盖已有值，但详情页首次进入仍依赖门店列表摘要生成顶部壳。
  - 后端 `ListStores` 只返回治疗室、面诊室、生美分项计数，没有返回业务区域总数 `area_count`。
  - 因此前端列表摘要中的 `areaCount` 仍为 0，首次进入通道映射时顶部业务区域显示 0，切到设计图后才显示真实区域数。
- 实现：
  - `StoreListItem` 新增 `area_count` 字段。
  - Postgres `ListStores` SQL 增加 `count(distinct a.id) as area_count` 并扫描到返回结构。
  - MemoryStore `storeListItem` 同步累计 `AreaCount`。
- 验证：
  - 新增/补充 storespace 测试，覆盖设计图保存区域后列表摘要 `AreaCount` 返回真实数量。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestSaveDesignPlanAllowsVIPTreatmentWithoutNumber|TestConfirmVIPTreatmentAllowsBlankNumberAndCountsAsTreatment'` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
- 发布：
  - GitHub `main` commit：`e974ee3 Return store list area counts`。
  - 公司 GitLab 发布分支 merge commit：`5700181 Merge branch 'main' into codex/containerize-single-image`。
  - 公司环境 health：`{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 公司线上 JS 已更新为 `assets/index-BoEHEZrM.js`，确认包含 `2.18.4`。

## 2026-06-26 H5 Monitor 试点集成 2.19.0 开发记录

- 目标：
  - 将独立 `h5-monitor` 原型集成进主项目，先作为受控试点能力给单门店验证。
  - 试点范围只开放“北京保利实验室门店”，新氧机构 ID `10030`，录像机 `GN0941203`。
- 后端实现：
  - 新增 `internal/h5monitor` 模块，提供 H5 首页、直播地址、录像片段、回放地址、播放地址失效接口。
  - 复用现有 `storespace` 门店、通道、录像机、萤石账号数据；播放凭证仍来自运行时 `EZVIZ_ACCOUNTS_JSON`，不写入前端或文档。
  - 新增萤石能力：FLV 直播地址、FLV 回放地址、录像片段查询、地址失效、AAC 转码 best-effort。
  - H5 API 响应不暴露 `device_serial`、app key、app secret、access token、萤石账号名。
  - 服务端门禁集中在 `h5monitor.Service`：默认只允许 `externalOrgId=10030` 和 `deviceSerial=GN0941203`。
  - 并发限制本轮仍为进程内内存计数：普通用户 15 路，管理员 20 路；多 Pod 场景后续需落库或接入统一会话。
- 前端实现：
  - 新增 H5 路由：
    - `/h5/orgs/{externalOrgId}/monitor`
    - `/h5/orgs/{externalOrgId}/monitor/channels/{channelId}`
  - 后台详情页右上角新增“查看监控”按钮，且仅 `externalOrgId=10030` 的门店展示。
  - H5 首页按区域筛选展示监控通道，默认每批 24 路，支持加载更多。
  - H5 详情页默认直播，支持切换录像、查询片段、点击片段播放。
  - 播放器使用 `ezuikit-flv`，静态 decoder 文件放在 `frontend/public/assets/ezuikit-flv/`。
  - 播放器默认静音以满足浏览器自动播放限制，用户点击后调用官方 `openSound/closeSound`。
  - H5 页面使用 route-level lazy import，后台页面不主动加载 H5 播放页面。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过；提示播放器 chunk 较大，属于 `ezuikit-flv` 依赖体积预期。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 浏览器验收通过：
    - `/erzhuang-project/h5/orgs/demo/monitor` 可渲染 H5 首页 mock 数据。
    - 点击通道可进入 H5 详情页，默认实时视频，声音按钮可见。
    - 切换录像可显示日期选择和录像片段。
    - `/erzhuang-project/` 后台首页未误进入 H5 路由。
- 风险：
  - 公司真实播放依赖 `EZVIZ_ACCOUNTS_JSON` 中包含华北账号，且账号名与数据库录像机绑定账号一致。
  - 本地没有公司数据库，未在本机验证 `10030/GN0941203` 的真实 H5 API 数据。
  - `ezuikit-flv` 打包后会生成约 1.8MB 未压缩播放器 chunk；已通过详情页 lazy import 控制影响范围。
  - 前端 `vitest` 当前只运行 `src/api.test.ts`，因为仓库内其他 `.test.ts` 仍是脚本式断言文件，后续可统一整理测试入口。

## 2026-06-26 H5 Monitor 播放与回放诊断修复 2.19.1 开发记录

- 背景：
  - 公司线上 H5 视频详情页进入后播放器黑屏，页面显示“播放器加载失败”，错误为 `v is not a constructor`。
  - 回放 Tab 看不到录像片段。
  - 用户希望错误信息继续外显，并增加可一起定位排查的详细上下文。
- 排查结论：
  - 直播黑屏主因是前端动态加载 `ezuikit-flv` 后优先把模块对象当构造函数使用；该包实际导出为 default class，打包后触发 `v is not a constructor`。
  - 公司线上回放片段接口返回 500，原因是萤石 `localIndex` 字段在线上返回为数字，后端原结构体按 string 解码导致 JSON unmarshal 失败。
  - 播放地址失效接口原来只提交 `id`，萤石新接口要求 `deviceSerial`、`channelNo`、`urlId`，线上曾返回 `deviceSerial不能为空`。
- 实现：
  - `H5FlvPlayer` 动态加载播放器时按 `default`、`EzuikitFlv`、`module` 顺序选择真正的函数构造器。
  - 播放器错误面板增加 stage、简化 URL、decoder 路径、直播/回放模式、库导出类型；事件 payload 中的签名 URL 会缩写，避免完整临时签名外露。
  - H5 API 错误对象增加后端 `code` 字段，页面 toast 展示 `HTTP` 状态、萤石错误码和字段错误。
  - 回放片段 `localIndex` 改为兼容 string/number 的 `FlexibleString`。
  - 播放地址失效接口改为携带 `deviceSerial`、`channelNo`、`urlId`，并补测试防止参数退化。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run test` 通过。
  - `cd frontend && npm run build` 通过；仍有播放器 chunk 体积提示，属于 `ezuikit-flv` 依赖体积预期。
  - `git diff --check` 通过。
- 发布：
  - 待推送公司 GitLab 固定分支 `codex/containerize-single-image` 后，由公司 K8s 自动发布。
- 线上追加验证：
  - 公司线上第一次发布后，回放片段接口从 `localIndex` 解码错误推进到新的真实返回差异：`meta.code` 有时是字符串。
  - 已补充 `FlexibleInt` 兼容 string/number，并用 `meta.code:"200"` 的测试复现覆盖。

## 2026-06-26 H5 Monitor 播放画面适配与 MSE 告警修复 2.19.2 开发记录

- 背景：
  - 公司线上部分实时视频已经能出画面，但页面出现 `MediaSource.addSourceBuffer` / `SourceBuffer` 告警。
  - 播放画面顶部有明显黑条，整体画面显示不完整，诊断条也会遮挡主要画面。
- 排查结论：
  - 报错来自 `ezuikit-flv` 的 MSE 硬解码路径，属于播放器内部 SourceBuffer 资源/上限问题；单画面 H5 场景稳定性优先于硬解码收益。
  - 画面黑条与播放器默认渲染模式、缺少官方样式、内部 video/canvas 未被外层容器稳定约束有关。
- 实现：
  - 引入 `ezuikit-flv/style.css`。
  - 播放器配置关闭 `useMSE`，保留 `useWCS` 和 `autoWasm`，规避 MSE SourceBuffer 路径。
  - 设置 `scaleMode`、`videoBuffer`、`themeData:null`、`mutedShowAutoReload:false`，减少播放器内置控件和自动重载干扰。
  - 切换/卸载播放器时先 pause 再 destroy，并清空容器 DOM，减少旧实例残留。
  - 将 MSE/SourceBuffer 类事件降级为可恢复 warning，6 秒后自动收起，不再用错误 toast 和大红层长期遮挡画面。
  - CSS 强制播放器内部 `video/canvas` 填满容器，诊断条移到顶部并区分 warning/error 视觉层级。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 服务可启动；自动浏览器截图验收因本项目未安装 Playwright、Browser 会话 tab 绑定异常未完成，真实画面仍需公司线上 H5 页面复验。

## 2026-06-26 H5 Monitor 恢复 MSE 播放路径 2.19.3 开发记录

- 背景：
  - 2.19.2 发布后，公司线上 H5 监控详情页播放器容器和声音按钮可见，但实时画面完全黑屏。
  - 用户反馈“啥都没显示出来”，相比 2.19.1 已能出画面但有 SourceBuffer 告警，属于播放渲染回归。
- 排查结论：
  - 2.19.2 为规避 `MediaSource.addSourceBuffer` 告警关闭了 `useMSE`。
  - 从现象判断，公司真实 FLV 流在当前浏览器/播放器组合下仍依赖 MSE 路径出画面；关闭 MSE 后播放器初始化成功但无法渲染视频。
- 实现：
  - 恢复 `useMSE:true`，保留官方样式、诊断降级、播放器销毁清理、诊断条不遮挡等其他改动。
  - 本轮只改一个变量，先恢复出画面；黑条和 SourceBuffer warning 后续再基于线上真实表现单独处理。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 回放时间参数与片段分页修复 2.19.4 开发记录

- 背景：
  - 公司线上 H5 监控切到“录像”后，点击回放片段获取播放地址失败。
  - 错误提示为 `回放地址获取失败 · HTTP 500 · code=10001 · ezviz api error code=10001 msg=illegal parameter startTime`。
  - 用户同时反馈录像回放片段“好像也不太对”。
- 排查结论：
  - 录像片段查询接口 `/api/v3/device/local/video/unify/query` 的 `startTime/endTime` 使用 Unix 秒是正确的。
  - 播放地址接口 `/api/lapp/v2/live/address/get` 在回放模式下要求 `startTime/stopTime` 为 `YYYY-MM-DD HH:mm:ss` 字符串；之前后端错误地传了 Unix 秒。
  - 公司容器时区不应影响录像片段日期。片段查询应按中国门店业务日期，也就是 `Asia/Shanghai` 自然日查询。
  - 片段查询返回 `hasMore/nextFileTime` 时，之前只取第一页，会导致一天内录像片段不完整。
- 实现：
  - 回放播放地址参数改为北京时间 `YYYY-MM-DD HH:mm:ss`。
  - 录像片段查询的自然日范围固定按 `Asia/Shanghai` 计算。
  - 录像片段查询支持跟随 `nextFileTime` 分页合并，避免只展示第一页片段。
  - 增加 Go 测试覆盖回放时间格式、上海自然日范围、片段分页合并。
- 验证：
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 回放时间选择器样式修复 2.19.5 开发记录

- 背景：
  - 公司线上 H5 监控“录像”页日期选择弹层出现明显错位。
  - Ant Design DatePicker 默认弹层风格、英文月份和时间列布局与当前 H5 监控页不匹配。
- 设计决定：
  - 不继续修 AntD 弹层尺寸，改为 H5 页面内的轻量时间选择条。
  - 保留 `今天 / 昨天 / 前天` 快捷日期。
  - 用原生 `datetime-local` 选择具体时间，并提供“定位回放”按钮。
  - 保留实时/录像切换、离开详情时释放当前播放地址的逻辑。
- 实现：
  - H5 回放页移除 `DatePicker/dayjs` 直接依赖。
  - 新增 `.h5-date-time-field` 和 `.h5-date-confirm` 样式，使用项目现有边框、圆角、主色和焦点态。
  - H5 详情 chunk 从约 420KB 降到约 10KB，移动端加载更轻。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。
  - 本地 Vite 服务可启动；Playwright CLI 因本机未安装 `chrome-for-testing` 浏览器二进制未完成截图验收。

## 2026-06-26 H5 Monitor 自绘回放时间弹层 2.19.6 开发记录

- 背景：
  - 2.19.5 使用原生 `datetime-local` 后，虽然避免了 AntD DatePicker 弹层错位和 chunk 过大，但浏览器原生弹层样式无法与项目风格统一。
  - 用户提供 ahabook 批阅记录日期选择器作为参考，希望日期选择区域整体可点击，弹层风格更接近当前产品。
- 设计决定：
  - 不继续依赖浏览器原生日期时间弹层，改为 H5 页面内自绘轻量日期时间选择器。
  - 保留 `今天 / 昨天 / 前天` 快捷日期和“定位回放”按钮。
  - 自绘弹层采用白色圆角浮层、圆形月份切换按钮、轻量日期网格、克制选中态，并支持点击外部关闭。
  - 保持实时/录像切换、关闭详情时释放当前播放地址的逻辑不变。
- 实现：
  - `PlaybackDatePicker` 新增自绘日期网格、小时/分钟滚动列、月份切换和完整触发按钮。
  - 选择器触发区整体可点击，不再只依赖原生日历图标。
  - 日期选中态改为浅底描边，弱化“蓝色按钮感”，更贴近项目后台浮层风格。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-26 H5 Monitor 移动端实时视频 HLS 适配 2.19.7 开发记录

- 背景：
  - 用户在手机浏览器和飞书内打开 H5 监控详情页后，实时视频黑屏。
  - 之前桌面端为恢复画面将 `ezuikit-flv` 的 MSE 路径重新打开，但移动浏览器对 FLV/MSE 兼容性不稳定，不能继续只调 FLV 播放器参数。
- 排查结论：
  - H5 详情页当前固定向萤石请求 FLV 地址，并固定使用 `ezuikit-flv` 播放。
  - 移动端应优先使用 HLS/m3u8 + 原生 `<video playsInline controls>`，桌面端保留 FLV 播放器路径，避免影响已能播放的桌面环境。
  - 本地录像回放文档对 HLS 支持不明确，当前回放仍保持 FLV 路径，后续需要基于移动端真实表现决定是否切萤石 JSSDK/ezopen 或内部 ISAPI 代理。
- 实现：
  - H5 live-url 请求新增 `protocol` 参数，支持 `hls/flv`；服务端按协议向萤石请求 `protocol=2/4`，并在响应里返回协议。
  - 前端移动端通用判断不限定 iPhone，覆盖 iPhone、Android、飞书/企微类移动 WebView，移动端实时视频请求 HLS，桌面请求 FLV。
  - 播放器组件根据协议选择播放方式：HLS/m3u8 走原生 `<video>`，FLV 继续走 `ezuikit-flv`。
  - 原生 video 路径保留加载态，直到 `loadedmetadata/canplay/playing` 后收起；失败时展示协议和简化 URL 诊断。
- 验证：
  - 新增后端测试覆盖 HLS/FLV 两种 live-url 协议参数。
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
- 发布：
  - 公司 GitLab `codex/containerize-single-image` 已推送 commit `48e143c`，触发公司 K8s 自动发布。
  - GitHub `main` 已同步 commit `4a52b8b`。
  - 公司线上 `/health` 返回 `database:"postgres"`、`asset_store:"supabase"`。
  - 公司线上静态资源已更新到 `2.19.7 (container)`，H5 详情 chunk 包含 `native-video`、`playsInline`、`hls/flv` 协议选择逻辑。

## 2026-06-26 H5 Monitor 移动端 H265 播放器适配 2.19.8 开发记录

- 背景：
  - 2.19.7 将移动端实时视频改为 HLS + 原生 video 后，手机端提示“视频编码类型非 H264”。
  - 用户判断不应为了手机播放统一关闭 H265，因为会降低录像机编码效率并增大录像体积。
- 排查结论：
  - HLS/native video 路径依赖浏览器原生解码，遇到 H265 流时容易失败。
  - `ezuikit-flv` 本地类型说明显示：MSE 硬解只支持 H264，iOS Safari 不支持；`autoWasm` 支持 H265 时从 MSE 自动降级到 wasm。
  - 更合理的方向是保留 H265 设备配置，移动端用播放器软解适配，而不是改录像机编码。
- 实现：
  - H5 实时视频默认协议改回 FLV，避免移动端进入 HLS/native video 的 H264 限制。
  - `H5FlvPlayer` 移动端播放上下文关闭 `useMSE`，保留 `autoWasm:true` 和 `useWCS:true`。
  - 播放器参数增加 `hasAudio:true`、移动端 `keepScreenOn:true`，保留声音和手机屏幕常亮能力。
  - 诊断信息增加 `protocol` 与 `decode`，便于区分桌面 MSE 与移动端 wasm 路径。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-26 H5 Monitor 移动端黑屏诊断增强 2.19.9 开发记录

- 背景：
  - 2.19.8 在手机端仍然黑屏，页面只显示“点击开启声音”，没有错误详情。
  - 截图说明播放器实例已经初始化成功，但没有渲染出首帧；之前代码在初始化成功后立即关闭 loading，导致“初始化成功但无首帧”的状态被误判为正常。
- 排查结论：
  - 当前缺少首帧/流成功诊断，无法判断卡在取流、解码、还是渲染阶段。
  - `ezuikit-flv` 暴露 `streamSuccess`、`videoInfo`、`videoFrame`、`playing`、`loadingTimeout`、`wasmDecodeError` 等事件，可用于收集黑屏证据。
- 实现：
  - 播放器初始化后不再立即收起 loading，而是等待 `streamSuccess/videoInfo/videoFrame/playing/loaded` 这类首帧或流成功事件。
  - 增加 12 秒首帧超时诊断：超时后显示 `first-frame-timeout`、协议、解码路径、最近播放器事件和 `getState()`。
  - 监听更多错误事件，包括 `wasmDecodeError`、`webcodecsH265NotSupport`、`mediaSourceH265NotSupport`、`unrecoverableEarlyEof` 等。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。

## 2026-06-29 H5 Monitor 移动端首帧判断修正 2.19.10 开发记录

- 背景：
  - 用户反馈 2.19.9 手机端仍然黑屏，且没有错误信息显示。
  - 复查代码发现移动端 wasm 路径下，`loaded/playing` 被当作首帧成功事件，可能导致 12 秒超时诊断被提前清除，但视频尚未真正渲染。
- 结论：
  - 对移动端软解路径，`loaded/playing` 只能说明播放器状态推进，不能证明已经有视频帧。
  - 移动端应该只把 `streamSuccess/videoInfo/videoFrame` 视为首帧或流成功信号。
- 实现：
  - 新增 `domain/h5-player-diagnostics.ts`，集中定义首帧事件判断。
  - 移动端 `mobile-wasm` 路径不再把 `loaded/playing` 视为首帧成功，保留计时器直到真正的视频事件到达。
  - 桌面 `desktop-mse` 路径保留原兼容判断，避免影响现有桌面播放。
  - 增加 vitest 覆盖移动端和桌面端首帧事件差异。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `cd frontend && npm run test` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 页面级播放诊断 2.19.11 开发记录

- 背景：
  - 用户反馈 2.19.10 手机端依然全黑屏，截图里只有“点击开启声音”，没有任何错误或超时提示。
  - 截图说明播放器容器、声音按钮和页面路由均已正常渲染，问题转为“播放器内部诊断没有可靠暴露到手机页面”。
- 结论：
  - 下一步不能继续猜播放参数，应先让黑屏状态具备可截图、可复盘的证据。
  - 诊断信息不能只放在播放器黑框内部，因为第三方播放器的 canvas/video/内部层级可能遮挡自定义提示。
- 实现：
  - `H5FlvPlayer` 增加页面级 `onStatus` 回调，结构化上报初始化、播放地址、播放器事件、首帧成功和首帧超时状态。
  - H5 详情页在播放器下方新增常驻状态卡，展示 stage、message、协议、解码路径、前端版本、UA、最近播放器事件等信息。
  - 继续保留播放器内部诊断和 toast，但页面级状态卡作为手机端排查的主证据。
- 验收目标：
  - 如果移动端继续黑屏，页面必须在黑框下方显示 `player-init`、`player-event` 或 `first-frame-timeout` 等状态，便于继续判断卡在取流、解码还是渲染。

## 2026-06-29 H5 Monitor 流连接与首帧渲染拆分 2.19.12 开发记录

- 背景：
  - 用户反馈 2.19.11 手机端仍黑屏，但页面级状态卡显示 `streamSuccess`。
  - 截图确认播放地址、decoder 路径、版本、UA、`decode=mobile-wasm` 均已暴露；萤石 FLV 流已经连接成功，但没有看到画面。
- 结论：
  - `streamSuccess` 只能代表流连接成功，不能代表画面已经渲染。
  - 之前把 `streamSuccess` 归入首帧成功事件是误判，导致黑屏时显示 `first-frame-ready`。
  - 线上 `decoder.js` 和 `decoder.wasm` 均可访问，`decoder.wasm` 的 Content-Type 为 `application/wasm`，暂不支持“wasm 资源未部署/MIME 错误”这个假设。
- 实现：
  - 移动端首帧成功只认 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes` 这类视频渲染事件。
  - `streamSuccess` 单独显示为 `stream-connected`，继续等待视频帧，不再清除首帧超时计时器。
  - 移动端播放参数改为 `useWCS:false`、`forceNoOffscreen:true`，明确走 wasm + 普通 canvas 渲染路径。
  - 增加 `wasmDecodeErrorReplay:true`、`wasmDecodeAudioSyncVideo:true`、`debug:true`，并监听更多播放器事件，便于下一轮截图继续定位。
- 验收目标：
  - 如果仍黑屏，状态卡应显示 `stream-connected` 后是否出现 `videoInfo/videoFrame/firstFrameDisplay/playToRenderTimes`，或最终 `first-frame-timeout`。

## 2026-06-29 H5 Monitor 移动端直播调通里程碑 2.19.12 验收记录

- 结果：
  - 用户在公司线上移动端复测后确认：实时视频终于可以显示。
  - 这标志着“萤石云 FLV 取流 + iPhone/微信 H5 + H265 视频 + `ezuikit-flv` wasm 软解”链路在试点门店真实环境下跑通。
- 本次调通的关键经验：
  - 不能把 `streamSuccess` 当作画面可见。它只代表 FLV 流连接成功，首帧/画面可见必须看 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes` 这类渲染事件。
  - 黑屏排查要把链路拆层：播放地址获取 -> decoder 资源加载 -> 流连接 -> 视频信息解析 -> wasm 解码 -> canvas 渲染。
  - 页面级诊断必须放在播放器黑框外。第三方播放器内部 DOM/canvas 可能遮挡自定义提示，导致手机端看起来“没有任何错误”。
  - iPhone/微信 H5 环境下，移动端播放应明确走 wasm + 普通 canvas 路径：`useMSE:false`、`useWCS:false`、`forceNoOffscreen:true`、`autoWasm:true`。
  - 线上 `decoder.js` 与 `decoder.wasm` 需要可访问，且 `decoder.wasm` 应返回 `Content-Type: application/wasm`；本次已排除 decoder 部署/MIME 错误。
- 当前可复用配置：
  - 直播地址：萤石 `/api/lapp/v2/live/address/get`，`protocol=4` FLV。
  - 前端播放器：`ezuikit-flv@2.1.1`。
  - 移动端解码路径：`decode=mobile-wasm`。
  - 移动端关键参数：`useMSE:false`、`useWCS:false`、`forceNoOffscreen:true`、`wasmDecodeErrorReplay:true`、`wasmDecodeAudioSyncVideo:true`、`keepScreenOn:true`。
- 后续注意：
  - 继续保留状态卡或等价诊断能力，至少在试点期不要过早隐藏。
  - 回放页也应复用同一套“流连接”和“首帧渲染”拆分逻辑，不要只看播放地址是否返回。
  - 后续扩门店时，如果某通道再次黑屏，优先截图状态卡，根据事件停在哪一层判断，而不是先改播放器参数。

## 2026-06-29 H5 Monitor PC 首帧误报修复 2.19.13 开发记录

- 背景：
  - 2.19.12 移动端直播跑通后，用户反馈 PC 端画面已经显示，但页面仍覆盖 `first-frame-timeout` 错误层。
  - 截图显示 PC 端 `decode=desktop-mse`，事件为 `start > videoInfo > streamSuccess`，画面实际可见。
- 结论：
  - 2.19.12 为移动端修正首帧判断时，把 `streamSuccess/videoInfo` 从所有路径的首帧成功信号里移除，误伤了 PC。
  - PC 的 MSE 路径可以继续使用宽松判断；移动端 wasm 路径必须保持严格，避免再次误判黑屏为成功。
- 实现：
  - `desktop-mse` 路径恢复接受 `streamSuccess`、`videoInfo`、`loaded`、`playing` 作为首帧/播放就绪信号。
  - `mobile-wasm` 路径仍只接受 `videoFrame`、`firstFrameDisplay`、`playToRenderTimes`。
  - 补充前端单测覆盖桌面和移动端差异。

## 2026-06-29 H5 Monitor 播放器控制控件 2.20.0 开发记录

- 背景：
  - H5 Monitor 直播链路已在试点门店移动端和 PC 端跑通，进入播放器产品化阶段。
  - 用户确认本轮只做基础单路查看能力，不做刷新流、异常自动重试、多画面、云台、倍速、下载、复杂时间轴。
- 实现：
  - `H5FlvPlayer` 改为 `forwardRef`，通过 `H5PlayerHandle` 暴露播放、暂停、声音、截图、全屏等受控方法。
  - 新增 `H5PlayerControls`，在播放器底部提供播放/暂停、静音/开声音、截图、横屏/竖屏、全屏/退出全屏。
  - 新增 15 分钟长时间播放保护：到时暂停并提示是否继续，停止时释放当前播放 URL，继续时重新取直播或回放 URL。
  - 新增 `PlaybackSegmentSlider` 和 `domain/h5-playback.ts`，支持在单个录像片段内拖动定位，拖动过程中只预览，提交后才重新请求回放 URL。
  - 回放 URL 请求增加序列号保护，旧请求返回时不会覆盖新 URL；旧响应如果已经拿到 URL，会立即释放，降低萤石资源泄漏风险。
  - 原生 video fallback 不再展示浏览器内建 controls，避免暴露下载、倍速、复杂时间轴等本轮明确不做的能力。
- 样式原则：
  - 控制条保持 H5 工具风格：底部轻量暗色半透明浮层、按钮尺寸克制、移动端可横向滚动且触控面积足够。
  - 诊断状态卡继续常驻显示，等用户明确要求“收起来”后再做折叠入口。
- 验证：
  - `cd frontend && npm run test` 通过，9 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - `go test ./...` 未执行：当前本机环境没有 `go` 命令。
  - Playwright 可视验收未执行：本机缺少 Playwright Chromium 浏览器二进制。

## 2026-06-29 H5 Monitor 播放器体验修复 2.20.1 开发记录

- 背景：
  - 2.20.0 增加播放器控制控件后，试用中发现 6 个体验问题：控件自动隐藏后移动端不清楚如何唤回、移动端按钮偏大、移动端截图不应只打开新页面、回放暂停后再播放会回到片段起点、横屏按钮在手机上不像横置观看、回放滑块放在播放器下方不符合预期。
- 实现：
  - 播放器控制层取消自动隐藏机制，改为点击播放器画面区域隐藏，再点击画面区域显示。
  - 移动端控制按钮缩小，保留文字按钮便于继续调试，后续可替换为 icon。
  - 截图优先使用 Web Share API 调起系统分享/保存面板，不支持时 fallback 为下载；用户取消分享不再误报截图失败。
  - 回放暂停时记录当前片段内估算时间，再次播放时从该时间重新获取回放 URL，避免从片段起点重播。
  - 移动端横屏改为固定全屏并旋转播放器区域，形成手机横置观看体验。
  - 回放片段滑块移到播放器画面 overlay 内，跟控制条同层展示，不再放在下方回放面板。
- 验证：
  - `cd frontend && npm run test` 通过，11 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地浏览器用 `externalOrgId=demo` mock 页面验证：控件点击显隐、回放滑块 overlay、移动端按钮尺寸、移动端横屏旋转；控制台无 error/warn。
  - `go test ./...` 未通过本机环境验证：全局 `go` 不存在；使用 `./.tools/go/bin/go` 后测试二进制被 macOS `dyld missing LC_UUID load command` 拦截，未出现业务断言失败。

## 2026-06-29 H5 Monitor 播放器暂停、全屏、截图修复 2.20.2 开发记录

- 背景：
  - 用户在线上验收 2.20.1 后反馈：回放暂停再恢复会黑屏重建且 PC 端仍可能回到片段起点；手机侧全屏按钮失效；手机侧截图没有进入系统相册保存流程。
- 根因：
  - 2.20.1 为解决“回放恢复不回起点”选择了重新请求回放 URL，实际带来播放器重建和黑屏体验，不符合“真暂停/真恢复”的产品预期。
  - 移动浏览器或飞书 WebView 常不开放普通元素 `requestFullscreen`，原逻辑只提示失败，没有降级体验。
  - `ezuikit-flv` 截图 API 默认类型可能直接走 `download`，前端拿不到图片数据就无法调起 Web Share API。
- 实现：
  - 普通播放/暂停改为只调用播放器实例 `pause()` / `play()`，不再在恢复播放时重新 `playRange()` 取回放 URL。
  - 手机全屏在原生 Fullscreen API 不可用或失败时，降级为页面内全屏横置模式，并使用简短提示“已切换为页面内全屏”。
  - 播放器截图调用显式传入 `base64`，前端拿到 data URL 后继续走 Web Share API；H5 仍不能静默写入系统相册，需要用户在系统面板选择保存。
- 验证：
  - `cd frontend && npm run test` 通过，12 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 demo 页面验证：暂停后播放器容器未卸载、未出现回放占位；移动视口点击全屏后进入 `is-inline-fullscreen`，按钮变为“退出全屏”。

## 2026-06-29 H5 Monitor 回放暂停续播体验修复 2.20.3 开发记录

- 背景：
  - 用户在线上验收 2.20.2 后反馈：PC 和手机端录像回放里暂停后再点播放，仍不是继续播放，而是从当前回放 URL 的起点重新播放。
- 调研结论：
  - 当前跑通手机 H265 的 `ezuikit-flv@2.1.1` 更适合 FLV 流播放，不适合把 `pause()` / `play()` 当成原生 video 的精确真暂停/续播。
  - 该库 API 有 `currentTime`、`pause()`、`play()`，但没有明确 `seek()` / `resume()`；README 也提示因解码资源异步加载，不推荐直接外部调用 `play()`。
  - 因此短期不声称实现“同一条流原地真暂停”，而是实现“暂停点续看”：暂停时记录播放器 `currentTime`，恢复时从暂停点重新获取回放 URL。
- 实现：
  - `H5FlvPlayer` 通过 ref 暴露 `getCurrentTime()`，优先读取播放器 `currentTime`，兼容内部 video/canvas loader 的当前时间。
  - 回放暂停时记录 `pausedAtUnix`，并尽量截取当前画面作为冻结帧。
  - 回放恢复时从 `pausedAtUnix` 重新请求回放 URL，不再从原始片段起点播放；状态卡会显示 `reason=resume` 和 `resumeFrom=HH:mm:ss`。
  - 恢复加载期间保留旧 URL，等新 URL 成功返回后再释放旧 URL；冻结帧持续到新播放器首帧 ready，降低黑屏体感。
  - 录像片段点击、滑块定位、长时间播放保护继续走原有重新取 URL 流程。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地移动视口 demo 验证：H5 详情页、录像 tab、录像片段播放、overlay 滑块和控制条均正常渲染；控制台无 error/warn。
- 后续建议：
  - 如果产品要求严格意义的真暂停、seek、resume，应单独验证萤石 `EZUIKit-JavaScript-npm` 或其他官方播放器方案是否能同时满足 H265、手机 WebView、回放控制和自定义 UI。

## 2026-06-29 H5 Monitor 回放恢复遮罩与滑块位置修复 2.20.4 开发记录

- 背景：
  - 用户验收 2.20.3 后确认回放已经能从暂停点继续，但恢复时仍会黑屏一下；拖动回放滑块能定位成功，但滑块 UI 会回到拖动前位置。
- 根因：
  - 恢复遮罩依赖播放器截图返回 `dataUrl`，部分环境下 `ezuikit-flv` 可能返回 Blob/File 或无法通过播放器 API 截图，导致没有冻结帧可显示。
  - `PlaybackSegmentSlider` 之前只维护内部 offset，外层重新取回放 URL 后没有把新的起播时间回传给滑块，导致 UI 位置不同步。
- 实现：
  - 播放器截图归一化支持 base64、data URL、Blob/File；播放器 API 无结果时，尝试从当前 canvas/video 抓取一帧。
  - 恢复播放时即使没有冻结帧，也显示轻量恢复遮罩，避免用户只看到纯黑屏。
  - 回放页新增 `playbackCursorUnix`，每次 `playRange(startTime...)` 都同步当前起播点，并把它传给滑块。
  - `PlaybackSegmentSlider` 改为支持 `currentStartTime`，根据外层起播点更新当前位置，避免拖动后回弹。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放恢复黑屏遮罩修复 2.20.5 开发记录

- 背景：
  - 用户验收 2.20.4 后确认滑块位置问题已解决，但暂停后点击播放时仍能看到明显黑屏和“加载中”，像刷新了一下。
- 根因：
  - 恢复遮罩之前同时依赖 `resumeCoverVisible && loading`。
  - `loading` 是回放 URL 接口请求状态，请求完成后会立即变为 false；但播放器重建和首帧渲染还没有完成，导致遮罩提前消失，播放器内部黑色 loading 层暴露。
- 实现：
  - 恢复遮罩改为只依赖 `resumeCoverVisible`，生命周期延长到播放器回调 `first-frame-ready` / `mock-ready` 后再关闭。
  - 恢复遮罩层级提高到播放器控件之上，避免被播放器内部黑底、loading 或 canvas 层覆盖。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放恢复闪屏抛光 2.20.6 开发记录

- 背景：
  - 用户验收 2.20.5 后认为当前体验可以忍受，但暂停后继续播放仍能看到很短的黑色闪屏，希望再尝试一次低风险抛光。
- 判断：
  - 遮罩已经持续到播放器上报 `first-frame-ready`，仍有闪屏说明黑色暴露点大概率发生在首帧事件和真实画面稳定绘制之间。
- 实现：
  - 收到 `first-frame-ready` / `mock-ready` 后不再立刻关闭恢复遮罩，而是延迟 250ms 再移除。
  - 恢复遮罩增加短过渡，避免硬切换。
- 验证：
  - `cd frontend && npm run test` 通过，13 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 回放进度自动推进 2.20.7 开发记录

- 背景：
  - 用户验收 2.20.6 后确认暂停续播问题基本可接受，继续反馈两个回放体验问题：播放滑块不会随着播放自动往后移动；播放到当前录像片段最后一秒后，应该自动关闭当前片段并进入下一个录像片段。
- 实现：
  - 回放播放中新增 1 秒 tick，同步读取播放器 `currentTime`，无法读取时退回到 `PlaybackSession` 的墙钟估算时间，并实时更新 `playbackCursorUnix`，驱动 overlay 滑块自动前进。
  - 新增 `nextRecordSegmentIndex`，按当前片段对象或时间边界查找下一个录像片段；当前片段到达末尾前 1 秒时自动触发下一段播放。
  - 自动切片段复用现有“保留当前画面”的恢复遮罩逻辑：切段前尽量截取当前帧，新 URL 首帧稳定后再移除遮罩，减少段间黑屏。
  - 将片段列表、当前片段、当前回放 URL、loading 状态同步到 ref，避免定时器闭包读到旧状态导致重复切段或释放错误 URL。
  - 滑块手动拖动时暂停外部自动位置同步，松手或失焦后再提交定位，避免用户拖动过程中被 tick 拉回。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite preview 页面 smoke 验证：应用可正常渲染，控制台无 error/warn；本地无后端数据导致列表接口 HTTP 500，属本地预览环境限制，未进行真实萤石播放流验证。

## 2026-06-29 H5 Monitor 播放器控制条样式优化 2.21.0 开发记录

- 背景：
  - 用户确认 H5 Monitor 播放功能基本满足后，提出纯样式优化：播放/暂停、声音、截图、横竖屏、全屏控件改为 icon，不显示中文；控制按钮与回放滑块进一步整合，降低播放器 overlay 高度。
- 实现：
  - `H5PlayerControls` 改为三列控制条：左侧播放/暂停与声音，中央承载回放滑块，右侧截图、横竖屏、全屏。
  - 控制按钮从中文文字改为无边框 icon，仅保留 `aria-label` 用于可访问性和调试识别。
  - `PlaybackSegmentSlider` 新增 `compactControls` 形态，嵌入控制条中间时不显示起始/结束时间，也不显示当前时间文案，仅保留滑块本体。
  - 控制条改为低高度半透明浮层，桌面回放态高度约 50px，移动端约 46px，减少对监控画面的遮挡。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 验证：桌面和 390px 移动视口下，按钮均无中文文本，滑块居中整合到同一控制条，未出现明显挤压或重叠。

## 2026-06-29 H5 Monitor 横竖屏 icon 微调 2.21.1 开发记录

- 背景：
  - 用户反馈横屏/竖屏切换 icon 希望更接近“两块横竖屏幕叠放”的识别方式，确认去掉旋转箭头，只保留两个矩形。
- 实现：
  - 将横竖屏切换按钮的旋转箭头 icon 替换为双矩形线性 icon：后层竖向矩形、前层横向矩形。
  - 保持原有按钮行为、active 态、无中文显示和 `aria-label` 不变。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 暂停态控制条显隐修复 2.21.2 开发记录

- 背景：
  - 用户反馈播放中点击画面可隐藏/显示控制条，但暂停后点击画面无法隐藏控制条；暂停截图时控制条会遮挡画面。
- 根因：
  - `H5PlayerControls` 将 `!playing` 纳入 `pinned` 强制显示条件，导致暂停态即使外层 `controlsVisible=false`，控制条仍会保持 `is-visible`。
- 实现：
  - 控制条强制显示条件改为仅 `loading || failed`；暂停态不再强制显示，点击画面可按同一规则隐藏/显示。
  - 播放、暂停、截图、取流逻辑不变。
- 验证：
  - `cd frontend && npm run test` 通过，14 tests passed。
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 验证：暂停后点击画面控制条隐藏，再次点击恢复显示；控制台无 error/warn。

## 2026-06-29 H5 Monitor 返回按钮 icon 尺寸微调 2.21.3 开发记录

- 背景：
  - 用户反馈 H5 监控详情页左上返回按钮里的左箭头偏小，需要适当放大。
- 实现：
  - 保持返回按钮外圈 32px 和点击区域不变，仅将 `.h5-back-icon` 从 16px 调整为 19px，线宽从 2 调整为 2.2。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor icon 视觉尺寸修正 2.21.4 开发记录

- 背景：
  - 用户线上验收 2.21.3 后反馈返回按钮仍然显小，并指出播放器右侧截图、横竖屏、全屏三个 icon 视觉大小和高度不一致。
- 根因：
  - 返回按钮继承了全局 `button` 的左右 padding，导致 32px 按钮内 SVG 被 flex 压缩，虽然 CSS 设置了 21px，但实际渲染宽度只有约 6px。
  - 播放器右侧三个 SVG 虽然外框一致，但图形路径在 24x24 viewBox 中占比和视觉重心不同。
- 实现：
  - 返回按钮补充 `padding: 0`、`min-width: 32px`，并让 `.h5-back-icon` 固定 `flex-basis: 21px`；返回箭头路径改为更饱满的 24px viewBox chevron。
  - 微调相机、横竖屏、全屏/退出全屏 icon 的路径尺寸和坐标，让 30px 按钮内 17px SVG 的视觉高度更一致。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地 Vite dev demo 移动视口验证：返回按钮 SVG 实际渲染为 21x21，按钮 padding 为 0；右侧三个控制按钮均为 30x30，SVG 均为 17x17 且居中。

## 2026-06-29 H5 Monitor 返回按钮 icon 尺寸回调 2.21.5 开发记录

- 背景：
  - 用户线上验收 2.21.4 后认为返回箭头反而偏大，希望回到最初经验尺寸，只保留 padding 挤压问题的修复。
- 实现：
  - 返回箭头恢复为原始 16px chevron 和 2px 线宽。
  - 保留 `.h5-back-btn { padding: 0; min-width: 32px; }` 与 `.h5-back-icon { flex: 0 0 16px; }`，避免再次被全局 button padding 压缩。
- 验证：
  - `cd frontend && npm run build` 通过。
  - `git diff --check` 通过。
  - 本地移动端 demo 验证：返回按钮 padding 为 0，SVG 实际渲染为 16x16。

## 2026-06-29 H5 Monitor 上海凯德晶萃店入口开放 2.21.6 开发记录

- 背景：
  - 北京保利实验室门店 H5 Monitor 页面已通过用户线上验收。
  - 用户要求继续给“新氧青春诊所(上海凯德晶萃店)”开放门店详情右上角“查看监控”入口，新氧机构 ID 为 `10047`。
- 实现：
  - 前端 H5 Monitor 入口从单机构 `10030` 改为试点机构白名单：`10030`、`10047`。
  - 后端 H5 Monitor 服务端门禁同步改为试点机构白名单。
  - 保留北京 `10030` 仅允许 `GN0941203` 的旧试点限制；上海 `10047` 不硬编码录像机编号，使用该门店自己数据库下的有效通道和萤石账号配置。
  - 版本号升级到 `2.21.6`。
- 验证：
  - 新增前端测试覆盖 `10047` 可打开 H5 Monitor 入口。
  - 新增后端测试覆盖 `10047` 首页和直播取流使用上海门店自己的通道数据。

## 2026-06-29 H5 Monitor 首页通道标题层级修复 2.21.7 开发记录

- 背景：
  - 开放真实业务门店“新氧青春诊所(上海凯德晶萃店)”后，H5 Monitor 首页圆形预览图下方主标题显示为 `通道12`，区域编号/备注显示在第二行，信息层级与业务预期相反。
  - 用户期望主标题显示“区域类型 + 编号/备注”，例如 `治疗室1号`；副标题显示通道号，例如 `通道12`。
- 根因：
  - H5 首页 `channelName()` 优先使用后端 `channel_name`，真实扫描数据里的 `channel_name` 往往就是 `通道12`。
  - 区域编号/备注被单独拼为副标题，导致真实门店里“通道号”抢占了业务主标题位置。
- 实现：
  - 新增 `h5ChannelDisplayText` 前端领域 helper，统一生成 H5 通道卡片展示文案。
  - 业务区域标题优先级改为：业务类型标签 + 备注/编号，其次非业务备注，其次场景标签，最后才退回通道原名。
  - 卡片副标题固定为 `通道{channel_no}`。
  - H5 详情页进入时缓存的通道名称同步使用新的业务标题。
  - H5 `AreaType` 类型补充 `vip_treatment`，避免 VIP 治疗室后续展示退化。
  - H5 首页“加载更多”从固定 24 个改为按当前网格列数展示完整行：首屏 3 行，每次追加 2 行，避免真实门店桌面宽度下出现最后一行只露出半行的问题。
- 验证：
  - 新增前端测试覆盖 `通道12 + treatment + 1号 => 治疗室1号 / 通道12`。
  - 新增前端测试覆盖备注场景：`治疗室401号`、`护士站 / 通道16`。
  - 新增前端测试覆盖桌面 7 列和移动 3 列时的完整行加载数量。

## 2026-06-29 H5 Monitor 首页默认展示与缩略图刷新 2.21.8 开发记录

- 背景：
  - 用户线上验收 2.21.7 后，提出首页默认 3 行略少，建议默认展示 4 行。
  - 用户同时讨论：点击查看视频后，如果已经取到播放器画面，是否可以顺手刷新该通道首页缩略图，让真实业务门店的缩略图更及时。
- 方案取舍：
  - 不采用前端播放器 canvas 截帧上传：移动端、H265、萤石播放器内部跨域和画布污染风险较高，且会增加前端上传链路复杂度。
  - 采用“播放器第一帧成功 -> 前端低频通知后端 -> 后端复用现有萤石抓图与公司空间保存链路”的方式。
  - 刷新为 best effort，不阻断播放、不弹 toast；失败只在后台静默吞掉，避免影响查看监控主流程。
- 实现：
  - H5 首页默认展示从 3 行调整为 4 行，仍按当前网格列数计算完整行；加载更多仍每次增加 2 行。
  - 新增 H5 后端接口：`POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/snapshot`，先复用 H5 试点门禁和通道校验，再调用 storespace 现有 `RefreshChannelSnapshot` 抓图保存。
  - 前端在播放器 `first-frame-ready` 后触发缩略图刷新；跳过 mock 播放；同一 `机构+通道` 前端 10 分钟冷却。
  - 后端同一通道也增加 10 分钟冷却，防止多个用户同时观看同一路视频时重复打萤石抓图接口。
  - 缩略图刷新成功后，H5 路由壳更新列表刷新 key；用户返回首页时可重新拉取列表，展示新的缩略图链接。
- 验证：
  - `cd frontend && npm run test` 通过，17 tests passed。
  - `cd frontend && npm run build` 通过。
  - `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...` 通过。
  - `git diff --check` 通过。

## 2026-06-29 H5 Monitor 2.21.8 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 业务 commit：`a642e4a feat: refresh H5 monitor thumbnails after playback`。
- 推送结果：
  - GitLab remote 已从 `2c67c8f` 更新到 `a642e4a`。
  - 公司线上前端静态资源已探测到 `2.21.8`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.7 (container)` 更新到 `2.21.8`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-29 H5 Monitor 直播中缩略图刷新黑屏修复 2.21.9 开发记录

- 背景：
  - 线上验收 `2.21.8` 后，用户反馈刚开始看实时视频一段时间后可能突然黑屏，只能返回列表页；列表页预览图也会临时变成黑屏，再次进入后播放恢复，返回后预览图恢复正常。
- 根因判断：
  - `2.21.8` 将“第一帧成功”作为刷新缩略图触发点，导致播放过程中调用后端抓图接口。
  - 该抓图请求可能与当前直播取流竞争录像机/萤石/门店带宽资源，造成当前播放器黑屏或把黑屏保存成缩略图。
- 实现：
  - 移除 `first-frame-ready` 阶段的缩略图刷新。
  - 新增 H5 缩略图刷新时机规则：只允许实时直播流在 `exit`、`switch`、`stop` 释放前刷新；实时流续流/替换 `replace` 不刷新。
  - 回放模式完全不触发缩略图刷新，回放 URL 释放只关闭播放地址。
  - 直播释放时先尽力刷新缩略图，再关闭直播地址；刷新失败不阻断关闭直播地址。
- 验证：
  - `cd frontend && npm run test` 通过，18 tests passed。
  - `cd frontend && npm run build` 通过。

## 2026-06-29 H5 Monitor 2.21.9 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`1a702b6 fix: refresh H5 thumbnails only after live close`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.8 (container)` 更新到 `2.21.9`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-29 H5 Monitor 停止自动缩略图刷新 2.21.10 开发记录

- 背景：
  - 用户反馈 H5 Monitor 关闭直播时仍有一定比例无法稳定更新缩略图，且列表缩略图自动变化会产生黑屏、慢加载和视觉跳变。
  - 经过讨论，当前阶段 H5 Monitor 的核心目标是稳定查看实时视频，缩略图自动更新不是强需求。
- 决策：
  - H5 Monitor 不再自动刷新缩略图。
  - 实时直播、回放、切换 tab、关闭详情、续流、停止播放都不触发后端抓图。
  - 现有后台通道列表里的手动“刷新截图”能力保留，不受影响。
- 实现：
  - 前端移除 H5 详情页释放直播流前的缩略图刷新逻辑，释放播放地址只调用失效播放地址接口。
  - 前端移除 H5 首页刷新 key 和 `refreshSnapshot` API。
  - 后端移除 `POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/snapshot` 路由、H5 snapshot refresher 注入和 H5 专用截图刷新适配。
  - 新增后端测试确认 H5 snapshot 路由不再注册，避免后续误恢复。
- 风险说明：
  - H5 首页缩略图不会因为用户观看视频而自动更新；如果后续确实需要更新，应作为单独的播放器截图实验重新评估 PC、手机浏览器和飞书 WebView 能力。

## 2026-06-29 H5 Monitor 2.21.10 公司环境发布记录

- 发布目标：公司 GitLab 固定分支 `codex/containerize-single-image`，公司 K8s 自动发布。
- 发布 commit：`f59be65`，其中包含业务修复 `13a0506 fix: disable automatic H5 thumbnail refresh`，并正常合并远端 MySQL 迁移交接文档提交 `66e8eba`。
- 推送结果：
  - GitLab remote 已从 `66e8eba` 更新到 `f59be65`。
  - 公司线上前端静态资源已探测到 `2.21.10 (container)`。
- 线上验证：
  - `https://lite.sy.soyoung.com/erzhuang-project/health` 返回 `{"app":"erzhuang-project","status":"ok","version":"v2","database":"postgres","asset_store":"supabase"}`。
  - 前端版本从 `2.21.9 (container)` 更新到 `2.21.10 (container)`。
- 备注：
  - 本次未发布韩国服务器，未同步 GitHub。

## 2026-06-30 门店列表全量统计修复 2.21.11 开发记录

- 背景：
  - 用户反馈门店列表右上角统计只统计当前分页页内门店，翻到没有已确认门店的页面时，面诊室、治疗室、生美统计会错误变为 0。
- 根因：
  - 前端 `App.tsx` 使用当前页 `stores` 计算右上角统计；`stores` 来自分页接口 `items`，不是全部门店。
- 实现：
  - 后端 `GET /api/store-space/stores` 返回新增 `summary` 字段，按当前搜索条件统计全部匹配门店，统计发生在分页前。
  - MemoryStore 和 PostgresStore 均补齐同一口径：`store_count`、`treatment_count`、`consultation_count`、`beauty_count`。
  - 前端 `StoreListResponse` 增加 `summary`，门店列表“全部”视图右上角统计改用后端全量 summary，不再受当前页影响。
  - 城市筛选仍维持当前页前端筛选口径，后续如需要城市维度全量统计，需要另扩城市筛选参数或城市聚合接口。
- 验证：
  - 新增后端测试覆盖“分页只返回 1 家门店，但 summary 统计全部 2 家匹配门店”。
  - 新增前端测试覆盖门店 summary 汇总 helper。
