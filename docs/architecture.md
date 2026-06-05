# Architecture And Collaboration Model

## Collaboration Structure

The project uses a hub-and-spoke Codex collaboration model.

The current main thread acts as the project architecture and delivery hub:

- clarify product requirements
- decide technical architecture
- split frontend/backend/deployment tasks
- define acceptance criteria
- coordinate specialist Codex threads
- review branch output
- decide whether to merge
- run deployment, verification, and rollback
- manage Tencent Cloud Lighthouse and nginx operations

Specialist threads are used when work becomes focused enough to isolate:

- frontend thread: `frontend/`, UI, frontend build, frontend verification
- backend thread: Go API, `cmd/server`, `internal/`, backend tests
- deployment thread, if needed later: infrastructure-only changes

## Responsibility Boundaries

Only the main architecture thread should operate production-like deployment paths:

- Tencent Cloud API/TAT
- Lighthouse instance operations
- nginx changes
- systemd deployment
- release verification
- rollback

Specialist implementation threads should:

- work on scoped branches or worktrees
- avoid touching server configuration
- avoid using cloud credentials
- avoid changing deployment scripts unless assigned
- document their state in `docs/*-learning-state.md`
- commit their changes to their working branch
- report verification and merge readiness back to the main thread

## Specialist Reporting Rhythm

Specialist threads should report progress to the main architecture thread at predictable checkpoints, not only at final handoff.

Required reporting checkpoints:

- After syncing `main` and creating or choosing a branch:
  - current branch
  - baseline commit
  - intended file scope
- After the main implementation draft:
  - changed files
  - core design choices
  - any scope expansion or architecture question
- Before verification:
  - planned commands
  - expected risks or local environment limitations
- After verification:
  - command results
  - failures and likely cause
  - whether the issue is code, environment, or product ambiguity
- Before commit:
  - staged file list
  - confirmation that build outputs, dependencies, `.DS_Store`, secrets, and server config are excluded
- After push:
  - branch
  - commit
  - validation result
  - known risks
  - suggested main-thread review focus

If a specialist thread hits a boundary question, it should report early instead of silently widening scope. Examples:

- frontend thread needing backend field changes
- backend thread needing UI behavior decisions
- any thread needing nginx, systemd, cloud, database secrets, OpenAI keys, or deploy script changes
- uncertainty about whether to refactor shared architecture before implementing a feature

## Branch Approval Delegation

The main architecture thread owns routine technical approval for specialist branches.

By default, the main thread may approve, request changes, merge, or reject specialist branch output without asking the user for every technical detail, as long as the work stays inside the agreed product scope and thread boundary.

Approval checks should include:

- product scope matches the assigned task
- implementation stays in the specialist thread's allowed files
- API contracts match the architecture documents
- tests or builds relevant to the change pass
- no secrets, cloud credentials, local-only files, build outputs, dependencies, or server config are committed
- rollback or recovery path is understood when the change affects deployed behavior

The main thread must ask the user before decisions that affect:

- product scope or user-facing workflow tradeoffs
- production-like deployment, rollback, nginx, systemd, Tencent Cloud, or database operations
- credentials, paid cloud resources, external AI/API usage, or irreversible data actions
- accepting a known major defect or skipping an important verification step

## Desired Operating Model

The user manages the main Codex architecture thread.

The main Codex architecture thread manages:

- frontend implementation thread
- backend implementation thread
- future specialist threads when useful

The main thread remains accountable for:

- product/technical coherence
- final review
- merge decision
- release
- production-like verification
- rollback readiness

## Current Specialist Threads

- Frontend setup thread:
  - branch: `frontend-setup`
  - scope: create `frontend/` with Vite + React + TypeScript
  - status: completed and standby
  - note: this thread is retired from business feature work; current frontend product work is assigned to Frontend Phase 2.

## Design Plan Marker Project

The design plan marker project should use the same hub-and-spoke model:

- Main architecture thread owns product scope, technical design, review, merge, deployment, verification, and rollback.
- Backend specialist thread owns Go API, PostgreSQL schema, file/PDF processing, AI recognition service integration, and tests.
- Frontend specialist thread owns React UI, store list, editor modal, floor-plan annotation interactions, and frontend verification.

Specialist threads must not use cloud credentials, database secrets, OpenAI API keys, Tencent Cloud API/TAT, nginx, systemd, or deployment scripts unless explicitly assigned by the main thread.

Planned specialist threads:

- Backend Phase 1:
  - scope: Go data model, PostgreSQL schema, CRUD APIs, validation, duplicate checking, operation logs
  - branch: `codex/design-plan-backend-phase1`
  - thread: `019e978c-9e0d-7f53-b48a-75679af9369b`
  - worktree: `/Users/sylar/.codex/worktrees/e6f9/erzhuang-project`
  - status: completed and merged to `main`
- Frontend Phase 2:
  - scope: React page, store list, editor modal, area cards, floor-plan annotation UI, mock/API adapter
  - branch: `codex/design-plan-frontend-phase2`
  - thread: `019e978c-f41f-78d0-a5db-6b940b928c3f`
  - worktree: `/Users/sylar/.codex/worktrees/34e2/erzhuang-project`
  - status: completed and merged to `main`; active again for API adapter follow-up
  - mock fixture: `testdata/design-plans/generated/sample-store-floor-plan.png`
