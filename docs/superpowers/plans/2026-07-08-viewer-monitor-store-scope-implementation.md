# Viewer Monitor Store Scope Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add store-level monitor viewing scopes for `viewer` users while keeping ordinary backend browsing open.

**Architecture:** Add a generic `tb_user_resource_scopes` table and expose it through the existing app user store. H5 Monitor stays in `internal/h5monitor`, but receives an authorization callback from `internal/app` so it can filter stores and block direct API access without depending on app internals.

**Tech Stack:** Go 1.22 `net/http`, MySQL, React + TypeScript + Vite, existing project API helpers and CSS.

---

## File Structure

- Modify `db/mysql_governance_schema_tb.sql`: add `tb_user_resource_scopes`.
- Modify `internal/app/auth_users.go`: add scope constants and structs to the auth user model.
- Modify `internal/app/mysql_store.go`: persist and query monitor store scopes.
- Modify `internal/app/memory_store.go`: keep tests and local auth behavior working.
- Modify `internal/app/users_handler.go`: parse and return scope fields.
- Modify `internal/app/authz.go`: expose monitor-scope authorization helpers.
- Modify `internal/app/handler.go`: register candidate-store endpoint and wire H5 authorization.
- Modify `internal/app/handler_test.go`: cover user-scope persistence and authorization.
- Modify `internal/h5monitor/handler.go`: accept an optional authorizer and enforce it before service calls.
- Modify `internal/h5monitor/handler_test.go`: cover store filtering and 403 behavior.
- Modify `frontend/src/api.ts`: add user scope and candidate store types/API.
- Modify `frontend/src/components/UserManagement.tsx`: implement the confirmed D interaction.
- Modify `frontend/src/App.tsx` and `frontend/src/domain/store-detail-navigation.ts`: hide monitor entry for unauthorized viewers based on store detail/list fields.
- Verify frontend API typing through `npm run build`.
- Modify `VERSION`: bump medium version before release.
- Update `work/current-plan.md` after implementation and validation.

## Task 1: Schema and Auth Model

**Files:**
- Modify: `db/mysql_governance_schema_tb.sql`
- Modify: `internal/app/auth_users.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Add a failing schema/source guard test**

Add this test to `internal/app/handler_test.go` near the MySQL auth user tests:

```go
func TestMySQLGovernanceSchemaDefinesUserResourceScopes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "db", "mysql_governance_schema_tb.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"create table if not exists tb_user_resource_scopes",
		"resource_type varchar(32) not null",
		"external_key varchar(128) not null",
		"scope varchar(64) not null",
		"unique key uk_user_resource_scope",
		"key idx_user_scope",
		"key idx_resource_external_scope",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
}
```

If `os`, `path/filepath`, or `strings` are not imported in `internal/app/handler_test.go`, add them.

- [ ] **Step 2: Run the failing test compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
```

Expected: compile fails until imports/schema references are fixed, or the test binary compiles and the test would fail when run in environments where direct Go tests work.

- [ ] **Step 3: Add schema**

Append this table definition to `db/mysql_governance_schema_tb.sql` after the user/role tables:

```sql
create table if not exists tb_user_resource_scopes (
  id bigint primary key auto_increment,
  user_id bigint not null,
  resource_type varchar(32) not null,
  resource_id bigint not null,
  external_key varchar(128) not null,
  scope varchar(64) not null,
  created_at datetime(3) not null default current_timestamp(3),
  updated_at datetime(3) not null default current_timestamp(3) on update current_timestamp(3),
  constraint fk_user_resource_scopes_user foreign key (user_id) references tb_users(id) on delete cascade,
  unique key uk_user_resource_scope (user_id, resource_type, resource_id, scope),
  key idx_user_scope (user_id, resource_type, scope),
  key idx_resource_external_scope (resource_type, external_key, scope)
) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_general_ci;
```

- [ ] **Step 4: Add auth model constants and structs**

In `internal/app/auth_users.go`, add:

```go
const (
	ResourceTypeStore = "store"
	ScopeMonitorView = "monitor:view"
)

type AuthUserResourceScope struct {
	StoreID       int64  `json:"store_id"`
	City          string `json:"city"`
	Name          string `json:"name"`
	ExternalOrgID string `json:"external_org_id"`
}
```

Extend `AuthUserRecord`:

```go
MonitorStoreScopeCount int
MonitorStoreScopes     []AuthUserResourceScope
```

Extend `AuthUserMutation`:

```go
MonitorStoreScopeIDs []int64 `json:"monitor_store_scope_ids"`
```

- [ ] **Step 5: Verify compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
```

Expected: PASS compile.

- [ ] **Step 6: Commit**

```bash
git add db/mysql_governance_schema_tb.sql internal/app/auth_users.go internal/app/handler_test.go
git commit -m "feat: add user monitor scope schema"
```

## Task 2: MySQL Scope Persistence

**Files:**
- Modify: `internal/app/mysql_store.go`
- Modify: `internal/app/auth_users.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Add store interface methods**

In `internal/app/auth_users.go`, extend `AuthUserStore`:

```go
ListMonitorStoreScopeCandidates(ctx context.Context) ([]AuthUserResourceScope, error)
GetUserMonitorStoreScopes(ctx context.Context, userID int64) ([]AuthUserResourceScope, error)
CanUserViewMonitorStore(ctx context.Context, user AuthUserRecord, externalOrgID string) (bool, error)
```

- [ ] **Step 2: Add MySQL helper to sync scopes**

In `internal/app/mysql_store.go`, add helper functions after `setMySQLUserRole`:

```go
func setMySQLUserMonitorScopes(ctx context.Context, tx *sql.Tx, userID int64, role string, storeIDs []int64) error {
	if normalizeRole(role) != RoleViewer {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		delete from tb_user_resource_scopes
		where user_id = ? and resource_type = ? and scope = ?
	`, userID, ResourceTypeStore, ScopeMonitorView); err != nil {
		return err
	}
	if len(storeIDs) == 0 {
		return nil
	}
	for _, storeID := range uniquePositiveInt64s(storeIDs) {
		result, err := tx.ExecContext(ctx, `
			insert into tb_user_resource_scopes (user_id, resource_type, resource_id, external_key, scope)
			select ?, ?, s.id, s.external_org_id, ?
			from tb_stores s
			where s.id = ? and nullif(trim(s.external_org_id), '') is not null
		`, userID, ResourceTypeStore, ScopeMonitorView, storeID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("invalid monitor store scope id: %d", storeID)
		}
	}
	return nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
```

Add `fmt` to imports.

- [ ] **Step 3: Call scope sync in create/update**

In `CreateAuthUser`, after `setMySQLUserRole` and before commit:

```go
if err := setMySQLUserMonitorScopes(ctx, tx, id, input.Role, input.MonitorStoreScopeIDs); err != nil {
	return AuthUserRecord{}, err
}
```

In `UpdateAuthUser`, after `setMySQLUserRole` and before commit:

```go
if err := setMySQLUserMonitorScopes(ctx, tx, id, input.Role, input.MonitorStoreScopeIDs); err != nil {
	return AuthUserRecord{}, err
}
```

- [ ] **Step 4: Add query methods**

In `internal/app/mysql_store.go`, add:

```go
func (s *MySQLStore) ListMonitorStoreScopeCandidates(ctx context.Context) ([]AuthUserResourceScope, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, city, name, external_org_id
		from tb_stores
		where nullif(trim(external_org_id), '') is not null
		order by city, name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthUserResourceScopes(rows)
}

func (s *MySQLStore) GetUserMonitorStoreScopes(ctx context.Context, userID int64) ([]AuthUserResourceScope, error) {
	rows, err := s.db.QueryContext(ctx, `
		select s.id, s.city, s.name, s.external_org_id
		from tb_user_resource_scopes urs
		join tb_stores s on s.id = urs.resource_id
		where urs.user_id = ?
			and urs.resource_type = ?
			and urs.scope = ?
			and nullif(trim(s.external_org_id), '') is not null
		order by s.city, s.name, s.id
	`, userID, ResourceTypeStore, ScopeMonitorView)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthUserResourceScopes(rows)
}

func (s *MySQLStore) CanUserViewMonitorStore(ctx context.Context, user AuthUserRecord, externalOrgID string) (bool, error) {
	if normalizeRole(user.Role) != RoleViewer {
		return true, nil
	}
	orgID := strings.TrimSpace(externalOrgID)
	if orgID == "" {
		return false, nil
	}
	var exists int
	err := s.db.QueryRowContext(ctx, `
		select 1
		from tb_user_resource_scopes
		where user_id = ?
			and resource_type = ?
			and external_key = ?
			and scope = ?
		limit 1
	`, user.ID, ResourceTypeStore, orgID, ScopeMonitorView).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func scanAuthUserResourceScopes(rows *sql.Rows) ([]AuthUserResourceScope, error) {
	scopes := []AuthUserResourceScope{}
	for rows.Next() {
		var scope AuthUserResourceScope
		if err := rows.Scan(&scope.StoreID, &scope.City, &scope.Name, &scope.ExternalOrgID); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}
```

- [ ] **Step 5: Populate scope counts when listing/getting users**

After scanning users in `ListAuthUsers`, query counts:

```go
if err := s.attachMonitorScopeCounts(ctx, users); err != nil {
	return nil, err
}
```

Add:

```go
func (s *MySQLStore) attachMonitorScopeCounts(ctx context.Context, users []AuthUserRecord) error {
	if len(users) == 0 {
		return nil
	}
	counts := map[int64]int{}
	rows, err := s.db.QueryContext(ctx, `
		select user_id, count(*)
		from tb_user_resource_scopes
		where resource_type = ? and scope = ?
		group by user_id
	`, ResourceTypeStore, ScopeMonitorView)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return err
		}
		counts[userID] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range users {
		users[index].MonitorStoreScopeCount = counts[users[index].ID]
	}
	return nil
}
```

For `getAuthUserByID`, after scan:

```go
record.MonitorStoreScopes, err = s.GetUserMonitorStoreScopes(ctx, record.ID)
if err != nil {
	return AuthUserRecord{}, err
}
record.MonitorStoreScopeCount = len(record.MonitorStoreScopes)
return record, nil
```

- [ ] **Step 6: Verify compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
```

Expected: PASS compile.

- [ ] **Step 7: Commit**

```bash
git add internal/app/auth_users.go internal/app/mysql_store.go
git commit -m "feat: persist viewer monitor store scopes"
```

## Task 3: User Management API

**Files:**
- Modify: `internal/app/users_handler.go`
- Modify: `internal/app/memory_store.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Extend response DTOs**

In `internal/app/users_handler.go`, add:

```go
type monitorStoreScopeResponse struct {
	StoreID       int64  `json:"store_id"`
	City          string `json:"city"`
	Name          string `json:"name"`
	ExternalOrgID string `json:"external_org_id"`
}
```

Extend `authUserItemResponse`:

```go
MonitorStoreScopeCount int                         `json:"monitor_store_scope_count"`
MonitorStoreScopes     []monitorStoreScopeResponse `json:"monitor_store_scopes,omitempty"`
```

In `authUserItem`, set:

```go
MonitorStoreScopeCount: user.MonitorStoreScopeCount,
MonitorStoreScopes:     monitorStoreScopesResponse(user.MonitorStoreScopes),
```

Add:

```go
func monitorStoreScopesResponse(scopes []AuthUserResourceScope) []monitorStoreScopeResponse {
	if len(scopes) == 0 {
		return nil
	}
	result := make([]monitorStoreScopeResponse, 0, len(scopes))
	for _, scope := range scopes {
		result = append(result, monitorStoreScopeResponse{
			StoreID:       scope.StoreID,
			City:          scope.City,
			Name:          scope.Name,
			ExternalOrgID: scope.ExternalOrgID,
		})
	}
	return result
}
```

- [ ] **Step 2: Add candidate endpoint**

In `internal/app/users_handler.go`, add:

```go
type monitorStoreScopeCandidatesResponse struct {
	Stores []monitorStoreScopeResponse `json:"stores"`
}

func (h *Handler) listMonitorStoreScopeCandidatesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, PermissionUserManage); !ok {
		return
	}
	stores, err := h.store.ListMonitorStoreScopeCandidates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list monitor store scope candidates failed"})
		return
	}
	writeJSON(w, http.StatusOK, monitorStoreScopeCandidatesResponse{Stores: monitorStoreScopesResponse(stores)})
}
```

Register in `internal/app/handler.go`:

```go
mux.HandleFunc("GET /api/users/monitor-store-scope-candidates", handler.listMonitorStoreScopeCandidatesHandler)
```

Place it before `GET /api/users` to keep route intent clear.

- [ ] **Step 3: Update memory store**

In `internal/app/memory_store.go`, add an in-memory candidate list and scope persistence sufficient for tests:

```go
monitorScopeCandidates []AuthUserResourceScope
monitorScopesByUserID map[int64][]AuthUserResourceScope
```

Initialize in `NewMemoryStore` with at least two stores:

```go
monitorScopeCandidates: []AuthUserResourceScope{
	{StoreID: 30, City: "北京", Name: "北京保利实验室门店", ExternalOrgID: "10030"},
	{StoreID: 19, City: "上海", Name: "新氧青春诊所(上海陆家嘴店)", ExternalOrgID: "10019"},
},
monitorScopesByUserID: map[int64][]AuthUserResourceScope{},
```

Implement the three new interface methods. `CanUserViewMonitorStore` returns true for admin/editor, and for viewer checks `monitorScopesByUserID[user.ID]`.

- [ ] **Step 4: Add handler tests**

Add tests to `internal/app/handler_test.go`:

```go
func TestUserMutationReturnsMonitorStoreScopes(t *testing.T) {
	privateKey := newTestRSAKey(t)
	t.Setenv("SSO_ENABLED", "true")
	t.Setenv("SSO_JWT_PUBLIC_KEY", publicKeyPEM(t, &privateKey.PublicKey))

	store := NewMemoryStore()
	handler := NewHandlerWithServices(store, nil, nil)
	body := strings.NewReader(`{"email":"viewer@example.com","username":"viewer","display_name":"Viewer","role":"viewer","enabled":true,"monitor_store_scope_ids":[30]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", body)
	req.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]any{
			"display":  "沙磊",
			"mail":     "shalei@soyoung.com",
			"username": "shalei",
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "lite.sy.soyoung.com",
	})})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response authUserItemResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.MonitorStoreScopeCount != 1 || len(response.MonitorStoreScopes) != 1 || response.MonitorStoreScopes[0].ExternalOrgID != "10030" {
		t.Fatalf("unexpected scopes: %+v", response)
	}
}
```

- [ ] **Step 5: Verify compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
```

Expected: PASS compile.

- [ ] **Step 6: Commit**

```bash
git add internal/app/users_handler.go internal/app/handler.go internal/app/memory_store.go internal/app/handler_test.go
git commit -m "feat: expose monitor store scopes in users api"
```

## Task 4: H5 Monitor Authorization Hook

**Files:**
- Modify: `internal/app/authz.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/h5monitor/handler.go`
- Test: `internal/h5monitor/handler_test.go`

- [ ] **Step 1: Add h5monitor auth types**

In `internal/h5monitor/handler.go`, add:

```go
type AuthContext struct {
	UserID int64
	Role   string
}

type Authorizer interface {
	CurrentUser(r *http.Request) (AuthContext, error)
	CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error)
	FilterMonitorStores(r *http.Request, stores MonitorStoresResponse) (MonitorStoresResponse, error)
}

var ErrUnauthorized = errors.New("h5monitor: unauthorized")
var ErrForbidden = errors.New("h5monitor: forbidden")
```

Add `errors` to imports.

Extend `Handler`:

```go
authorizer Authorizer
```

Change constructors:

```go
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func NewHandlerWithAuthorizer(service *Service, authorizer Authorizer) *Handler {
	return &Handler{service: service, authorizer: authorizer}
}

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	RegisterRoutesWithAuthorizer(mux, service, nil)
}

func RegisterRoutesWithAuthorizer(mux *http.ServeMux, service *Service, authorizer Authorizer) {
	handler := NewHandlerWithAuthorizer(service, authorizer)
	mux.HandleFunc("GET /api/h5/monitor/stores", handler.getMonitorStores)
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor", handler.getMonitorHome)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/live-url", handler.getLiveURL)
	mux.HandleFunc("GET /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/record-segments", handler.getRecordSegments)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/playback-url", handler.getPlaybackURL)
	mux.HandleFunc("POST /api/h5/orgs/{externalOrgId}/monitor/channels/{channelId}/disable-url", handler.disableURL)
}
```

- [ ] **Step 2: Enforce authorization in H5 handlers**

In `getMonitorStores`, after service result:

```go
if h.authorizer != nil {
	result, err = h.authorizer.FilterMonitorStores(r, result)
	if err != nil {
		writeAuthzError(w, err)
		return
	}
}
```

At the start of `getMonitorHome`, `getLiveURL`, `getRecordSegments`, `getPlaybackURL`, and `disableURL`, after parsing channel id where needed:

```go
if !h.ensureCanViewStore(w, r, r.PathValue("externalOrgId")) {
	return
}
```

Add:

```go
func (h *Handler) ensureCanViewStore(w http.ResponseWriter, r *http.Request, externalOrgID string) bool {
	if h.authorizer == nil {
		return true
	}
	ok, err := h.authorizer.CanViewMonitorStore(r, externalOrgID)
	if err != nil {
		writeAuthzError(w, err)
		return false
	}
	if !ok {
		writeError(w, http.StatusForbidden, "暂无监控访问权限", nil)
		return false
	}
	return true
}

func writeAuthzError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "暂无监控访问权限", nil)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error(), nil)
}
```

- [ ] **Step 3: Implement app authorizer**

In `internal/app/authz.go`, add:

```go
type h5MonitorAuthorizer struct {
	handler *Handler
}

func (a h5MonitorAuthorizer) CurrentUser(r *http.Request) (h5monitor.AuthContext, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return h5monitor.AuthContext{}, h5MonitorAuthError(err)
	}
	return h5monitor.AuthContext{UserID: user.ID, Role: normalizeRole(user.Role)}, nil
}

func (a h5MonitorAuthorizer) CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return false, h5MonitorAuthError(err)
	}
	return a.handler.store.CanUserViewMonitorStore(r.Context(), user, externalOrgID)
}

func (a h5MonitorAuthorizer) FilterMonitorStores(r *http.Request, response h5monitor.MonitorStoresResponse) (h5monitor.MonitorStoresResponse, error) {
	user, err := a.handler.currentAuthUser(r)
	if err != nil {
		return h5monitor.MonitorStoresResponse{}, h5MonitorAuthError(err)
	}
	if normalizeRole(user.Role) != RoleViewer {
		return response, nil
	}
	filtered := h5monitor.MonitorStoresResponse{}
	for _, group := range response.Cities {
		nextGroup := h5monitor.MonitorStoreCityGroup{City: group.City}
		for _, store := range group.Stores {
			ok, err := a.handler.store.CanUserViewMonitorStore(r.Context(), user, store.ExternalOrgID)
			if err != nil {
				return h5monitor.MonitorStoresResponse{}, err
			}
			if ok {
				nextGroup.Stores = append(nextGroup.Stores, store)
			}
		}
		if len(nextGroup.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, nextGroup)
		}
	}
	return filtered, nil
}

func h5MonitorAuthError(err error) error {
	if errors.Is(err, errUnauthorizedAuth) {
		return h5monitor.ErrUnauthorized
	}
	if errors.Is(err, errForbiddenAuth) {
		return h5monitor.ErrForbidden
	}
	return err
}
```

Add imports:

```go
import (
	"errors"

	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
)
```

- [ ] **Step 4: Wire route registration**

In `internal/app/handler.go`, change:

```go
h5monitor.RegisterRoutes(mux, h5MonitorService)
```

to:

```go
h5monitor.RegisterRoutesWithAuthorizer(mux, h5MonitorService, h5MonitorAuthorizer{handler: handler})
```

- [ ] **Step 5: Add H5 handler tests**

In `internal/h5monitor/handler_test.go`, add this fake authorizer near the other fake types:

```go
type fakeAuthorizer struct {
	allowed map[string]bool
}

func (a fakeAuthorizer) CurrentUser(r *http.Request) (AuthContext, error) {
	return AuthContext{UserID: 1, Role: "viewer"}, nil
}

func (a fakeAuthorizer) CanViewMonitorStore(r *http.Request, externalOrgID string) (bool, error) {
	return a.allowed[externalOrgID], nil
}

func (a fakeAuthorizer) FilterMonitorStores(r *http.Request, response MonitorStoresResponse) (MonitorStoresResponse, error) {
	filtered := MonitorStoresResponse{}
	for _, group := range response.Cities {
		nextGroup := MonitorStoreCityGroup{City: group.City}
		for _, store := range group.Stores {
			if a.allowed[store.ExternalOrgID] {
				nextGroup.Stores = append(nextGroup.Stores, store)
			}
		}
		if len(nextGroup.Stores) > 0 {
			filtered.Cities = append(filtered.Cities, nextGroup)
		}
	}
	return filtered, nil
}
```

Add these tests:

```go
func TestH5MonitorAuthorizerFiltersStores(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandlerWithAuthorizer(service, fakeAuthorizer{allowed: map[string]bool{}})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.getMonitorStores(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Cities) != 0 {
		t.Fatalf("expected no visible stores, got %#v", result.Cities)
	}
}

func TestH5MonitorAuthorizerBlocksDirectMonitorHome(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandlerWithAuthorizer(service, fakeAuthorizer{allowed: map[string]bool{}})
	request := httptest.NewRequest(http.MethodGet, "/api/h5/orgs/10030/monitor", nil)
	request.SetPathValue("externalOrgId", "10030")
	response := httptest.NewRecorder()

	handler.getMonitorHome(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want 403", response.Code, response.Body.String())
	}
}
```

- [ ] **Step 6: Verify compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor -o /private/tmp/h5monitor.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
```

Expected: PASS compile.

- [ ] **Step 7: Commit**

```bash
git add internal/app/authz.go internal/app/handler.go internal/h5monitor/handler.go internal/h5monitor/handler_test.go
git commit -m "feat: enforce viewer monitor scopes on h5"
```

## Task 5: Frontend API Types and Mock Data

**Files:**
- Modify: `frontend/src/api.ts`

- [ ] **Step 1: Add types**

In `frontend/src/api.ts`, add:

```ts
export type MonitorStoreScope = {
  storeId: number;
  city: string;
  name: string;
  externalOrgId: string;
};
```

Extend `ManagedUser`:

```ts
monitorStoreScopeCount: number;
monitorStoreScopes: MonitorStoreScope[];
```

Extend `ManagedUserPayload`:

```ts
monitorStoreScopeIds: number[];
```

Extend `BackendManagedUser`:

```ts
monitor_store_scope_count?: number;
monitorStoreScopeCount?: number;
monitor_store_scopes?: BackendMonitorStoreScope[];
monitorStoreScopes?: BackendMonitorStoreScope[];
```

Add:

```ts
type BackendMonitorStoreScope = {
  store_id?: number;
  storeId?: number;
  city?: string;
  name?: string;
  external_org_id?: string;
  externalOrgId?: string;
};
```

- [ ] **Step 2: Add mapping helpers**

Add:

```ts
function mapMonitorStoreScope(scope: BackendMonitorStoreScope): MonitorStoreScope {
  return {
    storeId: Number(scope.store_id ?? scope.storeId ?? 0),
    city: scope.city ?? "",
    name: scope.name ?? "",
    externalOrgId: scope.external_org_id ?? scope.externalOrgId ?? "",
  };
}
```

Update `mapManagedUser`:

```ts
const scopes = (user.monitor_store_scopes ?? user.monitorStoreScopes ?? []).map(mapMonitorStoreScope);
return {
  id: user.id,
  email: user.email,
  username: user.username ?? "",
  displayName: user.display_name ?? user.displayName ?? "",
  role: normalizeManagedRole(user.role),
  enabled: user.enabled ?? false,
  lastLoginAt: user.last_login_at ?? user.lastLoginAt,
  monitorStoreScopeCount: user.monitor_store_scope_count ?? user.monitorStoreScopeCount ?? scopes.length,
  monitorStoreScopes: scopes,
};
```

Update `toManagedUserPayload`:

```ts
monitor_store_scope_ids: payload.monitorStoreScopeIds,
```

- [ ] **Step 3: Add API method**

In `storeSpaceHttpAdapter`, add:

```ts
async listMonitorStoreScopeCandidates(): Promise<MonitorStoreScope[]> {
  const response = await requestJSON<{ stores: BackendMonitorStoreScope[] }>(`${APP_API_BASE}/users/monitor-store-scope-candidates`);
  return (response.stores ?? []).map(mapMonitorStoreScope);
},
```

In exported `storeSpaceApi`, add mock and HTTP behavior:

```ts
async listMonitorStoreScopeCandidates(): Promise<MonitorStoreScope[]> {
  if (API_MODE === "mock") {
    return mockStores
      .filter((store) => store.externalOrgId.trim())
      .map((store) => ({ storeId: store.id, city: store.city, name: store.name, externalOrgId: store.externalOrgId }));
  }
  return storeSpaceHttpAdapter.listMonitorStoreScopeCandidates();
},
```

- [ ] **Step 4: Update mock user create/update**

When creating mock user, include:

```ts
monitorStoreScopeCount: payload.monitorStoreScopeIds.length,
monitorStoreScopes: mockMonitorScopesByIds(payload.monitorStoreScopeIds),
```

When updating mock user, preserve/update similarly.

Add:

```ts
function mockMonitorScopesByIds(ids: number[]): MonitorStoreScope[] {
  const idSet = new Set(ids);
  return mockStores
    .filter((store) => idSet.has(store.id))
    .map((store) => ({ storeId: store.id, city: store.city, name: store.name, externalOrgId: store.externalOrgId }));
}
```

- [ ] **Step 5: Verify API typing through frontend build**

Do not add a separate unit test for private mapper helpers in this task. The API additions are exercised by `UserManagement.tsx` in Task 6 and by the TypeScript build in this task.

- [ ] **Step 6: Verify frontend build**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api.ts
git commit -m "feat: add monitor scope frontend api"
```

## Task 6: User Management D Interaction

**Files:**
- Modify: `frontend/src/components/UserManagement.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Extend form state**

In `UserManagement.tsx`, extend `UserFormState`:

```ts
monitorStoreScopeIds: number[];
```

Update `emptyForm`:

```ts
monitorStoreScopeIds: [],
```

Add local state:

```ts
const [scopeCandidates, setScopeCandidates] = useState<MonitorStoreScope[]>([]);
const [scopeLoading, setScopeLoading] = useState(false);
const [scopeCity, setScopeCity] = useState("全部");
const [scopeQuery, setScopeQuery] = useState("");
```

Import `MonitorStoreScope`.

- [ ] **Step 2: Load candidates**

Add:

```ts
async function ensureScopeCandidatesLoaded() {
  if (scopeCandidates.length > 0 || scopeLoading) return;
  setScopeLoading(true);
  try {
    setScopeCandidates(await storeSpaceApi.listMonitorStoreScopeCandidates());
  } catch (error) {
    onToast(errorMessage(error, "门店范围加载失败。"));
  } finally {
    setScopeLoading(false);
  }
}
```

Call `void ensureScopeCandidatesLoaded()` in `startCreate` and `startEdit`.

- [ ] **Step 3: Populate form on edit**

In `startEdit`, set:

```ts
monitorStoreScopeIds: user.monitorStoreScopes.map((scope) => scope.storeId),
```

In `submitUser`, include:

```ts
monitorStoreScopeIds: form.monitorStoreScopeIds,
```

- [ ] **Step 4: Compute filtered scope rows**

Add:

```ts
const scopeCities = useMemo(() => ["全部", ...Array.from(new Set(scopeCandidates.map((store) => store.city || "未分城市")))], [scopeCandidates]);

const filteredScopeCandidates = useMemo(() => {
  const queryValue = scopeQuery.trim().toLowerCase();
  const rows = scopeCandidates.filter((store) => {
    const city = store.city || "未分城市";
    if (scopeCity !== "全部" && city !== scopeCity) return false;
    if (!queryValue) return true;
    return `${city} ${store.name} ${store.externalOrgId}`.toLowerCase().includes(queryValue);
  });
  if (scopeCity !== "全部") return rows;
  const selected = new Set(form.monitorStoreScopeIds);
  return [...rows].sort((a, b) => Number(selected.has(b.storeId)) - Number(selected.has(a.storeId)));
}, [form.monitorStoreScopeIds, scopeCandidates, scopeCity, scopeQuery]);
```

- [ ] **Step 5: Add scope UI block**

Inside the modal, after the basic form grid, render only for viewer:

```tsx
{form.role === "viewer" ? (
  <section className="user-scope-panel">
    <div className="user-scope-head">
      <div>
        <strong>查看监控门店范围</strong>
        <p>仅控制门店监控入口和 H5 Monitor 访问，不影响普通页面浏览。</p>
      </div>
      <span>已选 {form.monitorStoreScopeIds.length} 家</span>
    </div>
    <div className="user-scope-city-tabs">
      {scopeCities.map((city) => (
        <button key={city} type="button" className={scopeCity === city ? "active" : ""} onClick={() => setScopeCity(city)}>
          {city}
        </button>
      ))}
    </div>
    <div className="user-scope-tools">
      <input value={scopeQuery} onChange={(event) => setScopeQuery(event.target.value)} placeholder="搜索门店名 / 城市 / 机构 ID" />
      <button type="button" onClick={() => selectScopeStores(filteredScopeCandidates.map((store) => store.storeId))}>全选</button>
      <button type="button" onClick={() => setForm((value) => ({ ...value, monitorStoreScopeIds: [] }))}>清空</button>
    </div>
    <div className="user-scope-summary">当前筛选：{scopeCity}，共 {filteredScopeCandidates.length} 家</div>
    <div className="user-scope-list">
      {scopeLoading ? <div className="empty-cell">正在加载门店范围</div> : null}
      {!scopeLoading && filteredScopeCandidates.length === 0 ? <div className="empty-cell">没有匹配门店</div> : null}
      {!scopeLoading
        ? filteredScopeCandidates.map((store) => (
            <label className="user-scope-row" key={store.storeId}>
              <input
                type="checkbox"
                checked={form.monitorStoreScopeIds.includes(store.storeId)}
                onChange={(event) => toggleScopeStore(store.storeId, event.target.checked)}
              />
              <span>{store.name}</span>
              <em>{store.externalOrgId}</em>
            </label>
          ))
        : null}
    </div>
  </section>
) : (
  <section className="user-scope-panel muted">
    <strong>查看监控门店范围</strong>
    <p>当前角色默认全量，不受门店范围限制。已有选择会保留，切回普通查看后恢复。</p>
  </section>
)}
```

Add helper functions:

```ts
function toggleScopeStore(storeId: number, checked: boolean) {
  setForm((value) => {
    const next = new Set(value.monitorStoreScopeIds);
    if (checked) next.add(storeId);
    else next.delete(storeId);
    return { ...value, monitorStoreScopeIds: [...next] };
  });
}

function selectScopeStores(storeIds: number[]) {
  setForm((value) => {
    const next = new Set(value.monitorStoreScopeIds);
    for (const storeId of storeIds) next.add(storeId);
    return { ...value, monitorStoreScopeIds: [...next] };
  });
}
```

- [ ] **Step 6: Add CSS**

In `frontend/src/styles.css`, add compact styles matching the prototype:

```css
.user-scope-panel {
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-top: 16px;
  overflow: hidden;
  background: #fff;
}

.user-scope-panel.muted {
  padding: 14px 16px;
  color: var(--muted);
}

.user-scope-head,
.user-scope-tools,
.user-scope-summary {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.user-scope-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  background: #fbfcfe;
}

.user-scope-head p,
.user-scope-panel.muted p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.user-scope-city-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.user-scope-city-tabs button {
  border-radius: 999px;
  padding: 7px 10px;
}

.user-scope-city-tabs button.active {
  border-color: var(--primary);
  background: var(--primary-soft);
  color: var(--primary);
  font-weight: 700;
}

.user-scope-tools {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto auto;
  gap: 10px;
}

.user-scope-summary {
  color: var(--muted);
  font-size: 13px;
  border-bottom: 0;
}

.user-scope-list {
  max-height: 336px;
  overflow: auto;
  padding: 0 16px 12px;
}

.user-scope-row {
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr) 96px;
  gap: 8px;
  align-items: center;
  min-height: 38px;
  padding: 7px 4px;
  border-bottom: 1px solid var(--border-soft);
}

.user-scope-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-scope-row em {
  color: var(--muted);
  font-style: normal;
  font-size: 12px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
```

Use actual CSS variable names present in `styles.css`; if `--border-soft` does not exist, use the local border color used by tables.

- [ ] **Step 7: Verify frontend build**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 8: Browser check**

Start local preview:

```bash
cd frontend
npm run dev -- --host 127.0.0.1
```

Open the printed local URL, go to 用户管理, and verify:

- viewer role shows the scope panel.
- admin/editor show muted full-access panel.
- city tabs filter stores.
- `全选` selects current filtered rows.
- `清空` clears all selected rows.
- selected rows are first in `全部`.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/api.ts frontend/src/components/UserManagement.tsx frontend/src/styles.css
git commit -m "feat: add viewer monitor scope ui"
```

## Task 7: Store Detail and H5 Frontend Handling

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/domain/store-detail-navigation.ts`
- Modify: `frontend/src/components/H5StoreSwitcher.tsx`
- Modify: `frontend/src/pages/H5Monitor.tsx`
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`

- [ ] **Step 1: Add per-store monitor permission field**

Extend `StoreSummary` and `StoreDetail` mapping with:

```ts
canViewMonitor: boolean;
```

Backend fields should support:

```ts
can_view_monitor?: boolean;
canViewMonitor?: boolean;
```

In mappers, default to existing behavior:

```ts
canViewMonitor: item.can_view_monitor ?? item.canViewMonitor ?? Boolean(externalOrgId),
```

- [ ] **Step 2: Update canOpenH5Monitor**

In `frontend/src/domain/store-detail-navigation.ts`:

```ts
export function canOpenH5Monitor(store: Pick<StoreSummary, "externalOrgId" | "canViewMonitor">): boolean {
  return store.externalOrgId.trim() !== "" && store.canViewMonitor !== false;
}
```

- [ ] **Step 3: Improve H5 403 states**

In `H5StoreSwitcher`, current error message already maps 403 to `暂无访问权限`; keep it.

In `H5Monitor.tsx` and `H5MonitorChannel.tsx`, ensure `H5ApiError` with status 403 shows a clear state and does not spin forever:

```ts
if (err instanceof H5ApiError && err.status === 403) {
  setError("暂无该门店监控访问权限");
  return;
}
```

Use the existing state setters in those pages; do not introduce a new routing system.

- [ ] **Step 4: Verify frontend build**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api.ts frontend/src/App.tsx frontend/src/domain/store-detail-navigation.ts frontend/src/components/H5StoreSwitcher.tsx frontend/src/pages/H5Monitor.tsx frontend/src/pages/H5MonitorChannel.tsx
git commit -m "feat: hide unauthorized monitor entry"
```

## Task 8: Backend Store List Permission Flag

**Files:**
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/handler.go`
- Modify: `internal/app/handler.go`
- Test: `internal/storespace/handler_test.go`
- Test: `internal/app/handler_test.go`

- [ ] **Step 1: Add response fields**

In `internal/storespace/models.go`, extend `Store` and `StoreListItem`:

```go
CanViewMonitor bool `json:"can_view_monitor"`
```

This field means “the current request user may open this store's H5 Monitor entry.” It is a visibility hint only; H5 backend authorization from Task 4 remains the security control.

- [ ] **Step 2: Add storespace context resolver**

In `internal/storespace/handler.go`, add `context` to the import block. Keep existing imports. Then add near the top:

```go
type MonitorVisibilityResolver func(ctx context.Context, externalOrgID string) (bool, error)

type monitorVisibilityResolverContextKey struct{}

func WithMonitorVisibilityResolver(ctx context.Context, resolver MonitorVisibilityResolver) context.Context {
	return context.WithValue(ctx, monitorVisibilityResolverContextKey{}, resolver)
}

func monitorVisibilityResolverFromContext(ctx context.Context) MonitorVisibilityResolver {
	resolver, _ := ctx.Value(monitorVisibilityResolverContextKey{}).(MonitorVisibilityResolver)
	return resolver
}
```

Add helper methods on `Handler`:

```go
func (h *Handler) applyMonitorVisibility(ctx context.Context, store *Store) error {
	if store == nil {
		return nil
	}
	resolver := monitorVisibilityResolverFromContext(ctx)
	if resolver == nil {
		store.CanViewMonitor = strings.TrimSpace(store.ExternalOrgID) != ""
		return nil
	}
	if strings.TrimSpace(store.ExternalOrgID) == "" {
		store.CanViewMonitor = false
		return nil
	}
	ok, err := resolver(ctx, store.ExternalOrgID)
	if err != nil {
		return err
	}
	store.CanViewMonitor = ok
	return nil
}

func (h *Handler) applyListMonitorVisibility(ctx context.Context, result *StoreListResult) error {
	resolver := monitorVisibilityResolverFromContext(ctx)
	for index := range result.Items {
		externalOrgID := strings.TrimSpace(result.Items[index].ExternalOrgID)
		if externalOrgID == "" {
			result.Items[index].CanViewMonitor = false
			continue
		}
		if resolver == nil {
			result.Items[index].CanViewMonitor = true
			continue
		}
		ok, err := resolver(ctx, externalOrgID)
		if err != nil {
			return err
		}
		result.Items[index].CanViewMonitor = ok
	}
	return nil
}
```

- [ ] **Step 3: Apply visibility before responses**

In `listStores`, after `service.ListStores` succeeds and before `writeJSON`:

```go
if err := h.applyListMonitorVisibility(r.Context(), &result); err != nil {
	writeDiagnosticError(w, http.StatusInternalServerError, "resolve monitor visibility failed", "monitor_visibility_failed", "storespace_authz", err.Error(), nil)
	return
}
```

In `getStore`, `getStoreDesignPlanData`, and `getStoreChannelData`, after loading `store` and before `writeJSON`:

```go
if err := h.applyMonitorVisibility(r.Context(), store); err != nil {
	writeDiagnosticError(w, http.StatusInternalServerError, "resolve monitor visibility failed", "monitor_visibility_failed", "storespace_authz", err.Error(), nil)
	return
}
```

- [ ] **Step 4: Inject resolver from app**

In `internal/app/handler.go`, add a read middleware function:

```go
func (h *Handler) monitorVisibilityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resolver := storespace.MonitorVisibilityResolver(func(ctx context.Context, externalOrgID string) (bool, error) {
			user, err := h.currentAuthUser(r)
			if errors.Is(err, errUnauthorizedAuth) || errors.Is(err, errForbiddenAuth) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return h.store.CanUserViewMonitorStore(ctx, user, externalOrgID)
		})
		next(w, r.WithContext(storespace.WithMonitorVisibilityResolver(r.Context(), resolver)))
	}
}
```

Add `context` and `errors` to the import block in `internal/app/handler.go`. Keep existing imports.

Change storespace route registration from:

```go
storespace.RegisterRoutesWithWriteGuard(mux, storeSpaceService, handler.storeWriteGuard)
```

To a new storespace registration function from Step 5:

```go
storespace.RegisterRoutesWithGuards(mux, storeSpaceService, handler.monitorVisibilityMiddleware, handler.storeWriteGuard)
```

- [ ] **Step 5: Add storespace route registration with read guard**

In `internal/storespace/handler.go`, keep existing public functions but route through a new function:

```go
func RegisterRoutesWithWriteGuard(mux *http.ServeMux, service *Service, writeGuard RouteMiddleware) {
	RegisterRoutesWithGuards(mux, service, nil, writeGuard)
}

func RegisterRoutesWithGuards(mux *http.ServeMux, service *Service, readGuard RouteMiddleware, writeGuard RouteMiddleware) {
	handler := NewHandler(service)
	read := func(next http.HandlerFunc) http.HandlerFunc {
		if readGuard == nil {
			return next
		}
		return readGuard(next)
	}
	write := func(next http.HandlerFunc) http.HandlerFunc {
		if writeGuard == nil {
			return read(next)
		}
		return read(writeGuard(next))
	}
	mux.HandleFunc("GET /api/store-space/ezviz-accounts", read(handler.listEzvizAccounts))
	mux.HandleFunc("POST /api/store-space/ezviz-accounts", write(handler.createEzvizAccount))
	mux.HandleFunc("POST /api/store-space/diagnostics/ezviz/live-address", write(handler.getEzvizLiveAddress))
	mux.HandleFunc("GET /api/store-space/stores", read(handler.listStores))
	mux.HandleFunc("POST /api/store-space/stores", write(handler.createStore))
	mux.HandleFunc("POST /api/store-space/stores/check-duplicate", read(handler.checkDuplicate))
	mux.HandleFunc("GET /api/store-space/stores/{id}", read(handler.getStore))
	mux.HandleFunc("PATCH /api/store-space/stores/{id}", write(handler.updateStoreBasicInfo))
	mux.HandleFunc("GET /api/store-space/stores/{id}/design-plan-data", read(handler.getStoreDesignPlanData))
	mux.HandleFunc("GET /api/store-space/stores/{id}/channel-data", read(handler.getStoreChannelData))
	mux.HandleFunc("GET /api/store-space/stores/{id}/channel-mappings/export.xlsx", read(handler.exportChannelMappings))
	mux.HandleFunc("PUT /api/store-space/stores/{id}/design-plan", write(handler.saveDesignPlan))
	mux.HandleFunc("POST /api/store-space/stores/{id}/recorders", write(handler.addRecorder))
	mux.HandleFunc("DELETE /api/store-space/stores/{id}", write(handler.deleteStore))
	mux.HandleFunc("DELETE /api/store-space/recorders/{recorder_id}", write(handler.deleteRecorder))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/scan-channels", write(handler.scanRecorderChannels))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/probe-recognize-channel", write(handler.probeRecognizeChannel))
	mux.HandleFunc("POST /api/store-space/recorders/{recorder_id}/recognize-channels", write(handler.recognizeRecorderChannels))
	mux.HandleFunc("GET /api/store-space/channel-snapshots/{name}", read(handler.getChannelSnapshot))
	mux.HandleFunc("GET /api/store-space/channel-snapshots/{name}/diagnostics", read(handler.getChannelSnapshotDiagnostics))
	mux.HandleFunc("DELETE /api/store-space/channels/{channel_id}", write(handler.deleteChannel))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/recognize", write(handler.recognizeChannel))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/snapshot", write(handler.refreshChannelSnapshot))
	mux.HandleFunc("POST /api/store-space/channels/{channel_id}/unlock", write(handler.unlockChannelForEdit))
	mux.HandleFunc("PUT /api/store-space/channels/{channel_id}/confirmation", write(handler.confirmChannel))
}
```

- [ ] **Step 6: Add storespace handler tests**

In `internal/storespace/handler_test.go`, add a test that registers routes with a read guard and denies one external org ID:

```go
func TestListStoresAppliesMonitorVisibilityResolver(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, ServiceOptions{})
	mux := http.NewServeMux()
	readGuard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			resolver := MonitorVisibilityResolver(func(ctx context.Context, externalOrgID string) (bool, error) {
				return externalOrgID != "10030", nil
			})
			next(w, r.WithContext(WithMonitorVisibilityResolver(r.Context(), resolver)))
		}
	}
	RegisterRoutesWithGuards(mux, service, readGuard, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/store-space/stores?page=1&page_size=100", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response StoreListResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	for _, item := range response.Items {
		if item.ExternalOrgID == "10030" && item.CanViewMonitor {
			t.Fatalf("expected 10030 to be hidden from monitor")
		}
	}
}
```

Use the actual `NewService` signature in this package. If it differs, follow existing tests in this file.

- [ ] **Step 7: Compile**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server
```

Expected: PASS compile.

- [ ] **Step 8: Commit**

```bash
git add internal/storespace/models.go internal/storespace/handler.go internal/storespace/handler_test.go internal/app/handler.go
git commit -m "feat: expose monitor visibility flag"
```

## Task 9: Version, Full Validation, and Release Notes

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: Bump version**

If current `VERSION` is `2.30.25`, update to:

```text
2.31.0
```

This is a medium-version feature within the existing user management/permission module.

- [ ] **Step 2: Run backend compile gates**

Run:

```bash
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/app -o /private/tmp/app.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor -o /private/tmp/h5monitor.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/storespace -o /private/tmp/storespace.test
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/server-check ./cmd/server
```

Expected: all PASS compile.

- [ ] **Step 3: Run frontend build**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 4: Browser validation**

Use the local dev server and verify:

- User management D interaction.
- Role switch preserves selected stores.
- Viewer empty scope can save.
- Viewer selected scope count updates in list.
- Admin/editor unaffected.

- [ ] **Step 5: Update docs**

In `docs/codex-learning-state.md`, add a short entry for:

- feature implemented,
- version,
- validation commands,
- release risk: direct H5 URLs are backend-enforced.

In `work/current-plan.md`, mark implementation tasks done and record next release step.

- [ ] **Step 6: Final commit**

```bash
git add VERSION docs/codex-learning-state.md work/current-plan.md
git commit -m "chore: prepare viewer monitor scope release"
```

## Self-Review

- Spec coverage:
  - Store-level viewer monitor scope: Tasks 1-3.
  - H5 backend 403 and store switch filtering: Task 4.
  - D version user management UI: Task 6.
  - Direct URL/API enforcement: Task 4.
  - Empty scope allowed: Tasks 2, 3, 6.
  - Future scope-oriented model: Task 1 schema.
- Known implementation caution:
  - Store detail monitor button visibility may need an additional storespace response hook. Task 8 explicitly calls out the boundary and requires either implementation or documenting first-release scope before release.
- Placeholder scan:
  - No TBD/TODO placeholders.
  - The only conditional language is a deliberate boundary decision in Task 8, because storespace/app coupling must be checked during implementation.
