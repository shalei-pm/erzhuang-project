# Store Space Formalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add store short names and channel bed split metadata while keeping existing data compatible and preparing MySQL/image/OSD follow-up work.

**Architecture:** Extend the existing store-space data model without changing existing enum values. Keep `beauty` as the internal area type and only change display labels to `美容室`; add `short_name` on stores and `bed_label` on video channels with empty-string defaults. Treat `areaType + areaNumber + bedLabel` as a temporary local channel mapping target so it can later be replaced by a company business-system region/bed catalog. Update backend schema/repositories/API first, then frontend forms/display/export/H5, then MySQL DDL and documentation.

**Tech Stack:** Go backend with PostgreSQL repository and memory store, Vite React TypeScript frontend, Supabase/local asset storage, MySQL DDL handoff file.

---

## File Map

- `internal/storespace/models.go` — add `ShortName` and `BedLabel` model/input fields.
- `internal/storespace/schema.go` — add Postgres migration statements for `stores.short_name` and `video_channels.bed_label`.
- `internal/storespace/store.go` — persist/query the two fields in memory and Postgres stores; preserve bed labels during scanner upserts; include bed label in export.
- `internal/storespace/h5_monitor_repository.go` — include `bed_label` in H5 monitor channel data.
- `internal/storespace/service_test.go` — add backend behavior tests for short name and bed label.
- `internal/storespace/handler_test.go` — add API-level JSON/export assertions.
- `frontend/src/api.ts` — add `shortName`/`bedLabel` frontend models, payload mapping, mock behavior, and display labels.
- `frontend/src/components/CreateStoreModal.tsx` — add short name field.
- `frontend/src/components/EditStoreModal.tsx` — add short name field.
- `frontend/src/components/StoreDetail.tsx` — show short name in header metadata if present.
- `frontend/src/components/StoreList.tsx` — rename `生美` table header to `美容室`.
- `frontend/src/components/AreaCardList.tsx` — rename area option label to `美容室`.
- `frontend/src/components/VideoChannelTab.tsx` — add bed split input and display, plus label changes.
- `frontend/src/domain/channel-mapping-target.ts` — create display/visibility helpers for the temporary local mapping target.
- `frontend/src/domain/h5-channel-display.ts` — include bed label in H5 camera titles.
- `frontend/src/domain/h5-types.ts` — add `bed_label` to H5 channel type.
- `frontend/src/api.test.ts` — add frontend display helper tests.
- `db/mysql_schema_tb.sql` — add `short_name` and `bed_label` columns.
- `docs/mysql-migration-handoff.md` — note new columns and migration compatibility.
- `docs/codex-learning-state.md` — record implementation and release.
- `VERSION` — bump version for release.

## Task 1: Backend Model And Schema Fields

**Files:**
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/schema.go`
- Test: `internal/storespace/service_test.go`

- [ ] **Step 1: Add failing model behavior tests**

Add tests to `internal/storespace/service_test.go`:

```go
func TestCreateAndUpdateStoreShortName(t *testing.T) {
	service := NewService(NewMemoryStore())

	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City:               "上海",
		Name:               "新氧青春诊所 上海凯德晶萃店",
		ShortName:          "凯德晶萃",
		DesignPlanUploadID: "upload_123",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.ShortName != "凯德晶萃" {
		t.Fatalf("short name = %q, want 凯德晶萃", store.ShortName)
	}

	updated, err := service.UpdateStoreBasicInfo(context.Background(), store.ID, UpdateStoreBasicInfoInput{
		City:          "上海",
		Name:          store.Name,
		ShortName:     "上海凯德",
		ExternalOrgID: "10047",
	})
	if err != nil {
		t.Fatalf("update store: %v", err)
	}
	if updated.ShortName != "上海凯德" {
		t.Fatalf("updated short name = %q, want 上海凯德", updated.ShortName)
	}

	result, err := service.ListStores(context.Background(), StoreFilters{})
	if err != nil {
		t.Fatalf("list stores: %v", err)
	}
	if result.Items[0].ShortName != "上海凯德" {
		t.Fatalf("list short name = %q, want 上海凯德", result.Items[0].ShortName)
	}
}

func TestConfirmChannelStoresBedLabel(t *testing.T) {
	repo := NewMemoryStore()
	account, err := repo.CreateEzvizAccount(context.Background(), CreateEzvizAccountInput{AccountName: "华东"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithScanner(repo, fakeChannelScanner{
		channels: []ScannedChannel{{ChannelNo: 1, ChannelName: "治疗室6", Active: true}},
	})
	store, err := service.CreateStore(context.Background(), CreateStoreInput{
		City: "上海",
		Name: "床位测试店",
		Recorders: []RecorderInput{
			{EzvizAccountID: account.ID, DeviceCode: "BEDLABEL01"},
		},
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	recorder, err := service.ScanRecorderChannels(context.Background(), store.Recorders[0].ID)
	if err != nil {
		t.Fatalf("scan recorder: %v", err)
	}

	updated, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
		AreaType:   AreaTypeTreatment,
		AreaNumber: "6",
		BedLabel:   "1",
	})
	if err != nil {
		t.Fatalf("confirm channel: %v", err)
	}
	channel := updated.Recorders[0].Channels[0]
	if channel.BedLabel != "1" {
		t.Fatalf("bed label = %q, want 1", channel.BedLabel)
	}
}
```

- [ ] **Step 2: Run backend tests and verify they fail**

Run:

```bash
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run 'TestCreateAndUpdateStoreShortName|TestConfirmChannelStoresBedLabel' -count=1
```

Expected: compile failure because `ShortName` and `BedLabel` fields do not exist.

- [ ] **Step 3: Add model fields**

In `internal/storespace/models.go`, add:

```go
type CreateStoreInput struct {
	City               string          `json:"city"`
	Name               string          `json:"name"`
	ShortName          string          `json:"short_name,omitempty"`
	ExternalOrgID      string          `json:"external_org_id,omitempty"`
	DesignPlanUploadID string          `json:"design_plan_upload_id,omitempty"`
	Recorders          []RecorderInput `json:"recorders,omitempty"`
}

type UpdateStoreBasicInfoInput struct {
	City          string `json:"city"`
	Name          string `json:"name"`
	ShortName     string `json:"short_name,omitempty"`
	ExternalOrgID string `json:"external_org_id,omitempty"`
}

type ChannelConfirmationInput struct {
	Kind       string    `json:"kind,omitempty"`
	AreaType   AreaType  `json:"area_type,omitempty"`
	AreaNumber string    `json:"area_number,omitempty"`
	BedLabel   string    `json:"bed_label,omitempty"`
	AreaNote   string    `json:"area_note,omitempty"`
	SceneType  SceneType `json:"scene_type,omitempty"`
}
```

Add `ShortName` to `Store`, `StoreListItem`, and `DuplicateMatch`:

```go
ShortName string `json:"short_name"`
```

Add `BedLabel` to `Channel`:

```go
BedLabel string `json:"bed_label,omitempty"`
```

- [ ] **Step 4: Add Postgres schema migrations**

In `internal/storespace/schema.go`, after existing store city migration:

```go
`alter table stores add column if not exists short_name text not null default ''`,
```

After `video_channels add column if not exists area_note`:

```go
`alter table video_channels add column if not exists bed_label text not null default ''`,
```

- [ ] **Step 5: Run tests to confirm compile still fails in repository code**

Run the same targeted command. Expected: failures in store repository scans/inserts because fields are not persisted yet.

## Task 2: Backend Persistence And API Mapping

**Files:**
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/h5_monitor_repository.go`
- Test: `internal/storespace/service_test.go`
- Test: `internal/storespace/handler_test.go`

- [ ] **Step 1: Persist short name in memory store**

Update create/update/list helpers in `internal/storespace/store.go`:

```go
store := &Store{
	ID:               s.nextStoreID,
	City:             strings.TrimSpace(input.City),
	Name:             strings.TrimSpace(input.Name),
	ShortName:        strings.TrimSpace(input.ShortName),
	NormalizedName:   NormalizeStoreName(input.Name),
	ExternalOrgID:    strings.TrimSpace(input.ExternalOrgID),
	DesignPlanStatus: designPlanStatus,
	OverallStatus:    OverallStatusPartial,
	CreatedAt:        now,
	UpdatedAt:        now,
}
```

In update:

```go
store.ShortName = strings.TrimSpace(input.ShortName)
```

In `storeListItem`:

```go
ShortName: store.ShortName,
```

- [ ] **Step 2: Persist bed label in memory channel confirmation**

In `MemoryStore.ConfirmChannel`, when non-business:

```go
channel.BedLabel = ""
```

When business:

```go
channel.BedLabel = strings.TrimSpace(input.BedLabel)
```

In scanner upsert logic, preserve `BedLabel` when preserving confirmed mappings:

```go
channel.BedLabel = existing.BedLabel
```

Search for SQL or memory-store branches that preserve `area_note`, `area_type`, `area_number`, `area_id`, or `confirmed_at`; add `bed_label` to those same preservation branches.

- [ ] **Step 3: Persist short name in Postgres create/update/list/detail**

Update `insert into stores` statements to include `short_name`.

Example:

```sql
insert into stores (city, name, short_name, normalized_name, external_org_id, design_plan_status, overall_status)
values ($1, $2, $3, $4, $5, $6, $7)
```

Update store scans:

```sql
select id, city, name, short_name, normalized_name, external_org_id, ...
```

Scan into:

```go
&store.ShortName,
```

Update basic info SQL:

```sql
short_name = $3,
external_org_id = $6,
```

Adjust parameter positions carefully.

- [ ] **Step 4: Persist bed label in Postgres channel queries and confirmation**

Update channel selects to include:

```sql
coalesce(bed_label, '')
```

Scan into:

```go
&channel.BedLabel,
```

Update `ConfirmChannel` business update:

```sql
bed_label = $7,
area_note = '',
...
```

Use:

```go
strings.TrimSpace(input.BedLabel)
```

Update non-business branch:

```sql
bed_label = '',
```

Update `SaveChannelSnapshot`/scanner upsert SQL to preserve or clear `bed_label` consistently with business mapping preservation.

- [ ] **Step 5: Add H5 bed label**

In `internal/storespace/h5_monitor_repository.go`, add `c.bed_label` to the query:

```sql
coalesce(c.bed_label, ''),
```

Add `BedLabel` to `h5monitor.ChannelInfo` in `internal/h5monitor/service.go` or models, and return `bed_label` in `MonitorChannel`.

- [ ] **Step 6: Add API handler JSON tests**

In `internal/storespace/handler_test.go`, add an assertion for create store response:

```go
request := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", bytes.NewBufferString(`{
  "city":"上海",
  "name":"新氧青春诊所 上海凯德晶萃店",
  "short_name":"凯德晶萃",
  "design_plan_upload_id":"upload_123"
}`))
```

Decode `Store` and assert:

```go
if store.ShortName != "凯德晶萃" {
	t.Fatalf("short_name = %q", store.ShortName)
}
```

- [ ] **Step 7: Run backend tests**

Run:

```bash
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -count=1
```

Expected: PASS.

## Task 3: Excel Export And Display Labels

**Files:**
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/service_test.go`
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/components/StoreList.tsx`
- Modify: `frontend/src/components/AreaCardList.tsx`

- [ ] **Step 1: Add failing Excel export assertion**

In `TestExportChannelMappingExcel`, confirm a treatment room with bed label:

```go
if _, err := service.ConfirmChannel(context.Background(), recorder.Channels[0].ID, ChannelConfirmationInput{
	AreaType:   AreaTypeTreatment,
	AreaNumber: "2",
	BedLabel:   "1",
}); err != nil {
	t.Fatalf("confirm treatment: %v", err)
}
```

Expected exported sheet contains:

```go
for _, want := range []string{"治疗室2-1", "美容室"} {
	if !strings.Contains(sheet, want) {
		t.Fatalf("expected exported sheet to contain %q, got %s", want, sheet)
	}
}
```

- [ ] **Step 2: Implement backend label helper**

Add helper near `areaDisplayName`:

```go
func channelAreaDisplayName(areaType AreaType, areaNumber int, bedLabel string) string {
	base := areaDisplayName(areaType, areaNumber)
	bed := strings.TrimSpace(bedLabel)
	if bed == "" {
		return base
	}
	return fmt.Sprintf("%s-%s", strings.ReplaceAll(base, " ", ""), bed)
}
```

Change `areaDisplayName(AreaTypeBeauty)`:

```go
return fmt.Sprintf("美容室 %d", number)
```

Use `channelAreaDisplayName` in Excel export rows for business channels.

- [ ] **Step 3: Update frontend labels**

In `frontend/src/api.ts`, change label maps:

```ts
beauty: "美容室",
```

In `frontend/src/components/StoreList.tsx` table header:

```tsx
<th>美容室</th>
```

In `frontend/src/components/AreaCardList.tsx` option:

```tsx
<option value="beauty">美容室</option>
```

- [ ] **Step 4: Run tests**

Run:

```bash
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace -run TestExportChannelMappingExcel -count=1
cd frontend && npm run test
```

Expected: PASS.

## Task 4: Frontend Store Short Name

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/components/CreateStoreModal.tsx`
- Modify: `frontend/src/components/EditStoreModal.tsx`
- Modify: `frontend/src/components/StoreDetail.tsx`

- [ ] **Step 1: Update frontend API types**

In `StoreSummary`:

```ts
shortName: string;
```

In `CreateStoreSpacePayload` and `UpdateStoreBasicInfoPayload`:

```ts
shortName: string;
```

Backend mapping:

```ts
shortName: store.short_name ?? store.shortName ?? "",
```

Payload mapping:

```ts
short_name: payload.shortName.trim(),
```

- [ ] **Step 2: Add create modal field**

In `CreateStoreModal.tsx`, add state:

```tsx
const [shortName, setShortName] = useState("");
```

Add field after full store name:

```tsx
<label>
  机构简称
  <input value={shortName} onChange={(event) => setShortName(event.target.value)} placeholder="选填，例如 凯德晶萃" />
</label>
```

Submit:

```ts
shortName: shortName.trim(),
```

- [ ] **Step 3: Add edit modal field**

In `EditStoreModal.tsx`:

```tsx
const [shortName, setShortName] = useState(store.shortName ?? "");
```

Add the same input field and submit:

```ts
shortName: shortName.trim(),
```

- [ ] **Step 4: Show short name in detail**

In `StoreDetail.tsx`, add a metadata row or inline secondary text:

```tsx
{store.shortName ? <span>简称：{store.shortName}</span> : null}
```

Use existing header/meta classes.

- [ ] **Step 5: Run frontend build**

Run:

```bash
cd frontend && npm run build
```

Expected: PASS.

## Task 5: Frontend Channel Bed Split

**Files:**
- Modify: `frontend/src/api.ts`
- Create: `frontend/src/domain/channel-mapping-target.ts`
- Modify: `frontend/src/components/VideoChannelTab.tsx`
- Modify: `frontend/src/domain/h5-channel-display.ts`
- Modify: `frontend/src/domain/h5-types.ts`
- Modify: `frontend/src/api.test.ts`

- [ ] **Step 1: Add frontend types and payload**

In `VideoChannel`:

```ts
bedLabel: string;
```

Backend channel mapping:

```ts
bedLabel: channel.bed_label ?? channel.bedLabel ?? "",
```

Confirmation payload:

```ts
bed_label: patch.bedLabel?.trim() || undefined,
```

Mock confirm:

```ts
bedLabel: isBusiness ? String(patch.bedLabel ?? "").trim() : "",
```

- [ ] **Step 2: Add bed input to channel confirmation UI**

Create `frontend/src/domain/channel-mapping-target.ts`:

```ts
import type { AreaType } from "../api";

export type LocalChannelMappingTarget = {
  areaType: AreaType | "";
  areaNumber: string;
  bedLabel?: string;
};

export function shouldShowBedLabel(areaType: AreaType | "") {
  return areaType === "treatment" || areaType === "vip_treatment" || areaType === "beauty";
}

export function areaTypeDisplayLabel(areaType: AreaType | "") {
  const labels: Record<AreaType, string> = {
    treatment: "治疗室",
    vip_treatment: "VIP治疗室",
    consultation: "面诊室",
    beauty: "美容室",
  };
  return areaType ? labels[areaType] : "";
}

export function channelMappingTargetTitle(target: LocalChannelMappingTarget) {
  const label = areaTypeDisplayLabel(target.areaType);
  const number = String(target.areaNumber ?? "").trim();
  const bed = String(target.bedLabel ?? "").trim();
  const base = `${label}${number ? `${number}号` : ""}`;
  return bed ? `${base.replace(/号$/, "")}-${bed}` : base;
}
```

In `VideoChannelTab.tsx`, wherever area type/number are edited, add a field visible when calling `shouldShowBedLabel(draft.areaType ?? "")`:

```ts
shouldShowBedLabel(draft.areaType ?? "")
```

Input:

```tsx
{shouldShowBedLabel(draft.areaType ?? "") ? (
  <label>
    床位拆分
    <input
      value={draft.bedLabel ?? ""}
      onChange={(event) => updateDraft(channel.id, { bedLabel: event.target.value })}
      placeholder="多床位填写，例如 1"
    />
    <span className="field-hint">区域内只有一张床可不填；多张床请填写床位编号。</span>
  </label>
) : null}
```

- [ ] **Step 3: Include bed label in optimistic updates**

Update `confirmedChannelDraft` signature:

```ts
bedLabel: string,
```

Business return:

```ts
bedLabel: isBusiness ? bedLabel.trim() : "",
```

Update call sites to pass draft bed label.

- [ ] **Step 4: Update display helpers**

Replace local string concatenation with `channelMappingTargetTitle`:

```ts
channelMappingTargetTitle({
  areaType: channel.areaType,
  areaNumber: channel.areaNumber,
  bedLabel: channel.bedLabel,
})
```

Use this in channel table display where business area name is shown.

- [ ] **Step 5: Update H5 channel display**

In `frontend/src/domain/h5-types.ts` add:

```ts
bed_label: string;
```

In `h5ChannelDisplayText`, for treatment/beauty categories:

```ts
const bedLabel = channel.bed_label?.trim();
if (bedLabel) return `${baseTitle}-${bedLabel}`;
```

Add test in `frontend/src/api.test.ts`:

```ts
expect(channelMappingTargetTitle({ areaType: "beauty", areaNumber: "3", bedLabel: "2" })).toBe("美容室3-2");

expect(
  h5ChannelDisplayText({
    id: 1,
    channel_no: 12,
    channel_name: "通道12",
    category: "treatment",
    area_type: "treatment",
    scene_type: "",
    area_number: 6,
    area_note: "",
    bed_label: "1",
    thumbnail_url: "",
  }).title,
).toBe("治疗室6-1");
```

- [ ] **Step 6: Run frontend tests**

Run:

```bash
cd frontend && npm run test
cd frontend && npm run build
```

Expected: PASS.

## Task 6: MySQL DDL And Migration Notes

**Files:**
- Modify: `db/mysql_schema_tb.sql`
- Modify: `docs/mysql-migration-handoff.md`

- [ ] **Step 1: Add MySQL columns**

In `tb_stores`:

```sql
short_name varchar(255) not null default '',
```

In `tb_video_channels`:

```sql
bed_label varchar(255) not null default '',
```

- [ ] **Step 2: Update migration handoff**

Add note:

```markdown
2026-06-30 新增字段：

- `tb_stores.short_name`：机构简称，空字符串兼容老数据。
- `tb_video_channels.bed_label`：床位拆分，空字符串兼容老数据。
- `beauty` 仍是内部枚举值，前端展示为“美容室”，迁移时不要改成中文枚举。
- `area_type + area_number + bed_label` 是当前隔离阶段的本地区域/床位映射目标；未来接公司业务系统目录时，应优先新增外部目录 ID 字段，不要把中文展示文案写入这些字段。
```

- [ ] **Step 3: Run diff check**

Run:

```bash
git diff --check
```

Expected: no output.

## Task 7: Final Verification And Release

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Bump version**

Update `VERSION` to the next visible product iteration version:

```text
2.22.0
```

- [ ] **Step 2: Add learning-state entry**

Append to `docs/codex-learning-state.md`:

```markdown
## 2026-06-30 机构简称与床位拆分 2.22.0 开发记录

- 新增机构简称字段，添加/编辑机构可维护。
- 新增通道床位拆分字段，治疗室、VIP治疗室、美容室确认时可填写。
- 生美展示文案改为美容室，内部枚举仍为 beauty。
- MySQL DDL 补齐 short_name 和 bed_label。
- 图片存储和 OSD 刷新保持为后续任务。
```

- [ ] **Step 3: Run full verification**

Run:

```bash
cd frontend && npm run test
cd frontend && npm run build
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...
git diff --check
```

Expected:

- Frontend tests pass.
- Frontend build passes.
- Go tests pass.
- `git diff --check` has no output.

- [ ] **Step 4: Commit**

Run:

```bash
git add VERSION docs/codex-learning-state.md db/mysql_schema_tb.sql docs/mysql-migration-handoff.md internal frontend
git commit -m "feat: add store short names and channel bed labels"
```

- [ ] **Step 5: Publish to company GitLab**

Read `docs/deploy-runbook.md`, then run:

```bash
GIT_ASKPASS=/private/tmp/git-askpass.LmX4so GIT_USERNAME=shalei GIT_PASSWORD='c3yF2WHADKMacuE3Xsui' git push gitlab codex/containerize-single-image
```

If rejected by company hook, inspect the rejection, avoid force push, amend code to satisfy the hook, rerun full verification, and push again.

- [ ] **Step 6: Verify company online**

Poll:

```bash
python3 - <<'PY'
import time, urllib.request, re
root='https://lite.sy.soyoung.com/erzhuang-project/'
health='https://lite.sy.soyoung.com/erzhuang-project/health'
def fetch(url):
    return urllib.request.urlopen(url, timeout=10).read().decode('utf-8', 'replace')
for _ in range(24):
    h=fetch(health).strip()
    html=fetch(root)
    assets=re.findall(r'(?:src|href)="([^"]+)"', html)
    version='version not found'
    for path in assets:
        if not path.endswith('.js'):
            continue
        url=path if path.startswith('http') else 'https://lite.sy.soyoung.com'+path
        body=fetch(url)
        m=re.search(r'2\.22\.0\s*\([^)]*\)', body)
        if m:
            version=m.group(0)
            break
    print(h, version, flush=True)
    if '"status":"ok"' in h and version.startswith('2.22.0'):
        break
    time.sleep(30)
PY
```

Expected:

- `/health` returns status ok, database postgres, asset_store supabase.
- Frontend version is `2.22.0 (container)` or `2.22.0 (<commit>)`.

- [ ] **Step 7: Record release**

Append company release record to `docs/codex-learning-state.md`, commit it:

```bash
git add docs/codex-learning-state.md
git commit -m "docs: record store formalization company release"
git push gitlab codex/containerize-single-image
```

## Self-Review

- Spec coverage:
  - Store short name: Tasks 1, 2, 4.
  - Bed split: Tasks 1, 2, 3, 5.
  - Beauty wording: Tasks 3 and 5.
  - MySQL DDL: Task 6.
  - Image/OSD planning: covered in design doc; no implementation task by scope.
  - Release: Task 7.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency:
  - Backend field name: `ShortName` / `short_name`, `BedLabel` / `bed_label`.
  - Frontend field name: `shortName`, `bedLabel`.
  - Internal `beauty` enum preserved.
