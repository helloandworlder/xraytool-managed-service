#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_PATH="${1:-${PROJECT_ROOT}/dist/xraytool-linux-amd64.tar.gz}"

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

[[ -f "${PACKAGE_PATH}" ]] || fail "release package not found: ${PACKAGE_PATH}"

TMP_DIR="$(mktemp -d -t xraytool-release-contract.XXXXXX)"
trap 'rm -rf "${TMP_DIR}"' EXIT
PACKAGE_ENTRIES="${TMP_DIR}/entries.txt"
tar -tzf "${PACKAGE_PATH}" > "${PACKAGE_ENTRIES}"

required_entries=(
  release/xray
  release/xraytool
  release/xraytoolctl
  release/deploy/online-upgrade.sh
  release/deploy/rollback.sh
  release/deploy/rolling-upgrade.sh
)

for entry in "${required_entries[@]}"; do
  grep -Fxq "${entry}" "${PACKAGE_ENTRIES}" || fail "release entry missing: ${entry}"
done

tar -xzf "${PACKAGE_PATH}" -C "${TMP_DIR}"

[[ -s "${TMP_DIR}/release/xray" ]] || fail "managed Xray Fork is empty"
[[ -x "${TMP_DIR}/release/deploy/online-upgrade.sh" ]] || fail "online-upgrade.sh is not executable"
[[ -x "${TMP_DIR}/release/deploy/rollback.sh" ]] || fail "rollback.sh is not executable"
[[ -x "${TMP_DIR}/release/deploy/rolling-upgrade.sh" ]] || fail "rolling-upgrade.sh is not executable"

echo "Release package contract passed: ${PACKAGE_PATH}"
