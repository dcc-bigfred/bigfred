#!/usr/bin/env bash
# Sign one or more files with minisign (writes <file>.minisig next to each),
# then verify each signature against the committed public key.
# Usage: minisign-sign.sh <file> [file...]
#
# Env:
#   MINISIGN_SECRET_KEY  — full contents of the minisign secret key file (required)
#   MINISIGN_PASSWORD    — password if the secret key is encrypted (optional)
#   MINISIGN_PUBLIC_KEY  — path to public key (default: minisign.pub next to repo root /
#                          or $SCRIPT_DIR/../minisign.pub)
#   GITHUB_REF_NAME / GITHUB_SHA — included in the trusted comment when set
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <file> [file...]" >&2
  exit 1
fi

: "${MINISIGN_SECRET_KEY:?MINISIGN_SECRET_KEY required}"

if ! command -v minisign >/dev/null 2>&1; then
  echo "error: minisign not found on PATH" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUBKEY="${MINISIGN_PUBLIC_KEY:-${SCRIPT_DIR}/../minisign.pub}"
if [[ ! -f "${PUBKEY}" ]]; then
  echo "error: public key not found: ${PUBKEY}" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

keyfile="${tmpdir}/minisign.key"
# Preserve exact file contents (including trailing newline).
printf '%s' "${MINISIGN_SECRET_KEY}" > "${keyfile}"
# minisign keys end with a newline; ensure one if the secret was pasted without it.
[[ "$(tail -c1 "${keyfile}" | wc -l)" -eq 0 ]] && printf '\n' >> "${keyfile}"
chmod 600 "${keyfile}"

trusted="BigFred ${GITHUB_REF_NAME:-unknown} ${GITHUB_SHA:-}"

for f in "$@"; do
  if [[ ! -f "${f}" ]]; then
    echo "error: file not found: ${f}" >&2
    exit 1
  fi
  if [[ "${f}" == *.minisig ]]; then
    echo "skip (already a signature): ${f}"
    continue
  fi

  if [[ -n "${MINISIGN_PASSWORD:-}" ]]; then
    minisign -Sm "${f}" -s "${keyfile}" -t "${trusted}" <<< "${MINISIGN_PASSWORD}"
  else
    # Unencrypted key (-W at generation time): no password prompt.
    minisign -Sm "${f}" -s "${keyfile}" -t "${trusted}" < /dev/null
  fi
  echo "signed: ${f} -> ${f}.minisig"

  # Fail closed if the secret key does not match the committed public key.
  if ! minisign -Vm "${f}" -x "${f}.minisig" -p "${PUBKEY}" >/dev/null; then
    echo "error: signature verification failed for ${f} against ${PUBKEY}" >&2
    echo "error: MINISIGN_SECRET_KEY does not match the committed public key" >&2
    exit 1
  fi
  echo "verified: ${f} (pubkey=$(basename "${PUBKEY}"))"
done
