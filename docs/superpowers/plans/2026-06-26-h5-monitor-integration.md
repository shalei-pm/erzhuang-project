# H5 Monitor Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate the H5 monitoring prototype into `erzhuang-project` so a store can expose a mobile camera list, live FLV playback, playback segment lookup, and playback FLV viewing by external org ID.

**Architecture:** Keep the H5 monitor as a bounded module with its own backend package, frontend pages, API client, and route parser. Reuse existing store-space data, Ezviz credentials, snapshot proxy, asset storage, and app base-path handling instead of duplicating business state.

**Tech Stack:** Go 1.22, `net/http`, existing `internal/ezviz` client, existing `internal/storespace` repository/service, React 19, Vite, `ezuikit-flv`, `antd`, `dayjs`.

---

## Scope

### In Scope

- H5 route:
  - `/h5/orgs/{externalOrgId}/monitor`
  - `/h5/orgs/{externalOrgId}/monitor/channels/{channelId}`
- Backend APIs under `/api/h5/orgs/{externalOrgId}/monitor`.
- Confirmed active channels only.
- Category filters: all, consultation, treatment, beauty, front/waiting, other.
- Live FLV playback through Ezviz `live/address/get` using protocol `4`, quality `2`, `supportH265=1`, `mute=0`.
- Local playback segment query through Ezviz v3 local video query.
- Playback FLV address through Ezviz `live/address/get` with `type=2`.
- Disable playback URL on leaving or switching stream.
- Best-effort AAC transfer before playback.
- In-memory concurrency guard for first release.
- Static decoder assets copied into frontend public assets.
- Route-level frontend branching so admin pages do not instantiate H5 player code unless an H5 route is opened.

### Out of Scope For This Release

- Feishu login, permission, admin recognition, and identity integration.
- Persisted playback session table and cross-instance concurrency counting.
- Multi-camera playback.
- PTZ, screenshots, downloads, speed controls, talkback.
- Cloud recording query.
- Full company SaaS QR-code placement, except leaving a clear route contract.

## Task 1: Backend Ezviz Playback Client

**Owner:** Backend worker.

**Files:**
- Modify: `internal/ezviz/client.go`
- Modify: `internal/ezviz/client_test.go`
- Create: `internal/ezviz/playback.go`
- Create: `internal/ezviz/playback_test.go`
- Create: `internal/ezviz/record_segments.go`
- Create: `internal/ezviz/record_segments_test.go`
- Create: `internal/ezviz/aac_transfer.go`
- Create: `internal/ezviz/aac_transfer_test.go`

**Requirements:**

- Extend `LiveAddressRequest` with:
  - `Type int`
  - `SupportH265 bool`
  - `Mute int`
- `LiveAddress` must include these form values only when meaningful:
  - `type` when `Type > 0`
  - `supportH265=1` when true
  - `mute` when `Mute` is `0` or `1`; use an explicit pointer or sentinel if needed so `mute=0` is sent for H5.
- Add `PlaybackAddress(ctx, account, PlaybackRequest)` using `/api/lapp/v2/live/address/get` with `type=2`.
- Add `DisableLiveAddress(ctx, account, urlID)` using `/api/lapp/v2/live/address/disable`.
- Add `QueryRecordSegments(ctx, account, RecordSegmentsQuery)` using `GET /api/v3/device/local/video/unify/query`.
- Add `EnsureAACTransfer(ctx, account, deviceSerial, channelNo)` using `POST /api/service/media/aac/transfer?enable=1`.
- Token expiration retry behavior must match existing `LiveAddress`.
- Tests must assert form/header/query parameters and token retry behavior where applicable.

**Verification:**

- Run `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/ezviz`.

## Task 2: Backend H5 Monitor Module

**Owner:** Backend worker.

**Files:**
- Create: `internal/h5monitor/models.go`
- Create: `internal/h5monitor/service.go`
- Create: `internal/h5monitor/handler.go`
- Create: `internal/h5monitor/handler_test.go`
- Modify: `internal/app/handler.go`
- Modify: `cmd/server/main.go`
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/models.go` only if a narrow exported helper type is needed.

**Requirements:**

- Add repository adapter methods that can fetch:
  - Store by `external_org_id`.
  - Confirmed active channels for that store, including store ID, channel ID, recorder device code, channel number, channel name, area type, area number, area note, snapshot path, Ezviz account credentials.
  - Channel context by channel ID with the same data.
- Do not expose `device_serial`, app key, app secret, Ezviz account ID, or account name to H5 frontend responses.
- H5 home response should return store name, city, external org ID, and grouped channels.
- Valid channels must satisfy:
  - channel status/effective state is active/valid.
  - confirmation status is confirmed.
  - recorder and Ezviz account exist.
- Category mapping:
  - consultation: area type consultation.
  - treatment: area type treatment and VIP treatment.
  - beauty: area type beauty.
  - front_waiting: area/note/name contains `前台`, `候诊`, or `等候`.
  - other: all other confirmed active channels.
- Sorting:
  - all: channel number ascending.
  - consultation/treatment/beauty: numeric area number ascending, then text.
  - front_waiting/other: Chinese text lexical order, then channel number.
- Live URL endpoint must:
  - enforce concurrency limit.
  - best-effort call AAC transfer.
  - request FLV with quality 2, H265 support, mute 0, expire time 600 seconds.
  - best-effort trigger snapshot refresh after successful URL creation if practical.
- Playback URL endpoint must enforce concurrency limit and return FLV playback URL.
- Disable endpoint must disable Ezviz URL and release concurrency; it may return `ok:false` on Ezviz failure without blocking frontend navigation.
- Register routes through app handler and preserve `/erzhuang-project` base path prefix behavior.

**Verification:**

- Run `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/h5monitor ./internal/storespace ./internal/app`.

## Task 3: Frontend H5 Monitor Pages

**Owner:** Frontend worker.

**Files:**
- Create: `frontend/src/api-h5.ts`
- Create: `frontend/src/domain/h5-types.ts`
- Create: `frontend/src/components/H5FlvPlayer.tsx`
- Create: `frontend/src/pages/H5Monitor.tsx`
- Create: `frontend/src/pages/H5MonitorChannel.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/styles.css`
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

**Requirements:**

- Detect H5 routes before rendering the admin app:
  - `/erzhuang-project/h5/orgs/{externalOrgId}/monitor`
  - `/erzhuang-project/h5/orgs/{externalOrgId}/monitor/channels/{channelId}`
  - legacy `/erzhuang/h5/...` should also work through backend base path behavior.
- Keep the admin app behavior unchanged for non-H5 paths.
- Use a lightweight H5 route parser; do not introduce full router unless needed.
- H5 homepage:
  - Mobile-first camera wall.
  - Store name and city visible.
  - Category tabs with counts or clear labels.
  - Camera cards use existing snapshot URLs when present.
  - 24 channels per batch with load more.
  - No edit controls.
- H5 detail page:
  - Default to live.
  - Large player area.
  - Back button.
  - Bottom mode switch: live / playback.
  - Playback date/time selection with today/yesterday/before yesterday shortcuts and precise date-time picker.
  - Segment list appears after query.
  - Clicking a segment plays that segment.
  - Switching stream or leaving page calls disable-url.
  - Player starts muted to satisfy browser autoplay; user can tap to enable sound.
- H5 player:
  - Use `ezuikit-flv`.
  - Load decoder assets from `/assets/ezuikit-flv/`.
  - Destroy player on unmount.
  - Show Chinese error messages.
- Keep H5 styles scoped with `h5-` class prefix.
- Do not change admin UI styling except what is necessary for route selection.

**Verification:**

- Run `cd frontend && npm install`.
- Run `cd frontend && npm run build`.
- Run `cd frontend && npm run test`.

## Task 4: Static Assets, Version, and Docs

**Owner:** Integration worker / architect.

**Files:**
- Create directory: `frontend/public/assets/ezuikit-flv/`
- Copy decoder runtime files from `frontend/node_modules/ezuikit-flv/` into that directory if package exposes them.
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Optionally create: `docs/h5-monitor-integration.md`

**Requirements:**

- Version should bump as a new medium feature, from `2.18.4` to `2.19.0`.
- Document:
  - H5 routes.
  - APIs.
  - Environment variables reused from current Ezviz account setup.
  - Known production limitation: in-memory concurrency counter is not cross-pod durable.
  - Feishu login not included.
- Confirm bundled H5 assets do not break existing admin routes.

**Verification:**

- Run full backend tests: `CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...`.
- Run frontend build and tests.
- Start local dev or preview server and manually check:
  - Admin list page still loads.
  - H5 unknown route shows useful not-found.
  - H5 route can render with API error state if no backend data exists.

## Final Review Checklist

- No secrets in frontend, docs, Dockerfile, or Git remote URL.
- No device serial or Ezviz credential leaks in H5 API responses.
- Existing admin routes and APIs still pass tests.
- H5 route does not force-load player code on admin route if avoidable.
- Disable URL is best-effort but concurrency release is guaranteed for the local process.
- User-visible errors are Chinese and actionable.
- Production risks are documented before publishing.
