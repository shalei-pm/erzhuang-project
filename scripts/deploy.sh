#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/apps/erzhuang-project}"
SERVICE_NAME="${SERVICE_NAME:-erzhuang-project.service}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:18081/health}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/erzhuang_project_deploy_key}"
BUILD_OUTPUT="${BUILD_OUTPUT:-erzhuang-project}"
BUILD_TARGET="${BUILD_TARGET:-./cmd/server}"
FRONTEND_DIR="${FRONTEND_DIR:-frontend}"

GIT_SSH_COMMAND_VALUE="ssh -i ${SSH_KEY} -o IdentitiesOnly=yes"

echo "==> Deploying ${SERVICE_NAME}"
echo "    app dir: ${APP_DIR}"
echo "    health:  ${HEALTH_URL}"

cd "${APP_DIR}"

echo "==> Fetching latest main"
GIT_SSH_COMMAND="${GIT_SSH_COMMAND_VALUE}" git fetch origin
git switch -C main origin/main

echo "==> Current commit"
git rev-parse --short HEAD
git log --oneline -1

echo "==> Running tests"
go test ./...

echo "==> Building"
go build -o "${BUILD_OUTPUT}" "${BUILD_TARGET}"

if [[ -f "${FRONTEND_DIR}/package.json" ]]; then
  if command -v npm >/dev/null 2>&1; then
    echo "==> Building frontend"
    (
      cd "${FRONTEND_DIR}"
      npm install
      npm run build
    )
  else
    echo "==> Skipping frontend build: npm not found"
  fi
fi

echo "==> Restarting ${SERVICE_NAME}"
sudo systemctl restart "${SERVICE_NAME}"
sudo systemctl status "${SERVICE_NAME}" --no-pager

echo "==> Checking health"
for attempt in {1..15}; do
  if curl -fsS "${HEALTH_URL}"; then
    break
  fi
  if [[ "${attempt}" == "15" ]]; then
    echo "health check failed after ${attempt} attempts" >&2
    exit 1
  fi
  echo "health check attempt ${attempt} failed; retrying..."
  sleep 1
done
echo

echo "==> Deploy complete"
