#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)." >&2
  exit 1
fi

SERVICE_NAME="xraytool"
INSTALL_DIR="/opt/xraytool"
BACKUP_DIR=""

usage() {
  cat <<'EOF'
Usage:
  sudo bash deploy/rollback.sh --service-name <name> --install-dir <dir> --backup-dir <dir>

The rollback restores runtime files and configuration from an online-upgrade
snapshot. The database backup is retained but is not restored automatically.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --service-name)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --backup-dir)
      BACKUP_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

[[ -n "${BACKUP_DIR}" ]] || { echo "--backup-dir is required" >&2; exit 1; }
[[ -d "${BACKUP_DIR}" ]] || { echo "Backup directory not found: ${BACKUP_DIR}" >&2; exit 1; }

ENV_FILE="/etc/default/${SERVICE_NAME}"
UNIT_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

read_env_var() {
  local file="$1" key="$2"
  [[ -f "$file" ]] || return 0
  awk -F'=' -v k="$key" '$1 == k {print substr($0, index($0, $2)); exit}' "$file"
}

restore_file() {
  local source="$1" target="$2" mode="$3"
  [[ -f "$source" ]] || return 0
  install -D -m "$mode" "$source" "$target"
}

OLD_ENV_FILE="${BACKUP_DIR}/xraytool.env"
OLD_XRAY_BIN="$(read_env_var "$OLD_ENV_FILE" XTOOL_XRAY_BIN || true)"
OLD_XRAY_CONFIG="$(read_env_var "$OLD_ENV_FILE" XTOOL_XRAY_CONFIG || true)"
[[ -n "${OLD_XRAY_BIN}" ]] || OLD_XRAY_BIN="${INSTALL_DIR}/data/xray/xray"
[[ -n "${OLD_XRAY_CONFIG}" ]] || OLD_XRAY_CONFIG="${INSTALL_DIR}/data/xray/config.json"

echo "==> stopping ${SERVICE_NAME}"
systemctl stop "${SERVICE_NAME}"

restore_file "${BACKUP_DIR}/xraytool.env" "${ENV_FILE}" 600
restore_file "${BACKUP_DIR}/xraytool.service" "${UNIT_FILE}" 644
restore_file "${BACKUP_DIR}/xraytool.bin" "${INSTALL_DIR}/xraytool" 755
restore_file "${BACKUP_DIR}/xraytoolctl.bin" "${INSTALL_DIR}/xraytoolctl" 755
restore_file "${BACKUP_DIR}/xray.bin" "${OLD_XRAY_BIN}" 755
restore_file "${BACKUP_DIR}/xray.config.json" "${OLD_XRAY_CONFIG}" 644

if [[ -f "${BACKUP_DIR}/web-dist.tar.gz" ]]; then
  rm -rf "${INSTALL_DIR}/web/dist"
  mkdir -p "${INSTALL_DIR}/web"
  tar -xzf "${BACKUP_DIR}/web-dist.tar.gz" -C "${INSTALL_DIR}/web"
fi

systemctl daemon-reload
systemctl restart "${SERVICE_NAME}"

LISTEN_RAW="$(read_env_var "${ENV_FILE}" XTOOL_LISTEN || true)"
LISTEN_PORT="${LISTEN_RAW#:}"
[[ -n "${LISTEN_PORT}" ]] || LISTEN_PORT=18080
for _ in $(seq 1 30); do
  if systemctl is-active --quiet "${SERVICE_NAME}" && curl -fsS "http://127.0.0.1:${LISTEN_PORT}/healthz" >/dev/null; then
    echo "Rollback completed: service=${SERVICE_NAME} healthz=ok"
    echo "Database backup retained at ${BACKUP_DIR}; restore it only after confirming schema compatibility."
    exit 0
  fi
  sleep 1
done

systemctl --no-pager status "${SERVICE_NAME}" || true
echo "Rollback failed: ${SERVICE_NAME} did not become healthy" >&2
exit 1
