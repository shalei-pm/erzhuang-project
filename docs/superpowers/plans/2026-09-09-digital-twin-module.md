# Digital Twin Module Implementation Plan

**Goal:** Migrate the local digital twin kit into `/erzhuang-project/digitaltwin/`, connect real cameras, and manage its institution scope in system settings.

**Architecture:** A session-protected React host embeds the trusted local vanilla kit in an isolated same-origin document. Real institution/camera metadata comes from whitelist-guarded NVR APIs. Only explicit camera clicks request a stream. Existing player, authorization and audit handling are reused. Operational figures remain explicitly synthetic.

**Tech Stack:** Existing React/Vite, Go, MySQL settings/audit stores; original HTML/CSS/SVG kit.

## Accepted Scope

- System settings menu: 数字孪生白名单. Admin adds institution IDs, sees resolved names, removes entries, and saves globally.
- Missing configuration defaults to institution 10001 (北京保利总部店), confirmed by prior real resource acceptance in project records. An explicit empty list disables all institutions.
- Whitelist intersects existing user institution permissions; it does not alter ordinary monitor access.
- Settings reuse tb_app_settings without DDL or instance changes, with transactional audit and optimistic conflict detection.
- Real cameras map by existing space metadata: reception includes front desk first then nurse station; consultation includes consultation rooms (面诊室) only; waiting includes 等候区/等待区; treatment includes 治疗室. Aftercare and unmapped cameras have no dashboard entry. Original monitoring remains unchanged.
- Camera dialogs embed the existing live player only; no playback controls. Screenshot capture retains existing watermark and audit handling.
- No database migration, production deployment, or source-project modifications. Live KPI integration is isolated behind the SdyService provider boundary; it is enabled only when the company-generated Triple client is available.

## Tasks

- [x] Migrate only kit runtime files, types and focused unit tests; preserve source provenance.
- [x] Implement tested settings persistence, transactional audit and conflict handling.
- [x] Add admin settings and whitelist-guarded camera/list/stream endpoints; exercise deny/empty/error states.
- [x] Add ID-based settings menu and session-protected dashboard host with lazy loading, version and cleanup.
- [x] Connect camera clicks to existing NVR player with close/switch/auth-expiry cancellation.
- [x] Run Go and frontend tests/builds; browser-check desktop/mobile, settings, camera opening, switches and empty/auth failure states.
- [ ] Record results and remaining real-stream verification; no push without release request.

## Live Metrics Addendum (2026-09-14)

- Copied the approved `sdy.proto` into `proto/soyoung/primecrm/sdy.proto` with a local `go_package`; the wire package and service name remain `com.soyoung.primecrm.SdyService`.
- Added `internal/digitaltwin.Service`, which calls overview, duty-staff and traffic-flow concurrently under one five-second timeout. A single RPC error fails the refresh as a whole, so an unavailable response cannot overwrite the last visible measurement with zero.
- Added the whitelist-protected endpoint `GET /api/digitaltwin/orgs/{externalOrgId}/dashboard` and connected the host page to consume it when it returns successfully. Existing camera routes and ordinary monitoring authorization are unchanged.
- Local verification: digital-twin Go package tests passed; server test binary compilation passed; Go build passed; frontend Vitest `19 files / 123 tests`, production build, and isolated Chrome checks (`7 passed`) passed.
- Local toolchain is now available under the ignored `.tools/` directory: native ARM64 `protoc 3.21.12`, `protoc-gen-go v1.26.0`, and `protoc-gen-go-triple 1.0.2`, built from the user-provided source archives. Generated `sdy.pb.go` and `sdy_triple.pb.go` are checked in under `internal/digitaltwin/primecrm`.
- Added `internal/digitaltwin/sdyrpc` to obtain `SdyServiceClientImpl` through `gitlab.sy.soyoung.com/go/library/service/dubbo`, map all three RPC responses, and recover client panics into contextual errors. `cmd/server` now injects the provider when the database-backed application starts; no web instance setting, Secret, MySQL DDL, or SSO change is required.
- Focused adapter tests and generated package compilation pass in an isolated local test workspace. Full local application test execution remains affected by the existing macOS `missing LC_UUID` loader issue; company-library end-to-end connectivity still requires the test Pod's runtime module/network environment.
