#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/apps/erzhuang-project}"
SERVICE_NAME="${SERVICE_NAME:-erzhuang-project.service}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:18081/health}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/erzhuang_project_deploy_key}"
BUILD_OUTPUT="${BUILD_OUTPUT:-erzhuang-project}"
BUILD_TARGET="${BUILD_TARGET:-./cmd/server}"

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

echo "==> Restarting ${SERVICE_NAME}"
sudo systemctl restart "${SERVICE_NAME}"
sudo systemctl status "${SERVICE_NAME}" --no-pager

echo "==> Checking health"
curl -fsS "${HEALTH_URL}"
echo

echo "==> Deploy complete"
