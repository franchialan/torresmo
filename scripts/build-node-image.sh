#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
IMAGE_NAME="${IMAGE_NAME:-torresmo/node}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG}..."

docker build \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f "${PROJECT_DIR}/internal/image/node/Dockerfile" \
    "${PROJECT_DIR}/internal/image/node/"

echo "✓ Image ${IMAGE_NAME}:${IMAGE_TAG} built successfully"
echo ""
echo "To use: torresmo create cluster --name dev --nodes 3"
