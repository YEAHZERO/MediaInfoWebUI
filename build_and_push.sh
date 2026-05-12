#!/bin/bash
set -e

VERSION="1.5.4"
REGISTRY="ghcr.io/yeahzero/mediainfowebui"
PLATFORMS="linux/amd64"

echo "================================================"
echo "  MediaInfoWebUI v${VERSION} Build & Push"
echo "================================================"

build_and_push() {
    local tag=$1
    local target=$2
    local full_tag="${REGISTRY}:${tag}"

    echo ""
    echo "[1/2] Building ${full_tag} ..."
    docker build \
        --network=host \
        --target "${target}" \
        -t "${full_tag}" .

    echo ""
    echo "[2/2] Pushing ${full_tag} ..."
    docker push "${full_tag}"
}

echo ""
echo ">>> Building light ..."
build_and_push "light" "runtime-light"

echo ""
echo ">>> Building latest (native, 推荐) ..."
build_and_push "latest" "runtime-native"

echo ""
echo "================================================"
echo "  All images built and pushed successfully!"
echo "================================================"
echo ""
echo "  ${REGISTRY}:light"
echo "  ${REGISTRY}:latest"
echo ""
echo "  Version: v${VERSION}"
echo "================================================"