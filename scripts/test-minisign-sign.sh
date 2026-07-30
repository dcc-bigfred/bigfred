#!/usr/bin/env bash
# Smoke-test minisign-sign.sh: sign + verify against matching pubkey must pass;
# verify against a mismatched pubkey must fail the script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MINISIGN_BIN="${MINISIGN_BIN:-minisign}"
if ! command -v "${MINISIGN_BIN}" >/dev/null 2>&1; then
  # Prefer local tooling from monorepo .secrets if present.
  if [[ -x "${ROOT}/../.secrets/minisign" ]]; then
    MINISIGN_BIN="${ROOT}/../.secrets/minisign"
  else
    echo "skip: minisign not on PATH" >&2
    exit 0
  fi
fi
export PATH="$(dirname "${MINISIGN_BIN}"):${PATH}"

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

# Fresh keypair for the test (unencrypted).
"${MINISIGN_BIN}" -G -p "${tmpdir}/test.pub" -s "${tmpdir}/test.key" -W
payload="${tmpdir}/artifact.bin"
printf 'hello-bigfred-release\n' > "${payload}"

export MINISIGN_SECRET_KEY
MINISIGN_SECRET_KEY="$(cat "${tmpdir}/test.key")"
export MINISIGN_PUBLIC_KEY="${tmpdir}/test.pub"

"${ROOT}/scripts/minisign-sign.sh" "${payload}"
[[ -f "${payload}.minisig" ]]

# Mismatch: point public key at a different key — signing must fail verification.
"${MINISIGN_BIN}" -G -p "${tmpdir}/other.pub" -s "${tmpdir}/other.key" -W
export MINISIGN_PUBLIC_KEY="${tmpdir}/other.pub"
rm -f "${payload}.minisig"
if "${ROOT}/scripts/minisign-sign.sh" "${payload}"; then
  echo "error: expected minisign-sign.sh to fail on pubkey mismatch" >&2
  exit 1
fi

echo "ok: minisign-sign.sh smoke test passed"
