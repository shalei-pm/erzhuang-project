# Monitor Screenshot Watermark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Watermark each user-triggered NVR monitor screenshot with the user's SSO display name and service-issued China Standard Time, controlled by an administrator setting.

**Architecture:** Store the boolean setting in the existing `tb_app_settings` table. The API authorizes the specific store/camera screenshot request, returns only trustworthy display data, and writes one audit event. The browser composites the existing screenshot with Canvas before its existing share/download flow; no image bytes reach the server.

**Tech Stack:** Go `net/http`, MySQL, existing SSO/session/audit code, React/TypeScript, HTML Canvas, Vitest.

---

## File Structure

- `internal/app/ai_settings.go`: setting key, default, and store interface.
- `internal/app/memory_store.go`, `internal/app/mysql_store.go`: memory and MySQL implementations using `tb_app_settings`.
- `internal/app/monitor_screenshot.go`: metadata and settings HTTP handlers.
- `internal/app/handler.go`: protected route registration.
- `internal/app/monitor_screenshot_test.go`: setting, permission, authorization, response and audit tests.
- `frontend/src/api.ts`: typed production and mock API methods.
- `frontend/src/domain/screenshot-watermark.ts`: Canvas composition with fail-closed behavior.
- `frontend/src/domain/screenshot-watermark.test.ts`: browser-side composition tests.
- `frontend/src/App.tsx`: administrator-only Security Settings tab.
- `frontend/src/pages/NVRLabCamera.tsx` and `frontend/src/components/NVRLabPlayer.tsx`: integration at the current formal NVR screenshot button only.
- `VERSION`, `docs/codex-learning-state.md`: version and release evidence.

### Task 1: Store the Security Setting

**Files:** `internal/app/ai_settings.go`, `internal/app/memory_store.go`, `internal/app/mysql_store.go`, `internal/app/monitor_screenshot_test.go`

- [ ] Write failing tests for a missing setting returning `true`, then saving and reading `false`.
- [ ] Run `go test ./internal/app -run 'TestScreenshotWatermark(DefaultsToEnabled|CanBeDisabled)' -count=1`; expect missing-method compilation failure.
- [ ] Add `MonitorScreenshotSettingsStore` with `GetMonitorScreenshotWatermarkEnabled(context.Context) (bool, error)` and `SetMonitorScreenshotWatermarkEnabled(context.Context, bool) error`.
- [ ] Persist `true`/`false` at `monitor_screenshot_watermark_enabled` with MySQL `insert ... on duplicate key update`; missing or malformed values must resolve to enabled.
- [ ] Re-run the focused tests; expect PASS.
- [ ] Commit: `feat: store monitor screenshot watermark setting`.

### Task 2: Authorize Metadata and Audit Explicit Screenshots

**Files:** `internal/app/monitor_screenshot.go`, `internal/app/handler.go`, `internal/app/monitor_screenshot_test.go`, `internal/app/handler_test.go` only if an interface test double requires it.

- [ ] Write a failing `POST /api/h5/orgs/10001/monitor/cameras/111/screenshot-metadata` test that checks a user without monitor-store access receives `403`.
- [ ] Write a failing permitted-user test that fixes `h.now`, expects `{ "watermark_enabled": true, "display_name": "测试管理员", "captured_at": "2026-09-02 09:18:36" }`, and asserts exactly one `monitor.screenshot` audit event.
- [ ] Register the route behind `storeReadGuard`; reject malformed IDs and confirm the user can view the requested store and that the NVR service resolves the camera within it.
- [ ] Format server time using `Asia/Shanghai`. When disabled return only the disabled state. Never accept or store a Data URL or image byte.
- [ ] Write a failing viewer test for setting update, then add authenticated `GET /api/monitor-screenshot-watermark-settings` and `POST` guarded by `PermissionUserManage`. The POST accepts only `{ "enabled": boolean }` and records `system.monitor_screenshot_watermark.update` with old/new booleans.
- [ ] Run `go test ./internal/app -count=1`; expect PASS.
- [ ] Commit: `feat: add monitor screenshot watermark metadata`.

### Task 3: Compose Screenshots in a New Canvas

**Files:** `frontend/src/domain/screenshot-watermark.ts`, `frontend/src/domain/screenshot-watermark.test.ts`

- [ ] Write a failing Vitest that passes enabled metadata and expects a PNG Data URL from `composeMonitorScreenshot`.
- [ ] Write a failing Vitest that expects rejection when enabled metadata lacks either display name or server time.
- [ ] Run `cd frontend && npm test -- screenshot-watermark.test.ts`; expect module-not-found failure.
- [ ] Implement image loading into a new Canvas, right-top two-line text, image-relative font/padding, and a semi-transparent dark background. Export PNG.
- [ ] Return the original Data URL only when the setting is disabled. Reject load, draw, metadata, and export failures while enabled so a bare image cannot be exported.
- [ ] Re-run the focused tests; expect PASS.
- [ ] Commit: `feat: compose watermarked monitor screenshots`.

### Task 4: Connect Existing UI Without Touching Thumbnail Flows

**Files:** `frontend/src/api.ts`, `frontend/src/api-nvr-lab.ts`, `frontend/src/App.tsx`, `frontend/src/pages/NVRLabCamera.tsx`, `frontend/src/components/NVRLabPlayer.tsx`, `frontend/src/domain/format.ts` and its test only when a new API error must be translated.

- [ ] Write a failing API test asserting `getMonitorScreenshotMetadata("10001", 111)` sends a `POST` to the protected metadata endpoint.
- [ ] Add API methods for get/set watermark settings and monitor screenshot metadata, with deterministic mock-adapter parity.
- [ ] Extend the existing settings modal union with `security`; show an administrator-only “安全设置” tab and one compact “监控截图水印” switch using existing control styles.
- [ ] In the current NVR player screenshot callback, capture the player frame, request metadata, compose the watermark, and download only the result.
- [ ] On metadata or composition failure, show a Chinese error message and exit without download. Do not change first-frame snapshot backfill, thumbnails, large-image opening, refresh-thumbnail behavior, legacy `H5MonitorChannel`, or the standalone NVR lab route.
- [ ] Run `cd frontend && npm test && npm run build`; expect PASS.
- [ ] Commit: `feat: watermark monitor screenshots`.

### Task 5: Release Preparation and Test Verification

**Files:** `VERSION`, `docs/codex-learning-state.md`

- [ ] Change `VERSION` from `3.4.1` to `3.4.2`.
- [ ] Run `go test ./... -count=1`, `go build ./...`, `cd frontend && npm test && npm run build`, and `git diff --check`; document any separately proven sandbox-only limitation.
- [ ] Commit: `chore: bump version to 3.4.2`.
- [ ] Push only `gitlab/codex/containerize-single-image`; never push `main`, run DDL, edit an instance, or edit a K8s Secret.
- [ ] After Wharf `752` auto-deploys, use the in-app browser without affecting the user's Chrome page to verify health, footer version, security switch, one live screenshot, one playback screenshot, and one audit event per click. Confirm thumbnails and frozen frames do not write `monitor.screenshot`.

## Plan Self-Review

- Setting persistence, authorization, time source, audit, Canvas behavior, global switch, UI scoping, versioning, and testing deployment all map to an approved requirement.
- The plan does not add a table, modify MySQL schema, persist image content, or broaden scope to lab/thumbnail code.
- Route payload names, setting key, event names, and `3.4.2` are consistent with the approved design.
