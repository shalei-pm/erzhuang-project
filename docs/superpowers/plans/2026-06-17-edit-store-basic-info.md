# Edit Store Basic Info Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a list-level edit action so city, store name, and 新氧机构 ID can be corrected after store creation.

**Architecture:** Keep basic store information separate from design-plan and recorder maintenance. Add a backend `PATCH /api/store-space/stores/{id}` endpoint for basic fields only, and add a lightweight edit modal in the frontend that reuses the same city options and form styling as creation without exposing PDF or recorder controls.

**Tech Stack:** Go `net/http` backend, existing `storespace` service/repository, React + TypeScript frontend.

---

### Task 1: Backend Basic Info Update

**Files:**
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/validation.go`
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/handler.go`
- Test: `internal/storespace/service_test.go`
- Test: `internal/storespace/handler_test.go`

- [ ] Add `UpdateStoreBasicInfoInput` with `city`, `name`, and `external_org_id`.
- [ ] Add validation requiring non-empty city and name.
- [ ] Add repository method that updates `stores.city`, `stores.name`, `stores.normalized_name`, `stores.external_org_id`, and `updated_at`.
- [ ] Add service method and `PATCH /api/store-space/stores/{id}` handler.
- [ ] Cover service update, validation failure, and HTTP endpoint behavior.

### Task 2: Frontend Edit Modal

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/CreateStoreModal.tsx`
- Create: `frontend/src/components/EditStoreModal.tsx`
- Modify: `frontend/src/components/StoreList.tsx`

- [ ] Export shared city options from the create modal.
- [ ] Add `UpdateStoreBasicInfoPayload` and `storeSpaceApi.updateStoreBasicInfo`.
- [ ] Add `EditStoreModal` for city, store name, and 新氧机构 ID only.
- [ ] Add list operation buttons: `详情 / 编辑 / 删除`.
- [ ] Refresh list and active detail state after save.

### Task 3: Version, Verification, Release Readiness

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] Bump version from `2.11.1` to `2.12.0` because this is an interaction capability added to an existing module.
- [ ] Run backend tests where local toolchain permits.
- [ ] Run frontend build.
- [ ] Verify list operation buttons do not overflow at desktop width.
