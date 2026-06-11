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
