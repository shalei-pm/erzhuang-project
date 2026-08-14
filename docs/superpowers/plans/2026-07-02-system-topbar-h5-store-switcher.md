# System Top Bar And H5 Store Switcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a shared system top bar, a clear unauthorized SSO state, and a H5 Monitor store switcher listing stores with effective monitor channels.

**Architecture:** Introduce small shared frontend components for top navigation and H5 store switching, backed by a focused H5 monitor store-list API. Keep authorization filtering on the backend boundary so future `tb_user_store_scopes` can be added without changing the frontend contract.

**Tech Stack:** Go `net/http`, existing `internal/h5monitor` service/repository pattern, React + TypeScript + Vite, Vitest.

---

## File Map

- Create `frontend/src/components/SystemTopBar.tsx`: shared top bar with optional back action and auth/logout display.
- Create `frontend/src/components/H5StoreSwitcher.tsx`: city-grouped H5 monitor store menu.
- Modify `frontend/src/App.tsx`: use `SystemTopBar`, wire unauthorized state, pass auth/logout into H5 routes.
- Modify `frontend/src/pages/H5Monitor.tsx`: load and render store switcher below top bar/page heading.
- Modify `frontend/src/pages/H5MonitorChannel.tsx`: remove local back placement from viewer header after top bar owns return.
- Modify `frontend/src/api-h5.ts`: add `listMonitorStores()`.
- Modify `frontend/src/domain/h5-types.ts`: add monitor store list response types.
- Modify `frontend/src/domain/auth.ts`: keep forbidden/blocking helpers and display-name helper.
- Modify `frontend/src/api.test.ts`: add helper and menu grouping tests.
- Modify `frontend/src/styles.css`: add top bar and switcher styles; adjust old header spacing.
- Modify `internal/h5monitor/models.go`: add store switcher response models.
- Modify `internal/h5monitor/service.go`: add `ListMonitorStores`.
- Modify `internal/h5monitor/handler.go`: add `GET /api/h5/monitor/stores`.
- Modify `internal/h5monitor/handler_test.go`: add service/handler tests for store list.
- Modify `internal/storespace/h5_monitor_repository.go`: add repository method that lists monitor stores with effective channels.
- Modify `docs/codex-learning-state.md` and `VERSION` before release.

Important existing worktree note:

- The 403 unauthorized page changes currently exist as uncommitted frontend edits in `frontend/src/App.tsx`, `frontend/src/api.test.ts`, and `frontend/src/domain/auth.ts`.
- Preserve and include them in Task 1, instead of reverting or overwriting them.
- Do not touch DBA work-in-progress files:
  - `db/mysql_business_schema_patch_tb.sql`
  - `db/mysql_stage_a_cleanup_sample_tb.sql`
  - `db/mysql_stage_a_seed_sample_tb.sql`
  - `docs/mysql-stage-a-execution-plan.md`
  - `docs/mysql-stage-a-preflight-checklist.md`

---

### Task 1: Finalize Unauthorized SSO State

**Files:**
- Modify: `frontend/src/domain/auth.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/api.test.ts`

- [ ] **Step 1: Confirm existing uncommitted 403 code is present**

Run:

```bash
git diff -- frontend/src/domain/auth.ts frontend/src/App.tsx frontend/src/api.test.ts
```

Expected: diff includes `forbidden?: boolean`, `shouldShowForbiddenAccess`, `shouldBlockBusinessData`, 403 handling in `getAuthMe().catch`, and `ForbiddenAccess`.

- [ ] **Step 2: Add or keep the auth helper tests**

Ensure `frontend/src/api.test.ts` contains this test in `describe("auth helpers", ...)`:

```ts
it("blocks business data and shows a forbidden access state for unauthorized sso users", () => {
  const forbiddenAuth = { enabled: true, authenticated: false, forbidden: true };
  expect(shouldShowForbiddenAccess(forbiddenAuth)).toBe(true);
  expect(shouldBlockBusinessData(forbiddenAuth)).toBe(true);
  expect(shouldShowLoginWelcome(forbiddenAuth)).toBe(false);
  expect(shouldBlockBusinessData({ enabled: true, authenticated: true })).toBe(false);
});
```

The import list must include:

```ts
shouldBlockBusinessData,
shouldShowForbiddenAccess,
```

- [ ] **Step 3: Run frontend tests**

Run:

```bash
cd frontend && npm test
```

Expected: tests pass.

- [ ] **Step 4: Commit just the unauthorized state if it is not already committed**

Only if this task's changes are not already committed:

```bash
git add frontend/src/App.tsx frontend/src/api.test.ts frontend/src/domain/auth.ts
git commit -m "feat: show unauthorized sso state"
```

Do not stage DBA files.

---

### Task 2: Add Backend H5 Monitor Store List API

**Files:**
- Modify: `internal/h5monitor/models.go`
- Modify: `internal/h5monitor/service.go`
- Modify: `internal/h5monitor/handler.go`
- Modify: `internal/h5monitor/handler_test.go`
- Modify: `internal/storespace/h5_monitor_repository.go`

- [ ] **Step 1: Write failing handler test**

Add to `internal/h5monitor/handler_test.go`:

```go
func TestMonitorStoresListsCitiesAndStoresWithEffectiveChannels(t *testing.T) {
	service, _ := newFakeService()
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/api/h5/monitor/stores", nil)
	response := httptest.NewRecorder()

	handler.getMonitorStores(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var result MonitorStoresResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Cities) != 1 {
		t.Fatalf("cities = %#v, want one city", result.Cities)
	}
	if result.Cities[0].City != "北京" {
		t.Fatalf("city = %q, want 北京", result.Cities[0].City)
	}
	if len(result.Cities[0].Stores) != 1 {
		t.Fatalf("stores = %#v, want one store", result.Cities[0].Stores)
	}
	store := result.Cities[0].Stores[0]
	if store.ExternalOrgID != "10030" || store.StoreName != "北京测试店" || store.AvailableChannelCount != 6 {
		t.Fatalf("unexpected store: %#v", store)
	}
}
```

Update `fakeRepo` in the same file:

```go
func (r *fakeRepo) ListMonitorStores(ctx context.Context) ([]MonitorStoreInfo, error) {
	if r.store == nil {
		return nil, nil
	}
	return []MonitorStoreInfo{{
		ExternalOrgID:          r.store.ExternalOrgID,
		StoreName:              r.store.Name,
		City:                   r.store.City,
		AvailableChannelCount:  len(r.channels),
	}}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
mkdir -p .cache/go-build .cache/go-tmp
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor
```

Expected: compile fails because `MonitorStoresResponse`, `MonitorStoreInfo`, `ListMonitorStores`, or `getMonitorStores` is undefined.

- [ ] **Step 3: Add response models**

Add to `internal/h5monitor/models.go`:

```go
type MonitorStoreInfo struct {
	ExternalOrgID          string `json:"external_org_id"`
	StoreName              string `json:"store_name"`
	City                   string `json:"city"`
	AvailableChannelCount  int    `json:"available_channel_count"`
}

type MonitorStoreCityGroup struct {
	City   string             `json:"city"`
	Stores []MonitorStoreInfo `json:"stores"`
}

type MonitorStoresResponse struct {
	Cities []MonitorStoreCityGroup `json:"cities"`
}
```

- [ ] **Step 4: Extend repository interface and service**

Modify `internal/h5monitor/service.go` `StoreRepository`:

```go
type StoreRepository interface {
	GetStoreByExternalOrgID(ctx context.Context, externalOrgID string) (*StoreInfo, error)
	ListActiveChannelsByOrgID(ctx context.Context, externalOrgID string) ([]ChannelInfo, error)
	GetChannelByID(ctx context.Context, channelID int64) (*ChannelInfo, error)
	ListMonitorStores(ctx context.Context) ([]MonitorStoreInfo, error)
}
```

Add this service method:

```go
func (s *Service) ListMonitorStores(ctx context.Context) (MonitorStoresResponse, error) {
	stores, err := s.repo.ListMonitorStores(ctx)
	if err != nil {
		return MonitorStoresResponse{}, err
	}
	cityMap := map[string][]MonitorStoreInfo{}
	for _, store := range stores {
		city := strings.TrimSpace(store.City)
		if city == "" {
			city = "未分组"
		}
		cityMap[city] = append(cityMap[city], store)
	}
	cities := make([]string, 0, len(cityMap))
	for city := range cityMap {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	response := MonitorStoresResponse{Cities: make([]MonitorStoreCityGroup, 0, len(cities))}
	for _, city := range cities {
		stores := cityMap[city]
		sort.Slice(stores, func(i, j int) bool {
			if stores[i].StoreName == stores[j].StoreName {
				return stores[i].ExternalOrgID < stores[j].ExternalOrgID
			}
			return stores[i].StoreName < stores[j].StoreName
		})
		response.Cities = append(response.Cities, MonitorStoreCityGroup{City: city, Stores: stores})
	}
	return response, nil
}
```

- [ ] **Step 5: Add handler and route**

Modify `internal/h5monitor/handler.go` `RegisterRoutes`:

```go
mux.HandleFunc("GET /api/h5/monitor/stores", handler.getMonitorStores)
```

Add:

```go
func (h *Handler) getMonitorStores(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListMonitorStores(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 6: Add Postgres repository query**

Add to `internal/storespace/h5_monitor_repository.go`:

```go
func (r *H5MonitorRepository) ListMonitorStores(ctx context.Context) ([]h5monitor.MonitorStoreInfo, error) {
	rows, err := r.store.db.QueryContext(ctx, `
		select
			s.external_org_id,
			s.name,
			s.city,
			count(c.id)::int
		from stores s
		join video_recorders r on r.store_id = s.id
		join video_channels c on c.recorder_id = r.id
		where trim(s.external_org_id) <> ''
			and c.is_active
			and r.device_code <> ''
			and r.ezviz_account_id is not null
		group by s.external_org_id, s.name, s.city
		having count(c.id) > 0
		order by s.city, s.name, s.external_org_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []h5monitor.MonitorStoreInfo{}
	for rows.Next() {
		var store h5monitor.MonitorStoreInfo
		if err := rows.Scan(&store.ExternalOrgID, &store.StoreName, &store.City, &store.AvailableChannelCount); err != nil {
			return nil, err
		}
		result = append(result, store)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
```

This query intentionally does not require `confirmed_business` or `confirmed_non_business`.

- [ ] **Step 7: Run backend compile checks**

Run:

```bash
./.tools/go/bin/gofmt -w internal/h5monitor/models.go internal/h5monitor/service.go internal/h5monitor/handler.go internal/h5monitor/handler_test.go internal/storespace/h5_monitor_repository.go
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server
```

Expected: both commands pass. If running the compiled Go tests hits the known macOS `dyld missing LC_UUID`, record it and rely on compile-level verification.

- [ ] **Step 8: Commit backend API**

```bash
git add internal/h5monitor/models.go internal/h5monitor/service.go internal/h5monitor/handler.go internal/h5monitor/handler_test.go internal/storespace/h5_monitor_repository.go
git commit -m "feat: add h5 monitor store list api"
```

---

### Task 3: Add Frontend H5 Store Switcher Data Layer

**Files:**
- Modify: `frontend/src/domain/h5-types.ts`
- Modify: `frontend/src/api-h5.ts`
- Modify: `frontend/src/api.test.ts`

- [ ] **Step 1: Add frontend types**

Add to `frontend/src/domain/h5-types.ts`:

```ts
export interface H5MonitorStoreItem {
  external_org_id: string;
  store_name: string;
  city: string;
  available_channel_count: number;
}

export interface H5MonitorStoreCityGroup {
  city: string;
  stores: H5MonitorStoreItem[];
}

export interface H5MonitorStoresResponse {
  cities: H5MonitorStoreCityGroup[];
}
```

- [ ] **Step 2: Add API method**

Modify `frontend/src/api-h5.ts` imports to include `H5MonitorStoresResponse`.

Add to `h5Api`:

```ts
async listMonitorStores(): Promise<H5MonitorStoresResponse> {
  if (import.meta.env.DEV) {
    return mockMonitorStores();
  }
  return requestJSON(`${API_BASE}/h5/monitor/stores`);
},
```

Add mock:

```ts
function mockMonitorStores(): H5MonitorStoresResponse {
  return {
    cities: [
      {
        city: "北京",
        stores: [
          { external_org_id: "demo", store_name: "新氧青春演示门店", city: "北京", available_channel_count: 36 },
        ],
      },
    ],
  };
}
```

- [ ] **Step 3: Add helper test for H5 route path if needed**

If a new helper is added for switching URLs, add a test in `frontend/src/api.test.ts`:

```ts
expect(h5MonitorPath("10047")).toBe("/erzhuang-project/h5/orgs/10047/monitor");
```

This existing test may already cover it; do not duplicate unnecessarily.

- [ ] **Step 4: Run frontend tests**

Run:

```bash
cd frontend && npm test
```

Expected: pass.

- [ ] **Step 5: Commit data layer**

```bash
git add frontend/src/domain/h5-types.ts frontend/src/api-h5.ts frontend/src/api.test.ts
git commit -m "feat: add h5 monitor store switcher data"
```

---

### Task 4: Build Shared System Top Bar

**Files:**
- Create: `frontend/src/components/SystemTopBar.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/StoreDetail.tsx`
- Modify: `frontend/src/styles.css`
- Modify: `frontend/src/api.test.ts`

- [ ] **Step 1: Create `SystemTopBar`**

Create `frontend/src/components/SystemTopBar.tsx`:

```tsx
import type { ReactNode } from "react";
import { authUserDisplayName, type AuthState } from "../domain/auth";

export interface SystemTopBarBackAction {
  label: string;
  onClick: () => void;
}

interface SystemTopBarProps {
  backAction?: SystemTopBarBackAction;
  auth?: AuthState | null;
  loggingOut?: boolean;
  onLogout?: () => void | Promise<void>;
  rightExtra?: ReactNode;
}

export function SystemTopBar({ backAction, auth, loggingOut = false, onLogout, rightExtra }: SystemTopBarProps) {
  const displayName = authUserDisplayName(auth?.user);
  const showAuth = Boolean(auth?.authenticated && onLogout);

  return (
    <header className="system-topbar" aria-label="系统导航">
      <div className="system-topbar-left">
        {backAction ? (
          <button type="button" className="system-topbar-back" onClick={backAction.onClick}>
            <span aria-hidden="true">‹</span>
            {backAction.label}
          </button>
        ) : null}
      </div>
      <div className="system-topbar-right">
        {rightExtra}
        {showAuth ? (
          <div className="auth-user-chip" aria-label="当前登录用户">
            <span className="auth-user-name">{displayName}</span>
            <button className="plain-button auth-logout-button" onClick={() => void onLogout?.()} disabled={loggingOut}>
              {loggingOut ? "退出中..." : "退出登录"}
            </button>
          </div>
        ) : null}
      </div>
    </header>
  );
}
```

- [ ] **Step 2: Replace local `AuthUserActions` usage in `App.tsx`**

Import `SystemTopBar`:

```ts
import { SystemTopBar } from "./components/SystemTopBar";
```

On store detail branch, render before `StoreDetail`:

```tsx
<SystemTopBar
  backAction={{
    label: "返回列表",
    onClick: () => {
      detailRequestIdRef.current += 1;
      setActiveStore(null);
      setLoadedDetailTabs(new Set());
      setLoadingDetailTabs(new Set());
      void loadStores();
    },
  }}
  auth={showLogoutEntry ? auth : null}
  loggingOut={loggingOut}
  onLogout={logout}
/>
```

Then pass no `authActions` into `StoreDetail`; keep existing business props.

On store list branch, render:

```tsx
<SystemTopBar auth={showLogoutEntry ? auth : null} loggingOut={loggingOut} onLogout={logout} />
```

Remove the `AuthUserActions` function from `App.tsx`.

- [ ] **Step 3: Keep StoreDetail focused on business actions**

In `frontend/src/components/StoreDetail.tsx`, remove `authActions` prop if no longer needed.

Keep `h5MonitorUrl` rendering in detail-level action area:

```tsx
{h5MonitorUrl ? (
  <button className="detail-back-button" onClick={() => window.location.assign(h5MonitorUrl)}>
    查看监控
  </button>
) : null}
```

- [ ] **Step 4: Add top bar CSS**

Add to `frontend/src/styles.css` near page header styles:

```css
.system-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  max-width: 1440px;
  min-height: 42px;
  margin: 0 auto 14px;
}

.system-topbar-left,
.system-topbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.system-topbar-left {
  flex: 1;
}

.system-topbar-right {
  justify-content: flex-end;
}

.system-topbar-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 32px;
  padding: 0 10px 0 6px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--button-radius);
  background: var(--surface);
  color: var(--text-main);
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.system-topbar-back span {
  font-size: 24px;
  line-height: 1;
}

.system-topbar-back:hover {
  border-color: var(--border-strong);
  color: var(--text-strong);
}
```

- [ ] **Step 5: Run frontend tests and visual build**

Run:

```bash
cd frontend && npm test
cd frontend && npm run build
```

Expected: both pass.

- [ ] **Step 6: Commit top bar**

```bash
git add frontend/src/components/SystemTopBar.tsx frontend/src/App.tsx frontend/src/components/StoreDetail.tsx frontend/src/styles.css frontend/src/api.test.ts
git commit -m "feat: add shared system topbar"
```

---

### Task 5: Add H5 Store Switcher UI And Top Bar Integration

**Files:**
- Create: `frontend/src/components/H5StoreSwitcher.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/pages/H5Monitor.tsx`
- Modify: `frontend/src/pages/H5MonitorChannel.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Create H5 store switcher component**

Create `frontend/src/components/H5StoreSwitcher.tsx`:

```tsx
import { useState } from "react";
import type { H5MonitorStoresResponse } from "../domain/h5-types";

interface H5StoreSwitcherProps {
  currentExternalOrgId: string;
  stores: H5MonitorStoresResponse | null;
  loading: boolean;
  error: string;
  onSelectStore: (externalOrgId: string) => void;
}

export function H5StoreSwitcher({ currentExternalOrgId, stores, loading, error, onSelectStore }: H5StoreSwitcherProps) {
  const [open, setOpen] = useState(false);
  const currentStore = stores?.cities.flatMap((city) => city.stores).find((store) => store.external_org_id === currentExternalOrgId);

  return (
    <div className="h5-store-switcher">
      <button type="button" className="h5-store-switcher-trigger" onClick={() => setOpen((value) => !value)}>
        <span>{currentStore?.store_name || "切换门店"}</span>
        <strong>切换门店</strong>
      </button>
      {open ? (
        <div className="h5-store-switcher-menu" role="menu">
          {loading ? <div className="h5-store-switcher-state">门店加载中...</div> : null}
          {error ? <div className="h5-store-switcher-state error">{error}</div> : null}
          {!loading && !error && stores?.cities.length === 0 ? <div className="h5-store-switcher-state">暂无可切换门店</div> : null}
          {!loading && !error
            ? stores?.cities.map((city) => (
                <section key={city.city} className="h5-store-switcher-city">
                  <h2>{city.city}</h2>
                  {city.stores.map((store) => (
                    <button
                      key={store.external_org_id}
                      type="button"
                      role="menuitem"
                      className={store.external_org_id === currentExternalOrgId ? "active" : ""}
                      onClick={() => {
                        setOpen(false);
                        onSelectStore(store.external_org_id);
                      }}
                    >
                      <span>{store.store_name}</span>
                      <small>{store.available_channel_count} 路</small>
                    </button>
                  ))}
                </section>
              ))
            : null}
        </div>
      ) : null}
    </div>
  );
}
```

- [ ] **Step 2: Load store list in `H5RouteShell`**

In `frontend/src/App.tsx`, import:

```ts
import { h5Api } from "./api-h5";
import type { H5MonitorStoresResponse } from "./domain/h5-types";
```

Add state in `H5RouteShell`:

```ts
const [auth, setAuth] = useState<AuthState | null>(null);
const [loggingOut, setLoggingOut] = useState(false);
const [monitorStores, setMonitorStores] = useState<H5MonitorStoresResponse | null>(null);
const [monitorStoresLoading, setMonitorStoresLoading] = useState(true);
const [monitorStoresError, setMonitorStoresError] = useState("");
```

Add effects:

```ts
useEffect(() => {
  void storeSpaceApi.getAuthMe().then(setAuth).catch(() => setAuth(null));
}, []);

useEffect(() => {
  let cancelled = false;
  setMonitorStoresLoading(true);
  h5Api
    .listMonitorStores()
    .then((response) => {
      if (cancelled) return;
      setMonitorStores(response);
      setMonitorStoresError("");
    })
    .catch((error) => {
      if (cancelled) return;
      setMonitorStoresError(errorMessage(error, "门店列表加载失败"));
    })
    .finally(() => {
      if (!cancelled) setMonitorStoresLoading(false);
    });
  return () => {
    cancelled = true;
  };
}, []);
```

Add local logout:

```ts
function h5Logout() {
  setLoggingOut(true);
  window.location.assign(authLogoutPath());
}
```

Pass `auth`, `loggingOut`, `onLogout`, and store list props to H5 pages.

- [ ] **Step 3: Add top bar to H5 routes**

In H5 home route render:

```tsx
<SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={h5Logout} />
<H5MonitorPage
  externalOrgId={route.externalOrgId}
  stores={monitorStores}
  storesLoading={monitorStoresLoading}
  storesError={monitorStoresError}
  onSelectStore={(externalOrgId) => {
    const url = `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor`;
    window.history.pushState({}, "", url);
    setRoute(parseH5Route());
  }}
  onOpenChannel={...}
/>
```

In H5 channel route render:

```tsx
<SystemTopBar
  backAction={{
    label: "返回",
    onClick: () => {
      const url = `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(route.externalOrgId)}/monitor`;
      window.history.pushState({}, "", url);
      setRoute(parseH5Route());
    },
  }}
  auth={auth}
  loggingOut={loggingOut}
  onLogout={h5Logout}
/>
<H5MonitorChannelPage ... />
```

- [ ] **Step 4: Update H5Monitor props and render switcher**

Modify `frontend/src/pages/H5Monitor.tsx` props:

```ts
import { H5StoreSwitcher } from "../components/H5StoreSwitcher";
import type { H5MonitorChannel, H5MonitorHomeResponse, H5MonitorStoresResponse, MonitorCategory } from "../domain/h5-types";

interface H5MonitorProps {
  externalOrgId: string;
  onOpenChannel: (channelId: number) => void;
  stores?: H5MonitorStoresResponse | null;
  storesLoading?: boolean;
  storesError?: string;
  onSelectStore?: (externalOrgId: string) => void;
  refreshKey?: number;
}
```

Render below header:

```tsx
{onSelectStore ? (
  <H5StoreSwitcher
    currentExternalOrgId={externalOrgId}
    stores={stores ?? null}
    loading={Boolean(storesLoading)}
    error={storesError ?? ""}
    onSelectStore={onSelectStore}
  />
) : null}
```

- [ ] **Step 5: Remove duplicated H5 channel back button**

In `frontend/src/pages/H5MonitorChannel.tsx`, remove the `<button className="h5-back-btn"...>` from `.h5-viewer-header`.

Keep the title and diagnostics button.

- [ ] **Step 6: Add CSS**

Add to `frontend/src/styles.css`:

```css
.h5-store-switcher {
  position: relative;
  margin: 10px 0 12px;
}

.h5-store-switcher-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-height: 40px;
  padding: 0 12px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 8px;
  background: #fff;
  color: #111827;
  font-size: 14px;
  font-weight: 700;
}

.h5-store-switcher-trigger span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.h5-store-switcher-trigger strong {
  flex: 0 0 auto;
  color: #2563eb;
  font-size: 12px;
}

.h5-store-switcher-menu {
  position: absolute;
  z-index: 30;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  max-height: min(420px, 64vh);
  overflow: auto;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 14px 36px rgba(15, 23, 42, 0.14);
}

.h5-store-switcher-city {
  padding: 10px;
}

.h5-store-switcher-city + .h5-store-switcher-city {
  border-top: 1px solid rgba(148, 163, 184, 0.2);
}

.h5-store-switcher-city h2 {
  margin: 0 0 8px;
  color: #64748b;
  font-size: 12px;
}

.h5-store-switcher-city button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  min-height: 36px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #111827;
  text-align: left;
}

.h5-store-switcher-city button.active,
.h5-store-switcher-city button:hover {
  background: #eef2ff;
}

.h5-store-switcher-city button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.h5-store-switcher-city button small {
  flex: 0 0 auto;
  color: #64748b;
}

.h5-store-switcher-state {
  padding: 14px;
  color: #64748b;
  font-size: 13px;
}

.h5-store-switcher-state.error {
  color: #b91c1c;
}
```

- [ ] **Step 7: Run frontend verification**

Run:

```bash
cd frontend && npm test
cd frontend && npm run build
```

Expected: both pass.

- [ ] **Step 8: Commit H5 switcher UI**

```bash
git add frontend/src/components/H5StoreSwitcher.tsx frontend/src/App.tsx frontend/src/pages/H5Monitor.tsx frontend/src/pages/H5MonitorChannel.tsx frontend/src/styles.css frontend/src/api-h5.ts frontend/src/domain/h5-types.ts
git commit -m "feat: add h5 monitor store switcher"
```

---

### Task 6: Final Verification, Version, And Release Prep

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Run all practical verification**

Run:

```bash
cd frontend && npm test
cd frontend && npm run build
cd ..
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go test -c ./internal/h5monitor
GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build GOTMPDIR=/Users/sylar/erzhuang-project/.cache/go-tmp ./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server
```

Expected:

- Frontend tests pass.
- Frontend build passes.
- Go compile-level checks pass.

- [ ] **Step 2: Increment version**

If the last released version is `2.24.2`, update `VERSION` to:

```text
2.25.0
```

Rationale: this adds a visible navigation component and a H5 monitor store switcher, so it is a minor feature release.

- [ ] **Step 3: Record development state**

Append to `docs/codex-learning-state.md`:

```md
## 2026-07-02 系统顶栏与 H5 切换门店 2.25.0 开发记录

- 背景：
  - 用户希望门店列表、机构详情、H5 Monitor 的返回和退出登录位置统一。
  - H5 Monitor 需要支持在有权限范围内切换门店。
- 实现：
  - 新增共享 `SystemTopBar`。
  - 后台首页、机构详情页、H5 Monitor 首页和播放页统一顶栏。
  - 新增 H5 Monitor 门店切换接口和前端菜单。
  - 门店切换列表口径：有 `external_org_id` 且至少一个有效通道；不要求区域确认。
  - 403 未授权页展示 `暂无访问权限` 并提供重新登录。
- 验证：
  - `cd frontend && npm test` 通过。
  - `cd frontend && npm run build` 通过。
  - `./.tools/go/bin/go test -c ./internal/h5monitor` 通过。
  - `./.tools/go/bin/go build -o /private/tmp/erzhuang-server-check ./cmd/server` 通过。
```

- [ ] **Step 4: Commit release prep**

```bash
git add VERSION docs/codex-learning-state.md
git commit -m "docs: record system topbar release prep"
```

- [ ] **Step 5: Push only when user asks to publish**

If the user says `发布到公司`, follow `docs/deploy-runbook.md`:

```bash
git push origin codex/containerize-single-image
git push gitlab codex/containerize-single-image
```

GitLab push may require interactive username/token. Do not put credentials in commands, files, or docs.

---

## Self-Review Notes

- Spec coverage:
  - System top bar: Task 4 and Task 5.
  - Unauthorized SSO state: Task 1.
  - H5 store switcher: Task 2, Task 3, Task 5.
  - Backend filtering by effective channel and external org id: Task 2 Step 6.
  - Future permission filtering is preserved as backend boundary and not implemented.
- No placeholders intentionally remain.
- Known risk:
  - Current H5 home channel query still filters confirmed statuses. The store switcher list does not. If a store has effective but unconfirmed channels, it may appear in switcher while H5 home shows no channels unless the channel list query is also relaxed. During Task 2 implementation, decide whether `ListActiveChannelsByOrgID` should follow the same “effective channel” rule. Recommended: relax it to `c.is_active` plus recorder/account requirements, and let category/display helpers handle unconfirmed channel labels.
