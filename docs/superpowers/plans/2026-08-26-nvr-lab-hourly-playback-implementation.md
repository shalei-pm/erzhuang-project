# NVR 实验页按小时定位回放 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 10001 NVR 实验页的录像模式中复用 2.x 日期时间选择交互，并以固定小时段快捷创建最长一小时的回放会话。

**Architecture:** 将当前内嵌在 `H5MonitorChannel` 的 2.x 日期时间选择器抽为无业务依赖组件，保持旧页 API 与视觉不变。NVR 页面只保存回放起点，通过纯函数生成最长一小时的起止 Unix 秒，再调用已有短期会话接口；后端同步把会话上限校验调整为一小时。

**Tech Stack:** Go `net/http`、React 19、TypeScript、Vitest、Vite、Chrome 插件测试环境验收。

---

## 文件结构

- 创建：`frontend/src/components/PlaybackDatePicker.tsx`，复用的 2.x 日期、日历、时分选择控件。
- 创建：`frontend/src/components/PlaybackDatePicker.test.tsx`，共享控件的静态渲染和可访问性断言。
- 修改：`frontend/src/pages/H5MonitorChannel.tsx`，改为使用共享控件，保留原有录像片段和 `playFromDateTime` 逻辑。
- 修改：`frontend/src/domain/nvr-lab.ts`，集中一小时窗口、小时段和“今天截断”纯计算。
- 修改：`frontend/src/domain/nvr-lab.test.ts`，覆盖一小时窗口和小时段计算。
- 修改：`frontend/src/pages/NVRLabCamera.tsx`，替换原生 `datetime-local` 表单并接入共享控件和小时段。
- 修改：`frontend/src/styles.css`，为 NVR 小时段与回放范围增加 2.x 风格的紧凑布局和移动端规则。
- 修改：`internal/nvrlab/service.go`，把 `maxPlaybackWindow` 调整为一小时。
- 修改：`internal/nvrlab/service_test.go`，把超时边界改为一小时并覆盖恰好一小时成功。
- 修改：`VERSION`、`docs/codex-learning-state.md`、`docs/decisions.md`、`work/current-plan.md`，记录实现和测试发布状态。

### Task 1: 抽取可复用的 2.x 日期时间选择器

**Files:**
- Create: `frontend/src/components/PlaybackDatePicker.tsx`
- Create: `frontend/src/components/PlaybackDatePicker.test.tsx`
- Modify: `frontend/src/pages/H5MonitorChannel.tsx:1-24,1119-1406`

- [ ] **Step 1: 为共享控件写静态渲染测试**

```tsx
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PlaybackDatePicker } from "./PlaybackDatePicker";

describe("PlaybackDatePicker", () => {
  it("renders the 2.x quick dates, replay trigger and locate action", () => {
    const markup = renderToStaticMarkup(
      createElement(PlaybackDatePicker, {
        value: "2026-08-26T10:18",
        onChange: () => undefined,
        onConfirm: () => undefined,
      }),
    );

    expect(markup).toContain("今天");
    expect(markup).toContain("昨天");
    expect(markup).toContain("前天");
    expect(markup).toContain("回放时间");
    expect(markup).toContain("定位回放");
  });
});
```

- [ ] **Step 2: 运行测试，确认它因组件不存在而失败**

Run: `cd frontend && npm test -- --run src/components/PlaybackDatePicker.test.tsx`

Expected: FAIL，提示无法解析 `./PlaybackDatePicker`。

- [ ] **Step 3: 创建共享组件并保留 2.x 行为**

从 `H5MonitorChannel.tsx` 移动以下组件和纯辅助函数到新文件：`PlaybackDatePicker`、`TimeColumn`、`initialDateTimeValue`、`startOfToday`、`addDays`、`addMonths`、`startOfMonth`、`formatDateInput`、`formatTimeInput`、`timePart`、`dateFromInput`、`formatDateTimeValue`、`formatDateTimeLabel`、`monthCalendarDays`、`range`、`pad2` 及其 `DateTimeParts`/`QuickDateKey` 类型。

组件必须导出：

```ts
export type PlaybackDatePickerProps = {
  value: string;
  onChange: (dateTime: string) => void;
  onConfirm: (dateTime: string) => void;
};

export function PlaybackDatePicker(props: PlaybackDatePickerProps): JSX.Element;
export function initialPlaybackDateTimeValue(now?: Date): string;
```

`initialPlaybackDateTimeValue` 在未传入 `now` 时使用当前时间；传入时间时返回 `YYYY-MM-DDTHH:mm`，供 NVR 页面和纯测试稳定复用。

在 `H5MonitorChannel.tsx` 顶部引入：

```ts
import { PlaybackDatePicker, initialPlaybackDateTimeValue } from "../components/PlaybackDatePicker";
```

把原状态初始化改为：

```ts
const [selectedDateTime, setSelectedDateTime] = useState(() => initialPlaybackDateTimeValue());
```

删除被移动的内嵌组件和辅助函数，不改变 `handleDateTimeChange`、`playFromDateTime`、录像片段查询与旧页 CSS class。

- [ ] **Step 4: 运行共享组件和现有前端测试**

Run: `cd frontend && npm test -- --run`

Expected: PASS；旧 H5 监控测试和新共享组件测试都通过。

- [ ] **Step 5: 提交抽取结果**

```bash
git add frontend/src/components/PlaybackDatePicker.tsx frontend/src/components/PlaybackDatePicker.test.tsx frontend/src/pages/H5MonitorChannel.tsx
git commit -m "refactor: share playback date picker"
```

### Task 2: 建立 NVR 一小时时间范围纯模型

**Files:**
- Modify: `frontend/src/domain/nvr-lab.ts:1-59`
- Modify: `frontend/src/domain/nvr-lab.test.ts:1-18`

- [ ] **Step 1: 为窗口限制和小时段写失败测试**

在 `nvr-lab.test.ts` 中添加：

```ts
import { buildNVRLabHourlyPlayback, validateNVRLabPlayback } from "./nvr-lab";

it("allows an exact one-hour playback window and rejects a longer range", () => {
  expect(validateNVRLabPlayback(100, 3700)).toBe("");
  expect(validateNVRLabPlayback(100, 3701)).toBe("单次回放最长支持 1 小时");
});

it("builds an hourly historical range", () => {
  const expectedStart = Math.floor(new Date("2026-08-20T10:00:00").getTime() / 1000);
  const expectedEnd = Math.floor(new Date("2026-08-20T11:00:00").getTime() / 1000);
  expect(buildNVRLabHourlyPlayback("2026-08-20", 10, new Date("2026-08-26T10:18:00"))).toEqual({
    startAt: "2026-08-20T10:00",
    endAt: "2026-08-20T11:00",
    startTime: expectedStart,
    endTime: expectedEnd,
  });
});

it("clips today's current hour and rejects a future hour", () => {
  expect(buildNVRLabHourlyPlayback("2026-08-26", 10, new Date("2026-08-26T10:18:00"))).toMatchObject({
    startAt: "2026-08-26T10:00",
    endAt: "2026-08-26T10:18",
  });
  expect(buildNVRLabHourlyPlayback("2026-08-26", 11, new Date("2026-08-26T10:18:00"))).toBeNull();
});
```

根据本机时区计算 Unix 秒时，测试不得硬编码 Unix 常量；应使用 `new Date("2026-08-20T10:00:00").getTime() / 1000` 作为期望值，以避免 CI 时区差异。

- [ ] **Step 2: 运行测试，确认一小时边界先失败**

Run: `cd frontend && npm test -- --run src/domain/nvr-lab.test.ts`

Expected: FAIL，旧实现仍返回“最长支持 30 分钟”，且 `buildNVRLabHourlyPlayback` 未导出。

- [ ] **Step 3: 实现单一时间范围来源**

在 `nvr-lab.ts` 定义：

```ts
export const NVR_LAB_MAX_PLAYBACK_SECONDS = 60 * 60;

export type NVRLabPlaybackRange = {
  startAt: string;
  endAt: string;
  startTime: number;
  endTime: number;
};

export function buildNVRLabHourlyPlayback(date: string, hour: number, now = new Date()): NVRLabPlaybackRange | null;
export function buildNVRLabPlaybackFromStart(startAt: string, now = new Date()): NVRLabPlaybackRange | null;
```

两个构造函数都必须：秒归零、结束不超过 `start + NVR_LAB_MAX_PLAYBACK_SECONDS`、当天结束不超过 `now`、未来起点返回 `null`。`validateNVRLabPlayback` 使用常量并把中文上限文案改为“单次回放最长支持 1 小时”。

- [ ] **Step 4: 运行领域测试**

Run: `cd frontend && npm test -- --run src/domain/nvr-lab.test.ts`

Expected: PASS。

- [ ] **Step 5: 提交领域模型**

```bash
git add frontend/src/domain/nvr-lab.ts frontend/src/domain/nvr-lab.test.ts
git commit -m "feat: support hourly nvr playback windows"
```

### Task 3: 同步后端一小时会话边界

**Files:**
- Modify: `internal/nvrlab/service.go:13,82-97`
- Modify: `internal/nvrlab/service_test.go:88-101`

- [ ] **Step 1: 修改 Go 测试为一小时边界**

把测试替换为：

```go
func TestCreateSessionAllowsPlaybackForExactlyOneHour(t *testing.T) {
    client := &fakeAuthorizationClient{url: "wss://example.test/session"}
    service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, client)

    _, err := service.CreateSession(context.Background(), ExperimentTenantID, 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 3700})
    if err != nil {
        t.Fatalf("CreateSession() error = %v", err)
    }
}

func TestCreateSessionRejectsPlaybackLongerThanOneHour(t *testing.T) {
    service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{ExperimentTenantID: sampleRecords()}}, &fakeAuthorizationClient{})

    _, err := service.CreateSession(context.Background(), ExperimentTenantID, 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 3701})
    if !errors.Is(err, ErrInvalidPlaybackWindow) {
        t.Fatalf("CreateSession() error = %v, want ErrInvalidPlaybackWindow", err)
    }
}
```

- [ ] **Step 2: 运行目标 Go 测试，确认旧限制导致失败**

Run: `go test ./internal/nvrlab -run 'TestCreateSession(AllowsPlaybackForExactlyOneHour|RejectsPlaybackLongerThanOneHour)' -count=1`

Expected: 第一项 FAIL，现有 30 分钟校验拒绝 3600 秒窗口。

- [ ] **Step 3: 最小改动服务限制**

```go
const maxPlaybackWindow = time.Hour
```

保持 `validateStreamSessionRequest`、鉴权调用、权限检查和错误码不变。

- [ ] **Step 4: 运行 Go 单包测试与格式化**

Run: `gofmt -w internal/nvrlab/service.go internal/nvrlab/service_test.go && go test ./internal/nvrlab -count=1`

Expected: PASS。

- [ ] **Step 5: 提交后端边界**

```bash
git add internal/nvrlab/service.go internal/nvrlab/service_test.go
git commit -m "feat: allow one-hour nvr playback sessions"
```

### Task 4: 将 NVR 录像页接入日期控件和小时段

**Files:**
- Modify: `frontend/src/pages/NVRLabCamera.tsx:1-136`
- Modify: `frontend/src/styles.css:3549-3581,4780-4815`

- [ ] **Step 1: 为 NVR 页面添加静态渲染测试**

创建或扩展 `frontend/src/pages/NVRLabCamera.test.tsx`，以 `renderToStaticMarkup` 断言录像模式 UI 的纯展示组件至少包含：`回放时间`、`定位回放`、`00:00 - 01:00`、`23:00 - 次日 00:00`、`回放范围`。将小时段视图提取为 `NVRLabHourlyPlaybackPicker` 并使其只接收 `startAt`、`range`、`onStartAtChange`、`onConfirm`，避免网络和路由依赖进入该测试。

- [ ] **Step 2: 运行测试，确认新组件尚不存在**

Run: `cd frontend && npm test -- --run src/pages/NVRLabCamera.test.tsx`

Expected: FAIL，提示 `NVRLabHourlyPlaybackPicker` 未导出或未渲染。

- [ ] **Step 3: 替换原生输入表单**

在 `NVRLabCamera.tsx`：

1. 用 `initialPlaybackDateTimeValue()` 初始化一个 `playbackStartAt` 状态；保留 URL `start_at` 预填兼容逻辑。
2. 通过 `buildNVRLabPlaybackFromStart(playbackStartAt)` 派生 `NVRLabPlaybackRange | null`，不要维护第二份可编辑结束时间状态。
3. 录像 Tab 内渲染共享 `PlaybackDatePicker`，`onChange` 仅更新 `playbackStartAt`，`onConfirm` 调用统一 `playPlayback(range)`。
4. 渲染 24 个按钮；按钮标签由小时段生成，23 时结束显示“次日 00:00”。点击按钮后调用 `buildNVRLabHourlyPlayback(selectedDate, hour)` 并更新 `playbackStartAt`；返回 `null` 时显示“请选择当前时刻之前的回放时间”。
5. 在定位按钮附近显示 `回放范围：YYYY/MM/DD HH:mm - YYYY/MM/DD HH:mm`；只有有效 range 才允许定位。
6. `onRetry` 在录像模式必须使用当前派生 range，直播模式仍不传起止时间。

删除 `startAt`、`endAt`、`localDateTimeToUnix` 和两个 `datetime-local` 控件。不得自动创建回放会话、不得改变直播自动建连行为。

在 CSS 中使用既有 `.h5-date-picker`、`.h5-date-quick-row`、`.h5-date-time-trigger` 风格；新增：

```css
.nvr-lab-hourly-picker { display: grid; gap: 10px; }
.nvr-lab-hourly-range { color: var(--text-muted); font-size: 13px; }
.nvr-lab-hour-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.nvr-lab-hour-grid button.is-selected { border-color: #bfdbfe; background: #eff6ff; color: var(--brand); }
```

在窄屏媒体查询中改为两列，日期选择弹层复用现有 `h5-date-*` 窄屏规则，确保主按钮不被弹层遮挡。

- [ ] **Step 4: 运行 NVR 页面和全量前端测试**

Run: `cd frontend && npm test -- --run`

Expected: PASS；现有 NVR URL 脱敏、播放器控制和路由测试全部保留。

- [ ] **Step 5: 生产构建并提交页面改动**

Run: `cd frontend && npm run build`

Expected: PASS；允许保留既有大 chunk warning，不新增 TypeScript 或 Vite error。

```bash
git add frontend/src/pages/NVRLabCamera.tsx frontend/src/pages/NVRLabCamera.test.tsx frontend/src/styles.css
git commit -m "feat: add hourly nvr playback locator"
```

### Task 5: 版本、发布与 Chrome 插件验收

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: 更新版本和项目记忆**

将 `VERSION` 从 `3.1.7` 升至 `3.1.8`。记录本次只影响 10001 NVR 实验页、最长窗口为一小时、旧萤石页不变，以及本机 Go 是否可执行的实际结果。

- [ ] **Step 2: 执行完整验证**

Run:

```bash
go test ./...
go build ./cmd/server
cd frontend && npm test -- --run
cd frontend && npm run build
git diff --check
```

Expected: 全部 PASS。若本机没有 Go 工具链，明确记录该缺口，由 Wharf 构建日志补验，不伪报通过。

- [ ] **Step 3: 发布公司测试环境**

```bash
git add VERSION docs/codex-learning-state.md docs/decisions.md work/current-plan.md
git commit -m "docs: record hourly nvr playback release"
git push gitlab codex/containerize-single-image
git push origin codex/containerize-single-image
```

使用项目 runbook 的临时 `GIT_ASKPASS` 流程推 GitLab，推送后删除临时脚本。等待 Wharf pipeline `752` 自动部署；不手工重复部署。

- [ ] **Step 4: 用 Chrome 插件实际验收测试页**

在已登录 Chrome 中打开：

```text
https://lite.sy.soyoung.com/erzhuang-project/h5/nvr-lab/10001/cameras/111
```

逐项检查：

1. 页面底部为 `3.1.8 (container)`。
2. 切换“录像”，确认没有原生 `datetime-local`，有 2.x 风格的快捷日期和回放时间弹层。
3. 选择 2026-08-25、点击 11:00-12:00，确认范围为 `2026/08/25 11:00 - 2026/08/25 12:00`；再以已知有效时间将起点精调为 `11:03` 并点击“定位回放”。
4. 检查页面网络请求的 `start_time`、`end_time` 差值不大于 3600，且 WSS 地址、token 与 Authorization 未出现在页面、控制台或可见错误文案中。
5. 等待首帧，确认播放器状态为“画面已开始播放”。
6. 选择今天未来小时，确认不会创建会话并显示中文提示。
7. 检查桌面与窄屏下 24 个小时段、日期弹层和“定位回放”均不重叠、不溢出。

- [ ] **Step 5: 记录实际验收并提交**

在 `docs/codex-learning-state.md` 写入 commit、Wharf 部署、页面版本、Chrome 插件验收结果和任何未完成的移动端风险，然后提交：

```bash
git add docs/codex-learning-state.md work/current-plan.md
git commit -m "docs: record nvr hourly playback verification"
git push gitlab codex/containerize-single-image
git push origin codex/containerize-single-image
```

如果 Chrome 实测发生失败，先记录失败阶段、脱敏现象和回滚 commit；不得把未验收版本发布到正式环境。
