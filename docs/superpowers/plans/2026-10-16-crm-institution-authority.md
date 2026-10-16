# CRM Institution Authority Implementation Plan

> Execute in this session with executing-plans; use test-driven-development for behavioral changes.

**Goal:** CRM is the authoritative institution directory for monitor permissions, without manual tb_stores mappings.

**Architecture:** New writes use resource_type=crm_institution and CRM IDs. Legacy store permissions are read using their saved external_key (never by assuming equal numeric IDs). Saving permissions deletes both old and new monitor scopes and inserts the complete CRM selection in one transaction. No background migration, CRM writes or schema changes.

**Tech Stack:** Go database/sql, MySQL, React/TypeScript, existing scripted SQL driver and browser tests.

## Decisions And Boundaries

- Candidate source: enabled tb_crm_admin_tenant, independent of edge/camera availability. Homepage retains its capability filter.
- Add explicit monitor_institution_ids request field; legacy monitor_store_scope_ids cannot be interpreted as CRM IDs. Reject obsolete writes with a refresh instruction. Responses expose institution_id; retain legacy store_id only as a compatibility label, never consume it for new writes.
- Legacy authorization uses its existing canonical external_key. New authorization requires resource_id and external_key to agree. Reads deduplicate by CRM ID. Invalid legacy identity is an explicit error when editing rather than silently dropping access.
- Permission edits require a successful fresh CRM candidate query. Previously selected unavailable institutions remain visible and may be retained or explicitly removed, but cannot be newly granted. Source failure aborts the transaction. Concurrent updates serialize through the user row lock.
- A nil institution slice preserves scope rows; an explicit empty array revokes them. Every user update, including a nil scope request, requires a revision bound to the raw scope tuples, role and enabled state. Validate it under user/scope locks before writing.
- All monitor roles require an enabled CRM institution; privileged roles skip only explicit scope restrictions. Mixed old/new scope identities fail closed. Noncanonical or unsafe IDs are rejected.
- Scope replacement applies for all roles so stale viewer grants cannot reappear after role changes. Frontend preserves selected scopes when switching roles.
- Preserve old tables, IDs, design plans, channels and logs. Do not add automatic legacy rows.
- Remove retired thumbnail reads and entry points; preserve current NVR snapshot authorization/storage.
- Retire legacy directory create/rename/external-ID/delete HTTP writes with a clear conflict response. Keep unrelated local resource operations.
- Rollback: old binaries cannot read CRM-only grants. After any new permission save, use a forward fix or a DBA-reviewed reverse mapping; do not advertise code-only rollback as safe.
- No production reads/writes or releases in this task. Test branch publication requires explicit release authorization.

## Tasks

- [x] 1. Independent DBA-specialty review of identity, replacement, unavailable grants and rollback; findings fixed and re-reviewed. This is not approval by the pinned DBA thread; a separate handoff was sent there.
- [x] 2. RED: SQL tests for CRM-only candidates, read failure, legacy conversion, collisions, mismatches, transactional replacement and revoked access.
- [x] 3. GREEN: implement CRM scope repository helper and explicit request/response contract, actual transition audit IDs, memory parity. Run app tests.
- [x] 4. RED/GREEN: remove legacy snapshot fallback and test current NVR thumbnails; retire old directory HTTP mutations and test routes.
- [x] 5. RED/GREEN: frontend institution IDs, retained unavailable selections, source failure/save states; browser checks at 1440/1024/390 widths. Fix clipped modal footer and narrow-screen tools.
- [x] 6. Full Go tests and server build, frontend 151 tests/build and targeted browser checks passed. VERSION=4.9.0; release notes and continuity log updated.
- [x] 7. Scoped diff reviewed; no push/deployment. Actual MySQL DDL, concurrent connections, other writers and deployed monitor behavior remain test-environment gates.

## Verification Commands

Use project Go binary with CGO_ENABLED=0 GOCACHE=/tmp/erzhuang-go-cache GOPROXY=off. Run app/resourceview/storespace targeted tests after each change; then go test ./... and go build. Frontend uses npm test and npm run build where defined. SQL-driver expectations must reject tb_stores access on CRM candidate/new-write/current-resource paths. Browser requests use local fixtures only; no real user permission mutations.

## 4.9.1 Follow-Up: Match Homepage Eligibility

User approved aligning permission candidates with the homepage after testing 4.9.0. This supersedes the earlier decision to list all enabled CRM institutions for new grants. Both now require an enabled CRM institution with at least one enabled, nondeleted edge device; an offline edge remains eligible. Reuse one SQL predicate for homepage list/detail and candidate/save validation. Existing grants outside the current candidate set remain manageable and are counted separately in the form. Metadata enrichment and monitor authorization keep their existing rules. No schema change, automatic grant cleanup or publication is included in this follow-up request.
