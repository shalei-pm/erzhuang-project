# Snapshot Reuse and NVR Control Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse safe 2.x channel thumbnails for single-recorder stores and make the NVR experiment player spacing, audio toggle, and live pause controls work.

**Architecture:** Resource-view repository returns optional old-channel thumbnail metadata only when the old store has exactly one recorder; the service attaches an application URL only for users with monitor permission. The NVR SDK performs local rendering pause and creates/resumes Web Audio directly from the unmute click, so neither control depends on undocumented WSS commands or click bubbling.

**Tech Stack:** Go, MySQL, React, TypeScript, Vitest, NVRPlayer WebSocket/WebCodecs/Web Audio.

---

### Task 1: Add legacy thumbnail retrieval to the read-only resource repository

**Files:**
- Modify: `internal/resourceview/models.go`
- Modify: `internal/resourceview/repository.go`
- Modify: `internal/resourceview/mysql_repository.go`
- Test: `internal/resourceview/mysql_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Add tests that require the query source to read `tb_stores`, `tb_video_recorders`, `tb_video_channels`, and latest `tb_channel_snapshots`, while retaining the repository's no-write guard.

- [ ] **Step 2: Run the focused repository test**

Run: `go test ./internal/resourceview -run TestMySQLRepository`

Expected: FAIL because the legacy snapshot query is absent.

- [ ] **Step 3: Implement minimal read-only lookup**

Add snapshot metadata keyed by business camera channel number. Return no mappings unless the old store has exactly one recorder, a channel number maps to one old channel, and the latest thumbnail path is nonempty.

- [ ] **Step 4: Re-run the focused repository test**

Run: `go test ./internal/resourceview -run TestMySQLRepository`

Expected: PASS.

### Task 2: Enforce authorization and expose preview URL

**Files:**
- Modify: `internal/resourceview/models.go`
- Modify: `internal/resourceview/service.go`
- Modify: `internal/app/handler.go`
- Test: `internal/resourceview/service_test.go`

- [ ] **Step 1: Write failing service tests**

Cover a single-recorder channel thumbnail shown with monitor access and withheld without monitor access. Add multi-recorder and missing-thumbnail cases that stay empty.

- [ ] **Step 2: Run focused service tests**

Run: `go test ./internal/resourceview -run 'Test.*Legacy.*Snapshot'`

Expected: FAIL because `Camera` lacks an authorized preview URL.

- [ ] **Step 3: Implement minimal authorization-gated mapping**

Attach the thumbnail URL only after `CanViewMonitor` is true. Serve it through an existing authenticated application route rather than exposing an OSS key.

- [ ] **Step 4: Re-run focused service tests**

Run: `go test ./internal/resourceview -run 'Test.*Legacy.*Snapshot'`

Expected: PASS.

### Task 3: Render thumbnail or fallback in the resource detail

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/components/ResourceStoreDetail.tsx`
- Test: `frontend/src/domain/resource-view.test.ts`

- [ ] **Step 1: Write a failing frontend mapping/render test**

Require a resource camera preview URL to map from the API and verify the detail row uses the existing placeholder when that URL is absent.

- [ ] **Step 2: Run focused frontend test**

Run: `cd frontend && npm test -- --run src/domain/resource-view.test.ts`

Expected: FAIL because the API type and mapper do not expose the preview URL.

- [ ] **Step 3: Implement minimal image rendering**

Add the optional field and render an image with `onError` fallback to the grey placeholder. Do not enable the disabled refresh button.

- [ ] **Step 4: Re-run focused frontend test**

Run: `cd frontend && npm test -- --run src/domain/resource-view.test.ts`

Expected: PASS.

### Task 4: Fix NVR sound control and local pause semantics

**Files:**
- Modify: `frontend/src/vendor/nvr-player/nvr-player.js`
- Modify: `frontend/src/components/NVRLabPlayer.tsx`
- Test: `frontend/src/components/NVRLabPlayer.test.tsx`

- [ ] **Step 1: Write failing control tests**

Mock NVRPlayer and require clicking unmute to invoke direct audio enabling before volume change, and clicking pause/resume to invoke the local player APIs.

- [ ] **Step 2: Run focused player test**

Run: `cd frontend && npm test -- --run src/components/NVRLabPlayer.test.tsx`

Expected: FAIL because the player has no explicit audio-enable API and pause only uses the undocumented WSS command.

- [ ] **Step 3: Implement minimal player fix**

Create/resume Web Audio synchronously from the unmute click. Make pause gate decoded media locally while keeping the signed stream connection open; resume resets decode state and waits for new frames. Do not log media or signed URLs.

- [ ] **Step 4: Re-run focused player test**

Run: `cd frontend && npm test -- --run src/components/NVRLabPlayer.test.tsx`

Expected: PASS.

### Task 5: Correct NVR play-page spacing and verify release

**Files:**
- Modify: `frontend/src/styles.css`
- Modify: `VERSION`
- Modify: `work/current-plan.md`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Add header-to-player spacing**

Apply a stable vertical gap on the NVR play page without changing the accepted 2.x monitor layout.

- [ ] **Step 2: Run all verification**

Run: `go test ./... && go build ./cmd/server && cd frontend && npm test -- --run && npm run build`

Expected: all commands pass; the existing Vite large-chunk warning may remain.

- [ ] **Step 3: Perform Chrome visual and control verification**

On the logged-in test environment, verify player spacing, live pause freezes and resume restores rendering, and unmute initializes audio without console errors. Verify one single-recorder image hit and one multi-recorder fallback.

- [ ] **Step 4: Publish test environment**

Commit only this task's source, tests, version, and project-memory updates; push GitLab `codex/containerize-single-image`, wait for Wharf pipeline `752` automatic deployment, then re-check page version, health, resource list and NVR lab route.
