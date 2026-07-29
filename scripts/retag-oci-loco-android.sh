#!/usr/bin/env bash
# Retag the loco-server-android-arm64 OCI artifact from :main to a release tag.
# Usage: retag-oci-loco-android.sh <release-tag>   e.g. v1.2.3
set -euo pipefail

RELEASE_TAG="${1:?usage: $0 <release-tag>}"
IMAGE="${BIGFRED_OCI_IMAGE:-ghcr.io/dcc-bigfred/loco-server-android-arm64}"

echo "Retagging ${IMAGE}:main → ${IMAGE}:${RELEASE_TAG}"
oras copy "${IMAGE}:main" "${IMAGE}:${RELEASE_TAG}"
