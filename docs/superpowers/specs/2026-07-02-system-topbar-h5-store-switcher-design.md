# System Top Bar And H5 Store Switcher Design

Date: 2026-07-02

## Goal

Unify navigation, logout, and monitor store switching across the admin pages and H5 Monitor.

The immediate product goal is:

- Keep logout in a consistent top-right position.
- Keep page return navigation in a consistent top-left position where applicable.
- Move page-specific actions below the system navigation bar.
- Add a H5 Monitor store switcher that only shows stores with at least one effective monitor channel.
- Show a clear no-access state for SSO users who are not authorized in the project user table.

## Current Problems

- Store list, store detail, and H5 Monitor place logout and return controls differently.
- H5 Monitor currently has no logout entry.
- H5 Monitor can enter a store only from a fixed URL or detail-page button; users cannot switch stores inside the monitor view.
- Unauthorized SSO users receive 403 from `/api/auth/me`, but the UI needs a clear product-facing state.

## Product Rules

### System Top Bar

Use one shared top bar component across admin and H5 pages.

Left side:

- Store list home: empty.
- Store detail: `返回列表`.
- H5 Monitor home: empty or `返回后台` only when a safe admin return target is available.
- H5 Monitor channel page: `返回`.

Right side:

- Auth user display and `退出登录`.
- Display only `display_name`, falling back to `username`, then `已登录`.
- Never display enterprise email in the visible top bar.

Page-specific actions stay below the top bar:

- Store list: `添加门店`.
- Store detail: `查看监控`, tabs, model switch.
- H5 Monitor home: store switcher and area tabs.
- H5 Monitor channel page: playback/live controls stay in the player area.

### Unauthorized SSO State

When `/api/auth/me` returns 403:

- Show a full-page state: `暂无访问权限`.
- Text: `当前公司账号尚未被授权访问二壮系统，请联系项目负责人开通权限，或更换账号登录。`
- Primary action: `重新登录`.
- Clicking `重新登录` must first trigger SSO logout, then the user can revisit the project homepage to invoke SSO again.
- No store list, account list, AI settings, or H5 APIs should load while in this state.

### H5 Monitor Store Switcher

The H5 Monitor home page gets a `切换门店` menu.

Menu behavior:

- Group stores by city.
- Highlight the current store.
- Click a store to navigate to `/h5/orgs/{externalOrgId}/monitor`.
- The menu list includes any store that has:
  - non-empty `external_org_id`
  - at least one effective monitor channel
- It does not require store completion, business-area confirmation, or channel-area confirmation.
- Future permissions should filter this list on the backend. The frontend should not implement authorization logic.

## Backend API

Add a focused H5 endpoint:

```http
GET /api/h5/monitor/stores
```

Suggested response:

```json
{
  "cities": [
    {
      "city": "上海",
      "stores": [
        {
          "external_org_id": "10047",
          "store_name": "新氧青春诊所(上海凯德晶萃店)",
          "city": "上海",
          "available_channel_count": 12
        }
      ]
    }
  ]
}
```

First implementation filtering:

- Use existing store and video channel data.
- Include stores with `external_org_id <> ''`.
- Include only stores with at least one effective channel that can appear in H5 Monitor.
- Do not require channel confirmation status beyond being effective/active enough for monitor playback.

Future permission filtering:

- Once `tb_user_store_scopes` or equivalent permission data is active, this endpoint must filter by the current authenticated user.
- The response contract should stay stable.

## Frontend Components

### `SystemTopBar`

Props:

- `backAction?: { label: string; onClick: () => void }`
- `auth?: AuthState`
- `loggingOut?: boolean`
- `onLogout?: () => void | Promise<void>`
- `rightExtra?: ReactNode` only if needed later

Responsibilities:

- Render consistent shell navigation.
- Keep logout at the far right.
- Not know page-specific business actions.

### `H5StoreSwitcher`

Props:

- `currentExternalOrgId`
- `cities`
- `onSelectStore`
- loading/error state

Responsibilities:

- Render `切换门店`.
- Group stores by city.
- Highlight current store.
- Navigate through callback.

### Existing Pages

Admin store list:

- Top bar appears above the current page header.
- Page header keeps title, summary, and `添加门店`.

Store detail:

- Top bar owns `返回列表` and logout.
- Detail header no longer independently places auth actions.
- `查看监控` remains in detail-level actions below the top bar.

H5 Monitor home:

- Top bar owns logout.
- Store name remains page heading.
- Store switcher appears below the top bar, near the monitor page heading.

H5 Monitor channel:

- Top bar owns back and logout.
- Existing player header focuses on channel title, diagnostics, and playback context.

## Error Handling

- H5 store switcher load failure: show a compact inline error in the menu area, not a full-page failure.
- If the current store is missing from switcher results, still allow the current page to render; the switcher can show `当前门店`.
- If user loses authorization, `/api/auth/me` 403 goes to the no-access page.
- H5 API 401/403 should use the same auth/no-access flow when practical. If not done in the first pass, errors should still be understandable.

## Testing

Automated:

- Auth helper tests for 403 forbidden state and re-login path.
- Top bar display helper test: no email fallback.
- H5 store switcher grouping/filter display helper tests.
- Backend H5 monitor store list repository/service/handler tests.
- Frontend build and tests.

Manual:

- Store list: logout is top-right; `添加门店` remains below top bar.
- Store detail: `返回列表` top-left, logout top-right, `查看监控` below.
- H5 Monitor home: logout top-right; switch store menu lists stores with effective channels.
- H5 Monitor channel: return top-left, logout top-right.
- Unauthorized SSO user: sees `暂无访问权限`; `重新登录` triggers SSO logout path.

## Out Of Scope

- Full user/role/scope management UI.
- Long-term RBAC configuration screens.
- Complex organization tree.
- Showing stores that have no effective monitor channels.
- Hiding individual channels by permission before store-scope permissions are implemented.
