# OSS Asset Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move design-plan files and camera snapshot assets from current storage into the company Aliyun OSS bucket while preserving existing frontend/API paths and enabling safe historical migration.

**Architecture:** Keep frontend URLs stable and route all asset access through Go backend proxy APIs. Add an `oss` implementation behind the existing `internal/assets.Store` interface, then migrate historical objects into OSS and record old-key-to-OSS mapping in `tb_asset_objects`. Roll out in stages: provider sample validation, dual-read compatibility, migration backfill, then company environment switch.

**Tech Stack:** Go backend, existing `internal/assets` storage abstraction, Aliyun OSS HTTP API or official Go SDK, PostgreSQL/Supabase source data, MySQL target schema, company K8s Secret/env configuration.

---

## Scope And Ownership

### In Scope

- Design-plan assets:
  - `uploads/{upload_id}/original.pdf`
  - `uploads/{upload_id}/preview.png`
  - `uploads/{upload_id}/thumbnail.png`
- Channel snapshot assets:
  - historical API path form: `/api/store-space/channel-snapshots/{name}`
  - logical key form: `channel-snapshots/{name}`
- OSS provider support:
  - `ASSET_STORE=oss`
  - `OSS_BUCKET`
  - `OSS_ENDPOINT`
  - `OSS_ACCESS_KEY_ID`
  - `OSS_ACCESS_KEY_SECRET`
- Database object mapping:
  - `tb_asset_objects`
  - `tb_asset_access_logs`
  - `tb_channel_snapshots.snapshot_key`
  - `tb_channel_snapshots.snapshot_key_hash`
- Compatibility:
  - Existing frontend/API asset URLs remain unchanged.
  - Backend accepts old logical keys and API paths.
  - Missing migrated object falls back to legacy storage during the gray period.

### Out Of Scope For First Pass

- Direct frontend OSS signed URLs.
- Public OSS bucket.
- Long-retention security audit logs beyond the existing short diagnostic logging plan.
- Deleting legacy Supabase/local objects immediately after migration.

### Security Rules

- Never commit OSS AccessKey ID or Secret.
- Never write OSS credentials to docs, tests, logs, Dockerfile, frontend env, or command history.
- Company environment must inject OSS credentials through K8s Secret or protected runtime environment variables.
- Logs may include provider name, bucket name, logical key hash, and sanitized key prefixes; logs must not include request signatures or secret values.

---

## Target Runtime Configuration

Runtime env only:

```text
ASSET_STORE=oss
OSS_BUCKET=sy-camera-erzhuang-project
OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
OSS_ACCESS_KEY_ID=<from K8s Secret>
OSS_ACCESS_KEY_SECRET=<from K8s Secret>
```

Local sample validation may use temporary shell environment variables. Do not write these values into files.

---

## Target Object Key Convention

For newly written objects after OSS is enabled:

```text
design-plans/{store_id}/{upload_id}/original.pdf
design-plans/{store_id}/{upload_id}/preview.png
design-plans/{store_id}/{upload_id}/thumbnail.png

channel-snapshots/{external_org_id}/{recorder_id}/{channel_id}/{snapshot_name}.jpg
```

Compatibility keys still accepted:

```text
uploads/{upload_id}/original.pdf
uploads/{upload_id}/preview.png
uploads/{upload_id}/thumbnail.png
channel-snapshots/{snapshot_name}.jpg
```

API proxy paths remain stable:

```text
/api/design-plan/uploads/{upload_id}/{asset}
/api/store-space/channel-snapshots/{name}
```

---

## Task 1: Add OSS AssetStore Provider

**Files:**
- Create: `internal/assets/oss.go`
- Modify: `internal/assets/store.go`
- Modify: `internal/assets/store_test.go`

- [ ] **Step 1: Write failing env selection test**

Add this test to `internal/assets/store_test.go`:

```go
func TestNewStoreFromEnvRequiresOSSConfig(t *testing.T) {
	t.Setenv("ASSET_STORE", "oss")
	t.Setenv("OSS_BUCKET", "")
	t.Setenv("OSS_ENDPOINT", "")
	t.Setenv("OSS_ACCESS_KEY_ID", "")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "")

	_, err := NewStoreFromEnv()
	if err == nil {
		t.Fatalf("expected missing oss config error")
	}
	if !strings.Contains(err.Error(), "ASSET_STORE=oss requires") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run focused test and verify it fails**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/assets -run TestNewStoreFromEnvRequiresOSSConfig
```

Expected: FAIL because `ASSET_STORE=oss` is unsupported.

- [ ] **Step 3: Add OSS config type and env selection**

In `internal/assets/store.go`, extend `NewStoreFromEnv()`:

```go
if mode == "oss" {
	bucket := strings.TrimSpace(os.Getenv("OSS_BUCKET"))
	endpoint := strings.TrimSpace(os.Getenv("OSS_ENDPOINT"))
	accessKeyID := strings.TrimSpace(os.Getenv("OSS_ACCESS_KEY_ID"))
	accessKeySecret := strings.TrimSpace(os.Getenv("OSS_ACCESS_KEY_SECRET"))
	if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
		return nil, errors.New("ASSET_STORE=oss requires OSS_BUCKET, OSS_ENDPOINT, OSS_ACCESS_KEY_ID, and OSS_ACCESS_KEY_SECRET")
	}
	return NewOSSStore(OSSConfig{
		Bucket:          bucket,
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
	}), nil
}
```

- [ ] **Step 4: Create minimal OSS store skeleton**

Create `internal/assets/oss.go`:

```go
package assets

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type OSSConfig struct {
	Bucket          string
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	HTTPClient      *http.Client
}

type OSSStore struct {
	bucket          string
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	client          *http.Client
}

func NewOSSStore(config OSSConfig) *OSSStore {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &OSSStore{
		bucket:          strings.Trim(strings.TrimSpace(config.Bucket), "/"),
		endpoint:        strings.TrimRight(strings.TrimSpace(config.Endpoint), "/"),
		accessKeyID:     strings.TrimSpace(config.AccessKeyID),
		accessKeySecret: strings.TrimSpace(config.AccessKeySecret),
		client:          client,
	}
}

func (s *OSSStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	return errors.New("oss save not implemented")
}

func (s *OSSStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("oss open not implemented")
}

func (s *OSSStore) DeletePrefix(ctx context.Context, prefix string) error {
	return errors.New("oss delete prefix not implemented")
}
```

- [ ] **Step 5: Run focused env test and verify it passes**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/assets -run TestNewStoreFromEnvRequiresOSSConfig
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/assets/store.go internal/assets/store_test.go internal/assets/oss.go
git commit -m "feat: add oss asset store configuration"
```

---

## Task 2: Implement OSS Save/Open/Delete With Tests

**Files:**
- Modify: `internal/assets/oss.go`
- Modify: `internal/assets/store_test.go`

- [ ] **Step 1: Write failing OSS HTTP behavior test**

Add a test to `internal/assets/store_test.go` using the existing `roundTripFunc` helper pattern:

```go
func TestOSSStoreSaveOpenDeletePrefix(t *testing.T) {
	var requests []string
	var savedContentType string
	var savedBody string

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing Authorization header")
		}
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/uploads/tmp_1/preview.png":
			savedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			savedBody = string(body)
			return textResponse(r, http.StatusOK, "", "text/plain"), nil
		case r.Method == http.MethodGet && r.URL.Path == "/uploads/tmp_1/preview.png":
			return textResponse(r, http.StatusOK, "oss-png", "image/png"), nil
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.RawQuery, "list-type=2"):
			return textResponse(r, http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Contents><Key>uploads/tmp_1/preview.png</Key></Contents></ListBucketResult>`, "application/xml"), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/uploads/tmp_1/preview.png":
			return textResponse(r, http.StatusNoContent, "", "text/plain"), nil
		default:
			return textResponse(r, http.StatusNotFound, "not found", "text/plain"), nil
		}
	})}

	store := NewOSSStore(OSSConfig{
		Bucket:          "sy-camera-erzhuang-project",
		Endpoint:        "sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		HTTPClient:      client,
	})

	if err := store.Save(context.Background(), "uploads/tmp_1/preview.png", strings.NewReader("png-data"), "image/png"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if savedContentType != "image/png" || savedBody != "png-data" {
		t.Fatalf("unexpected save contentType=%q body=%q", savedContentType, savedBody)
	}

	reader, contentType, err := store.Open(context.Background(), "uploads/tmp_1/preview.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	body, _ := io.ReadAll(reader)
	_ = reader.Close()
	if contentType != "image/png" || string(body) != "oss-png" {
		t.Fatalf("unexpected open contentType=%q body=%q", contentType, string(body))
	}

	if err := store.DeletePrefix(context.Background(), "uploads/tmp_1/"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}

	if strings.Join(requests, "\n") != strings.Join([]string{
		"PUT /uploads/tmp_1/preview.png",
		"GET /uploads/tmp_1/preview.png",
		"GET /",
		"DELETE /uploads/tmp_1/preview.png",
	}, "\n") {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/assets -run TestOSSStoreSaveOpenDeletePrefix
```

Expected: FAIL with `oss save not implemented`.

- [ ] **Step 3: Implement signed OSS requests**

In `internal/assets/oss.go`, implement:

- `Save`: `PUT https://{endpoint}/{key}` with content type.
- `Open`: `GET https://{endpoint}/{key}`.
- `DeletePrefix`: list `GET /?list-type=2&prefix={prefix}` then delete each key.
- Signature: use Aliyun OSS Header V1 signature for simple canonical resource `/{bucket}/{key}` and `/{bucket}/?list-type=2&prefix=...`.
- Map `404` to `ErrNotFound`.
- Do not log secrets.

Implementation may use standard library only. If the official Aliyun SDK is introduced, update `go.mod`, keep the wrapper API unchanged, and add tests that do not require real network.

- [ ] **Step 4: Run assets package tests**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/assets
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/assets/oss.go internal/assets/store_test.go go.mod go.sum
git commit -m "feat: implement oss asset storage"
```

---

## Task 3: Preserve Stable API Paths While Writing New OSS Keys

**Files:**
- Modify: `internal/designplan/uploads.go`
- Modify: `internal/designplan/uploads_test.go`
- Modify: `internal/storespace/snapshots.go`
- Modify: `internal/storespace/service_test.go`

- [ ] **Step 1: Write design-plan key test**

Add a test proving uploaded assets still return existing API/proxy paths but are saved under logical keys that OSS can store. Keep compatibility with existing `uploads/{upload_id}/...` for first pass.

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/designplan -run Upload
```

Expected: PASS after minimal compatibility; no frontend API path changes.

- [ ] **Step 2: Write snapshot key compatibility test**

Add a test proving `SaveRemote()` returns `/api/store-space/channel-snapshots/{name}` while internally storing `channel-snapshots/{name}`.

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/storespace -run Snapshot
```

Expected: PASS.

- [ ] **Step 3: Commit Task 3**

```bash
git add internal/designplan/uploads.go internal/designplan/uploads_test.go internal/storespace/snapshots.go internal/storespace/service_test.go
git commit -m "test: lock stable asset proxy paths"
```

---

## Task 4: Add Asset Object Mapping Repository

**Files:**
- Modify: `db/mysql_governance_schema_tb.sql`
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/store.go`
- Test: `internal/storespace/store_test.go`

- [ ] **Step 1: Confirm `tb_asset_objects` fields**

Ensure this table includes:

```sql
logical_key varchar(1024) not null,
logical_key_hash char(64) not null,
storage_provider varchar(32) not null default 'oss',
bucket varchar(255) not null default '',
file_id varchar(255) not null default '',
proxy_path varchar(1024) not null default '',
content_type varchar(128) not null default '',
size_bytes bigint null,
checksum_sha256 varchar(64) not null default '',
sensitivity varchar(32) not null default 'internal',
owner_entity_type varchar(64) not null default '',
owner_entity_id bigint null,
migration_status varchar(32) not null default 'pending',
migrated_at datetime(3) null
```

If default provider is still `supabase`, change it to `oss` for the new company target.

- [ ] **Step 2: Add store methods for asset objects**

Add repository methods:

```go
type AssetObjectInput struct {
	LogicalKey      string
	StorageProvider string
	Bucket          string
	FileID          string
	ProxyPath       string
	ContentType     string
	SizeBytes       *int64
	ChecksumSHA256  string
	Sensitivity     string
	OwnerEntityType string
	OwnerEntityID   *int64
	MigrationStatus string
}

type AssetObject struct {
	ID int64
	AssetObjectInput
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

Implement:

```go
UpsertAssetObject(ctx context.Context, input AssetObjectInput) (*AssetObject, error)
FindAssetObjectByLogicalKey(ctx context.Context, logicalKey string) (*AssetObject, error)
```

- [ ] **Step 3: Write repository tests**

Test:

- upsert inserts a new object.
- upsert on same logical key updates migration status.
- find by logical key returns the OSS mapping.
- missing key returns a typed not-found result.

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/storespace -run AssetObject
```

Expected: PASS.

- [ ] **Step 4: Commit Task 4**

```bash
git add db/mysql_governance_schema_tb.sql internal/storespace/models.go internal/storespace/store.go internal/storespace/store_test.go
git commit -m "feat: add asset object mapping repository"
```

---

## Task 5: Add Migration Inventory Command

**Files:**
- Create: `cmd/asset-migrate/main.go`
- Create: `internal/assetmigration/inventory.go`
- Create: `internal/assetmigration/inventory_test.go`

- [ ] **Step 1: Write inventory unit test**

Create inventory normalization tests:

```go
func TestNormalizeAssetReferences(t *testing.T) {
	cases := map[string]string{
		"/api/store-space/channel-snapshots/a.jpg": "channel-snapshots/a.jpg",
		"channel-snapshots/a.jpg":                  "channel-snapshots/a.jpg",
		"uploads/tmp_1/preview.png":                "uploads/tmp_1/preview.png",
		"/api/design-plan/uploads/tmp_1/preview":   "uploads/tmp_1/preview.png",
	}
	for input, expected := range cases {
		got, ok := NormalizeLogicalKey(input)
		if !ok || got != expected {
			t.Fatalf("NormalizeLogicalKey(%q)=%q,%v want %q,true", input, got, ok, expected)
		}
	}
}
```

- [ ] **Step 2: Implement inventory normalization**

Implement:

```go
func NormalizeLogicalKey(value string) (string, bool)
func TargetOSSKey(ref AssetReference) string
```

`TargetOSSKey` should preserve old keys for compatibility in the first migration:

```text
uploads/{upload_id}/{file}
channel-snapshots/{name}
```

Do not switch to richer `design-plans/{store_id}/...` until historical migration is complete.

- [ ] **Step 3: Add dry-run command**

`cmd/asset-migrate/main.go` supports:

```bash
go run ./cmd/asset-migrate --dry-run --limit 20
```

Output columns:

```text
source_table,source_id,owner_type,owner_id,logical_key,target_key,proxy_path
```

No writes in dry-run.

- [ ] **Step 4: Commit Task 5**

```bash
git add cmd/asset-migrate/main.go internal/assetmigration/inventory.go internal/assetmigration/inventory_test.go
git commit -m "feat: add asset migration inventory"
```

---

## Task 6: Add Sample Migration Flow

**Files:**
- Modify: `cmd/asset-migrate/main.go`
- Create: `internal/assetmigration/migrate.go`
- Create: `internal/assetmigration/migrate_test.go`
- Modify: `docs/mysql-stage-a-execution-plan.md`

- [ ] **Step 1: Write copy behavior test**

Use fake legacy `assets.Store` and fake OSS `assets.Store`:

- source `Open(logicalKey)` returns bytes and content type.
- target `Save(targetKey)` records payload.
- mapping writer records migration status.

Test successful migration writes `migration_status=migrated`.

- [ ] **Step 2: Implement copy one asset**

Implement:

```go
func CopyAsset(ctx context.Context, source assets.Store, target assets.Store, ref AssetReference, writer MappingWriter) error
```

Behavior:

- open old logical key from legacy store.
- save to OSS target key.
- compute SHA-256 checksum while copying.
- upsert `tb_asset_objects` with provider `oss`, bucket, key, content type, size, checksum, owner, status `migrated`.
- on failure, upsert status `failed` with no secret details.

- [ ] **Step 3: Add command flags**

Support:

```bash
go run ./cmd/asset-migrate --dry-run
go run ./cmd/asset-migrate --limit 10
go run ./cmd/asset-migrate --only-store 10030
go run ./cmd/asset-migrate --apply --limit 10
```

`--apply` is required for writes.

- [ ] **Step 4: Document sample validation**

Update `docs/mysql-stage-a-execution-plan.md` with a section:

```text
OSS sample migration:
1. export env through local shell or K8s Secret, never commit credentials.
2. dry run inventory for store 10030.
3. apply limit 10.
4. request design-plan preview and channel snapshot proxy APIs.
5. inspect tb_asset_objects migration_status.
```

- [ ] **Step 5: Commit Task 6**

```bash
git add cmd/asset-migrate/main.go internal/assetmigration/migrate.go internal/assetmigration/migrate_test.go docs/mysql-stage-a-execution-plan.md
git commit -m "feat: add sample oss asset migration"
```

---

## Task 7: Add Dual-Read Compatibility

**Files:**
- Modify: `internal/assets/store.go`
- Create: `internal/assets/fallback.go`
- Create: `internal/assets/fallback_test.go`
- Modify: `internal/app/handler.go`

- [ ] **Step 1: Write fallback store test**

Test:

- primary OSS returns `ErrNotFound`.
- fallback Supabase/local returns bytes.
- result content type and body come from fallback.

- [ ] **Step 2: Implement fallback store**

Create:

```go
type FallbackStore struct {
	Primary  Store
	Fallback Store
}
```

Behavior:

- `Open`: try primary; if `ErrNotFound`, try fallback.
- `Save`: write primary only.
- `DeletePrefix`: delete primary only in first pass.

- [ ] **Step 3: Wire fallback only during migration**

Use env:

```text
ASSET_STORE=oss
ASSET_FALLBACK_STORE=supabase
```

If fallback env is set, build fallback store from existing Supabase env. Do not default to fallback silently.

- [ ] **Step 4: Commit Task 7**

```bash
git add internal/assets/store.go internal/assets/fallback.go internal/assets/fallback_test.go internal/app/handler.go
git commit -m "feat: add asset store fallback reads"
```

---

## Task 8: Company Environment Switch And Acceptance

**Files:**
- Modify: `docs/deploy-runbook.md`
- Modify: `docs/mysql-migration-acceptance-cases.md`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Add company env checklist**

Document:

```text
ASSET_STORE=oss
OSS_BUCKET=sy-camera-erzhuang-project
OSS_ENDPOINT=sy-camera-erzhuang-project.oss-cn-beijing-internal.aliyuncs.com
OSS_ACCESS_KEY_ID=<K8s Secret>
OSS_ACCESS_KEY_SECRET=<K8s Secret>
```

- [ ] **Step 2: Add acceptance cases**

Acceptance:

- upload new design plan, preview loads.
- recognize design plan using uploaded preview.
- refresh channel snapshot, image loads.
- H5 Monitor thumbnails load.
- Excel export embeds images.
- missing object diagnostics shows `asset_store=oss`, sanitized key, and no secret.
- fallback read works for one intentionally unmigrated legacy object during gray period.

- [ ] **Step 3: Commit Task 8**

```bash
git add docs/deploy-runbook.md docs/mysql-migration-acceptance-cases.md docs/codex-learning-state.md
git commit -m "docs: add oss asset migration acceptance"
```

---

## Rollout Plan

1. Implement and test OSS provider locally with fake HTTP tests.
2. Run a real sample upload using temporary local env variables.
3. Ask ops to add OSS env values to company K8s Secret.
4. Deploy with `ASSET_STORE=oss` in a test/company environment only after sample validation.
5. Run migration dry-run for a small store such as 10030.
6. Apply migration for 10 assets.
7. Verify UI and proxy APIs.
8. Migrate all assets in batches.
9. Keep fallback enabled for one observation window.
10. Disable fallback only after validation SQL and UI sampling pass.

---

## Validation Commands

```bash
cd frontend && npm test
cd frontend && npm run build
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/assets ./internal/assetmigration ./internal/designplan ./internal/storespace
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server
```

Company smoke checks after deploy:

```bash
curl -I -L https://lite.sy.soyoung.com/erzhuang-project/
curl -I -L https://lite.sy.soyoung.com/erzhuang-project/health
```

Authenticated browser checks:

- Upload design plan.
- Open existing design plan preview.
- Refresh one channel snapshot.
- Open H5 Monitor list and channel detail.
- Export channel mapping Excel.

---

## Self-Review

- Spec coverage: covers historical object migration, DB reference/mapping, and read/write path switch.
- Placeholder scan: no `TBD` or unspecified implementation-only tasks remain.
- Type consistency: tasks use existing `assets.Store` interface and add new migration types under `internal/assetmigration`.
- Security check: OSS credentials are represented only as env placeholders; no secrets are included in this plan.
