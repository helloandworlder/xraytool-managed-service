#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SERVICE_NAME_DEFAULT="xraytool"
SERVICE_NAME_INPUT="${XTOOL_SERVICE_NAME:-}"
SERVICE_NAME="${SERVICE_NAME_DEFAULT}"
ENV_FILE=""
PUBLIC_INSTALLER="${SCRIPT_DIR}/public-install.sh"
REGRESSION_SCRIPT="${ROOT_DIR}/scripts/online_regression.py"

RELEASE_VERSION=""
INSTALL_DIR_INPUT=""
PACKAGE_PATH="${XTOOL_PACKAGE_PATH:-}"
PACKAGE_SHA256="${XTOOL_PACKAGE_SHA256:-}"
BACKUP_DIR_INPUT=""
SKIP_REGRESSION=false
SKIP_BACKUP=false

usage() {
  cat <<'EOF'
Usage:
  sudo bash deploy/online-upgrade.sh [options]

Options:
  --version <tag>             Upgrade target immutable release tag (required)
  --install-dir <dir>         Install directory (default: inferred from env)
  --service-name <name>       systemd service name (default: xraytool)
  --package-path <file>       Use a local release package instead of downloading
  --package-sha256 <sha256>   Expected SHA256 for the local release package
  --backup-dir <dir>          Explicit snapshot directory for rollback
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
    --package-path)
      PACKAGE_PATH="$2"
      shift 2
      ;;
    --package-sha256)
      PACKAGE_SHA256="$2"
      shift 2
      ;;
    --backup-dir)
      BACKUP_DIR_INPUT="$2"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME_INPUT="$2"
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

[[ -n "${RELEASE_VERSION}" && "${RELEASE_VERSION}" != "latest" ]] || fail "--version <immutable-tag> is required; latest is not allowed for upgrades"

is_valid_service_name() {
  local name="$1"
  [[ "$name" =~ ^[A-Za-z0-9_.@-]+$ ]]
}

if [[ -n "${SERVICE_NAME_INPUT}" ]]; then
  if [[ "${SERVICE_NAME_INPUT}" == *.service ]]; then
    fail "--service-name should not include .service"
  fi
  is_valid_service_name "${SERVICE_NAME_INPUT}" || fail "invalid service name: ${SERVICE_NAME_INPUT}"
  SERVICE_NAME="${SERVICE_NAME_INPUT}"
fi
ENV_FILE="/etc/default/${SERVICE_NAME}"

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

# Read only the specific values needed below with read_env_var. Do not source
# an existing /etc/default file: legacy deployments may contain passwords or
# other values that are not valid shell syntax, and sourcing would execute them.

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
XRAY_BIN_PATH="$(read_env_var XTOOL_XRAY_BIN || true)"
if [[ -z "${XRAY_BIN_PATH}" ]]; then
  XRAY_BIN_PATH="${INSTALL_DIR}/data/xray/xray"
fi
XRAY_CONFIG_PATH="$(read_env_var XTOOL_XRAY_CONFIG || true)"
if [[ -z "${XRAY_CONFIG_PATH}" ]]; then
  XRAY_CONFIG_PATH="${INSTALL_DIR}/data/xray/config.json"
fi

TS="$(date +%Y%m%d-%H%M%S)"
BACKUP_FILE=""
ROLLBACK_DIR="${INSTALL_DIR}/upgrade-backups/${TS}"
if [[ -n "${BACKUP_DIR_INPUT}" ]]; then
  ROLLBACK_DIR="${BACKUP_DIR_INPUT}"
fi
[[ ! -e "${ROLLBACK_DIR}" ]] || fail "backup directory already exists: ${ROLLBACK_DIR}"

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
if [[ -f "${XRAY_BIN_PATH}" ]]; then
  cp -f "${XRAY_BIN_PATH}" "${ROLLBACK_DIR}/xray.bin"
fi
if [[ -f "${XRAY_CONFIG_PATH}" ]]; then
  cp -f "${XRAY_CONFIG_PATH}" "${ROLLBACK_DIR}/xray.config.json"
fi
if [[ -d "${INSTALL_DIR}/web/dist" ]]; then
  tar -czf "${ROLLBACK_DIR}/web-dist.tar.gz" -C "${INSTALL_DIR}/web" dist
fi

log "upgrading to ${RELEASE_VERSION}"
if [[ -n "${PACKAGE_PATH}" ]]; then
  [[ -f "${PACKAGE_PATH}" ]] || fail "local package not found: ${PACKAGE_PATH}"
fi
INSTALL_ARGS=(
  --non-interactive
  --preserve-existing-env
  --service-name "${SERVICE_NAME}"
  --install-dir "${INSTALL_DIR}"
  --version "${RELEASE_VERSION}"
)
if [[ -n "${PACKAGE_PATH}" ]]; then
  INSTALL_ARGS+=(--package-path "${PACKAGE_PATH}")
fi
if [[ -n "${PACKAGE_SHA256}" ]]; then
  INSTALL_ARGS+=(--package-sha256 "${PACKAGE_SHA256}")
fi
XTOOL_PACKAGE_PATH="${PACKAGE_PATH}" \
XTOOL_PACKAGE_SHA256="${PACKAGE_SHA256}" \
bash "${PUBLIC_INSTALLER}" "${INSTALL_ARGS[@]}"

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
