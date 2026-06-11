# Single Image Containerization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one Docker image that contains the React production build and Go backend, with runtime configuration injected by environment variables for external database/K8s deployment.

**Architecture:** Use a multi-stage Dockerfile: Node builds `frontend/dist`, Go runs tests and builds `./cmd/server`, and a slim Debian runtime image contains only runtime tools, the binary, and static frontend files. The Go server remains the single process and serves API, `/health`, uploaded assets, and the bundled frontend under `/erzhuang/`.

**Tech Stack:** Go 1.22, net/http, Vite React TypeScript, Docker multi-stage builds, Debian slim runtime, shell build script.

---

## File Structure

- Create: `Dockerfile` — multi-stage image build for frontend, backend, and runtime.
- Create: `.dockerignore` — keep local dependencies, secrets, uploads, and build output out of Docker context.
- Create: `scripts/build-image.sh` — release-system-friendly image build entrypoint.
- Create: `docs/containerization.md` — build/run/K8s handoff guide without company secrets.
- Modify: `internal/app/handler.go` — optionally serve embedded or filesystem frontend static files under `/erzhuang/`.
- Modify: `internal/app/handler_test.go` — tests for static frontend serving behavior.
- Modify: `cmd/server/main.go` — default to container-accessible address when appropriate.
- Modify: `docs/codex-learning-state.md` — record what was added and how to verify it.

### Task 1: Container build scaffolding

- [x] **Step 1: Create `Dockerfile`**

```Dockerfile
# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS frontend-builder
WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.22-bookworm AS go-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/erzhuang-project ./cmd/server

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates poppler-utils \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/erzhuang-project /app/erzhuang-project
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

ENV ADDR=0.0.0.0:18080 \
    FRONTEND_DIR=/app/frontend/dist \
    UPLOAD_DIR=/app/uploads/design-plan

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
```

- [x] **Step 2: Create `.dockerignore`**

```text
.git
.gitignore
.DS_Store
.env
.env.*
.ssh/
bin/
dist/
frontend/dist/
erzhuang-project
*.out
*.test
frontend/node_modules/
.cache/
.tmp/
.tools/
.venv-tccli/
uploads/
```

- [x] **Step 3: Create `scripts/build-image.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-erzhuang-project}"
IMAGE_TAG="${IMAGE_TAG:-$(git rev-parse --short HEAD 2>/dev/null || echo local)}"
DOCKERFILE="${DOCKERFILE:-Dockerfile}"
CONTEXT="${CONTEXT:-.}"

IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"

echo "==> Building container image"
echo "    image:      ${IMAGE_REF}"
echo "    dockerfile: ${DOCKERFILE}"
echo "    context:    ${CONTEXT}"
echo "==> Command"
echo "docker build -f ${DOCKERFILE} -t ${IMAGE_REF} ${CONTEXT}"

docker build -f "${DOCKERFILE}" -t "${IMAGE_REF}" "${CONTEXT}"
```

### Task 2: Frontend static serving

- [ ] **Step 1: Write failing test in `internal/app/handler_test.go`**

```go
func TestServesFrontendIndexUnderErzhuang(t *testing.T) {
    t.Setenv("FRONTEND_DIR", t.TempDir())
    if err := os.WriteFile(filepath.Join(os.Getenv("FRONTEND_DIR"), "index.html"), []byte("<html>container frontend</html>"), 0o644); err != nil {
        t.Fatalf("write index: %v", err)
    }

    request := httptest.NewRequest(http.MethodGet, "/erzhuang/", nil)
    recorder := httptest.NewRecorder()

    NewHandler().ServeHTTP(recorder, request)

    if recorder.Code != http.StatusOK {
        t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
    }
    if !strings.Contains(recorder.Body.String(), "container frontend") {
        t.Fatalf("expected frontend index body, got %q", recorder.Body.String())
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/app -run TestServesFrontendIndexUnderErzhuang -count=1`

Expected: FAIL because `/erzhuang/` is not yet served.

- [ ] **Step 3: Implement minimal static handler**

Add `FRONTEND_DIR` support in `internal/app/handler.go`, while keeping `/api` and `/health` unchanged.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/app -run TestServesFrontendIndexUnderErzhuang -count=1`

Expected: PASS.

### Task 3: Verification and docs

- [ ] **Step 1: Verify Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Verify frontend production build**

Run: `npm ci && npm run build` from `frontend/`.

Expected: PASS and `frontend/dist` generated.

- [ ] **Step 3: Verify build script syntax**

Run: `bash -n scripts/build-image.sh`

Expected: no output and exit code 0.

- [ ] **Step 4: Verify Docker build if Docker is available**

Run: `IMAGE_TAG=local ./scripts/build-image.sh`

Expected: image `erzhuang-project:local` built successfully.

- [ ] **Step 5: Update docs**

Record containerization status and verification commands in `docs/codex-learning-state.md`.
