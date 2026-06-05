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
  - status: active

