#!/usr/bin/env bash
# Retag the hub linux/arm64 OCI artifact from :master to a release tag and
# latest-release. Injects .bigfred.version into loco-server and signs both
# binaries with minisign (fail-closed).
# Usage: retag-oci-bigfred-hub.sh <release-tag>   e.g. v1.2.3
#
# Requires: GITHUB_SHA (tag commit), oras, objcopy (binutils), minisign,
#           MINISIGN_SECRET_KEY (and optional MINISIGN_PASSWORD).
set -euo pipefail

RELEASE_TAG="${1:?usage: $0 <release-tag>}"
IMAGE="${BIGFRED_HUB_OCI_IMAGE:-ghcr.io/dcc-bigfred/bigfred-hub-linux-arm64}"
SERVER_MEDIA_TYPE="application/vnd.dcc-bigfred.loco-server.linux.arm64.v1"
ICMP_MEDIA_TYPE="application/vnd.dcc-bigfred.remote-icmp.linux.arm64.v1"
MINISIG_MEDIA_TYPE="application/vnd.dcc-bigfred.minisig.v1"
TAG_COMMIT="${GITHUB_SHA:?GITHUB_SHA required (tag commit)}"
TAG_COMMIT_SHORT="${TAG_COMMIT::7}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

echo "Pulling ${IMAGE}:master…"
oras pull "${IMAGE}:master" -o "${tmpdir}"

SERVER_BIN="${tmpdir}/loco-server-linux-arm64"
ICMP_BIN="${tmpdir}/bigfred-remote-icmp-linux-arm64"
if [[ ! -f "${SERVER_BIN}" || ! -f "${ICMP_BIN}" ]]; then
  echo "error: expected loco-server-linux-arm64 and bigfred-remote-icmp-linux-arm64 in OCI artifact, found:" >&2
  find "${tmpdir}" -type f >&2
  exit 1
fi

"${SCRIPT_DIR}/inject-elf-version.sh" "${SERVER_BIN}" "${RELEASE_TAG}" "${TAG_COMMIT_SHORT}"

if [[ -z "${MINISIGN_SECRET_KEY:-}" ]]; then
  echo "error: MINISIGN_SECRET_KEY is required to publish a signed OCI release (fail-closed)" >&2
  exit 1
fi
if ! command -v minisign >/dev/null 2>&1; then
  echo "error: minisign not on PATH" >&2
  exit 1
fi

"${SCRIPT_DIR}/minisign-sign.sh" "${SERVER_BIN}" "${ICMP_BIN}"

push_args=(
  "${SERVER_BIN}:${SERVER_MEDIA_TYPE}"
  "${ICMP_BIN}:${ICMP_MEDIA_TYPE}"
  "${SERVER_BIN}.minisig:${MINISIG_MEDIA_TYPE}"
  "${ICMP_BIN}.minisig:${MINISIG_MEDIA_TYPE}"
)

annotate=(
  --annotation "org.opencontainers.image.source=${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-dcc-bigfred/bigfred}"
  --annotation "org.opencontainers.image.revision=${TAG_COMMIT}"
  --annotation "org.opencontainers.image.version=${RELEASE_TAG}"
)

echo "Publishing ${IMAGE}:${RELEASE_TAG} and :latest-release"
echo "  loco-server: $(wc -c < "${SERVER_BIN}") bytes"
echo "  remote-icmp: $(wc -c < "${ICMP_BIN}") bytes"
oras push "${IMAGE}:${RELEASE_TAG}" "${push_args[@]}" "${annotate[@]}"
oras push "${IMAGE}:latest-release" "${push_args[@]}" "${annotate[@]}"
