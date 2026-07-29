#!/usr/bin/env bash
# Publish loco-server-android-arm64 to GHCR as an OCI artifact (ORAS).
# Intended for CI on push to master/main only.
# Usage: publish-oci-loco-android.sh <path-to-binary>
#
# Tags: main, sha-<7>
set -euo pipefail

BINARY="${1:?usage: $0 <path-to-loco-server-android-arm64>}"
IMAGE="${BIGFRED_OCI_IMAGE:-ghcr.io/dcc-bigfred/loco-server-android-arm64}"
MEDIA_TYPE="application/vnd.dcc-bigfred.loco-server.android.arm64.v1"

if [[ ! -f "${BINARY}" ]]; then
  echo "error: binary not found: ${BINARY}" >&2
  exit 1
fi

BRANCH="${GITHUB_REF_NAME:?GITHUB_REF_NAME required}"
if [[ "${BRANCH}" != "master" && "${BRANCH}" != "main" ]]; then
  echo "error: OCI publish is only allowed from master/main (got ${BRANCH})" >&2
  exit 1
fi

SHA_TAG="sha-${GITHUB_SHA::7}"

annotate=(
  --annotation "org.opencontainers.image.source=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}"
  --annotation "org.opencontainers.image.revision=${GITHUB_SHA}"
)

echo "Publishing ${IMAGE}:main and :${SHA_TAG} ($(wc -c < "${BINARY}") bytes)"
oras push "${IMAGE}:main" "${BINARY}:${MEDIA_TYPE}" "${annotate[@]}"
oras push "${IMAGE}:${SHA_TAG}" "${BINARY}:${MEDIA_TYPE}" "${annotate[@]}"
