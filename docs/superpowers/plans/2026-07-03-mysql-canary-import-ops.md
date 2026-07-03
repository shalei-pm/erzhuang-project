# MySQL Canary Import Ops Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a guarded admin-only ops endpoint that can dry-run and apply the reviewed PostgreSQL-to-MySQL `external_org_id=10030` canary import inside the company runtime.

**Architecture:** Reuse the existing ops handler pattern. The HTTP endpoint validates scope and SQL shape, then delegates execution to an injectable runner. The default runner opens a MySQL connection from runtime/K8s secret env vars, optionally executes the import SQL, and returns compact validation counts without exposing secrets.

**Tech Stack:** Go `net/http`, `database/sql`, `github.com/go-sql-driver/mysql`, existing auth/ops guard, existing Go tests.

---

### Task 1: Define Request, Response, Runner, and Route

**Files:**
- Modify: `internal/app/ops_handler.go`
- Modify: `internal/app/handler.go`
- Test: `internal/app/handler_test.go`

- [ ] Add `mysqlCanaryImportRequest`, `mysqlCanaryImportResponse`, `mysqlCanaryImportRunRequest`, `mysqlCanaryImportRunResult`, and `mysqlCanaryImportRunner`.
- [ ] Add package-level `currentMySQLCanaryImportRunner = runMySQLCanaryImportFromEnv`.
- [ ] Add `POST /api/admin/ops/mysql-canary-import`.
- [ ] Add tests proving endpoint is hidden unless ops is enabled and requires admin permission.

### Task 2: Validate Canary Import Input

**Files:**
- Modify: `internal/app/ops_handler.go`
- Test: `internal/app/handler_test.go`

- [ ] Validate request body size.
- [ ] Require `import_sql`.
- [ ] Default `external_org_id` to `10030`.
- [ ] Reject `external_org_id` other than `10030`.
- [ ] Reject `apply=true` unless SQL contains `-- Scope external_org_id: 10030`.
- [ ] Reject SQL mentioning other `external_org_id` literals in `tb_stores` insert statements.
- [ ] Return `400` before calling runner when validation fails.

### Task 3: Implement MySQL Runner

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/app/ops_handler.go`
- Test: `internal/app/handler_test.go`

- [ ] Add `github.com/go-sql-driver/mysql`.
- [ ] Read DSN from `MYSQL_DSN` or `K8S_SECRET_MYSQL_DSN`.
- [ ] For dry-run, connect and run lightweight environment/table checks but do not execute import SQL.
- [ ] For apply, execute import SQL inside a transaction.
- [ ] Run validation queries for `10030`: store count, recorder count, channel count, snapshot count, orphan counts, invalid JSON count.
- [ ] Return sanitized errors.

### Task 4: Documentation and Verification

**Files:**
- Modify: `docs/codex-learning-state.md`
- Modify: `docs/postgres-to-mysql-data-migration-runbook.md`

- [ ] Document the endpoint, dry-run/apply sequence, and sensitive-data handling.
- [ ] Run `go test ./internal/app` compile/test gate where possible.
- [ ] Run `go test ./internal/mysqlmigration`.
- [ ] Run `go build -o /private/tmp/server-check ./cmd/server`.
- [ ] Commit and publish to company only after tests pass.
