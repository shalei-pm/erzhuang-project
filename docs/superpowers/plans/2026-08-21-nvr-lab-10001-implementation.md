# 10001 NVR Lab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an admin-only, hidden NVR streaming experiment for tenant `10001` without changing the existing EZVIZ H5 Monitor flow.

**Architecture:** Add an isolated `internal/nvrlab` domain which reads only the existing resource-view repository, validates the requested camera belongs to the fixed tenant, then calls the company authorization API to issue an in-memory WSS session. Add separate React routes and an NVRPlayer adapter that is deliberately unable to log or persist its signed URL; reuse the approved H5 monitor visual components and styles.

**Tech Stack:** Go `net/http`, existing MySQL resource-view repository, APISIX SSO role guard, React/TypeScript/Vite, locally versioned NVRPlayer/WASM assets.

---

## File map

- Create `internal/nvrlab/models.go`, `service.go`, `handler.go`, `authorization_client.go` and matching Go tests: isolated list/session domain and no-secret error contract.
- Modify `internal/app/handler.go` and `cmd/server/main.go`: construct and register the isolated service only when its configuration is valid.
- Create `frontend/src/api-nvr-lab.ts`, `frontend/src/domain/nvr-lab.ts`, `frontend/src/components/NVRLabPlayer.tsx`, `frontend/src/pages/NVRLabMonitor.tsx`, `frontend/src/pages/NVRLabCamera.tsx`: route/API/player/page ownership.
- Modify `frontend/src/App.tsx` and `frontend/src/styles.css`: recognize hidden routes and reuse the H5 layout language.
- Create `frontend/src/vendor/nvr-player/nvr-player.js` and `frontend/public/nvr-player/wasm/*`: pinned, reviewed player assets, not an external pre-production CDN.

### Task 1: Define and test the NVR Lab backend boundary

**Files:**
- Create: `internal/nvrlab/models.go`
- Create: `internal/nvrlab/service.go`
- Create: `internal/nvrlab/service_test.go`

- [ ] **Step 1: Write failing tests for fixed tenant, valid camera selection, and stream request validation.**

```go
func TestCreateSessionRejectsCameraOutsideTenant(t *testing.T) {
  service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{10001: sampleRecords()}}, fakeAuthorizer{})
  _, err := service.CreateSession(context.Background(), 10001, 999, StreamSessionRequest{Mode: ModeLive})
  if !errors.Is(err, ErrCameraNotFound) { t.Fatalf("err=%v", err) }
}

func TestCreateSessionRejectsPlaybackLongerThanThirtyMinutes(t *testing.T) {
  service := NewService(fakeRepository{records: map[int64]resourceview.StoreRecords{10001: sampleRecords()}}, fakeAuthorizer{})
  _, err := service.CreateSession(context.Background(), 10001, 111, StreamSessionRequest{Mode: ModePlayback, StartTime: 100, EndTime: 1901})
  if !errors.Is(err, ErrInvalidPlaybackWindow) { t.Fatalf("err=%v", err) }
}
```

- [ ] **Step 2: Run the targeted Go test and verify the expected missing-package failure.**

Run: `go test ./internal/nvrlab -run 'TestCreateSession' -count=1`

- [ ] **Step 3: Implement minimal types and service.**

```go
const experimentTenantID int64 = 10001
const maxPlaybackWindow = 30 * time.Minute
type Mode string
const ( ModeLive Mode = "live"; ModePlayback Mode = "playback" )
type StreamSessionRequest struct { Mode Mode `json:"mode"`; StartTime int64 `json:"start_time,omitempty"`; EndTime int64 `json:"end_time,omitempty"` }
type StreamSessionResponse struct { URL string `json:"url"`; Mode Mode `json:"mode"` }
```

`CreateSession` must only accept tenant `10001`, select cameras where `Category == "camera"`, `Provider == "HikVisionNvrChannel"`, `Status == 1`, and `DeletedAt == nil`, and never put the URL into an error.

- [ ] **Step 4: Run the targeted test and the package test suite.**

Run: `go test ./internal/nvrlab -count=1`
Expected: PASS.

### Task 2: Implement the server-side authorization client and HTTP routes

**Files:**
- Create: `internal/nvrlab/authorization_client.go`
- Create: `internal/nvrlab/handler.go`
- Create: `internal/nvrlab/handler_test.go`

- [ ] **Step 1: Write failing HTTP tests for admin-only access, no-store headers, and authorization request shape.**

```go
func TestStreamSessionSetsNoStoreAndDoesNotExposeCredential(t *testing.T) {
  request := httptest.NewRequest(http.MethodPost, "/api/h5/nvr-lab/10001/cameras/111/stream-session", strings.NewReader(`{"mode":"live"}`))
  recorder := httptest.NewRecorder()
  handler.ServeHTTP(recorder, request)
  if got := recorder.Header().Get("Cache-Control"); got != "no-store" { t.Fatalf("cache=%q", got) }
  if strings.Contains(recorder.Body.String(), "long-lived-secret") { t.Fatal("credential leaked") }
}
```

- [ ] **Step 2: Implement `HTTPAuthorizationClient`.**

`HTTPAuthorizationClient` must call `https://sec.sy.soyoung.com/api/auth/camera` with `Authorization` only on the outgoing request. It sends `camera_id`, `stream_type=2`, and only playback time parameters. It parses `{"code":0,"data":{"token":"..."}}`, rejects an empty token, then returns `wss://prime-crm.soyoung.com/nvrapi/ws?token=<url.QueryEscape(token)>`. Its error values are stable categories (`authorization_unconfigured`, `authorization_failed`, `authorization_timeout`) and contain no response body, token, URL, or header value.

- [ ] **Step 3: Implement routes and the app-owned admin guard.**

```text
GET  /api/h5/nvr-lab/10001/cameras
POST /api/h5/nvr-lab/10001/cameras/{cameraId}/stream-session
```

Register route handlers through a callback that checks `normalizeRole(currentAuthUser(r).Role) == app.RoleAdmin`; return `403 {"code":"nvr_lab_forbidden","error":"暂无实验页面访问权限"}` otherwise. Decode only `live` and `playback`; reject invalid JSON and invalid paths with stable `400` codes.

- [ ] **Step 4: Run route and client tests.**

Run: `go test ./internal/nvrlab -count=1`
Expected: PASS.

### Task 3: Wire configuration and preserve old monitor behavior

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `cmd/server/main_test.go`
- Modify: `internal/app/handler.go`
- Modify: `internal/app/handler_test.go`

- [ ] **Step 1: Write failing configuration and registration tests.**

```go
func TestNVRLabConfigReadsKubernetesSecret(t *testing.T) {
  t.Setenv("K8S_SECRET_NVR_STREAM_AUTHORIZATION", "test-value")
  config := nvrLabConfigFromEnv()
  if !config.Enabled { t.Fatal("expected configured nvr lab") }
}
```

- [ ] **Step 2: Add `nvrLabConfigFromEnv` and dependency injection.**

Use `K8S_SECRET_NVR_STREAM_AUTHORIZATION` first, then `NVR_STREAM_AUTHORIZATION`; do not log its presence/value. The service is `nil` when absent, so existing deployments remain operational. Pass `resourceview.NewMySQLRepository(db)` to the experiment service and preserve all `h5monitor` setup unchanged.

- [ ] **Step 3: Add the route to the handler constructor without breaking old callers.**

The registration helper must support a nil lab service, and `withBasePathAPIPrefixes` continues to map `/erzhuang-project/api/...` before routing.

- [ ] **Step 4: Run backend checks.**

Run: `go test ./... && go build ./cmd/server`
Expected: PASS.

### Task 4: Vendor a reviewed NVR player and build a no-persistence React adapter

**Files:**
- Create: `frontend/src/vendor/nvr-player/nvr-player.js`
- Create: `frontend/public/nvr-player/wasm/systemTransform-worker.js`
- Create: `frontend/public/nvr-player/wasm/libSystemTransform.js`
- Create: `frontend/public/nvr-player/wasm/libSystemTransform.wasm`
- Create: `frontend/src/components/NVRLabPlayer.tsx`
- Create: `frontend/src/components/NVRLabPlayer.test.tsx`

- [ ] **Step 1: Write a failing component test for mode and retry behavior.**

```tsx
it("creates a replay player without automatic reconnect", () => {
  render(<NVRLabPlayer session={{ url: "wss://example.test/session", mode: "playback" }} onState={onState} />);
  expect(mockNVRPlayer).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), expect.objectContaining({ autoReconnect: false, forceWasm: true }));
});
```

- [ ] **Step 2: Copy the supplied SDK assets and patch them before importing.**

Replace all logging that includes `wsUrl`, token-bearing URLs, packet payloads, or raw server errors with no-op behavior; set the worker base to `/erzhuang-project/nvr-player/wasm/systemTransform-worker.js` via `import.meta.env.BASE_URL`; retain no hidden retry. The SDK must not access localStorage/sessionStorage.

- [ ] **Step 3: Implement `NVRLabPlayer`.**

Create a canvas player with `autoReconnect: false`, `forceWasm: session.mode === "playback"`, event callbacks for connection and first frame, and `stop()` cleanup on every session or component change. Its single retry button must call the parent to request a fresh session; it must not invoke `play()` with an old URL.

- [ ] **Step 4: Run the focused frontend test.**

Run: `cd frontend && npm test -- --run src/components/NVRLabPlayer.test.tsx`
Expected: PASS.

### Task 5: Add isolated API/types/routes and reuse the approved H5 visual experience

**Files:**
- Create: `frontend/src/api-nvr-lab.ts`
- Create: `frontend/src/domain/nvr-lab.ts`
- Create: `frontend/src/domain/nvr-lab.test.ts`
- Create: `frontend/src/pages/NVRLabMonitor.tsx`
- Create: `frontend/src/pages/NVRLabCamera.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Write failing route and playback-window tests.**

```ts
expect(parseNVRLabRoute("/h5/nvr-lab/10001/cameras/111")).toEqual({ name: "camera", cameraId: 111 });
expect(validateNVRLabPlayback(200, 100)).toEqual("结束时间必须晚于开始时间");
expect(validateNVRLabPlayback(0, 100)).toEqual("请选择完整的回放时间范围");
```

- [ ] **Step 2: Implement API requests and memory-only session state.**

Use existing `requestJSON` behavior with credentials included. `createStreamSession` returns `{url, mode}` but must never log it; no API field is stored in browser persistence.

- [ ] **Step 3: Implement `/h5/nvr-lab/10001` and camera detail routes.**

Recognize only exact `10001` routes before the general H5 parser. Reuse `SystemTopBar`, `.h5-monitor-page`, `.h5-camera-wall`, `.h5-channel-page`, `.h5-player-wrapper`, existing controls, and visual hierarchy. The main page derives concise camera labels from business space type/name and includes the existing area tab pattern. The detail page offers “实时视频 / 录像”, local time inputs for a max-30-minute range, and a collapsed diagnostic panel with only mode/state/duration/error category.

- [ ] **Step 4: Add narrowly scoped styles and run frontend checks.**

Run: `cd frontend && npm test && npm run build`
Expected: PASS.

### Task 6: Integrate, verify and record deployment prerequisites

**Files:**
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/decisions.md`
- Modify: `work/current-plan.md`

- [ ] **Step 1: Run the complete automated suite.**

Run: `go test ./... && go build ./cmd/server && cd frontend && npm test && npm run build`
Expected: PASS.

- [ ] **Step 2: Browser-check the hidden route with a non-sensitive mock/failure response.**

Verify desktop structure for list, player loading, error, replay inputs and diagnostic panel. Use Chrome plugin if exposed; otherwise document the reason for the fallback. Confirm the console does not contain a token or WSS address.

- [ ] **Step 3: Update project memory with deployment precondition and verification scope.**

Record that testing requires test K8s Secret `K8S_SECRET_NVR_STREAM_AUTHORIZATION`, and that live/replay first-frame verification is still pending on the real environment. Do not record its value.

- [ ] **Step 4: Commit only this feature's files.**

Run: `git add cmd/server/main.go cmd/server/main_test.go internal/app/handler.go internal/app/handler_test.go internal/nvrlab frontend/src/App.tsx frontend/src/api-nvr-lab.ts frontend/src/domain/nvr-lab.ts frontend/src/domain/nvr-lab.test.ts frontend/src/components/NVRLabPlayer.tsx frontend/src/components/NVRLabPlayer.test.tsx frontend/src/pages/NVRLabMonitor.tsx frontend/src/pages/NVRLabCamera.tsx frontend/src/vendor/nvr-player frontend/public/nvr-player frontend/src/styles.css docs/superpowers/plans/2026-08-21-nvr-lab-10001-implementation.md`

Run: `git commit -m "feat: add 10001 nvr streaming lab"`

## Plan self-review

- Spec coverage: fixed tenant/admin access (Tasks 1-3), business camera validation (Task 1), server-side short-lived session authorization (Task 2), no-store/no-secret behavior (Tasks 2 and 4), reviewed SDK/WASM (Task 4), familiar H5 experience and playback controls (Task 5), and test/deployment gates (Task 6).
- Placeholder scan: no deferred implementation marker is used; each external dependency is named, and the permitted first release scope is explicit.
- Type consistency: session request uses `mode`, `start_time`, `end_time` at handler/API boundaries; UI session uses the same `Mode` union; only stream response carries the signed WSS address.
