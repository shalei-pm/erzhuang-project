# NVR Snapshot Backfill: OSS-Only Design

**Status:** approved replacement for the earlier MySQL-table draft.
**Date:** 2026-08-26
**Owner:** Erzhuang main session

## Goal

Initialize one private thumbnail for each eligible NVR camera without changing
the business tables, adding a new database table, configuring the Web Pod, or
changing the existing live and playback flow.

## Data Boundary

- Candidate read: `tb_crm_iot_device` only.
- Candidate rule: `tenant_id`, `category='camera'`,
  `provider='HikVisionNvrChannel'`, `status=1`, `deleted_at is null`.
- Write: existing private OSS only.
- Object key: `nvr-camera-snapshots/{tenant_id}/{camera_id}.jpg`.
- No MySQL writes, DDL, stream URL, JWT, media payload, or detailed upstream
  error is retained.

## Execution

The one-shot runner lists candidates in ascending camera ID. Before it asks the
stream service for a frame, it checks the deterministic OSS key. Existing
objects are skipped by default; `--force` is the explicit and exceptional
override. Captures are strictly serial, every stream request has a 20-second
deadline, and adjacent stream requests are at least two seconds apart. Three
consecutive authorization/WSS failures open the circuit and stop the Job.

The Kubernetes Job has `parallelism: 1`, `completions: 1`, and one fixed name.
Submitting a second Job with that name while it is active is rejected by
Kubernetes. This is the replacement for the discarded database advisory lock.

## Page Read Path

The NVR camera-list API returns a same-origin protected thumbnail route when
the OSS snapshot store is configured. That route repeats normal store-monitor
authorization and opens only the camera's deterministic key. Missing objects
return 404, which the existing UI converts to the neutral thumbnail placeholder.

## Rollout

1. Build and run the runner for `tenant=10001,camera=111`.
2. Confirm the protected image route renders a real JPEG on the 10001 monitor
   page.
3. Run the remaining 10001 cameras.
4. Run all eligible test-environment cameras once the first store is stable.
5. Record only selected/skipped/succeeded/failed totals and failure-code counts.

The ordinary NVR monitor, its authorization service, and existing store scope
checks remain untouched throughout this work.
