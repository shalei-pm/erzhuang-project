# H5 Player Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add production-ready single-camera H5 player controls: play/pause, sound, fullscreen/landscape, screenshot, playback segment seeking, and long-play session protection.

**Architecture:** Keep `H5FlvPlayer` responsible for the underlying `ezuikit-flv` instance and expose a small imperative handle to the page. Put visual controls in focused components that receive state and callbacks, and keep H5 business API orchestration in `H5MonitorChannel`. Playback segment seeking should request a new playback URL instead of relying on native FLV seek.

**Tech Stack:** React 19, TypeScript, Vite, `ezuikit-flv@2.1.1`, existing Go H5 monitor APIs.

---

## File Structure

- Modify: `frontend/src/components/H5FlvPlayer.tsx`
  - Convert to `forwardRef`.
  - Export `H5PlayerHandle`, `H5PlaybackState`, and `H5PlayerScreenshot`.
  - Remove the old centered sound button.
  - Report playback state changes to the parent.
- Create: `frontend/src/components/H5PlayerControls.tsx`
  - Render the bottom overlay controls.
  - Own only UI reveal/hide behavior and button rendering.
  - Receive state and callbacks from `H5MonitorChannel`.
- Create: `frontend/src/components/PlaybackSegmentSlider.tsx`
  - Render selected segment start/end/current time and a range input.
  - Submit only on pointer release / keyboard commit.
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`
  - Hold `playerRef`, unified UI state, long-play guard state, and current playback seek time.
  - Connect controls to `H5FlvPlayer`.
  - Release old URL before replacing it with a new live/playback URL.
- Modify: `frontend/src/styles.css`
  - Add H5 player control, landscape, screenshot feedback, slider, and long-play modal styles.
  - Remove old `.h5-sound-toggle` visual dependency.
- Modify: `frontend/src/api.test.ts`
  - Add unit tests for pure time helpers exported from `PlaybackSegmentSlider` or a small domain helper.
- Optional Create: `frontend/src/domain/h5-playback.ts`
  - Put testable pure helpers here if `PlaybackSegmentSlider` would otherwise export too much UI logic.
- Modify: `docs/codex-learning-state.md`
  - Record implementation and verification results after coding.

## Collaboration Model and Non-Regression Guardrails

### Roles

- Product owner: 验收页面交互、控件位置、移动端体验、回放滑块是否符合使用习惯。
- Architecture owner: 四喜负责拆解任务、守住技术边界、代码 review、回归验证、版本和发布判断。
- H5 implementer: 按本计划逐 task 实现，不扩展产品范围，不改未授权链路。
- Spec reviewer: 每个 task 完成后检查是否满足计划，不允许少做，也不允许把明确排除项做进去。
- Code reviewer: 每个 task 完成后检查代码质量、状态管理、资源释放、浏览器兼容、移动端风险。

### Baseline That Must Not Be Broken

These are already verified decisions. Implementation must preserve them:

- Live protocol remains `flv` for the current trial.
- Mobile playback continues to use the working soft decode path:
  - `mobile-wasm`
  - `useMSE: false`
  - `useWCS: false`
  - `forceNoOffscreen: true`
  - `autoWasm: true`
  - `wasmDecodeErrorReplay: true`
  - `wasmDecodeAudioSyncVideo: true`
  - `keepScreenOn: true`
- Desktop playback keeps the existing MSE path:
  - `desktop-mse`
  - `useMSE: true`
  - `useWCS: true`
- Desktop first-frame readiness may accept `streamSuccess / videoInfo / loaded / playing`.
- Mobile first-frame readiness only treats real render events as success:
  - `videoFrame`
  - `firstFrameDisplay`
  - `playToRenderTimes`
- Do not treat mobile `streamSuccess` alone as successful video display.
- The diagnostic status card stays visible during the debugging stage.
- Do not add refresh-stream, automatic abnormal-stream retry, multi-screen, PTZ, playback speed, download, or complex timeline controls.
- Do not change H5 monitor entry gating. The trial entrance remains scoped to the configured trial store unless a later task explicitly changes it.
- Do not change backend Ezviz token, URL creation, URL disable, or quota logic unless the task explicitly says so.

### Coordination Rules

- Work task-by-task in the order of this plan. Do not batch several UI and player lifecycle changes into one unreviewable diff.
- Each task must leave the project buildable before moving to the next task.
- The implementer must report:
  - files changed
  - what behavior changed
  - which verification command passed
  - any uncertainty around browser/player API support
- After each implementation task, architecture owner performs two reviews:
  - spec compliance review
  - code quality and regression review
- If a task needs to alter the player decode options or first-frame detection rules, stop and escalate before coding.
- If screenshot API behavior is uncertain, fail gracefully with toast. Do not change playback strategy to make screenshots work.
- If fullscreen or orientation APIs are unsupported in a browser, show a controlled message. Do not introduce blocking prompts or hard failures.

### Required Regression Checklist

Run this checklist before release:

- Desktop live view still shows video and does not regress to the old top black bar issue.
- Mobile browser / Feishu / WeChat still shows video instead of a silent black screen.
- PC first-frame timeout does not appear when the picture is already visible.
- Mobile black screen must show diagnostic details if it happens again.
- Live URL is disabled when leaving detail, switching to playback, or stopping after long-play prompt.
- Playback URL is disabled when switching to live, changing segment time, or stopping after long-play prompt.
- Diagnostic card remains visible below the player.
- Control bar does not cover the whole picture and is usable on 390px mobile width.
- Playback date picker remains aligned after slider is added.
- Playback segment slider only requests a new URL on commit, not continuously while dragging.

### Visual Style Gate for Player Controls

The control layer is part of the existing H5 business tool, not a standalone media product. Review must reject changes that feel visually disconnected from the current page.

Required:

- Use existing tokens from `frontend/src/styles.css`: `--radius-card`, `--radius-control`, `--button-radius`, `--brand`, `--text-main`, `--text-muted`, `--border-subtle`, `--surface`.
- Keep desktop controls compact: small text, stable button height, no large pill group that dominates the picture.
- Keep mobile controls touchable but restrained: no overlapping labels, no horizontal overflow outside the player, no full-width opaque bottom slab.
- Use neutral dark translucent overlay only because it sits on video; opacity should be low enough to preserve picture context.
- Buttons should look like the project's existing light enterprise controls adapted for dark video, not like a consumer video app skin.
- Loading, failed, and paused states may keep controls visible; normal playing state may fade controls after a short delay.

Reject:

- Decorative gradients, glow, bokeh, dramatic shadows, or purple/blue tech styling.
- Oversized rounded rectangles that cover meaningful parts of the video.
- New icon system or SVG artwork introduced only for this task.
- Mismatched colors that do not appear elsewhere in the H5 page.
- Text wrapping inside control buttons at 390px mobile width.

## Task 1: Player Handle and State Contract

**Files:**
- Modify: `frontend/src/components/H5FlvPlayer.tsx`

- [ ] **Step 1: Add exported player types near existing `H5PlayerStatus`**

```ts
export type H5PlaybackState = {
  playing: boolean;
  muted: boolean;
  loading: boolean;
  failed: boolean;
};

export type H5PlayerScreenshot = {
  dataUrl?: string;
};

export type H5PlayerHandle = {
  play: () => Promise<void>;
  pause: () => Promise<void>;
  openSound: () => Promise<void>;
  closeSound: () => Promise<void>;
  screenshot: () => Promise<H5PlayerScreenshot>;
  enterFullscreen: () => Promise<void>;
  exitFullscreen: () => Promise<void>;
};
```

- [ ] **Step 2: Expand the local player interface**

Add optional methods so wrapper code can call player capabilities when available:

```ts
interface EzuikitFlvPlayer {
  destroy: () => void;
  closeSound: () => void;
  openSound: () => void;
  play: () => unknown;
  pause: () => unknown;
  getState?: () => unknown;
  screenshot?: () => unknown;
  capturePicture?: () => unknown;
}
```

- [ ] **Step 3: Update props to report playback state**

```ts
export interface H5FlvPlayerProps {
  url: string;
  protocol?: string;
  isLive: boolean;
  onError?: (message: string) => void;
  onStatus?: (status: H5PlayerStatus) => void;
  onPlaybackStateChange?: (state: H5PlaybackState) => void;
}
```

- [ ] **Step 4: Convert the component to `forwardRef`**

Change the import line:

```ts
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type MutableRefObject,
  type SetStateAction,
} from "react";
```

Wrap the component:

```ts
export const H5FlvPlayer = forwardRef<H5PlayerHandle, H5FlvPlayerProps>(function H5FlvPlayer(
  { url, protocol, isLive, onError, onStatus, onPlaybackStateChange },
  ref,
) {
  // existing body
});
```

- [ ] **Step 5: Add state reporting effect**

Place this after `useState` declarations:

```ts
useEffect(() => {
  onPlaybackStateChange?.({ playing: !loading && !loadFailed, muted, loading, failed: loadFailed });
}, [loading, loadFailed, muted, onPlaybackStateChange]);
```

- [ ] **Step 6: Add a Promise adapter and handle methods**

Add inside the component before `return`:

```ts
function callPlayer(action: "play" | "pause" | "openSound" | "closeSound") {
  const player = playerRef.current;
  if (!player) return Promise.resolve();
  try {
    return Promise.resolve(player[action]());
  } catch (err) {
    return Promise.reject(err);
  }
}

useImperativeHandle(ref, () => ({
  async play() {
    await callPlayer("play");
    setLoading(false);
  },
  async pause() {
    await callPlayer("pause");
  },
  async openSound() {
    await callPlayer("openSound");
    setMuted(false);
  },
  async closeSound() {
    await callPlayer("closeSound");
    setMuted(true);
  },
  async screenshot() {
    const player = playerRef.current;
    const candidate = player?.screenshot ?? player?.capturePicture;
    if (typeof candidate !== "function") {
      throw new Error("screenshot is not supported by current player");
    }
    const result = await Promise.resolve(candidate.call(player));
    return normalizeScreenshotResult(result);
  },
  async enterFullscreen() {
    await requestElementFullscreen(wrapperRef.current);
  },
  async exitFullscreen() {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
    }
  },
}), []);
```

Also add `const wrapperRef = useRef<HTMLDivElement | null>(null);` next to `playerRef`.

- [ ] **Step 7: Add fullscreen and screenshot helpers below the component**

```ts
async function requestElementFullscreen(element: HTMLElement | null) {
  if (!element || typeof element.requestFullscreen !== "function") {
    throw new Error("fullscreen is not supported by current browser");
  }
  await element.requestFullscreen();
}

function normalizeScreenshotResult(result: unknown): H5PlayerScreenshot {
  if (typeof result === "string") {
    return { dataUrl: result };
  }
  if (result && typeof result === "object" && "dataUrl" in result) {
    return { dataUrl: String((result as { dataUrl?: unknown }).dataUrl || "") };
  }
  return {};
}
```

- [ ] **Step 8: Remove the old sound button from JSX**

Delete the old `toggleMute()` function and this block:

```tsx
{!loading && !loadFailed && (
  <button className={`h5-sound-toggle ${muted ? "muted" : "unmuted"}`} ...>
    ...
  </button>
)}
```

Set the wrapper ref:

```tsx
<div className="h5-player-wrapper" ref={wrapperRef}>
```

- [ ] **Step 9: Run TypeScript build**

Run:

```bash
cd frontend && npm run build
```

Expected: TypeScript compiles. If the player method binding causes type errors, keep `candidate.call(player)` and avoid unbound method calls.

## Task 2: H5 Player Controls Component

**Files:**
- Create: `frontend/src/components/H5PlayerControls.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Create the component**

```tsx
import { useEffect, useRef, useState } from "react";

export type H5PlayerControlState = {
  playing: boolean;
  muted: boolean;
  loading: boolean;
  failed: boolean;
  fullscreen: boolean;
  landscape: boolean;
};

export type H5PlayerControlsProps = {
  state: H5PlayerControlState;
  onTogglePlay: () => void;
  onToggleSound: () => void;
  onScreenshot: () => void;
  onToggleLandscape: () => void;
  onToggleFullscreen: () => void;
};

export function H5PlayerControls({
  state,
  onTogglePlay,
  onToggleSound,
  onScreenshot,
  onToggleLandscape,
  onToggleFullscreen,
}: H5PlayerControlsProps) {
  const [visible, setVisible] = useState(true);
  const timerRef = useRef<number | null>(null);
  const pinned = state.loading || state.failed || !state.playing;

  function reveal() {
    setVisible(true);
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    if (!pinned) {
      timerRef.current = window.setTimeout(() => setVisible(false), 3600);
    }
  }

  useEffect(() => {
    reveal();
    return () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    };
  }, [state.loading, state.failed, state.playing, state.muted, state.fullscreen, state.landscape]);

  return (
    <div
      className={`h5-player-controls ${visible || pinned ? "is-visible" : ""}`}
      onPointerDown={reveal}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={reveal}
    >
      <button type="button" onClick={onTogglePlay} disabled={state.loading || state.failed} aria-label={state.playing ? "暂停" : "播放"}>
        {state.playing ? "暂停" : "播放"}
      </button>
      <button type="button" onClick={onToggleSound} disabled={state.loading || state.failed} aria-label={state.muted ? "开启声音" : "关闭声音"}>
        {state.muted ? "开声音" : "静音"}
      </button>
      <button type="button" onClick={onScreenshot} disabled={state.loading || state.failed} aria-label="截图">
        截图
      </button>
      <button type="button" onClick={onToggleLandscape} aria-label={state.landscape ? "退出横屏" : "横屏查看"}>
        {state.landscape ? "竖屏" : "横屏"}
      </button>
      <button type="button" onClick={onToggleFullscreen} aria-label={state.fullscreen ? "退出全屏" : "全屏"}>
        {state.fullscreen ? "退出全屏" : "全屏"}
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Add styles**

Append near existing H5 player styles in `frontend/src/styles.css`:

```css
.h5-player-controls {
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: 10px;
  z-index: 7;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: var(--radius-card);
  background: rgba(15, 23, 42, 0.72);
  opacity: 0;
  transform: translateY(8px);
  transition: opacity 0.18s, transform 0.18s;
  pointer-events: none;
}

.h5-player-controls.is-visible {
  opacity: 1;
  transform: translateY(0);
  pointer-events: auto;
}

.h5-player-controls button {
  min-height: 34px;
  min-width: 56px;
  padding: 0 10px;
  border-color: rgba(255, 255, 255, 0.24);
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  font-size: 13px;
}

.h5-player-controls button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Mobile sizing**

Add within existing mobile media area or append:

```css
@media (max-width: 720px) {
  .h5-player-controls {
    left: 8px;
    right: 8px;
    bottom: 8px;
    gap: 6px;
    overflow-x: auto;
    justify-content: flex-start;
    scrollbar-width: none;
  }

  .h5-player-controls::-webkit-scrollbar {
    display: none;
  }

  .h5-player-controls button {
    flex: 0 0 auto;
    min-height: 40px;
    min-width: 66px;
    font-size: 13px;
  }
}
```

- [ ] **Step 4: Run build**

Run:

```bash
cd frontend && npm run build
```

Expected: component compiles and no unused imports remain.

## Task 3: Wire Controls Into H5 Monitor Channel

**Files:**
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Update imports**

```ts
import { H5FlvPlayer, type H5PlayerHandle, type H5PlayerStatus, type H5PlaybackState } from "../components/H5FlvPlayer";
import { H5PlayerControls, type H5PlayerControlState } from "../components/H5PlayerControls";
```

- [ ] **Step 2: Add UI state**

Inside `H5MonitorChannel`:

```ts
const playerRef = useRef<H5PlayerHandle | null>(null);
const [playbackState, setPlaybackState] = useState<H5PlaybackState>({
  playing: false,
  muted: true,
  loading: false,
  failed: false,
});
const [fullscreen, setFullscreen] = useState(false);
const [landscape, setLandscape] = useState(false);
const [screenshotNotice, setScreenshotNotice] = useState("");
```

Then derive:

```ts
const controlState: H5PlayerControlState = {
  ...playbackState,
  loading: loading || playbackState.loading,
  fullscreen,
  landscape,
};
```

- [ ] **Step 3: Add control handlers**

```ts
async function handleTogglePlay() {
  try {
    if (playbackState.playing) {
      await playerRef.current?.pause();
      setPlaybackState((current) => ({ ...current, playing: false }));
      return;
    }
    await playerRef.current?.play();
    setPlaybackState((current) => ({ ...current, playing: true }));
  } catch (err) {
    setToast(`播放控制失败 · ${errMessage(err, "播放器返回异常")}`);
  }
}

async function handleToggleSound() {
  try {
    if (playbackState.muted) {
      await playerRef.current?.openSound();
    } else {
      await playerRef.current?.closeSound();
    }
  } catch (err) {
    setToast(`当前浏览器暂无法切换声音 · ${errMessage(err, "播放器返回异常")}`);
  }
}

async function handleScreenshot() {
  try {
    const shot = await playerRef.current?.screenshot();
    setScreenshotNotice(shot?.dataUrl ? "截图已生成" : "截图已触发");
    window.setTimeout(() => setScreenshotNotice(""), 2600);
  } catch (err) {
    setToast(`当前浏览器暂不支持截图 · ${errMessage(err, "播放器返回异常")}`);
  }
}

function handleToggleLandscape() {
  setLandscape((current) => !current);
}

async function handleToggleFullscreen() {
  try {
    if (fullscreen) {
      await playerRef.current?.exitFullscreen();
      setFullscreen(false);
      return;
    }
    await playerRef.current?.enterFullscreen();
    setFullscreen(true);
  } catch (err) {
    setToast(`当前浏览器暂不支持全屏 · ${errMessage(err, "播放器返回异常")}`);
  }
}
```

- [ ] **Step 4: Track browser fullscreen exit**

```ts
useEffect(() => {
  function handleFullscreenChange() {
    setFullscreen(Boolean(document.fullscreenElement));
  }
  document.addEventListener("fullscreenchange", handleFullscreenChange);
  return () => document.removeEventListener("fullscreenchange", handleFullscreenChange);
}, []);
```

- [ ] **Step 5: Render player with ref and controls**

Replace the current `H5FlvPlayer` render:

```tsx
<div className={`h5-player-shell ${landscape ? "is-landscape" : ""}`}>
  <H5FlvPlayer
    ref={playerRef}
    url={currentPlayerUrl}
    protocol={currentPlayer?.protocol}
    isLive={mode === "live"}
    onError={handlePlayerError}
    onStatus={handlePlayerStatus}
    onPlaybackStateChange={setPlaybackState}
  />
  <H5PlayerControls
    state={controlState}
    onTogglePlay={handleTogglePlay}
    onToggleSound={handleToggleSound}
    onScreenshot={handleScreenshot}
    onToggleLandscape={handleToggleLandscape}
    onToggleFullscreen={handleToggleFullscreen}
  />
  {screenshotNotice && <div className="h5-screenshot-notice">{screenshotNotice}</div>}
</div>
```

- [ ] **Step 6: Add shell styles**

```css
.h5-player-shell {
  position: relative;
  width: 100%;
}

.h5-player-shell.is-landscape {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
  background: #000;
}

.h5-player-shell.is-landscape .h5-player-wrapper {
  width: min(100vw, calc(100vh * 16 / 9));
  max-height: 100vh;
}

.h5-screenshot-notice {
  position: absolute;
  right: 12px;
  top: 12px;
  z-index: 8;
  padding: 6px 10px;
  border-radius: var(--button-radius);
  background: rgba(15, 23, 42, 0.78);
  color: #fff;
  font-size: 12px;
}
```

- [ ] **Step 7: Run build**

Run:

```bash
cd frontend && npm run build
```

Expected: `H5MonitorChannel` compiles and the page still renders in demo mode.

## Task 4: Release Old URLs Before Replacement

**Files:**
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`

- [ ] **Step 1: Add a release helper**

Inside `H5MonitorChannel`:

```ts
function releaseUrl(urlId: string | undefined) {
  if (!urlId) return Promise.resolve();
  return h5Api.disableUrl(externalOrgId, channelId, urlId, userId.current).catch(() => {});
}
```

- [ ] **Step 2: Use it before fetching a new live URL**

In live URL fetch, before `setLiveUrl(resp)`, release any old live URL:

```ts
.then(async (resp) => {
  await releaseUrl(liveUrl?.url_id);
  setLiveUrl(resp);
  ...
})
```

- [ ] **Step 3: Use it in `playRange` before replacing playback URL**

```ts
.then(async (resp) => {
  await releaseUrl(playbackUrl?.url_id);
  setPlaybackUrl(resp);
  ...
})
```

- [ ] **Step 4: Use it in `switchMode`**

Replace direct `h5Api.disableUrl(...)` with:

```ts
void releaseUrl(activeUrlId);
```

- [ ] **Step 5: Run build**

Run:

```bash
cd frontend && npm run build
```

Expected: no stale closure TypeScript errors. If React hook dependencies complain during linting in the future, convert `releaseUrl` to `useCallback`.

## Task 5: Long-Play Session Protection

**Files:**
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Add constants and state**

At module level:

```ts
const LONG_PLAY_LIMIT_MS = 15 * 60 * 1000;
```

Inside component:

```ts
const longPlayTimerRef = useRef<number | null>(null);
const [longPlayPromptOpen, setLongPlayPromptOpen] = useState(false);
const [guardPausedAt, setGuardPausedAt] = useState<number | null>(null);
```

- [ ] **Step 2: Start and clear timer based on active playback**

```ts
useEffect(() => {
  if (!currentPlayerUrl || !playbackState.playing || loading || longPlayPromptOpen) return;
  longPlayTimerRef.current = window.setTimeout(async () => {
    setGuardPausedAt(Date.now());
    setLongPlayPromptOpen(true);
    try {
      await playerRef.current?.pause();
    } catch {
      // The prompt is still useful even if pause fails.
    }
    setPlaybackState((current) => ({ ...current, playing: false }));
  }, LONG_PLAY_LIMIT_MS);
  return () => {
    if (longPlayTimerRef.current !== null) {
      window.clearTimeout(longPlayTimerRef.current);
      longPlayTimerRef.current = null;
    }
  };
}, [currentPlayerUrl, playbackState.playing, loading, longPlayPromptOpen]);
```

- [ ] **Step 3: Add continue/stop handlers**

```ts
async function continueAfterLongPlayPrompt() {
  setLongPlayPromptOpen(false);
  if (mode === "live") {
    await releaseUrl(liveUrl?.url_id);
    setLiveUrl(null);
    return;
  }
  if (!selectedSegment) {
    setToast("未找到当前回放片段，请重新选择录像片段");
    return;
  }
  const elapsedSeconds = guardPausedAt ? Math.floor((Date.now() - guardPausedAt) / 1000) : 0;
  const nextStart = clampUnix(selectedSegment.start_time + elapsedSeconds, selectedSegment.start_time, selectedSegment.end_time - 1);
  await releaseUrl(playbackUrl?.url_id);
  playRange(nextStart, selectedSegment.end_time, selectedSegment);
}

async function stopAfterLongPlayPrompt() {
  setLongPlayPromptOpen(false);
  await releaseUrl(activeUrlId);
  if (mode === "live") {
    setLiveUrl(null);
  } else {
    setPlaybackUrl(null);
  }
}
```

Add helper at module level:

```ts
function clampUnix(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
```

- [ ] **Step 4: Render modal**

Place after toast:

```tsx
{longPlayPromptOpen && (
  <div className="h5-long-play-backdrop" role="dialog" aria-modal="true" aria-label="继续观看确认">
    <div className="h5-long-play-modal">
      <strong>已连续播放较长时间</strong>
      <p>为避免长时间占用播放资源，请确认是否继续观看。</p>
      <div>
        <button type="button" onClick={stopAfterLongPlayPrompt}>停止观看</button>
        <button type="button" className="primary" onClick={continueAfterLongPlayPrompt}>继续观看</button>
      </div>
    </div>
  </div>
)}
```

- [ ] **Step 5: Add modal styles**

```css
.h5-long-play-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(15, 23, 42, 0.42);
}

.h5-long-play-modal {
  width: min(360px, 100%);
  padding: 18px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-card);
  background: var(--surface);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.18);
}

.h5-long-play-modal strong {
  color: var(--text-strong);
  font-size: 17px;
}

.h5-long-play-modal p {
  margin: 8px 0 16px;
  color: var(--text-muted);
  font-size: 14px;
  line-height: 1.6;
}

.h5-long-play-modal > div {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.h5-long-play-modal button {
  min-height: 34px;
  padding: 0 14px;
}

.h5-long-play-modal button.primary {
  border-color: var(--brand);
  background: var(--brand);
  color: #fff;
}
```

- [ ] **Step 6: Run a short-threshold local check**

Temporarily change `LONG_PLAY_LIMIT_MS` to `10 * 1000`, run demo, and verify the modal appears. Revert to `15 * 60 * 1000` before committing.

Run:

```bash
cd frontend && npm run build
```

Expected: build passes and the constant is back to 15 minutes.

## Task 6: Playback Segment Slider

**Files:**
- Create: `frontend/src/domain/h5-playback.ts`
- Create: `frontend/src/components/PlaybackSegmentSlider.tsx`
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`
- Modify: `frontend/src/api.test.ts`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Create pure helper**

```ts
export function segmentDurationSeconds(startTime: number, endTime: number): number {
  return Math.max(0, endTime - startTime);
}

export function shouldShowSegmentSlider(startTime: number, endTime: number): boolean {
  return segmentDurationSeconds(startTime, endTime) >= 2;
}

export function clampSegmentOffset(offset: number, startTime: number, endTime: number): number {
  const duration = segmentDurationSeconds(startTime, endTime);
  if (!Number.isFinite(offset)) return 0;
  return Math.min(duration, Math.max(0, Math.floor(offset)));
}

export function segmentOffsetToUnix(startTime: number, endTime: number, offset: number): number {
  return startTime + clampSegmentOffset(offset, startTime, endTime);
}
```

- [ ] **Step 2: Add tests**

Append to `frontend/src/api.test.ts`:

```ts
import {
  clampSegmentOffset,
  segmentDurationSeconds,
  segmentOffsetToUnix,
  shouldShowSegmentSlider,
} from "./domain/h5-playback";

test("calculates H5 playback segment slider bounds", () => {
  expect(segmentDurationSeconds(100, 130)).toBe(30);
  expect(segmentDurationSeconds(130, 100)).toBe(0);
  expect(shouldShowSegmentSlider(100, 101)).toBe(false);
  expect(shouldShowSegmentSlider(100, 102)).toBe(true);
  expect(clampSegmentOffset(-5, 100, 130)).toBe(0);
  expect(clampSegmentOffset(35, 100, 130)).toBe(30);
  expect(segmentOffsetToUnix(100, 130, 12.8)).toBe(112);
});
```

- [ ] **Step 3: Run tests to verify helper**

Run:

```bash
cd frontend && npm run test -- --runInBand
```

Expected: PASS. If Vitest rejects `--runInBand`, run `cd frontend && npm run test`.

- [ ] **Step 4: Create slider component**

```tsx
import { useEffect, useState } from "react";
import type { H5RecordSegment } from "../domain/h5-types";
import { segmentDurationSeconds, segmentOffsetToUnix, shouldShowSegmentSlider } from "../domain/h5-playback";

export function PlaybackSegmentSlider({
  segment,
  disabled,
  onCommit,
}: {
  segment: H5RecordSegment;
  disabled: boolean;
  onCommit: (startTime: number) => void;
}) {
  const duration = segmentDurationSeconds(segment.start_time, segment.end_time);
  const [offset, setOffset] = useState(0);
  const currentUnix = segmentOffsetToUnix(segment.start_time, segment.end_time, offset);

  useEffect(() => {
    setOffset(0);
  }, [segment.start_time, segment.end_time]);

  if (!shouldShowSegmentSlider(segment.start_time, segment.end_time)) {
    return <div className="h5-playback-slider compact">片段较短：{formatDateTime(segment.start_time)} - {formatDateTime(segment.end_time)}</div>;
  }

  function commit() {
    onCommit(currentUnix);
  }

  return (
    <div className="h5-playback-slider" aria-label="片段内定位">
      <div className="h5-playback-slider-head">
        <span>{formatTime(segment.start_time)}</span>
        <strong>当前：{formatDateTime(currentUnix)}</strong>
        <span>{formatTime(segment.end_time)}</span>
      </div>
      <input
        type="range"
        min={0}
        max={duration}
        step={1}
        value={offset}
        disabled={disabled}
        onChange={(event) => setOffset(Number(event.target.value))}
        onPointerUp={commit}
        onKeyUp={(event) => {
          if (event.key === "Enter" || event.key === " ") commit();
        }}
      />
    </div>
  );
}

function formatTime(unix: number): string {
  return new Date(unix * 1000).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatDateTime(unix: number): string {
  return new Date(unix * 1000).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
```

- [ ] **Step 5: Wire slider in playback panel**

Import:

```ts
import { PlaybackSegmentSlider } from "../components/PlaybackSegmentSlider";
```

Render after `PlaybackDatePicker` when selected segment exists:

```tsx
{selectedSegment && (
  <PlaybackSegmentSlider
    segment={selectedSegment}
    disabled={loading}
    onCommit={(startTime) => playRange(startTime, selectedSegment.end_time, selectedSegment)}
  />
)}
```

- [ ] **Step 6: Add slider styles**

```css
.h5-playback-slider {
  display: grid;
  gap: 8px;
  padding: 12px 16px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-card);
  background: var(--surface);
  box-shadow: var(--shadow-panel);
}

.h5-playback-slider.compact {
  color: var(--text-muted);
  font-size: 13px;
}

.h5-playback-slider-head {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
  color: var(--text-muted);
  font-size: 12px;
}

.h5-playback-slider-head strong {
  color: var(--text-main);
  font-size: 13px;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.h5-playback-slider input[type="range"] {
  width: 100%;
  accent-color: var(--brand);
}
```

- [ ] **Step 7: Run tests and build**

Run:

```bash
cd frontend && npm run test
cd frontend && npm run build
```

Expected: tests and build pass.

## Task 7: Local Browser Verification

**Files:**
- No source files required unless verification finds visual bugs.

- [ ] **Step 1: Start local frontend**

Run:

```bash
cd frontend && npm run dev -- --port 5173
```

Expected: Vite serves at `http://127.0.0.1:5173/`.

- [ ] **Step 2: Open demo H5 monitor route**

Open:

```text
http://127.0.0.1:5173/erzhuang-project/h5/orgs/demo/monitor
```

Expected: demo channel wall loads.

- [ ] **Step 3: Verify live controls**

Click a demo channel and verify:

- Control bar is visible at first.
- Play/pause button toggles state.
- Sound button toggles state.
- Screenshot button shows feedback or a clear unsupported message.
- Landscape button enters fixed black viewing mode and exits.
- Fullscreen button either enters fullscreen or shows unsupported message.
- Diagnostic card remains visible below the player.

- [ ] **Step 4: Verify playback slider**

Switch to `录像`, choose a segment, and verify:

- Slider appears below the player.
- Dragging the slider changes “当前” time without immediate repeated URL requests.
- Releasing the slider restarts playback from the selected time.
- Date picker and segment list remain visually aligned.

- [ ] **Step 5: Verify responsive layout**

Check widths:

- Desktop: 1440px and 1024px.
- Mobile: 390px and 430px.
- Mobile landscape or landscape mode: player fills the viewport without covering controls incoherently.

Expected: no text overlap, no clipped date picker, no controls blocking the whole picture.

## Task 8: Final Verification, Version, and Release Prep

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Bump version**

This is an existing H5 module interaction iteration, so bump minor version:

```text
2.20.0
```

- [ ] **Step 2: Run full verification**

Run:

```bash
git diff --check
cd frontend && npm run test
cd frontend && npm run build
go test ./...
```

Expected:

- `git diff --check`: no whitespace errors.
- `npm run test`: all tests pass.
- `npm run build`: TypeScript and Vite build pass.
- `go test ./...`: Go tests pass.

- [ ] **Step 3: Update progress doc**

Append a short entry to `docs/codex-learning-state.md` containing:

```markdown
### 2026-06-29 H5 player controls

- Added player control layer for play/pause, sound, screenshot, fullscreen, and landscape viewing.
- Added playback segment slider that requests a new playback URL from the chosen segment time.
- Added 15-minute long-play protection to release stale live/playback resources.
- Verification: npm test, npm build, go test.
```

- [ ] **Step 4: Commit**

Run:

```bash
git add frontend/src/components/H5FlvPlayer.tsx frontend/src/components/H5PlayerControls.tsx frontend/src/components/PlaybackSegmentSlider.tsx frontend/src/domain/h5-playback.ts frontend/src/pages/H5MonitorChannel.tsx frontend/src/styles.css frontend/src/api.test.ts VERSION docs/codex-learning-state.md
git commit -m "feat: add H5 player controls"
```

- [ ] **Step 5: Publish to company if requested**

Only after explicit user approval:

```bash
git push gitlab codex/containerize-single-image
```

Expected: GitLab CI/K8s picks up the protected branch and deploys automatically.

## Self-Review

- Spec coverage: all confirmed scope is covered: play/pause, sound, landscape, fullscreen, screenshot, status visibility, playback segment slider, long-play guard, and URL release. Explicitly excluded items are not added as buttons or hidden features.
- Placeholder scan: no `TBD`, `TODO`, or “similar to” placeholders are present.
- Type consistency: `H5PlayerHandle`, `H5PlaybackState`, `H5PlayerControlState`, and `H5RecordSegment` names are consistent across tasks.
- Risk note: screenshot support depends on the exact `ezuikit-flv` runtime method. The implementation must fail gracefully with a visible toast if the method is absent.
