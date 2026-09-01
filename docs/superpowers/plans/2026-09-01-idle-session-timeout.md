# Idle Session Timeout Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Enforce a server-side 30-minute idle timeout for authenticated users, revoke the local session and company SSO session after timeout, and require a new QR-code login on the next visit.

**Architecture:** Add a shared authentication-session port implemented by MySQL and an in-memory fake for tests. Every protected request validates and atomically touches an opaque HttpOnly session cookie stored as a SHA-256 hash in tb_auth_sessions; expired sessions are revoked and return a structured 401. The React shell recognizes that code and uses the existing company SSO logout path before redirecting to login.

**Tech Stack:** Go 1.22, net/http, database/sql, MySQL 8, React, TypeScript, Vitest, existing APISIX SSO and audit-log abstractions.

---

## File Map

- Create: internal/app/auth_session.go - session port, cookie constants, timeout decision, and safe cookie helpers.
- Create: internal/app/auth_session_test.go - deterministic session lifecycle and timeout tests.
- Modify: internal/app/auth.go - create/clear the local session during authentication and expose the timeout response.
- Modify: internal/app/authz.go - apply session validation to every protected authorization path.
- Modify: internal/app/audit_recorder.go - record auth.idle_timeout without sensitive values.
- Modify: internal/app/handler.go - inject the session store and keep public routes outside the protected activity gate.
- Modify: internal/app/mysql_store.go - implement session create, atomic touch, revoke, and cleanup queries.
- Modify: internal/app/handler_test.go and internal/app/auth_logout_test.go - update handler fixtures and protect existing SSO/logout behavior.
- Modify: db/mysql_governance_schema_tb.sql - make the tb_auth_sessions DDL match the implementation, including idle fields and indexes.
- Create: db/mysql_auth_sessions.sql - DBA-facing inspection and migration SQL; application startup will not execute it.
- Modify: frontend/src/api.ts - preserve structured code values for auth failures and include credentials in JSON requests.
- Modify: frontend/src/api-h5.ts and frontend/src/api-nvr-lab.ts - expose the same structured timeout error to H5/NVR flows.
- Modify: frontend/src/App.tsx - centralize timeout handling and redirect through the existing SSO joint logout path.
- Modify: frontend/src/domain/auth.ts - add a timeout error-code predicate and preserve current test/prod SSO host behavior.
- Modify: frontend/src/domain/auth.test.ts - verify timeout classification and redirect inputs.
- Modify: docs/mysql-test-table-inventory.md - record actual test DDL verification and the new session table status.
- Modify: docs/codex-learning-state.md - record implementation, tests, test deployment, and rollback point.

## Task 0: Create the isolated feature branch

**Files:**
- No source files

- [ ] Step 1: Confirm the documentation commit is the current clean base.

Run:

~~~bash
git status --short
git log -1 --oneline
~~~

Expected: the two approved design documents are committed, and no tracked source changes are present.

- [ ] Step 2: Create the feature branch.

Run:

~~~bash
git switch -c codex/idle-session-timeout
~~~

Expected: the branch is created from the approved design commit and no production branch is changed.

## Task 1: Define the session contract and test fixtures

**Files:**
- Create: internal/app/auth_session.go
- Create: internal/app/auth_session_test.go
- Modify: internal/app/handler.go

- [ ] Step 1: Add failing unit tests for the session contract.

Define tests using a fixed clock and an in-memory fake implementing:

~~~go
type authSessionStore interface {
    CreateAuthSession(context.Context, AuthSessionCreate) (string, error)
    TouchAuthSession(context.Context, string, int64, time.Time, time.Duration) (bool, error)
    RevokeAuthSession(context.Context, string, string, time.Time) error
}
~~~

Cover: a new session returns a non-empty opaque value; a request at 29*time.Minute+59*time.Second touches successfully; a request at 30*time.Minute+1*time.Second returns false; a revoked session cannot touch; the raw session value is never stored by the fake.

- [ ] Step 2: Run the focused tests and verify they fail.

~~~bash
go test ./internal/app -run 'TestAuthSession' -count=1
~~~

Expected: FAIL because the session types and implementation do not exist yet.

- [ ] Step 3: Implement the minimal session contract.

Add:

~~~go
const (
    authSessionCookieName = "erzhuang_session"
    defaultAuthIdleTimeout = 30 * time.Minute
)

type AuthSessionCreate struct {
    UserID     int64
    SSOSubject string
    IPAddress  string
    UserAgent  string
    Now        time.Time
}
~~~

Generate at least 32 random bytes with crypto/rand, return the base64url value to the cookie writer, and hash it with SHA-256 before passing it to storage. Add a mutex-protected fake store and an injectable now function for deterministic tests.

- [ ] Step 4: Run the focused tests and verify they pass.

~~~bash
go test ./internal/app -run 'TestAuthSession' -count=1
~~~

Expected: PASS.

- [ ] Step 5: Commit the contract and test fixture.

~~~bash
git add internal/app/auth_session.go internal/app/auth_session_test.go internal/app/handler.go
git commit -m "feat: add idle auth session contract"
~~~

## Task 2: Add MySQL persistence and DBA migration material

**Files:**
- Modify: internal/app/mysql_store.go
- Modify: db/mysql_governance_schema_tb.sql
- Create: db/mysql_auth_sessions.sql
- Test: internal/app/auth_session_test.go

- [ ] Step 1: Write SQL contract tests.

Assert the MySQL implementation contains these constraints and operations:

~~~sql
unique key uq_tb_auth_sessions_token_hash (session_token_hash)
key idx_tb_auth_sessions_expires_at (expires_at)
update tb_auth_sessions
set last_activity_at = ?, expires_at = ?
where session_token_hash = ?
  and user_id = ?
  and revoked_at is null
  and expires_at > ?
~~~

The test must also reject queries containing raw session values in logs or SQL comments.

- [ ] Step 2: Run the SQL contract tests and verify they fail.

~~~bash
go test ./internal/app -run 'TestMySQLAuthSession' -count=1
~~~

Expected: FAIL because the DDL lacks last_activity_at and the MySQL methods are absent.

- [ ] Step 3: Update the schema and implement MySQL methods.

Extend tb_auth_sessions with:

~~~sql
last_activity_at datetime(3) not null,
key idx_tb_auth_sessions_user_activity (user_id, last_activity_at),
~~~

Implement CreateAuthSession, TouchAuthSession, and RevokeAuthSession on MySQLStore. TouchAuthSession must perform the conditional update and return RowsAffected() == 1; it must never fall back to a non-conditional update. CreateAuthSession stores only sha256(rawSession), sets last_activity_at = created_at, and sets expires_at = created_at + 30 minutes.

Create db/mysql_auth_sessions.sql with a preflight query for the table, columns, and indexes, followed by guarded migration statements for last_activity_at. The migration must initialize existing rows from created_at before changing the column to NOT NULL. It must contain no credentials and must not be invoked by application startup.

- [ ] Step 4: Run focused tests and inspect the migration.

~~~bash
go test ./internal/app -run 'TestMySQLAuthSession' -count=1
git diff --check
~~~

Expected: PASS and no whitespace errors.

- [ ] Step 5: Commit persistence and migration material.

~~~bash
git add internal/app/mysql_store.go db/mysql_governance_schema_tb.sql db/mysql_auth_sessions.sql internal/app/auth_session_test.go
git commit -m "feat: persist idle auth sessions in mysql"
~~~

## Task 3: Enforce sessions in the Go authentication path

**Files:**
- Modify: internal/app/auth.go
- Modify: internal/app/authz.go
- Modify: internal/app/handler.go
- Modify: internal/app/audit_recorder.go
- Test: internal/app/auth_session_test.go, internal/app/handler_test.go, internal/app/auth_logout_test.go

- [ ] Step 1: Add failing handler tests.

Use a signed SSO JWT, a fake authSessionStore, and a fixed clock to assert:

~~~go
response.Code == http.StatusUnauthorized
body.Code == "session_idle_timeout"
response.Header().Get("Set-Cookie") contains "erzhuang_session=;"
fakeSession.revokedReason == "idle_timeout"
audit.Action == "auth.idle_timeout"
~~~

Also assert that GET /api/auth/me, store APIs, H5 APIs, NVR APIs, and direct monitor authorization all share the same session check; public /health, SSO callback, and /logout remain outside the activity check.

- [ ] Step 2: Run the new handler tests and verify they fail.

~~~bash
go test ./internal/app -run 'TestAuth.*Idle|TestProtected.*Session|TestAuth.*Session' -count=1
~~~

Expected: FAIL because protected handlers currently trust only the SSO cookie.

- [ ] Step 3: Add session creation and protected-request touch.

Add sessionStore authSessionStore and an injectable clock/idle-timeout dependency to Handler. In newHandlerWithServices, use the MySQL implementation when the store supports it and the in-memory implementation for memory-backed tests. When SSO is enabled and no session store is available, fail closed with a service error instead of silently bypassing the policy.

Refactor the shared authenticated-user path so it:

1. validates the SSO JWT;
2. loads the enabled user;
3. creates a local session when the authenticated request has no local session cookie;
4. conditionally touches the existing local session;
5. maps expired or revoked local sessions to errIdleSessionTimeout.

authMeHandler must use this shared path. requirePermission, nvrLabAdminGuard, and both monitor authorizers must use it indirectly through currentAuthUser, so no protected route can bypass the check. Do not add a timer or goroutine in the server.

- [ ] Step 4: Implement timeout response and logout cleanup.

Add:

~~~json
{
  "enabled": true,
  "authenticated": false,
  "code": "session_idle_timeout",
  "message": "登录已因长时间未操作失效，请重新扫码登录"
}
~~~

Before writing the response, revoke the session and reuse clearAuthCookie for both the local session cookie and the existing SSO cookie domains. Keep POST /api/auth/logout idempotent and revoke the local session when present. Keep GET /logout as the browser redirect endpoint that can continue to the company SSO joint logout path.

- [ ] Step 5: Add the timeout audit event.

Record auth.idle_timeout with the authenticated user identity, result success, request metadata, and detail_json containing only reason: "idle_timeout". Do not include raw cookies, hashes, JWT claims, URLs, or media data.

- [ ] Step 6: Run the full backend tests.

~~~bash
gofmt -w internal/app/auth_session.go internal/app/auth_session_test.go internal/app/auth.go internal/app/authz.go internal/app/handler.go internal/app/mysql_store.go internal/app/audit_recorder.go
go test ./... -count=1 -timeout=120s
go vet ./...
go build ./...
git diff --check
~~~

Expected: all tests pass, vet is clean, the binary builds, and no sensitive value appears in test output.

- [ ] Step 7: Commit the backend enforcement.

~~~bash
git add internal/app/auth_session.go internal/app/auth_session_test.go internal/app/auth.go internal/app/authz.go internal/app/handler.go internal/app/mysql_store.go internal/app/audit_recorder.go internal/app/handler_test.go internal/app/auth_logout_test.go
git commit -m "feat: enforce idle auth session timeout"
~~~

## Task 4: Handle timeout consistently in the React clients

**Files:**
- Modify: frontend/src/api.ts
- Modify: frontend/src/api-h5.ts
- Modify: frontend/src/api-nvr-lab.ts
- Modify: frontend/src/App.tsx
- Modify: frontend/src/domain/auth.ts
- Test: frontend/src/domain/auth.test.ts and existing API tests

- [ ] Step 1: Add failing frontend tests.

Assert that session_idle_timeout is recognized as a forced-authentication error and that the existing authLogoutPath("lite.sy.soyoung.com") remains the redirect target. Assert that ordinary 401, 403, and session_idle_timeout remain distinct.

- [ ] Step 2: Run the focused frontend tests and verify they fail.

~~~bash
npm --prefix frontend test -- --run frontend/src/domain/auth.test.ts
~~~

Expected: FAIL for the new timeout predicate.

- [ ] Step 3: Implement a shared timeout response path.

Keep the existing ApiError, H5ApiError, and NVRLabApiError status/code parsing. Add this predicate in frontend/src/domain/auth.ts:

~~~ts
export function isIdleSessionTimeout(error: { status?: number; code?: string } | null | undefined) {
  return error?.status === 401 && error.code === "session_idle_timeout";
}
~~~

In App.tsx, when initial getAuthMe() receives this code, clear the React auth state and navigate once to authLogoutPath(). Use sessionStorage only as a one-navigation loop guard and clear it after authenticated state is restored. Existing page-level 401 handlers must call the same callback or return to the shell so H5/NVR requests cannot leave stale authenticated UI visible.

- [ ] Step 4: Run frontend tests and build.

~~~bash
npm --prefix frontend test -- --run
npm --prefix frontend run build
git diff --check
~~~

Expected: all frontend tests pass and the production bundle builds.

- [ ] Step 5: Commit the frontend timeout handling.

~~~bash
git add frontend/src/api.ts frontend/src/api-h5.ts frontend/src/api-nvr-lab.ts frontend/src/App.tsx frontend/src/domain/auth.ts frontend/src/domain/auth.test.ts
git commit -m "feat: redirect expired sessions through sso logout"
~~~

## Task 5: Verify the DDL and publish only to the test environment

**Files:**
- Modify: docs/mysql-test-table-inventory.md
- Modify: docs/codex-learning-state.md
- Reference: docs/deploy-runbook.md
- Reference: docs/frontend-review-checklist.md

- [ ] Step 1: Run the repository verification suite before publishing.

~~~bash
go test ./... -count=1 -timeout=120s
go build ./...
npm --prefix frontend test -- --run
npm --prefix frontend run build
git diff --check
~~~

Expected: all checks pass. The release remains on the test branch; no GitLab main operation is allowed.

- [ ] Step 2: Have the test database operator run the preflight.

Use db/mysql_auth_sessions.sql to confirm tb_auth_sessions, last_activity_at, the unique token-hash index, and the expiry index. If the table is absent or the column is absent, execute only the reviewed DDL in the test database and record the result. Do not run the DDL against production in this plan.

- [ ] Step 3: Publish through the normal test branch flow.

~~~bash
git fetch gitlab
git switch codex/containerize-single-image
git merge codex/idle-session-timeout
go test ./... -count=1 -timeout=120s
go build ./...
npm --prefix frontend test -- --run
npm --prefix frontend run build
git push gitlab codex/containerize-single-image
~~~

Wait for Wharf pipeline 752 and the automatic test deployment. Do not click manual production deployment and do not modify main.

- [ ] Step 4: Perform browser acceptance in the existing test tab.

Use the existing Chrome/plugin tab and verify:

- authenticated user can load the home page and protected data;
- an active request before the threshold keeps the session valid;
- after a controlled timeout test, refresh returns 401/session_idle_timeout and the browser reaches the company SSO logout flow;
- the next entry requires QR login;
- manual logout, permissions, monitor live/replay, audit log, and user management still work;
- browser console has no project errors and page version matches the deployed commit.

For acceptance without waiting 30 minutes, use a test-only injected clock/fake or a temporary test environment timeout setting that is removed before reporting completion; the production default remains exactly 30 minutes.

- [ ] Step 5: Update the table inventory and learning state.

Record the verified test database result, the tb_auth_sessions DDL execution time, commit, Wharf build/deployment IDs, page version, acceptance results, and rollback commit. Keep tb_nvr_camera_snapshots marked “废止未创建”; do not add it to the formal synchronization list.

- [ ] Step 6: Commit release documentation.

~~~bash
git add docs/mysql-test-table-inventory.md docs/codex-learning-state.md
git commit -m "docs: record idle session timeout test release"
~~~

## Task 6: Rollback rehearsal and completion gate

**Files:**
- Modify: docs/codex-learning-state.md

- [ ] Step 1: Define the rollback point.

Record the last known-good test commit immediately before the session enforcement commit and verify it still uses the current MySQL/OSS configuration.

- [ ] Step 2: Rehearse code-only rollback in test.

Revert to the recorded test commit through the normal GitLab test branch flow, verify /health, SSO login, and one protected page, then restore the feature commit. Do not drop tb_auth_sessions or delete audit rows.

- [ ] Step 3: Record residual risk.

Document that a user who has an active request continuously can keep the session alive, because the requirement defines activity as a request; network failures during joint SSO logout may require the next navigation to retry logout, but the local session and SSO cookie are already cleared by the server response.

- [ ] Step 4: Run the completion gate.

~~~bash
git status --short
git log --oneline -8
git diff --check
~~~

Expected: only intentionally preserved untracked workspace files remain, the feature and release records are committed, and no production branch or production database was changed.

