# Channel Mapping Excel Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real Excel export for the current store's channel mapping table, with the button placed on the channel list header to the left of the type filters.

**Architecture:** Backend exposes `GET /api/store-space/stores/{id}/channel-mappings/export.xlsx`, builds an `.xlsx` with standard library ZIP/XML, and embeds available channel screenshots from `SnapshotStore`. Frontend adds `storeSpaceApi.exportChannelMappings()` and a `导出 Excel` button in `VideoChannelTab` beside the filter controls.

**Tech Stack:** Go standard library (`archive/zip`, XML escaping, `net/http`), existing `storespace.Service`, existing React/Vite frontend.

---

### Task 1: Backend Export Service

**Files:**
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/models.go`
- Test: `internal/storespace/service_test.go`
- Create: `internal/storespace/channel_mapping_excel.go`

- [ ] Write failing service test for exporting only active channels in order.
- [ ] Add export row model and service method `ExportChannelMappingExcel(ctx, storeID)`.
- [ ] Implement minimal XLSX writer with text columns and optional image embedding.
- [ ] Verify targeted tests pass.

### Task 2: Backend HTTP Endpoint

**Files:**
- Modify: `internal/storespace/handler.go`
- Test: `internal/storespace/handler_test.go`

- [ ] Write failing handler test for `GET /api/store-space/stores/{id}/channel-mappings/export.xlsx`.
- [ ] Register route and stream XLSX response with attachment filename.
- [ ] Verify handler test passes.

### Task 3: Frontend Button And Download

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/components/VideoChannelTab.tsx`
- Modify: `frontend/src/styles.css`

- [ ] Add `storeSpaceApi.exportChannelMappings(storeId)` that downloads a blob using the backend filename.
- [ ] Add `导出 Excel` button in the channel filter bar, to the left of segmented filters.
- [ ] Add loading/disabled state and toast errors.
- [ ] Style button using existing admin button tokens.

### Task 4: Verification And Release Prep

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] Bump version as feature iteration.
- [ ] Run `go test ./...`.
- [ ] Run `npm run build`.
- [ ] Browser-smoke the channel tab button placement.
- [ ] Push to GitHub main and company GitLab branch if requested.
