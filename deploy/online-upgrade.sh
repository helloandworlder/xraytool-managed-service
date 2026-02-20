#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SERVICE_NAME="xraytool"
ENV_FILE="/etc/default/${SERVICE_NAME}"
PUBLIC_INSTALLER="${SCRIPT_DIR}/public-install.sh"
REGRESSION_SCRIPT="${ROOT_DIR}/scripts/online_regression.py"

RELEASE_VERSION="latest"
INSTALL_DIR_INPUT=""
SKIP_REGRESSION=false
SKIP_BACKUP=false

usage() {
  cat <<'EOF'
Usage:
  sudo bash deploy/online-upgrade.sh [options]

Options:
  --version <tag|latest>      Upgrade target version (default: latest)
  --install-dir <dir>         Install directory (default: inferred from env)
  --skip-regression           Skip post-upgrade regression script
  --skip-backup               Skip pre-upgrade database backup
  -h, --help                  Show help

Notes:
  - Script preserves current listen port / admin user+pass / xray api port.
  - Script creates a pre-upgrade DB backup by default.
  - After upgrade it runs scripts/online_regression.py by default.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      RELEASE_VERSION="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR_INPUT="$2"
      shift 2
      ;;
    --skip-regression)
      SKIP_REGRESSION=true
      shift
      ;;
    --skip-backup)
      SKIP_BACKUP=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

log() {
  echo "==> $*"
}

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

read_env_var() {
  local key="$1"
  [[ -f "${ENV_FILE}" ]] || return 0
  awk -F'=' -v k="$key" '$1==k {print substr($0, index($0,$2)); exit}' "${ENV_FILE}"
}

extract_port_from_addr() {
  local value="$1"
  if [[ "$value" == *:* ]]; then
    echo "${value##*:}"
    return
  fi
  echo "$value"
}

ensure_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || fail "missing command: ${cmd}"
}

if [[ ! -x "${PUBLIC_INSTALLER}" ]]; then
  fail "installer not found: ${PUBLIC_INSTALLER}"
fi

ensure_cmd systemctl
ensure_cmd curl
ensure_cmd python3

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
  set +a
fi

if [[ -n "${INSTALL_DIR_INPUT}" ]]; then
  INSTALL_DIR="${INSTALL_DIR_INPUT}"
else
  DATA_DIR_RAW="$(read_env_var XTOOL_DATA_DIR || true)"
  if [[ -n "${DATA_DIR_RAW}" ]]; then
    INSTALL_DIR="$(dirname "${DATA_DIR_RAW}")"
  else
    INSTALL_DIR="/opt/xraytool"
  fi
fi

LISTEN_RAW="$(read_env_var XTOOL_LISTEN || true)"
LISTEN_PORT="${LISTEN_RAW#:}"
if [[ -z "${LISTEN_PORT}" ]]; then
  LISTEN_PORT="18080"
fi

ADMIN_USER="$(read_env_var XTOOL_ADMIN_USER || true)"
ADMIN_PASS="$(read_env_var XTOOL_ADMIN_PASS || true)"
if [[ -z "${ADMIN_USER}" ]]; then
  ADMIN_USER="admin"
fi
if [[ -z "${ADMIN_PASS}" ]]; then
  ADMIN_PASS="admin123456"
fi

XRAY_API_RAW="$(read_env_var XTOOL_XRAY_API || true)"
XRAY_API_PORT="$(extract_port_from_addr "${XRAY_API_RAW}")"
if [[ -z "${XRAY_API_PORT}" ]]; then
  XRAY_API_PORT="10085"
fi

DB_PATH="$(read_env_var XTOOL_DB_PATH || true)"
if [[ -z "${DB_PATH}" ]]; then
  DB_PATH="${INSTALL_DIR}/data/xraytool.db"
fi

TS="$(date +%Y%m%d-%H%M%S)"
BACKUP_FILE=""
ROLLBACK_DIR="${INSTALL_DIR}/upgrade-backups/${TS}"

if [[ "${SKIP_BACKUP}" != true ]]; then
  if [[ -f "${DB_PATH}" ]]; then
    mkdir -p "${INSTALL_DIR}/data/backups"
    BACKUP_FILE="${INSTALL_DIR}/data/backups/pre-upgrade-${TS}.db"
    log "creating pre-upgrade DB backup: ${BACKUP_FILE}"
    if command -v sqlite3 >/dev/null 2>&1; then
      if ! sqlite3 "${DB_PATH}" ".backup '${BACKUP_FILE}'"; then
        cp -f "${DB_PATH}" "${BACKUP_FILE}"
      fi
    else
      cp -f "${DB_PATH}" "${BACKUP_FILE}"
    fi
  else
    log "db file not found, skip db backup: ${DB_PATH}"
  fi
fi

mkdir -p "${ROLLBACK_DIR}"
if [[ -f "${ENV_FILE}" ]]; then
  cp -f "${ENV_FILE}" "${ROLLBACK_DIR}/xraytool.env"
fi
if [[ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]]; then
  cp -f "/etc/systemd/system/${SERVICE_NAME}.service" "${ROLLBACK_DIR}/xraytool.service"
fi
if [[ -f "${INSTALL_DIR}/xraytool" ]]; then
  cp -f "${INSTALL_DIR}/xraytool" "${ROLLBACK_DIR}/xraytool.bin"
fi
if [[ -f "${INSTALL_DIR}/xraytoolctl" ]]; then
  cp -f "${INSTALL_DIR}/xraytoolctl" "${ROLLBACK_DIR}/xraytoolctl.bin"
fi

log "upgrading to ${RELEASE_VERSION}"
bash "${PUBLIC_INSTALLER}" \
  --non-interactive \
  --install-dir "${INSTALL_DIR}" \
  --version "${RELEASE_VERSION}" \
  --port "${LISTEN_PORT}" \
  --xray-api-port "${XRAY_API_PORT}" \
  --admin-user "${ADMIN_USER}" \
  --admin-pass "${ADMIN_PASS}"

if [[ -f "${ENV_FILE}" ]]; then
  LISTEN_RAW="$(read_env_var XTOOL_LISTEN || true)"
  LISTEN_PORT="${LISTEN_RAW#:}"
  if [[ -z "${LISTEN_PORT}" ]]; then
    LISTEN_PORT="18080"
  fi
  ADMIN_USER="$(read_env_var XTOOL_ADMIN_USER || true)"
  ADMIN_PASS="$(read_env_var XTOOL_ADMIN_PASS || true)"
fi

for _ in $(seq 1 20); do
  if systemctl is-active --quiet "${SERVICE_NAME}"; then
    break
  fi
  sleep 1
done

if ! systemctl is-active --quiet "${SERVICE_NAME}"; then
  journalctl -u "${SERVICE_NAME}" -n 80 --no-pager || true
  fail "service is not active after upgrade"
fi

log "health check: http://127.0.0.1:${LISTEN_PORT}/healthz"
python3 - <<PY
import json
import sys
import time
import urllib.request

url = "http://127.0.0.1:${LISTEN_PORT}/healthz"
for _ in range(20):
    try:
        with urllib.request.urlopen(url, timeout=5) as resp:
            data = json.loads(resp.read().decode("utf-8", errors="replace"))
            if data.get("ok") is True:
                print("healthz ok")
                sys.exit(0)
    except Exception:
    time.sleep(1)
    continue
sys.exit(1)
PY

if [[ "${SKIP_REGRESSION}" != true ]]; then
  if [[ -f "${REGRESSION_SCRIPT}" ]]; then
    log "running post-upgrade regression"
    python3 "${REGRESSION_SCRIPT}" \
      --host "127.0.0.1" \
      --port "${LISTEN_PORT}" \
      --admin-user "${ADMIN_USER}" \
      --admin-pass "${ADMIN_PASS}"
  else
    log "regression script not found, skipped: ${REGRESSION_SCRIPT}"
  fi
fi

echo
echo "Upgrade completed successfully"
echo "- Version target : ${RELEASE_VERSION}"
echo "- Install dir    : ${INSTALL_DIR}"
echo "- Panel          : http://127.0.0.1:${LISTEN_PORT}"
echo "- Rollback files : ${ROLLBACK_DIR}"
if [[ -n "${BACKUP_FILE}" ]]; then
  echo "- DB backup      : ${BACKUP_FILE}"
fi
