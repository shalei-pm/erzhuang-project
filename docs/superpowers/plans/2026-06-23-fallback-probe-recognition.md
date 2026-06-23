# Fallback Probe Recognition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When Ezviz `camera/list` is unavailable, scan a recorder by probing one channel at a time, saving the successful snapshot and AI recognition result immediately.

**Architecture:** Keep the fast path unchanged: `camera/list` success still returns a normal recorder channel list. For `10026` fallback cases, the frontend drives a short-request queue by calling a new single-channel probe endpoint; each successful probe upserts one active channel, stores the snapshot, runs AI recognition, and returns the updated channel. The frontend stops after 5 consecutive failed probes but only shows user-facing progress as “已检测 X 个，有效 Y 个”.

**Tech Stack:** Go `net/http`, existing storespace service/repository, Ezviz OpenAPI client, React/Vite frontend.

---

### Task 1: Backend Single-Channel Probe

**Files:**
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/handler.go`
- Test: `internal/storespace/service_test.go`

- [ ] Add `ProbeRecognizeChannelInput` and `ProbeRecognizeChannelResult` models.
- [ ] Add repository method `UpsertRecorderChannel(ctx, recorderID, ChannelInput) (*Channel, error)`.
- [ ] Implement memory and Postgres upsert behavior without clearing confirmed mappings.
- [ ] Implement `Service.ProbeRecognizeChannel`: capture one channel, upsert it if capture succeeds, store snapshot via `SnapshotStore`, run AI recognition for unconfirmed channels, and return `{channel, active, message}`.
- [ ] Add `POST /api/store-space/recorders/{recorder_id}/probe-recognize-channel`.
- [ ] Add tests for successful probe and failed probe.

### Task 2: Frontend Fallback Queue

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/components/VideoChannelTab.tsx`
- Modify: `frontend/src/styles.css` only if existing progress UI cannot be reused.

- [ ] Add API adapter method `probeRecognizeRecorderChannel(storeId, recorderId, channelNo)`.
- [ ] In `scanRecorder`, if scan returns a `10026`-style failure message, start fallback probing from channel 1.
- [ ] For each successful probe, immediately merge the returned channel into the recorder table and clear stale thumbnail error state.
- [ ] Track progress as detected count and active count; do not display consecutive failure count.
- [ ] Stop after 5 consecutive failed probes or after channel 32.
- [ ] Keep the existing “识别区域” flow unchanged for recorders that scan normally.

### Task 3: Version, Docs, Verification

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] Bump version to `2.14.0`.
- [ ] Record the product decision and root cause: `camera/list` returns `10026`; slow failed capture probes can exceed request timeouts; fallback probing is now short-request and incremental.
- [ ] Run `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...`.
- [ ] Run `cd frontend && npm run build`.
- [ ] Run `git diff --check`.
