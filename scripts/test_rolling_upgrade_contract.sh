#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d -t xraytool-rolling-contract.XXXXXX)"
trap 'rm -rf "${TMP_DIR}"' EXIT

package_path="${TMP_DIR}/package.tar.gz"
printf 'contract-package' > "${package_path}"
if command -v sha256sum >/dev/null 2>&1; then
  package_sha256="$(sha256sum "${package_path}" | awk '{print $1}')"
else
  package_sha256="$(shasum -a 256 "${package_path}" | awk '{print $1}')"
fi

inventory="${TMP_DIR}/inventory.csv"
{
  echo 'host,port,user'
  for index in $(seq 1 20); do
    echo "192.0.2.${index},22,root"
  done
} > "${inventory}"

output="${TMP_DIR}/output.log"
if bash "${PROJECT_ROOT}/deploy/rolling-upgrade.sh" \
  --inventory "${inventory}" \
  --package "${package_path}" \
  --package-sha256 "${package_sha256}" \
  --version v-contract \
  --ssh-key /dev/null \
  --evidence-dir "${TMP_DIR}/evidence" >"${output}" 2>&1; then
  echo "[ERROR] a 20-host inventory was accepted" >&2
  exit 1
fi

grep -Fq 'production inventory must contain exactly 21 unique hosts; got 20' "${output}" || {
  echo "[ERROR] 20-host inventory did not fail with the production count guard" >&2
  cat "${output}" >&2
  exit 1
}

echo "Rolling upgrade contract passed: incomplete inventory is rejected before SSH"
