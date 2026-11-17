# Playback Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Follow the tasks below in this session; the user has approved execution and subagents.

**Goal:** Add replay-only forward ten seconds and verified speed selection with real pause/resume.

**Architecture:** The SDK reports presented media elapsed time and verifies upstream speed using frame PTS against elapsed time. The page owns the playback window, desired rate, paused location, and request generation. Pause closes the stream, resume and seek acquire fresh sessions; controls do not rebuild sessions unless an actual transport change requires it.

**Tech Stack:** React, TypeScript, WebSocket, WebCodecs, existing WASM transform, Vitest, browser regression harness.

## Task 1: SDK playback media time and verified speed (subagent)

Files: `frontend/src/vendor/nvr-player/nvr-player.js`, `frontend/src/vendor/nvr-player/nvr-player.d.ts`, `frontend/src/components/NVRPlayerPlayback.test.ts`.

- [x] Write failing tests for rendered-PTS progress, discarded/paused frames, elapsed-time reset, valid rates, speed verification failure, stop cancellation, and scheduled audio cleanup.
- [x] Run `npm test -- src/components/NVRPlayerPlayback.test.ts`; observe feature failures.
- [x] Add the following API using existing SDK style:

```ts
type NVRPlayerOptions = {
  onPlaybackTime?: (elapsedSeconds: number) => void;
};
// Playback-only; returns relative time of the last presented video frame.
getPlaybackTime(): number | null;
// Allowed: 1, 2, 4, 8, 16. Promise resolves only on observed frame cadence.
setSpeed(rate: number): Promise<void>;
```

- [x] Wire progress after successful canvas presentation. Validate timestamps, ignore initialization frames and handle timestamp resets without false jumps. Leave live RTP timing unchanged.
- [x] Send the existing speed command and verify media advancement over a bounded stable observation interval; reject stopped sockets, absent progress, and wrong rates. Do not infer success from send completion.
- [x] Stop rejects pending verification; reset queued Worklet/fallback audio and stop scheduled sources. Preserve existing 4.9.2 live sound behavior.
- [x] Run playback, audio, and RTP tests; self-review the diff and report changes for review.

## Task 2: Page/session coordination and controls (main)

Files: `frontend/src/pages/NVRLabCamera.tsx`, `frontend/src/components/NVRLabPlayer.tsx`, `frontend/src/components/H5PlayerControls.tsx`, new `PlaybackSpeedSheet.tsx`, `frontend/src/styles.css`, `frontend/src/components/NVRLabPlayer.test.tsx`.

- [x] First add failing tests for replay-only buttons and speed options.
- [x] SDK emits elapsed media seconds; page computes `min(window.endTime - 1, session.startTime + elapsed)` and stores it synchronously in a ref.
- [x] Add generation guards to session requests and invalidate on mode/date/seek/unmount. Keep callbacks stable and exclude display-only state from decoder lifecycle dependencies.
- [x] Pause captures last presented position, stops the player, preserves canvas, and stores speed. Resume asks for a fresh session at that position, reapplies rate, and waits for the first frame before indicating playing.
- [x] Forward uses `min(window.endTime - 1, currentUnix + 10)`; paused forward only changes saved location. Paused speed selection updates intended rate without resuming.
- [x] Speed selection is `1 -> sheet -> chosen rate`; active-speed click selects 1 directly. Show pending state during verification. On verification failure reset intended rate and reacquire a normal stream at current position.
- [x] Mount the sheet in the player shell so fullscreen works. Add dialog focus trap, Escape/cancel/backdrop closing, safe-area padding and 320px layout.
- [x] Run the focused component tests and `npm run build`.

## Task 3: Integration and browser acceptance (subagent then main)

Files: `frontend/src/playback-controls.browser.mjs`, `scripts/playback-controls-preview.mjs`, `VERSION`, `docs/codex-learning-state.md`, release evidence in `docs/releases/`.

- [x] Extend the existing synthetic browser harness with a controlled media source. Use real UI controls and SDK progress instead of faking cursor state.
- [x] Verify all four rates, return to 1, reopen/cancel, forward10, media stall, 30-second pause, paused seek/rate, request response inversion, speed rejection, no control-driven remounts, and live exclusion.
- [x] Capture desktop/mobile screenshots and inspect actual controls, dialog, canvas and geometry. Use browser plugin for visible review; headless harness may supply reproducible regressions.
- [x] Run `npm test` and `npm run build`. No Go files change; report Go runtime availability honestly.
- [x] Set `VERSION` to `4.10.0`, record local evidence and outstanding real-device protocol check. Keep local preview available with a verified HTTP URL.
- [x] Review specification compliance before code quality, fix findings, commit locally only after green checks. Do not push or deploy.

Final evidence: 230 unit tests, 22 playback browser scenarios and 4 existing audio browser scenarios passed. Production frontend build passed. See `docs/releases/2026-11-17-playback-controls-4.10.0.md` for remaining device verification and local evidence paths.
