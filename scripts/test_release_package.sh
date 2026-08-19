#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_PATH="${1:-${PROJECT_ROOT}/dist/xraytool-linux-amd64.tar.gz}"

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

[[ -f "${PACKAGE_PATH}" ]] || fail "release package not found: ${PACKAGE_PATH}"

required_entries=(
  release/xray
  release/xraytool
  release/xraytoolctl
  release/deploy/online-upgrade.sh
  release/deploy/rollback.sh
  release/deploy/rolling-upgrade.sh
  release/scripts/online_regression.py
)

for entry in "${required_entries[@]}"; do
  tar -tzf "${PACKAGE_PATH}" | grep -Fxq "${entry}" || fail "release entry missing: ${entry}"
done

TMP_DIR="$(mktemp -d -t xraytool-release-contract.XXXXXX)"
trap 'rm -rf "${TMP_DIR}"' EXIT
tar -xzf "${PACKAGE_PATH}" -C "${TMP_DIR}"

[[ -s "${TMP_DIR}/release/xray" ]] || fail "managed Xray Fork is empty"
[[ -x "${TMP_DIR}/release/deploy/online-upgrade.sh" ]] || fail "online-upgrade.sh is not executable"
[[ -x "${TMP_DIR}/release/deploy/rollback.sh" ]] || fail "rollback.sh is not executable"
[[ -x "${TMP_DIR}/release/deploy/rolling-upgrade.sh" ]] || fail "rolling-upgrade.sh is not executable"
[[ -s "${TMP_DIR}/release/scripts/online_regression.py" ]] || fail "online regression script is empty"

echo "Release package contract passed: ${PACKAGE_PATH}"
