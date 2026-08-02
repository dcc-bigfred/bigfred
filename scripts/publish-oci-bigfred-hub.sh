#!/usr/bin/env bash
# Publish hub linux/arm64 binaries to GHCR as one OCI artifact (ORAS).
# Intended for CI on push to master only.
# Usage: publish-oci-bigfred-hub.sh <loco-server-linux-arm64> <bigfred-remote-icmp-linux-arm64>
#
# Tags: master, sha-<7>
set -euo pipefail

SERVER_BIN="${1:?usage: $0 <loco-server-linux-arm64> <bigfred-remote-icmp-linux-arm64>}"
ICMP_BIN="${2:?usage: $0 <loco-server-linux-arm64> <bigfred-remote-icmp-linux-arm64>}"
IMAGE="${BIGFRED_HUB_OCI_IMAGE:-ghcr.io/dcc-bigfred/bigfred-hub-linux-arm64}"
SERVER_MEDIA_TYPE="application/vnd.dcc-bigfred.loco-server.linux.arm64.v1"
ICMP_MEDIA_TYPE="application/vnd.dcc-bigfred.remote-icmp.linux.arm64.v1"

if [[ ! -f "${SERVER_BIN}" ]]; then
  echo "error: binary not found: ${SERVER_BIN}" >&2
  exit 1
fi
if [[ ! -f "${ICMP_BIN}" ]]; then
  echo "error: binary not found: ${ICMP_BIN}" >&2
  exit 1
fi

BRANCH="${GITHUB_REF_NAME:?GITHUB_REF_NAME required}"
if [[ "${BRANCH}" != "master" ]]; then
  echo "error: hub OCI publish is only allowed from master (got ${BRANCH})" >&2
  exit 1
fi

SHA_TAG="sha-${GITHUB_SHA::7}"

# ORAS layer file names come from basename; stage fixed names for consumers.
tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

cp -f "${SERVER_BIN}" "${tmpdir}/loco-server-linux-arm64"
cp -f "${ICMP_BIN}" "${tmpdir}/bigfred-remote-icmp-linux-arm64"
chmod 755 "${tmpdir}/loco-server-linux-arm64" "${tmpdir}/bigfred-remote-icmp-linux-arm64"

annotate=(
  --annotation "org.opencontainers.image.source=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}"
  --annotation "org.opencontainers.image.revision=${GITHUB_SHA}"
)

layers=(
  "${tmpdir}/loco-server-linux-arm64:${SERVER_MEDIA_TYPE}"
  "${tmpdir}/bigfred-remote-icmp-linux-arm64:${ICMP_MEDIA_TYPE}"
)

echo "Publishing ${IMAGE}:master and :${SHA_TAG}"
echo "  loco-server: $(wc -c < "${tmpdir}/loco-server-linux-arm64") bytes"
echo "  remote-icmp: $(wc -c < "${tmpdir}/bigfred-remote-icmp-linux-arm64") bytes"
oras push "${IMAGE}:master" "${layers[@]}" "${annotate[@]}"
oras push "${IMAGE}:${SHA_TAG}" "${layers[@]}" "${annotate[@]}"
