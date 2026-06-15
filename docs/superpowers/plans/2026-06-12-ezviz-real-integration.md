# Ezviz Real Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the real Ezviz recorder integration in small verifiable slices: token refresh, channel scan, screenshot capture, then AI recognition.

**Architecture:** Add an `internal/ezviz` package that owns OpenAPI calls, per-account token refresh, response parsing, and redaction. Keep credentials outside Git: local probe reads `/Users/sylar/Downloads/ezviz-real-data.md`, while server runtime will use env/local config later. Store-space service will consume a narrow Ezviz client interface so tests can use fakes.

**Tech Stack:** Go 1.22 standard `net/http`, form-encoded Ezviz OpenAPI, existing Go service, Postgres store-space tables, later OpenAI-compatible vision API for screenshots.

---

### Task 1: Ezviz Client And Token Refresh Probe

**Files:**
- Create: `internal/ezviz/client.go`
- Create: `internal/ezviz/client_test.go`
- Create: `cmd/ezviz-probe/main.go`
- Modify: `.gitignore`
- Modify: `docs/ezviz-openapi-notes.md`

- [ ] **Step 1: Write failing tests**

Add tests in `internal/ezviz/client_test.go` for:

```go
func TestClientRefreshesTokenAfterExpiredTokenResponse(t *testing.T) {
	// httptest server returns 10002 for first camera list call, then token/get returns a new token,
	// then the retry returns code 200 with one channel.
}

func TestClientDoesNotExposeSecretsInErrors(t *testing.T) {
	// appKey/appSecret/accessToken appear in test input but must not appear in err.Error().
}

func TestParseRealDataMarkdownSelectsNorthChina(t *testing.T) {
	// parser reads a markdown table and returns account name plus device serial for 华北.
}
```

- [ ] **Step 2: Verify red**

Run:

```sh
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/ezviz
```

Expected: FAIL because package/functions do not exist.

- [ ] **Step 3: Implement minimal client**

Implement:

```go
type Account struct {
	Name        string
	AppKey      string
	AppSecret   string
	AccessToken string
}

type Client struct {
	BaseURL string
	HTTPClient *http.Client
}

func (c *Client) CameraList(ctx context.Context, account Account, deviceSerial string) ([]Camera, error)
func (c *Client) Capture(ctx context.Context, account Account, deviceSerial string, channelNo int) (CaptureResult, error)
func ParseAccountsMarkdown(markdown []byte) ([]AccountWithDevice, error)
```

Rules:
- request body must be `application/x-www-form-urlencoded`.
- token cache is per account name.
- code `10002` / `10014` refreshes token via `token/get`, retries once.
- error strings must redact appKey/appSecret/accessToken.

- [ ] **Step 4: Implement probe command**

`cmd/ezviz-probe/main.go` supports:

```sh
go run ./cmd/ezviz-probe --data /Users/sylar/Downloads/ezviz-real-data.md --region 华北 --capture-limit 3
```

Output only safe values:
- account name and region.
- device serial.
- token success and expireTime.
- channel count and channelNo/cameraName/status.
- capture success/failure per first N channels.
- no appSecret/accessToken.

- [ ] **Step 5: Verify green locally**

Run:

```sh
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/ezviz
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...
```

Expected: PASS.

- [ ] **Step 6: Run real 华北 probe**

Run with network approval:

```sh
go run ./cmd/ezviz-probe --data /Users/sylar/Downloads/ezviz-real-data.md --region 华北 --capture-limit 3
```

Expected:
- token OK or refreshed OK.
- camera list returns code 200 or a structured Ezviz error.
- capture attempts produce safe output.

Observed on 2026-06-12:
- 华北 / `GN0941203` camera list returned 32 channels.
- Active channels were `1`, `2`, `3`, `4`.
- Captures for active channels `1`, `2`, `3` succeeded.
- Captures for inactive channels `10`, `11` returned Ezviz code `60012`.

- [ ] **Step 7: Commit**

```sh
git add internal/ezviz cmd/ezviz-probe docs/ezviz-openapi-notes.md .gitignore
git commit -m "Add Ezviz probe and token refresh client"
```

### Task 2: Store-Space Scan Channels

**Files:**
- Modify: `internal/storespace/models.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/handler.go`
- Modify: `internal/storespace/service_test.go`
- Modify: `internal/storespace/handler_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write failing service tests**

Test `ScanRecorderChannels` with a fake Ezviz client:
- missing account returns structured validation error.
- successful camera list upserts active channels.
- recorder status becomes online when at least one active channel exists.
- channelNo is saved exactly as returned, no +/- 1.

- [ ] **Step 2: Implement repository methods**

Add methods:

```go
GetRecorder(ctx context.Context, recorderID int64) (*Recorder, error)
GetEzvizAccount(ctx context.Context, accountID int64) (*EzvizAccount, error)
ReplaceRecorderChannels(ctx context.Context, recorderID int64, channels []ChannelInput) (*Recorder, error)
```

- [ ] **Step 3: Wire service**

Service constructor receives optional scanner client. If absent, return `ErrNotImplemented` so memory/mock tests remain explicit.

- [ ] **Step 4: Verify**

Run:

```sh
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./internal/storespace
CGO_ENABLED=0 GOCACHE=/Users/sylar/erzhuang-project/.cache/go-build ./.tools/go/bin/go test ./...
```

### Task 3: Screenshot Capture And Persistence

**Files:**
- Modify: `internal/ezviz/client.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/store.go`
- Modify: `internal/storespace/schema.go`
- Create: `internal/storespace/snapshots.go`

Scope:
- serial capture per recorder.
- save latest thumbnail and full image under server-controlled storage.
- do not expose Ezviz `picUrl`.
- mark channel inactive only after capture confirms unavailable or configured retry threshold is met.

### Task 4: AI Recognition From Channel Screenshots

**Files:**
- Create: `internal/channelai/recognizer.go`
- Create: `internal/channelai/openai.go`
- Modify: `internal/storespace/service.go`
- Modify: `internal/storespace/store.go`
- Modify: `frontend/src/components/VideoChannelTab.tsx`

Scope:
- run recognition serially per recorder.
- classify business types: treatment, consultation, beauty.
- classify non-business types: front desk, corridor, passage, waiting area, hall, entrance, storage, pharmacy, machine room, unknown.
- AI result is prefill only; human confirmation creates/updates official area master data.
