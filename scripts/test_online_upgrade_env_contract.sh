#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UPGRADE_SCRIPT="${PROJECT_ROOT}/deploy/online-upgrade.sh"

bash -n "${UPGRADE_SCRIPT}"

if rg -n '^[[:space:]]*\.[[:space:]]+"\$\{ENV_FILE\}"' "${UPGRADE_SCRIPT}"; then
  echo "[ERROR] online-upgrade.sh must not source an existing env file" >&2
  exit 1
fi

grep -Fq 'Do not source' "${UPGRADE_SCRIPT}" || {
  echo "[ERROR] safe env parsing guard comment is missing" >&2
  exit 1
}

echo "Online upgrade environment contract passed: legacy env values are parsed, not sourced"
