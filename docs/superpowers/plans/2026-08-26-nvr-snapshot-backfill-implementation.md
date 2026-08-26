# NVR Snapshot One-Time Backfill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use an isolated, strict-serial backend task to capture one private OSS thumbnail for each eligible NVR camera and display it only to users permitted to view its store.

**Architecture:** A new `nvr-snapshot-backfill` command runs in a temporary Job image, rather than in the normal Web instance. It reads eligible cameras from `tb_crm_*`, obtains a short-lived NVR WSS session, turns RTP/H.265 into a bounded JPEG with ffmpeg, writes private OSS plus the owned snapshot table, then exits. The normal NVR camera list reads a successful new snapshot first, then the existing safe legacy fallback, then the frontend gray placeholder.

**Tech Stack:** Go 1.22, MySQL, OSS through `internal/assets`, NVR authorization HTTP/WSS, RTP/H.265, ffmpeg, K8s Job, existing Go/Vitest tests.

---

## Fixed Boundaries

- No recurring Job, CronJob, browser queue, user-facing maintenance action, or Web write API.
- No write to synchronized business tables `tb_crm_*`.
- No new environment variables, packages, entrypoint, or sidecar in the Web instance.
- One active capture only; 20-second deadline per camera; default is missing-only.
- Never log or persist an authorization value, issued token, full WSS URL, raw media payload, or raw upstream error body.
- The first test operation is one known live camera in tenant `10001`. A failed native capture stops the project before a 44-camera batch.

## File Structure

| Path | Purpose |
| --- | --- |
| `db/nvr_camera_snapshots.sql` | DBA-approved DDL, validation, and rollback SQL. Never executed by Web startup. |
| `internal/nvrsnapshot/models.go` | Stable status/error codes and request/result types. |
| `internal/nvrsnapshot/repository.go` | Candidate and snapshot metadata interface. |
| `internal/nvrsnapshot/mysql_repository.go` | Parameterized `tb_crm_*` reads and owned-table writes. |
| `internal/nvrsnapshot/capture.go` | WSS/RTP/H.265 depacketization and bounded ffmpeg JPEG capture. |
| `internal/nvrsnapshot/service.go` | Serial queue, resume rules, durable result summary. |
| `cmd/nvr-snapshot-backfill/main.go` | Explicit one-shot CLI, no HTTP server. |
| `Dockerfile.nvr-snapshot-backfill` | Runner-only image containing ffmpeg. |
| `deploy/nvr-snapshot-backfill-job.yaml` | Temporary Job template, existing Secret refs only. |
| `internal/nvrmonitor/*`, `internal/app/handler.go` | Authorized thumbnail read preference, no write route. |

### Task 1: Produce And Obtain Approval For The DDL

**Files:**
- Create: `db/nvr_camera_snapshots.sql`
- Modify: `docs/deploy-runbook.md`
- Modify: `docs/decisions.md`

- [ ] **Step 1: Add the owned-table proposal.**

```sql
create table tb_nvr_camera_snapshots (
  id bigint unsigned not null auto_increment,
  tenant_id bigint unsigned not null,
  camera_id bigint unsigned not null,
  status varchar(32) not null,
  object_key varchar(512) not null default '',
  content_type varchar(64) not null default '',
  width int unsigned not null default 0,
  height int unsigned not null default 0,
  byte_size int unsigned not null default 0,
  captured_at datetime(3) null,
  attempted_at datetime(3) not null,
  error_code varchar(64) not null default '',
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  primary key (id),
  unique key uk_nvr_camera_snapshot (tenant_id, camera_id),
  key idx_nvr_snapshot_status_attempted (status, attempted_at),
  key idx_nvr_snapshot_camera (camera_id)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_general_ci;

select tenant_id, camera_id, status, object_key, content_type, width, height,
       byte_size, captured_at, attempted_at, error_code
from tb_nvr_camera_snapshots where tenant_id = 10001 order by camera_id;

-- Only after the Job and the Web preference have been rolled back:
drop table tb_nvr_camera_snapshots;
```

- [ ] **Step 2: Submit it to DBA/operations for test-environment execution.**

The submission explicitly says: Web code must not use `CREATE TABLE IF NOT EXISTS`; the `drop table` rollback leaves OSS objects untouched unless a separate deletion is approved.

- [ ] **Step 3: Commit only the reviewed DDL and runbook.**

```bash
git add db/nvr_camera_snapshots.sql docs/deploy-runbook.md docs/decisions.md
git commit -m "docs: add nvr snapshot backfill ddl proposal"
```

### Task 2: Add Metadata Contracts And Repository Boundary

**Files:**
- Create: `internal/nvrsnapshot/models.go`
- Create: `internal/nvrsnapshot/repository.go`
- Create: `internal/nvrsnapshot/mysql_repository.go`
- Test: `internal/nvrsnapshot/mysql_repository_test.go`

- [ ] **Step 1: Write failing repository tests.**

They must verify the candidate predicate is exactly:

```sql
d.category = 'camera'
and d.provider = 'HikVisionNvrChannel'
and d.status = 1
and d.deleted_at is null
```

They must also prove tenant/camera filters are parameterized, missing-only excludes `succeeded`, `--resume-failed` includes failed records, and upserts never target a `tb_crm_*` table.

- [ ] **Step 2: Confirm the tests fail.**

```bash
go test ./internal/nvrsnapshot -run TestMySQLRepository -count=1
```

Expected: package does not exist.

- [ ] **Step 3: Implement the stable contract.**

```go
type ErrorCode string
const (
  ErrorAuthorizationFailed ErrorCode = "authorization_failed"
  ErrorWSSConnectTimeout ErrorCode = "wss_connect_timeout"
  ErrorMediaTimeout ErrorCode = "media_timeout"
  ErrorDemuxFailed ErrorCode = "demux_failed"
  ErrorDecodeFailed ErrorCode = "decode_failed"
  ErrorThumbnailInvalid ErrorCode = "thumbnail_invalid"
  ErrorOSSUploadFailed ErrorCode = "oss_upload_failed"
  ErrorDatabaseWriteFailed ErrorCode = "database_write_failed"
)
type Candidate struct { TenantID, CameraID int64 }
type Snapshot struct { TenantID, CameraID int64; Status, ObjectKey, ContentType string; Width, Height int; ByteSize int64; ErrorCode ErrorCode }
type Repository interface {
  ListCandidates(context.Context, Selection) ([]Candidate, error)
  UpsertSnapshot(context.Context, Snapshot) error
  GetSucceededSnapshot(context.Context, int64, int64) (Snapshot, error)
}
```

`UpsertSnapshot` uses `insert ... on duplicate key update`; successful writes clear `error_code`, failure writes clear object metadata.

- [ ] **Step 4: Run tests and commit.**

```bash
go test ./internal/nvrsnapshot -run TestMySQLRepository -count=1
git add internal/nvrsnapshot
git commit -m "feat: add nvr snapshot repository"
```

### Task 3: Prove The Native Capture Technical Gate

**Files:**
- Create: `internal/nvrsnapshot/capture.go`
- Test: `internal/nvrsnapshot/capture_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `Dockerfile.nvr-snapshot-backfill`

- [ ] **Step 1: Write synthetic RTP unit tests before live connectivity.**

Tests cover RTP version 2, video payload type `96`, H.265 VPS/SPS/PPS NAL types `32/33/34`, key frames `16..21`, and FU type `49`, based on `frontend/src/vendor/nvr-player/nvr-player.js`. They assert Annex-B output starts each NAL with `00 00 00 01`, malformed packets return `demux_failed`, and no media before deadline returns `media_timeout`.

- [ ] **Step 2: Implement the injectable capture path.**

```go
type StreamAuthorizer interface {
  CreateStreamURL(context.Context, int64, nvrmonitor.StreamSessionRequest) (string, error)
}
type JPEGCapture interface { Capture(context.Context, string) (JPEG, ErrorCode) }
type JPEG struct { Bytes []byte; Width, Height int; ContentType string }
```

Use a Go WSS client with a context deadline. Do not send undocumented socket commands. Accept binary messages only, depacketize to Annex-B, then use a short-lived ffmpeg process:

```text
ffmpeg -hide_banner -loglevel error -f hevc -i pipe:0 -frames:v 1 -vf scale='min(640,iw)':-2 -q:v 5 -f image2 pipe:1
```

Close WebSocket, pipes, and ffmpeg before returning. Reject non-JPEG, zero-size, long edge over `640`, or output over `1 MiB`.

- [ ] **Step 3: Build the runner-only image.**

Its runtime installs `ffmpeg` and CA certificates, starts `/app/nvr-snapshot-backfill`, and does not replace the normal `Dockerfile` or Web image.

- [ ] **Step 4: Run deterministic checks.**

```bash
go test ./internal/nvrsnapshot -run 'TestRTP|TestJPEG|TestCapture' -count=1
docker build -f Dockerfile.nvr-snapshot-backfill -t erzhuang-nvr-snapshot-spike:local .
```

- [ ] **Step 5: Run exactly one temporary test Job.**

Use the already desktop-verified `10001 / camera_id=111` live stream with `--tenant-id 10001 --camera-id 111 --missing-only --timeout-per-camera 20s --concurrency 1`. Success means private valid JPEG, one `succeeded` row, authorized UI display, and sanitized logs. Any other result stops the plan before the 44-camera batch.

- [ ] **Step 6: Commit only after the gate passes.**

```bash
git add go.mod go.sum internal/nvrsnapshot Dockerfile.nvr-snapshot-backfill
git commit -m "feat: capture one nvr snapshot with ffmpeg"
```

### Task 4: Add The Strict-Serial One-Shot Runner

**Files:**
- Create: `internal/nvrsnapshot/service.go`
- Test: `internal/nvrsnapshot/service_test.go`
- Create: `cmd/nvr-snapshot-backfill/main.go`
- Test: `cmd/nvr-snapshot-backfill/main_test.go`
- Create: `deploy/nvr-snapshot-backfill-job.yaml`

- [ ] **Step 1: Write failing queue tests.**

With a blocking fake capture, assert the maximum concurrent invocation is `1`. For `[success, media_timeout, success]`, assert the third camera runs and failure is persisted. Assert `succeeded` is skipped by default and failed rows run only under `--resume-failed`.

- [ ] **Step 2: Implement explicit service options.**

```go
type Options struct {
  TenantID, CameraID int64
  MissingOnly, ResumeFailed bool
  TimeoutPerCamera time.Duration
}
type Summary struct { Selected, Succeeded, Failed, Skipped int; Failures map[ErrorCode]int }
```

Use one `for _, candidate := range candidates` loop. Each iteration uses `context.WithTimeout`, writes one durable success/failure result, cancels, then starts the next. Classified camera failures do not make the process fail; configuration, database connection, and unclassified errors do.

- [ ] **Step 3: Implement the CLI and Job template.**

Flags: `--tenant-id`, `--camera-id` (requires tenant), `--missing-only=true`, `--resume-failed=false`, `--timeout-per-camera=20s`, and `--concurrency=1` (reject all other values). Read existing MySQL/OSS/NVR secret environment names with existing precedence. The Job has `restartPolicy: Never`, `backoffLimit: 0`, `parallelism: 1`, `completions: 1`, TTL, and secret references only; it is not a CronJob.

- [ ] **Step 4: Run tests and commit.**

```bash
go test ./internal/nvrsnapshot ./cmd/nvr-snapshot-backfill -count=1
git add internal/nvrsnapshot cmd/nvr-snapshot-backfill deploy/nvr-snapshot-backfill-job.yaml
git commit -m "feat: add one-shot nvr snapshot backfill runner"
```

### Task 5: Add Authorized Read Preference, Never A Write API

**Files:**
- Modify: `internal/nvrmonitor/models.go`
- Modify: `internal/nvrmonitor/service.go`
- Modify: `internal/nvrmonitor/handler.go`
- Modify: `internal/nvrmonitor/*_test.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/app/handler_test.go`

- [ ] **Step 1: Write failing precedence and authorization tests.**

They prove a valid owned `succeeded` snapshot wins over legacy; legacy applies only in its current unambiguous one-recorder case; missing rows return no thumbnail; an out-of-scope user gets `403`; and no `POST`, `PUT`, `PATCH`, or `DELETE` snapshot route exists.

- [ ] **Step 2: Implement a scope-guarded GET route.**

```text
GET /api/h5/nvr-monitor/orgs/{externalOrgId}/cameras/{cameraId}/thumbnail
```

Reuse `ensureCanViewStore`, confirm the camera belongs to the eligible tenant list, accept only `status='succeeded'`, `content_type='image/jpeg'`, and key `nvr-camera-snapshots/<tenant>/<camera>.jpg`, open OSS server-side, and return `Cache-Control: private, no-store`. Never send a signed URL or object key.

- [ ] **Step 3: Change list output and run backend tests.**

Set `thumbnail_url` to the new GET route only when a valid owned snapshot exists. The frontend remains unchanged and retains its gray empty state.

```bash
go test ./internal/nvrmonitor ./internal/app -count=1
go test ./...
```

- [ ] **Step 4: Commit the read path.**

```bash
git add internal/nvrmonitor internal/app
git commit -m "feat: serve authorized nvr thumbnails"
```

### Task 6: Staged Release, Acceptance, And Production Control

**Files:**
- Modify: `docs/deploy-runbook.md`
- Modify: `docs/codex-learning-state.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: Test-release the Web read path only.**

Run `go test ./...`, `cd frontend && npm test -- --run && npm run build`, then follow the GitLab test branch/pipeline `752` runbook. Verify health, NVR list, authorized thumbnail `200`, unauthorized thumbnail `403`, and no exposed secrets. This does not start a Job.

- [ ] **Step 2: Stage executions behind explicit approvals.**

Order: one test `10001` camera technical gate -> approved serial batch for all `10001` candidates -> separately approved full test batch -> separately approved production DDL/Web release/one-camera run -> separately approved production widening. Record selected/succeeded/skipped/failed counts and stable failure-code distribution each time.

- [ ] **Step 3: Use the production release process only after test acceptance.**

Merge verified code to GitLab `main`, wait for pipeline `771`, deploy with approval, validate the production URL/SSO/role scope, then create a separate temporary production Job with production Secret refs. Test and production object prefixes and databases must never be shared.

- [ ] **Step 4: Roll back without harming business data.**

Delete the temporary Job and roll the Web thumbnail preference back through its tested Git commit. Preserve owned table rows and private OSS objects unless data retention owners separately approve deletion. Never alter `tb_crm_*`, source NVR settings, or user permissions as rollback.

## Plan Self-Review

- The plan covers isolated Job, serial capture, exact ownership boundary, native decoding spike, private OSS, authorization, resume, staged approvals, and rollback.
- It deliberately contains no scheduled capture, normal Web write endpoint, manual user flow, or business-table mutation.
- Browser NVRPlayer success is not proof of native decoding. Task 3 is the stop/go gate; if it fails, report the stable technical failure and wait for an upstream screenshot API or media protocol contract.
