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
  - status: active
- Frontend Phase 2:
  - scope: React page, store list, editor modal, area cards, floor-plan annotation UI, mock/API adapter
  - branch: `codex/design-plan-frontend-phase2`
  - thread: `019e978c-f41f-78d0-a5db-6b940b928c3f`
  - worktree: `/Users/sylar/.codex/worktrees/34e2/erzhuang-project`
  - status: active
  - mock fixture: `testdata/design-plans/generated/sample-store-floor-plan.png`
