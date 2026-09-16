# Digital Twin BI Naming and Arrival Stack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update all digital-twin BI terminology and units, then render arrival visits as an explicit front-desk/non-consult/consult stack only when all three path values are present.

**Architecture:** Extend the T+1 provider contract with an optional `visit_front_desk` value and carry it unchanged through Go and TypeScript models. Keep rendering decisions inside the reusable chart-data layer: each arrival datum uses the three path series only when all are non-null, otherwise it uses the total series. The SVG renderer derives its legend from series actually drawn in the 30-day dataset and always keeps all raw series available in the tooltip.

**Tech Stack:** Go 1.22, React 19, TypeScript, plain JavaScript SVG chart renderer, Vitest, Playwright.

---

### Task 1: Extend the T+1 data contract

**Files:**
- Modify: `internal/digitaltwin/t1bi/models.go`
- Modify: `internal/digitaltwin/t1bi/provider.go`
- Modify: `internal/digitaltwin/t1bi/service.go`
- Test: `internal/digitaltwin/t1bi/provider_test.go`
- Test: `internal/digitaltwin/t1bi/service_test.go`

- [ ] **Step 1: Write failing provider and service tests**

Create `provider_test.go` with a provider response fixture containing `"visit_front_desk":3`; call a focused `decodeRows` helper and assert that `DailyMetric.VisitFrontDesk` equals `3`. Extend `TestServiceMapsDailyMetricsAndSortsDates` with `VisitFrontDesk: number(3)` and assert the mapped `Trend.VisitFrontDesk` equals `3`.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
GOCACHE="$PWD/.cache/go-build" GOTMPDIR="$PWD/.cache/go-tmp" ./.tools/go/bin/go test ./internal/digitaltwin/t1bi
```

Expected: compile failure because `VisitFrontDesk` does not exist.

- [ ] **Step 3: Add the optional field through every Go layer**

Add:

```go
VisitFrontDesk *float64
```

to `DailyMetric`, add:

```go
VisitFrontDesk *float64 `json:"visit_front_desk"`
```

to `Trend`, parse the provider response field `visit_front_desk`, and map it in `Service.Get` without deriving it from other values. Extract the existing response unmarshal/status/row mapping block into `decodeRows(raw string)` so the mapping can be tested without a live Hprose server; `GetDailyMetrics` must delegate to it.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run the Task 1 command again. Expected: PASS.

- [ ] **Step 5: Commit the contract change**

```bash
git add internal/digitaltwin/t1bi/models.go internal/digitaltwin/t1bi/provider.go internal/digitaltwin/t1bi/provider_test.go internal/digitaltwin/t1bi/service.go internal/digitaltwin/t1bi/service_test.go
git commit -m "feat: accept front desk arrival metrics"
```

### Task 2: Define arrival fallback and presentation metadata

**Files:**
- Modify: `frontend/src/vendor/digital-twin/chart-data.js`
- Test: `frontend/src/vendor/digital-twin/chart-data.test.js`

- [ ] **Step 1: Write failing chart-data tests**

Add assertions that:

```js
seriesForDatum(visits,{visitAll:55,frontDesk:3,noConsult:34,consult:18})
```

returns `frontDesk`, `noConsult`, `consult`, while any missing segment returns only `visitAll`. Also assert the new titles, units, and labels:

```text
到院人次 / 人次
在店时长
升单率
核销客单价 / 元/人次
人均服务点数 / 点/人次
全部顾客 / 非面诊 / 面诊
```

- [ ] **Step 2: Run the chart-data test and verify RED**

```bash
cd frontend && npm test -- src/vendor/digital-twin/chart-data.test.js
```

Expected: FAIL because `frontDesk` and the new metadata do not exist.

- [ ] **Step 3: Implement the chart specification**

Add the blue `frontDesk` series, change all requested titles/units/labels, and change `seriesForDatum` to use `every(...)` across the three segment keys rather than `some(...)`. Update demo data so `visitAll = frontDesk + noConsult + consult`.

- [ ] **Step 4: Run the chart-data test and verify GREEN**

Run the Task 2 command again. Expected: PASS.

- [ ] **Step 5: Commit the presentation metadata**

```bash
git add frontend/src/vendor/digital-twin/chart-data.js frontend/src/vendor/digital-twin/chart-data.test.js
git commit -m "feat: define arrival path stack presentation"
```

### Task 3: Render dynamic legends and complete tooltips

**Files:**
- Modify: `frontend/src/vendor/digital-twin/charts.js`
- Modify: `frontend/src/vendor/digital-twin/styles.css`
- Test: `frontend/src/digital-twin.browser.mjs`

- [ ] **Step 1: Add failing browser assertions**

Assert that a complete arrival dataset renders 30 bars for each `frontDesk`, `noConsult`, and `consult`, renders zero `visitAll` bars, uses title `到院人次`, unit `人次`, and shows the note `全部顾客 = 前台签到 + 非面诊 + 面诊`. Add an incomplete fixture day and assert that day renders one `visitAll` bar and no segment bars.

- [ ] **Step 2: Run the browser check and verify RED**

Start the local Vite fixture server and run:

```bash
QA_LOCAL_ORIGIN=http://127.0.0.1:<port> PLAYWRIGHT_MODULE=<playwright-path> node frontend/src/digital-twin.browser.mjs
```

Expected: FAIL on the new legend, note, and `frontDesk` bar assertions.

- [ ] **Step 3: Implement dynamic legend and note rendering**

Compute the union of series returned by `seriesForDatum` across the dataset for the visible legend. Keep tooltip rows based on the full `spec.series`, so it can show total plus all three paths. Render the arrival equation as a low-emphasis legend note and add only the minimal wrapping styles needed for desktop and mobile widths.

- [ ] **Step 4: Run the browser check and verify GREEN**

Run the Task 3 browser command again. Expected: all checks pass without horizontal overflow.

- [ ] **Step 5: Commit the renderer behavior**

```bash
git add frontend/src/vendor/digital-twin/charts.js frontend/src/vendor/digital-twin/styles.css frontend/src/digital-twin.browser.mjs
git commit -m "feat: render complete arrival path stacks"
```

### Task 4: Carry the field through the frontend API and fixtures

**Files:**
- Modify: `frontend/src/api-digital-twin.ts`
- Modify: `frontend/src/pages/DigitalTwin.tsx`
- Modify: `frontend/src/pages/DigitalTwin.test.ts`
- Modify: `scripts/digital-twin-preview.mjs`

- [ ] **Step 1: Write a failing mapping test**

Extend the T+1 fixture with `visit_front_desk: 3` and expect:

```ts
expect(patch.trends?.[0]).toMatchObject({
  visitAll: 55,
  frontDesk: 3,
  noConsult: 34,
  consult: 18,
});
```

- [ ] **Step 2: Run the focused frontend test and verify RED**

```bash
cd frontend && npm test -- src/pages/DigitalTwin.test.ts
```

Expected: FAIL because `frontDesk` is not mapped.

- [ ] **Step 3: Implement the frontend mapping**

Add `visit_front_desk: number | null` to `DigitalTwinT1BI`, map it to `frontDesk`, and update the 30-day preview fixture to return a small explicit front-desk value while keeping `visit_all` equal to the three paths.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the Task 4 command again. Expected: PASS.

- [ ] **Step 5: Commit the frontend contract**

```bash
git add frontend/src/api-digital-twin.ts frontend/src/pages/DigitalTwin.tsx frontend/src/pages/DigitalTwin.test.ts scripts/digital-twin-preview.mjs
git commit -m "feat: map front desk arrival metrics"
```

### Task 5: Version, regression verification, and test release

**Files:**
- Modify: `VERSION`
- Modify: `docs/codex-learning-state.md`

- [ ] **Step 1: Bump the version to `4.8.0`**

This is an existing-module business presentation and chart behavior iteration, so increment the middle version and reset the patch number.

- [ ] **Step 2: Run the complete verification suite**

```bash
cd frontend && npm test -- --run
cd frontend && npm run build
GOCACHE="$PWD/.cache/go-build" GOTMPDIR="$PWD/.cache/go-tmp" ./.tools/go/bin/go test -c ./cmd/server -o /private/tmp/erzhuang-4.8.0-server.test
GOCACHE="$PWD/.cache/go-build" GOTMPDIR="$PWD/.cache/go-tmp" ./.tools/go/bin/go test -c ./internal/digitaltwin/t1bi -o /private/tmp/erzhuang-4.8.0-t1bi.test
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOCACHE="$PWD/.cache/go-build" GOTMPDIR="$PWD/.cache/go-tmp" ./.tools/go/bin/go build -o /private/tmp/erzhuang-4.8.0-server ./cmd/server
```

Expected: all tests and builds pass; only the existing Vite chunk-size warning may remain.

- [ ] **Step 3: Run independent browser acceptance**

Verify desktop, laptop, and mobile widths; complete and incomplete arrival days; new tooltip units; no old labels; and no horizontal overflow. Use an isolated headless browser, not the user's active Chrome page.

- [ ] **Step 4: Record the release result**

Add the version, commit, verification results, fallback behavior, and remaining dependency on the upstream `visit_front_desk` field to `docs/codex-learning-state.md`.

- [ ] **Step 5: Commit and publish to the test branch**

```bash
git add VERSION docs/codex-learning-state.md
git commit -m "chore: release digital twin bi labels 4.8.0"
git push origin codex/digital-twin-module
```

Then create an isolated test release from `gitlab/codex/containerize-single-image`, merge or cherry-pick the 4.8.0 commits while retaining test environment configuration, rerun verification, and normally push:

```bash
git push gitlab codex/containerize-single-image
```

Do not modify GitLab `main`, Dockerfile, nginx, CI, Secret, database, or instance configuration.
