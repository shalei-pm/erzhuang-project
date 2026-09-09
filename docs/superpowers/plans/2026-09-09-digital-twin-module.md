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
- No live KPI backend, database migration, production deployment, or source-project modifications.

## Tasks

- [ ] Migrate only kit runtime files, types and focused unit tests; preserve source provenance.
- [x] Implement tested settings persistence, transactional audit and conflict handling.
- [x] Add admin settings and whitelist-guarded camera/list/stream endpoints; exercise deny/empty/error states.
- [x] Add ID-based settings menu and session-protected dashboard host with lazy loading, version and cleanup.
- [x] Connect camera clicks to existing NVR player with close/switch/auth-expiry cancellation.
- [ ] Run Go and frontend tests/builds; browser-check desktop/mobile, settings, camera opening, switches and empty/auth failure states.
- [ ] Record results and remaining real-stream verification; no push without release request.
