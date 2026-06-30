# Store Space Formalization Design

Date: 2026-06-30

## Goal

Prepare the store-space system for company formal environments while delivering two immediate product changes:

- Add store short name to create/edit flows.
- Add bed split information to confirmed channel mappings for treatment-like spaces.

The work must preserve existing data and keep current company test environment usable during migration planning.

## Scope

In scope for immediate implementation:

- Add `short_name` to stores.
- Add `bed_label` to video channels.
- Keep old data valid when `short_name` or `bed_label` is empty.
- Rename user-facing “生美” wording to “美容室” while keeping the existing internal enum value `beauty`.
- Require or prompt bed split only for `treatment`, `vip_treatment`, and `beauty` channel confirmations.
- Include bed split in channel display, export, and H5 camera naming where channel mapping text is shown.

In scope for planning and documentation:

- MySQL migration plan for formal company test/prod environments.
- Image storage verification plan.
- OSD refresh placeholder and future integration notes.

Out of scope for this pass:

- Full MySQL repository implementation.
- Full image migration away from Supabase Storage.
- OSD refresh implementation.
- A separate normalized bed entity/table.

## Data Model Decisions

### Store Short Name

Add a nullable or default-empty string field:

- Postgres: `stores.short_name text not null default ''`
- MySQL DDL: `tb_stores.short_name varchar(...) not null default ''`
- API JSON: `short_name`
- Frontend model: `shortName`

Existing stores default to empty short name. Search and duplicate matching continue using full store name unless a later product decision says otherwise.

### Channel Bed Split

Add a string field to video channels:

- Postgres: `video_channels.bed_label text not null default ''`
- MySQL DDL: `tb_video_channels.bed_label varchar(...) not null default ''`
- API JSON: `bed_label`
- Frontend model: `bedLabel`

The field is text, not integer. Numeric values such as `1` and `2` are expected, but text values are allowed for real-world exceptions.

Compatibility rule:

- Empty `bed_label` means no split is recorded. Existing mappings remain valid.
- For treatment-like areas, display with bed split when present:
  - `治疗室1`
  - `治疗室6-1`
  - `VIP治疗室2-1`
  - `美容室3-2`

### Beauty Wording

Keep internal value:

```text
beauty
```

Change user-facing label:

```text
生美 -> 美容室
```

This avoids data migration for the enum while matching the new business vocabulary.

## Product Rules

When confirming a channel:

- Business area type remains required for business channels.
- Area number/note remains required according to current rules.
- Bed split is shown for:
  - 治疗室
  - VIP治疗室
  - 美容室
- Bed split is optional. If there is only one bed in the room, operation can leave it empty.
- If the room has two or more beds, operation should fill it.

The UI should phrase the field as guidance rather than a hard validation blocker:

```text
床位拆分
区域内只有一张床可不填；多张床请填写床位编号，例如 1、2。
```

This avoids blocking old data and avoids forcing the system to know total bed count before confirmation.

## API And UI Impact

### Create/Edit Store

- Add `short_name` / `shortName` to payloads and responses.
- Add input field in create and edit modals.
- Show short name in detail header if present. List display can remain full store name for now.

### Channel Confirmation

- Add `bed_label` / `bedLabel` to channel models, confirm payload, mock adapter, and backend confirmation input.
- Preserve bed label when scanning refreshes existing confirmed channels.
- Clear bed label when a channel is unlocked or changed to non-business if that matches current unlock behavior.
- Include bed label in display helpers and channel mapping export.

### H5 Monitor

Camera title should use the same channel display convention:

- No bed split: `治疗室1号`
- With bed split: `治疗室6-1`

H5 should continue to work if `bed_label` is absent from older backend responses.

## MySQL Migration Plan

The formal environment migration is a separate task, but today’s changes should keep it ready:

1. Update `db/mysql_schema_tb.sql` with new columns.
2. Keep image/PDF/snapshot fields as paths or logical keys, not binary data.
3. Use a staged migration:
   - Sample data migration into MySQL test.
   - Backend MySQL compatibility work.
   - Full data migration with short write freeze.
   - Formal test verification.
   - Production migration with rollback point.
4. Preserve IDs and foreign keys.
5. Keep Supabase/Postgres as read-only rollback source until formal MySQL is verified.

## Image Storage Verification Plan

Current design should be treated as:

- Database stores paths/logical keys.
- Image bodies live in asset storage, currently Supabase Storage or local asset store depending on environment.

When operations provides the AI inspection report, verify against:

- `/health` `asset_store`
- `store_design_plans.original_pdf_path`
- `store_design_plans.preview_image_path`
- `store_design_plans.thumbnail_path`
- `channel_snapshots.thumbnail_path`
- `channel_snapshots.full_image_path`
- asset serving code in `internal/assets` and `internal/storespace`

If the report says images are in DB, confirm whether it means binary data or only URL/path strings.

## OSD Refresh Placeholder

Add no OSD code in this pass.

Future implementation should first answer:

- Which provider API is used: Ezviz OpenAPI or Hikvision ISAPI/internal proxy?
- Is refresh per recorder or per channel?
- Is it a read-only sync into our database, or an operation that writes OSD text to devices?
- What should be displayed to operations: last refresh time, success/failure, device/channel errors?

## Testing

Minimum verification for immediate implementation:

- Backend tests for store short name create/update/list/detail.
- Backend tests for channel confirmation with bed label.
- Frontend tests for labels/display helpers.
- `npm run test`
- `npm run build`
- `go test ./...`

Manual verification:

- Create store with short name.
- Edit store short name.
- Confirm treatment channel without bed split.
- Confirm treatment/VIP/beauty channel with bed split.
- Confirm consultation channel and verify bed split is not required.
- Check list/detail/channel mapping/H5 labels remain readable.

## Rollout

- Version bump as bugfix/minor iteration according to project version rules.
- Deploy to company test environment first.
- Do not publish Korea unless explicitly requested.
- Record release in `docs/codex-learning-state.md`.
