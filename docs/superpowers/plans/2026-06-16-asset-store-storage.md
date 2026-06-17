# Asset Store Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move design-plan assets and channel snapshots behind a storage abstraction so company K8s deployments can persist files in Supabase Storage instead of relying on container-local disk.

**Architecture:** Add a shared `internal/assets` package with `AssetStore` implementations for local files and Supabase Storage. Design-plan uploads continue rendering PDFs locally in a temporary work directory, then persist original/preview/thumbnail through `AssetStore`; channel snapshots download remote images and persist through the same abstraction. Existing database columns stay unchanged and store asset keys/paths.

**Tech Stack:** Go standard library HTTP client, existing React frontend, Supabase Storage REST API, PostgreSQL metadata via existing repositories.

---

## Context

- Current design-plan upload code writes `original.pdf`, `preview.png`, and `thumbnail.png` under `UPLOAD_DIR`.
- Current channel snapshot code writes downloaded camera screenshots under `CHANNEL_SNAPSHOT_DIR`.
- In K8s, container-local storage can disappear on restart or redeploy unless `/app/uploads` is backed by PVC.
- Company deployment prefers Supabase Storage bucket `design-plan-assets`.
- Secrets must stay in K8s Secret/env vars and never enter Git.

## Target Environment Variables

Local/default mode:

```bash
ASSET_STORE=local
UPLOAD_DIR=/app/uploads
```

Supabase Storage mode:

```bash
ASSET_STORE=supabase
SUPABASE_URL=https://<project-ref>.supabase.co
SUPABASE_SERVICE_ROLE_KEY=<server-only-service-role-key>
SUPABASE_STORAGE_BUCKET=design-plan-assets
UPLOAD_DIR=/tmp/erzhuang-work
```

`SUPABASE_SERVICE_ROLE_KEY` is server-side only. Do not expose it to frontend `VITE_*` variables.

## Storage Key Convention

Design-plan uploads initially use upload-level keys because `store_id` is not always known at upload time:

```text
uploads/{upload_id}/original.pdf
uploads/{upload_id}/preview.png
uploads/{upload_id}/thumbnail.png
```

Channel snapshots use:

```text
channel-snapshots/{snapshot_name}.jpg
```

Future migration can move assets to `stores/{store_id}/...` after save, but this plan keeps the change minimal and compatible with the current upload-first flow.

## Tasks

### Task 1: Add shared AssetStore package

**Files:**
- Create: `internal/assets/store.go`
- Create: `internal/assets/local.go`
- Create: `internal/assets/supabase.go`
- Create: `internal/assets/store_test.go`

- [ ] **Step 1: Write tests for local storage**

Test expected behavior:

```go
func TestLocalStoreSaveOpenDelete(t *testing.T) {
	store := assets.NewLocalStore(t.TempDir())
	ctx := context.Background()
	err := store.Save(ctx, "uploads/tmp_1/preview.png", strings.NewReader("png"), "image/png")
	requireNoError(t, err)
	reader, contentType, err := store.Open(ctx, "uploads/tmp_1/preview.png")
	requireNoError(t, err)
	defer reader.Close()
	assertEqual(t, "image/png", contentType)
	body, _ := io.ReadAll(reader)
	assertEqual(t, "png", string(body))
	requireNoError(t, store.DeletePrefix(ctx, "uploads/tmp_1/"))
	_, _, err = store.Open(ctx, "uploads/tmp_1/preview.png")
	assertTrue(t, errors.Is(err, assets.ErrNotFound))
}
```

- [ ] **Step 2: Implement `AssetStore` interface**

Interface:

```go
type AssetStore interface {
	Save(ctx context.Context, key string, body io.Reader, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	DeletePrefix(ctx context.Context, prefix string) error
}
```

- [ ] **Step 3: Implement `LocalStore`**

Local store must reject unsafe keys containing `..`, absolute paths, empty parts, or backslashes.

- [ ] **Step 4: Implement `SupabaseStorageStore`**

Use Supabase Storage REST endpoints:

```text
POST/PUT {SUPABASE_URL}/storage/v1/object/{bucket}/{key}
GET      {SUPABASE_URL}/storage/v1/object/{bucket}/{key}
DELETE   {SUPABASE_URL}/storage/v1/object/{bucket}
```

Send `Authorization: Bearer <service-role-key>` and `apikey: <service-role-key>`.

- [ ] **Step 5: Add factory from env**

Function:

```go
func NewStoreFromEnv() (AssetStore, error)
```

Default to local mode for backward compatibility.

### Task 2: Connect design-plan uploads to AssetStore

**Files:**
- Modify: `internal/designplan/uploads.go`
- Modify: `internal/designplan/service.go`
- Modify: `internal/designplan/handler.go`
- Test: `internal/designplan/uploads_test.go`

- [ ] **Step 1: Keep PDF rendering local, persist final assets through AssetStore**

Render into a temporary work directory under `UPLOAD_DIR` or OS temp, then save final files into `AssetStore` keys:

```text
uploads/{upload_id}/original.pdf
uploads/{upload_id}/preview.png
uploads/{upload_id}/thumbnail.png
```

- [ ] **Step 2: Replace `FilePath` serving with stream serving**

Add service method:

```go
OpenUploadAsset(uploadID string, kind UploadAssetKind) (io.ReadCloser, string, error)
```

Handler uses `http.ServeContent` or copies stream with `Content-Type`.

- [ ] **Step 3: Preserve old DB path compatibility**

`parseStoredPath` continues accepting `uploads/{upload_id}/{file}`.

### Task 3: Connect channel snapshots to AssetStore

**Files:**
- Modify: `internal/storespace/snapshots.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/handler.go`
- Test: `internal/storespace/service_test.go`

- [ ] **Step 1: Replace `SnapshotStore` with shared `assets.AssetStore` wrapper**

Saved snapshot key:

```text
channel-snapshots/{name}.jpg
```

Returned API path remains:

```text
/api/store-space/channel-snapshots/{name}
```

- [ ] **Step 2: Stream snapshot assets from AssetStore**

Handler opens `channel-snapshots/{name}` and writes content type.

- [ ] **Step 3: Keep old local URLs compatible**

Existing DB rows with `/api/store-space/channel-snapshots/{name}` should continue to work in local mode.

### Task 4: Wire service startup

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Build one AssetStore at startup**

Call `assets.NewStoreFromEnv()` once. Pass it to design-plan service and store-space service.

- [ ] **Step 2: Fail fast on invalid storage config**

If `ASSET_STORE=supabase` but required env vars are missing, service should log fatal instead of silently writing to memory/local.

### Task 5: Documentation and deployment notes

**Files:**
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/deploy-runbook.md`
- Modify: `docs/technical-architecture-index.md`

- [ ] **Step 1: Record what changed**

Document:

- Storage abstraction package.
- Environment variables.
- Company K8s deployment recommendation.
- Backward compatibility.

- [ ] **Step 2: Add Supabase bucket setup**

Bucket name:

```text
design-plan-assets
```

Recommended private bucket. Go backend reads/writes with service role key.

### Task 6: Verification

Run:

```bash
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...
cd frontend && npm run build
git diff --check
```

Manual local check:

- Upload PDF still returns preview/thumbnail URLs.
- Existing local mode still reads old-style `uploads/{upload_id}/preview.png`.
- Channel snapshot refresh stores a stable backend URL.

