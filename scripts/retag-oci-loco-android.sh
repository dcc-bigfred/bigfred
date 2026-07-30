#!/usr/bin/env bash
# Retag the loco-server-android-arm64 OCI artifact from :main to a release tag,
# injecting .bigfred.version (ELF section) with the release tag + tag commit.
# Optionally signs the binary with minisign and pushes .minisig as a second layer
# when MINISIGN_SECRET_KEY is set.
# Usage: retag-oci-loco-android.sh <release-tag>   e.g. v1.2.3
#
# Requires: GITHUB_SHA (tag commit), oras, objcopy (binutils).
# Optional: MINISIGN_SECRET_KEY, MINISIGN_PASSWORD, minisign on PATH.
set -euo pipefail

RELEASE_TAG="${1:?usage: $0 <release-tag>}"
IMAGE="${BIGFRED_OCI_IMAGE:-ghcr.io/dcc-bigfred/loco-server-android-arm64}"
MEDIA_TYPE="application/vnd.dcc-bigfred.loco-server.android.arm64.v1"
MINISIG_MEDIA_TYPE="application/vnd.dcc-bigfred.minisig.v1"
TAG_COMMIT="${GITHUB_SHA:?GITHUB_SHA required (tag commit)}"
TAG_COMMIT_SHORT="${TAG_COMMIT::7}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

echo "Pulling ${IMAGE}:main…"
oras pull "${IMAGE}:main" -o "${tmpdir}"

BINARY="${tmpdir}/loco-server-android-arm64"
if [[ ! -f "${BINARY}" ]]; then
  mapfile -t files < <(find "${tmpdir}" -type f ! -name 'manifest.json' ! -name 'config.json')
  if [[ ${#files[@]} -eq 1 ]]; then
    BINARY="${files[0]}"
  else
    echo "error: expected loco-server-android-arm64 in OCI artifact, found:" >&2
    find "${tmpdir}" -type f >&2
    exit 1
  fi
fi

"${SCRIPT_DIR}/inject-elf-version.sh" "${BINARY}" "${RELEASE_TAG}" "${TAG_COMMIT_SHORT}"

push_args=(
  "${BINARY}:${MEDIA_TYPE}"
)

if [[ -n "${MINISIGN_SECRET_KEY:-}" ]]; then
  if ! command -v minisign >/dev/null 2>&1; then
    echo "error: MINISIGN_SECRET_KEY set but minisign not on PATH" >&2
    exit 1
  fi
  "${SCRIPT_DIR}/minisign-sign.sh" "${BINARY}"
  push_args+=("${BINARY}.minisig:${MINISIG_MEDIA_TYPE}")
else
  echo "MINISIGN_SECRET_KEY unset — publishing unsigned OCI artifact"
fi

annotate=(
  --annotation "org.opencontainers.image.source=${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-dcc-bigfred/bigfred}"
  --annotation "org.opencontainers.image.revision=${TAG_COMMIT}"
  --annotation "org.opencontainers.image.version=${RELEASE_TAG}"
)

echo "Publishing ${IMAGE}:${RELEASE_TAG} ($(wc -c < "${BINARY}") bytes)"
oras push "${IMAGE}:${RELEASE_TAG}" "${push_args[@]}" "${annotate[@]}"
