# User Management Permissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add first-version user management and global role permissions for SSO users.

**Architecture:** Keep the current SSO + `tb_users` authorization loop, then extend it with global roles: `admin`, `editor`, and `viewer`. Backend APIs enforce permissions; frontend hides unavailable actions for usability but never relies on hiding alone. DBA专项 will review the role storage direction before implementation starts.

**Tech Stack:** Go HTTP handlers, PostgreSQL/Supabase schema bootstrap, React + TypeScript + Vite frontend, existing `AuthState` model.

---

## Scope

In scope:
- Seed users:
  - `admin`: `shalei@soyoung.com`, `maming@soyoung.com`
  - `editor`: `changwenxia@soyoung.com`, `wangxiaofan@soyoung.com`
  - `viewer`: role reserved, no initial user
- Add user management visible only to admins from the store list home.
- Add backend permission enforcement:
  - `admin`: all access, user management.
  - `editor`: all store/list/detail edit operations, no user management.
  - `viewer`: read-only.
- Keep first version global-only; no per-store scope UI yet.

Out of scope for this pass:
- Per-city/per-store user scope.
- Long-term audit log UI.
- Feishu contact picker.
- Physical deletion of users.

## Ownership And Parallel Work

- 主会话 / 四喜：own final architecture decisions, review subagent work, run full verification, publish only when requested.
- DBA专项：review user/role schema direction and MySQL migration compatibility. Required before Task 2 is implemented.
- Backend worker：Tasks 1-4.
- Frontend worker：Tasks 5-7 after backend API contract is stable.
- QA/review worker：Task 8 after implementation.

## File Structure

Likely backend files:
- `internal/app/auth_users.go`: role constants, permission helpers, user management models.
- `internal/app/postgres_store.go`: seed users and CRUD/list user methods.
- `internal/app/memory_store.go`: in-memory user methods for tests/local.
- `internal/app/auth.go`: auth response permissions if needed.
- `internal/app/handler.go`: register user-management routes and wrap protected routes.
- `internal/app/authz.go`: new focused file for permission extraction and guards.
- `internal/app/users_handler.go`: new focused file for user management HTTP handlers.
- `internal/app/handler_test.go`: auth and user management tests.
- `internal/storespace/handler.go`: wrap edit routes with role guard if guard cannot be applied centrally.
- `internal/designplan/handler.go`: wrap legacy design-plan write routes if still active.

Likely frontend files:
- `frontend/src/domain/auth.ts`: role/permission helpers.
- `frontend/src/api.ts`: user management API client/types.
- `frontend/src/App.tsx`: settings route, settings entry button, role-aware action visibility.
- `frontend/src/components/SystemTopBar.tsx`: optional settings entry via `rightExtra`.
- `frontend/src/components/UserManagement.tsx`: new user management page.
- `frontend/src/components/StoreList.tsx`: hide edit/delete for viewer.
- `frontend/src/components/StoreDetail.tsx`, `DesignPlanTab.tsx`, `VideoChannelTab.tsx`: hide edit controls for viewer.
- `frontend/src/api.test.ts`, `frontend/src/components/*.test.tsx`: permission/helper tests.

Docs:
- `docs/codex-learning-state.md`: development and release records.
- DBA output doc/thread: schema review result.

## Task 0: DBA Gate

**Files:**
- Read-only: `internal/app/postgres_store.go`
- Read-only: `db/mysql_schema_tb.sql`
- Read-only: DBA专项 output thread

**Decision recorded 2026-07-02:**
- First version role storage: keep `tb_users.role`.
- Reason: current code already reads `tb_users.role`, first version only needs global roles, and role tables add migration and implementation weight without immediate product value.
- Migration note: MySQL should keep `tb_users.role varchar(32)` first; future RBAC can seed `tb_roles(admin/editor/viewer)` and backfill `tb_user_roles` from `tb_users.role`.
- Audit note: operation audit can be next stage; first version should still pass actor email/role through backend request context so audit can be added without redesign.
- Backend guard note: user management is `admin`; store/list/detail edits are `admin/editor`; read-only APIs are `admin/editor/viewer`.

- [ ] **Step 1: Wait for DBA专项 recommendation**

Expected recommendation must answer:
- Use `tb_users.role` for first version or introduce role join tables now.
- How to seed four users idempotently without overwriting profile fields.
- MySQL migration path for global roles.

- [ ] **Step 2: Main session accepts or revises DBA recommendation**

Decision record format:

```markdown
Decision:
- First version role storage: <tb_users.role | role tables>
- Reason:
- Migration note:
- Risks:
```

- [ ] **Step 3: Only proceed to Task 1 after decision**

Expected: no code changes before this gate.

## Task 1: Backend Role Helpers And Seed Users

**Files:**
- Modify: `internal/app/auth_users.go`
- Modify: `internal/app/postgres_store.go`
- Modify: `internal/app/memory_store.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Write failing tests for seeded users and permissions**

Add tests near existing auth tests in `internal/app/handler_test.go`:

```go
func TestAuthUserPermissionsForAdminEditorViewer(t *testing.T) {
	tests := []struct {
		role string
		want []string
	}{
		{role: "admin", want: []string{"admin", "store:read", "store:write", "user:manage"}},
		{role: "editor", want: []string{"editor", "store:read", "store:write"}},
		{role: "viewer", want: []string{"viewer", "store:read"}},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			record := AuthUserRecord{Role: tt.role, Enabled: true}
			if got := record.permissions(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("permissions()=%v, want %v", got, tt.want)
			}
		})
	}
}
```

Add `reflect` import if missing.

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/app -run TestAuthUserPermissionsForAdminEditorViewer
```

Expected: fail because current `permissions()` only returns `admin` or role.

- [ ] **Step 3: Implement role constants and permissions**

In `internal/app/auth_users.go`, add:

```go
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"

	PermissionStoreRead  = "store:read"
	PermissionStoreWrite = "store:write"
	PermissionUserManage = "user:manage"
)
```

Replace `permissions()` with:

```go
func (record AuthUserRecord) permissions() []string {
	switch strings.ToLower(strings.TrimSpace(record.Role)) {
	case RoleAdmin:
		return []string{RoleAdmin, PermissionStoreRead, PermissionStoreWrite, PermissionUserManage}
	case RoleEditor:
		return []string{RoleEditor, PermissionStoreRead, PermissionStoreWrite}
	default:
		return []string{RoleViewer, PermissionStoreRead}
	}
}
```

- [ ] **Step 4: Seed initial users idempotently**

In `internal/app/postgres_store.go`, replace the single seed statement with idempotent inserts for four users:

```sql
insert into tb_users (email, username, display_name, role, enabled)
values
	('shalei@soyoung.com', 'shalei', '', 'admin', true),
	('maming@soyoung.com', 'maming', '', 'admin', true),
	('changwenxia@soyoung.com', 'changwenxia', '', 'editor', true),
	('wangxiaofan@soyoung.com', 'wangxiaofan', '', 'editor', true)
on conflict (lower(email)) do nothing
```

If PostgreSQL rejects expression-index conflict targets in this codebase, use four `where not exists` inserts matching the current style.

In `internal/app/memory_store.go`, seed the same four users for local/test.

- [ ] **Step 5: Run tests**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/app
```

Expected: pass or known macOS runtime limitation; if full test execution hits the known `dyld LC_UUID` issue, run `go test -c ./internal/app` and `go build ./cmd/server`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/auth_users.go internal/app/postgres_store.go internal/app/memory_store.go internal/app/handler_test.go
git commit -m "feat: seed global auth roles"
```

## Task 2: Backend User Management API

**Files:**
- Modify: `internal/app/auth_users.go`
- Modify: `internal/app/postgres_store.go`
- Modify: `internal/app/memory_store.go`
- Create: `internal/app/users_handler.go`
- Modify: `internal/app/handler.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Write failing tests for admin-only user list**

Add a handler test:

```go
func TestUserManagementRequiresAdmin(t *testing.T) {
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID: 2, Email: "editor@example.com", Username: "editor", Role: RoleEditor, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), authUserContextKey{}, AuthUserRecord{
		Email: "editor@example.com", Role: RoleEditor, Enabled: true,
	}))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", resp.Code)
	}
}
```

If the project does not yet have request context auth helpers, create them in Task 3 first or keep this test focused on `usersHandler.requireAdmin(record)`.

- [ ] **Step 2: Define user management store interface**

In `internal/app/auth_users.go`, extend `AuthUserStore`:

```go
	ListAuthUsers(ctx context.Context) ([]AuthUserRecord, error)
	CreateAuthUser(ctx context.Context, input AuthUserMutation) (AuthUserRecord, error)
	UpdateAuthUser(ctx context.Context, id int64, input AuthUserMutation) (AuthUserRecord, error)
```

Add:

```go
type AuthUserMutation struct {
	Email       string
	Username    string
	DisplayName string
	Role        string
	Enabled     bool
}
```

Add validation helper:

```go
func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return RoleAdmin
	case RoleEditor:
		return RoleEditor
	default:
		return RoleViewer
	}
}
```

- [ ] **Step 3: Implement Postgres methods**

Use SQL:

```sql
select id, email, username, display_name, feishu_user_id, phone, role, enabled, last_login_at
from tb_users
order by enabled desc, lower(email) asc
```

Create:

```sql
insert into tb_users (email, username, display_name, role, enabled)
values ($1, $2, $3, $4, $5)
returning id, email, username, display_name, feishu_user_id, phone, role, enabled, last_login_at
```

Update:

```sql
update tb_users
set username = $2,
	display_name = $3,
	role = $4,
	enabled = $5,
	updated_at = now()
where id = $1
returning id, email, username, display_name, feishu_user_id, phone, role, enabled, last_login_at
```

Do not update email in first version after creation unless the user explicitly requests editable email.

- [ ] **Step 4: Implement MemoryStore methods**

List sorted by email, create with incremental id, update by id.

- [ ] **Step 5: Create HTTP handlers**

Create `internal/app/users_handler.go` with:

```go
func (h *Handler) listUsersHandler(w http.ResponseWriter, r *http.Request)
func (h *Handler) createUserHandler(w http.ResponseWriter, r *http.Request)
func (h *Handler) updateUserHandler(w http.ResponseWriter, r *http.Request)
```

Routes:

```go
mux.HandleFunc("GET /api/users", handler.listUsersHandler)
mux.HandleFunc("POST /api/users", handler.createUserHandler)
mux.HandleFunc("PUT /api/users/{id}", handler.updateUserHandler)
```

Request JSON:

```json
{
  "email": "maming@soyoung.com",
  "username": "maming",
  "display_name": "",
  "role": "admin",
  "enabled": true
}
```

Response JSON:

```json
{
  "users": [
    {
      "id": 1,
      "email": "shalei@soyoung.com",
      "username": "shalei",
      "display_name": "沙磊",
      "role": "admin",
      "enabled": true,
      "last_login_at": "2026-07-02T10:00:00Z"
    }
  ]
}
```

- [ ] **Step 6: Add admin guard**

Use the auth helper from Task 3. For this task, all `/api/users*` must return 403 unless role is admin.

- [ ] **Step 7: Run tests**

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./internal/app
```

- [ ] **Step 8: Commit**

```bash
git add internal/app/auth_users.go internal/app/postgres_store.go internal/app/memory_store.go internal/app/users_handler.go internal/app/handler.go internal/app/handler_test.go
git commit -m "feat: add auth user management api"
```

## Task 3: Backend Permission Guards For Write APIs

**Files:**
- Create: `internal/app/authz.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/storespace/handler.go`
- Modify: `internal/designplan/handler.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Write failing tests for viewer write denial**

Add tests that call a representative write endpoint as viewer and expect 403:

```go
func TestViewerCannotCreateStoreSpaceStore(t *testing.T) {
	store := NewMemoryStore()
	handler := NewHandlerWithStore(store)

	body := strings.NewReader(`{"name":"只读门店","external_org_id":"10099","design_plan":null,"recorders":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/store-space/stores", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), authUserContextKey{}, AuthUserRecord{
		Email: "viewer@example.com", Role: RoleViewer, Enabled: true,
	}))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", resp.Code)
	}
}
```

- [ ] **Step 2: Implement auth context and guards**

In `internal/app/authz.go`:

```go
type authUserContextKey struct{}

func authUserFromContext(ctx context.Context) (AuthUserRecord, bool) {
	record, ok := ctx.Value(authUserContextKey{}).(AuthUserRecord)
	return record, ok
}

func (h *Handler) requirePermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		record, ok := authUserFromContext(r.Context())
		if !ok {
			if !h.auth.Enabled {
				record = AuthUserRecord{Role: RoleAdmin, Enabled: true}
			} else {
				h.writeUnauthorizedAuth(w)
				return
			}
		}
		if !record.Enabled || !hasPermission(record.permissions(), permission) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "暂无操作权限"})
			return
		}
		next(w, r)
	}
}

func hasPermission(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
```

If current request pipeline does not populate `authUserContextKey`, add middleware that parses SSO cookie once for API requests. Do not trust frontend role input.

- [ ] **Step 3: Apply guards**

Required write APIs:
- `POST /api/store-space/stores`
- `PUT /api/store-space/stores/{id}`
- `DELETE /api/store-space/stores/{id}`
- design plan save/upload/recognize endpoints used by current UI
- recorder add/delete/scan/recognize endpoints
- channel confirm/delete/recognize/refresh endpoints
- AI settings toggle should be admin/editor or admin only; choose admin/editor if it is operational, admin only if it is system setting.

Read APIs stay accessible to admin/editor/viewer.

- [ ] **Step 4: Run backend tests/build**

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./...
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server
```

- [ ] **Step 5: Commit**

```bash
git add internal/app/authz.go internal/app/handler.go internal/storespace/handler.go internal/designplan/handler.go internal/app/handler_test.go
git commit -m "feat: enforce role permissions on write api"
```

## Task 4: Frontend Auth Helpers

**Files:**
- Modify: `frontend/src/domain/auth.ts`
- Modify: `frontend/src/api.test.ts`

- [ ] **Step 1: Write failing tests**

In `frontend/src/api.test.ts` add:

```ts
describe("role permissions", () => {
  it("allows admin and editor to edit stores", () => {
    expect(canEditStores({ permissions: ["admin", "store:write"] })).toBe(true);
    expect(canEditStores({ permissions: ["editor", "store:write"] })).toBe(true);
  });

  it("allows only admin to manage users", () => {
    expect(canManageUsers({ permissions: ["admin", "user:manage"] })).toBe(true);
    expect(canManageUsers({ permissions: ["editor", "store:write"] })).toBe(false);
    expect(canManageUsers({ permissions: ["viewer", "store:read"] })).toBe(false);
  });
});
```

- [ ] **Step 2: Implement helpers**

In `frontend/src/domain/auth.ts`:

```ts
export function hasPermission(auth: Pick<AuthState, "permissions"> | null | undefined, permission: string) {
  return Boolean(auth?.permissions?.includes(permission));
}

export function canManageUsers(auth: Pick<AuthState, "permissions"> | null | undefined) {
  return hasPermission(auth, "user:manage");
}

export function canEditStores(auth: Pick<AuthState, "permissions"> | null | undefined) {
  return hasPermission(auth, "store:write");
}
```

- [ ] **Step 3: Run frontend tests**

```bash
cd frontend && npm test
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/domain/auth.ts frontend/src/api.test.ts
git commit -m "feat: add frontend auth permission helpers"
```

## Task 5: Frontend User Management API Client And Page

**Files:**
- Modify: `frontend/src/api.ts`
- Create: `frontend/src/components/UserManagement.tsx`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/components/UserManagement.test.tsx`

- [ ] **Step 1: Add API types**

In `frontend/src/api.ts`:

```ts
export type AuthUserListItem = {
  id: number;
  email: string;
  username: string;
  display_name: string;
  role: "admin" | "editor" | "viewer";
  enabled: boolean;
  last_login_at?: string | null;
};

export type AuthUserMutationPayload = {
  email?: string;
  username?: string;
  display_name?: string;
  role: "admin" | "editor" | "viewer";
  enabled: boolean;
};
```

Add methods:

```ts
async listAuthUsers(): Promise<{ users: AuthUserListItem[] }> {
  return requestJSON(`${APP_API_BASE}/users`);
}

async createAuthUser(payload: AuthUserMutationPayload): Promise<AuthUserListItem> {
  return requestJSON(`${APP_API_BASE}/users`, { method: "POST", body: JSON.stringify(payload) });
}

async updateAuthUser(id: number, payload: AuthUserMutationPayload): Promise<AuthUserListItem> {
  return requestJSON(`${APP_API_BASE}/users/${id}`, { method: "PUT", body: JSON.stringify(payload) });
}
```

- [ ] **Step 2: Create UserManagement component**

Minimum UI:
- Header: `用户管理`
- Add user form:
  - 企业邮箱 input
  - 展示名 input
  - 角色 select: 管理员 / 编辑运维 / 普通查看
  - 启用 checkbox
  - 保存 button
- Table:
  - 企业邮箱
  - 展示名
  - 角色
  - 状态
  - 最近登录
  - 操作: 编辑 / 禁用或启用

For first version, inline edit is acceptable. Use existing enterprise backend styling; no card-in-card.

- [ ] **Step 3: Add component test**

Use server render as existing component tests do:

```ts
it("renders user management title and role labels", () => {
  const markup = renderToStaticMarkup(createElement(UserManagement, {
    users: [
      { id: 1, email: "maming@soyoung.com", username: "maming", display_name: "", role: "admin", enabled: true },
      { id: 2, email: "changwenxia@soyoung.com", username: "changwenxia", display_name: "", role: "editor", enabled: true },
    ],
    loading: false,
    saving: false,
    onCreateUser: async () => undefined,
    onUpdateUser: async () => undefined,
    onBack: () => undefined,
  }));
  expect(markup).toContain("用户管理");
  expect(markup).toContain("管理员");
  expect(markup).toContain("编辑运维");
});
```

- [ ] **Step 4: Run tests**

```bash
cd frontend && npm test
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api.ts frontend/src/components/UserManagement.tsx frontend/src/components/UserManagement.test.tsx frontend/src/styles.css
git commit -m "feat: add user management page"
```

## Task 6: Frontend Route, Settings Entry, And Viewer Read-Only UI

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/StoreList.tsx`
- Modify: `frontend/src/components/StoreDetail.tsx`
- Modify: `frontend/src/components/DesignPlanTab.tsx`
- Modify: `frontend/src/components/VideoChannelTab.tsx`
- Modify: `frontend/src/components/CreateStoreModal.tsx` if needed
- Modify: `frontend/src/components/EditStoreModal.tsx` if needed
- Test: `frontend/src/api.test.ts` or component tests

- [ ] **Step 1: Add route state**

Add a route like:

```ts
type AppRoute =
  | { name: "list" }
  | { name: "detail"; storeId: number; tab?: StoreDetailTab }
  | { name: "settings-users" };
```

Settings URL can be `/settings/users` under existing base path.

- [ ] **Step 2: Show settings button only for admin**

On store list page, pass `rightExtra` to `SystemTopBar`:

```tsx
{canManageUsers(auth) ? (
  <button type="button" className="secondary-button" onClick={() => setRoute({ name: "settings-users" })}>
    设置
  </button>
) : null}
```

- [ ] **Step 3: Wire UserManagement page**

On route `settings-users`, load users from API, render `UserManagement`, and back to store list.

- [ ] **Step 4: Hide edit controls for viewer**

Compute:

```ts
const canEdit = canEditStores(auth);
```

Pass `canEdit` to store list/detail children.

Expected UI:
- viewer sees no “添加门店”
- viewer sees no edit/delete actions
- viewer sees no design-plan upload/recognize/save controls
- viewer sees no channel scan/recognize/confirm/delete controls
- viewer can still open detail and H5 Monitor if authorized.

- [ ] **Step 5: Keep editor controls visible**

Editor should see same editing UI as admin except settings/user management.

- [ ] **Step 6: Run frontend tests/build**

```bash
cd frontend && npm test
cd frontend && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components frontend/src/domain/auth.ts frontend/src/api.test.ts
git commit -m "feat: gate frontend actions by role"
```

## Task 7: Integration Verification

**Files:**
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Backend verification**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test ./...
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server
```

Expected:
- Tests pass, or known macOS Go test runtime issue is documented.
- Build passes.

- [ ] **Step 2: Frontend verification**

Run:

```bash
cd frontend && npm test
cd frontend && npm run build
```

- [ ] **Step 3: Manual QA checklist**

As admin:
- Store list shows Settings.
- User management opens.
- Can see four seeded users after first login/schema bootstrap.
- Can create a viewer user.
- Can disable/enable a user.

As editor:
- Store list does not show Settings.
- Edit operations are visible.
- User management URL returns forbidden or UI blocks it.

As viewer:
- Store list and detail are readable.
- Edit buttons are hidden.
- Direct POST/PUT/DELETE edit API calls return 403.

Unknown SSO user:
- `/api/auth/me` returns 403.
- Frontend shows no-access page with re-login button.

- [ ] **Step 4: Update learning state**

Append:

```markdown
## 2026-07-02 用户管理与全局权限开发记录

- 实现：
- 验证：
- 已知风险：
- 未发布/发布状态：
```

- [ ] **Step 5: Commit verification docs**

```bash
git add docs/codex-learning-state.md
git commit -m "docs: record user management verification"
```

## Task 8: Review And Publish Gate

**Files:**
- Read: `docs/deploy-runbook.md`
- Read: `docs/codex-learning-state.md`
- Read: `docs/frontend-review-checklist.md`
- Read: `docs/ui-standards.md`

- [ ] **Step 1: Main session code review**

Review:
- No secrets committed.
- No DBA WIP files included.
- User API returns no SSO raw token.
- Viewer write APIs return 403 server-side.
- Settings entry only shown to admin.
- Editor cannot access user management.

- [ ] **Step 2: Ask user before publishing**

Do not publish unless user explicitly says “发布到公司”.

- [ ] **Step 3: If publishing, follow company path**

Run final:

```bash
cd frontend && npm test
cd frontend && npm run build
```

Commit any final changes, then:

```bash
git push origin codex/containerize-single-image
git push gitlab codex/containerize-single-image
```

After push:

```bash
curl -I -L https://lite.sy.soyoung.com/erzhuang-project/health
curl -I -L https://lite.sy.soyoung.com/erzhuang-project/
```

Expected:
- HTTP 200 through APISIX.
- User verifies browser UI after company deploy completes.

## Plan Self-Review

- Spec coverage: covers seeded users, global roles, settings entry, user management, backend enforcement, viewer read-only, editor edit access, DBA gate, and publishing gate.
- Placeholder scan: no `TBD` or unconstrained “do appropriate thing” steps remain; permission API list requires route enumeration during Task 3.
- Type consistency: role names are consistently `admin`, `editor`, `viewer`; permissions are consistently `store:read`, `store:write`, `user:manage`.
- Scope check: per-store scope and audit UI are intentionally excluded from first version.
