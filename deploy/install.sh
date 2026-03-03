#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

INSTALL_DIR_DEFAULT="/opt/xraytool"
INSTALL_DIR="${INSTALL_DIR:-}"
INSTALL_DIR_SET=false
if [[ -z "${INSTALL_DIR}" ]]; then
  INSTALL_DIR="${INSTALL_DIR_DEFAULT}"
else
  INSTALL_DIR_SET=true
fi
SERVICE_NAME_DEFAULT="xraytool"
SERVICE_NAME="${SERVICE_NAME_DEFAULT}"
INSTANCE_ID_INPUT="${XTOOL_INSTALL_INSTANCE_ID:-}"
SERVICE_NAME_INPUT="${XTOOL_INSTALL_SERVICE_NAME:-}"
XRAY_BIN="${XRAY_BIN:-}"

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy/install.sh [--install-dir /opt/xraytool] [--instance-id foo] [--service-name xraytool-foo] [--xray-bin /path/to/xray]

Env:
  INSTALL_DIR   Installation directory (default: /opt/xraytool)
  XRAY_BIN      Path to private xray binary to copy into data/xray/xray
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="$2"
      INSTALL_DIR_SET=true
      shift 2
      ;;
    --instance-id)
      INSTANCE_ID_INPUT="$2"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME_INPUT="$2"
      shift 2
      ;;
    --xray-bin)
      XRAY_BIN="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1"
      usage
      exit 1
      ;;
  esac
done

is_valid_instance_id() {
  local instance_id="$1"
  [[ "$instance_id" =~ ^[a-z0-9][a-z0-9_-]*$ ]]
}

is_valid_service_name() {
  local name="$1"
  [[ "$name" =~ ^[A-Za-z0-9_.@-]+$ ]]
}

trim_ws() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

resolve_instance_identity() {
  local instance_id
  instance_id="$(trim_ws "${INSTANCE_ID_INPUT}" | tr 'A-Z' 'a-z')"
  SERVICE_NAME_INPUT="$(trim_ws "${SERVICE_NAME_INPUT}")"

  if [[ -n "${instance_id}" ]]; then
    is_valid_instance_id "${instance_id}" || {
      echo "Invalid instance id: ${instance_id}"
      exit 1
    }
  fi

  if [[ -n "${SERVICE_NAME_INPUT}" ]]; then
    if [[ "${SERVICE_NAME_INPUT}" == *.service ]]; then
      echo "--service-name should not include .service"
      exit 1
    fi
    is_valid_service_name "${SERVICE_NAME_INPUT}" || {
      echo "Invalid service name: ${SERVICE_NAME_INPUT}"
      exit 1
    }
    SERVICE_NAME="${SERVICE_NAME_INPUT}"
  elif [[ -n "${instance_id}" ]]; then
    SERVICE_NAME="xraytool-${instance_id}"
  fi

  if [[ "${INSTALL_DIR_SET}" != true && -n "${instance_id}" ]]; then
    INSTALL_DIR="/opt/xraytool-${instance_id}"
  fi
}

resolve_instance_identity

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing command: $1"
    exit 1
  fi
}

echo "==> checking dependencies"
require_cmd go
require_cmd pnpm
require_cmd systemctl

echo "==> building frontend (Vue + Ant Design Vue)"
cd "${PROJECT_ROOT}/frontend"
pnpm install --frozen-lockfile
pnpm run build

echo "==> building backend binaries"
cd "${PROJECT_ROOT}"
go mod tidy
go build -o xraytool ./cmd/xraytool
go build -o xraytoolctl ./cmd/xtoolctl

echo "==> preparing install directory: ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}" "${INSTALL_DIR}/deploy" "${INSTALL_DIR}/web" "${INSTALL_DIR}/data/xray"

echo "==> copying binaries and assets"
install -m 0755 "${PROJECT_ROOT}/xraytool" "${INSTALL_DIR}/xraytool"
install -m 0755 "${PROJECT_ROOT}/xraytoolctl" "${INSTALL_DIR}/xraytoolctl"
install -m 0755 "${PROJECT_ROOT}/deploy/xtool" "${INSTALL_DIR}/deploy/xtool"
cp -R "${PROJECT_ROOT}/web/dist" "${INSTALL_DIR}/web/"
install -m 0644 "${PROJECT_ROOT}/README.md" "${INSTALL_DIR}/README.md"
install -m 0644 "${PROJECT_ROOT}/.env.example" "${INSTALL_DIR}/.env.example"

if [[ -n "${XRAY_BIN}" ]]; then
  if [[ ! -f "${XRAY_BIN}" ]]; then
    echo "Provided xray binary not found: ${XRAY_BIN}"
    exit 1
  fi
  install -m 0755 "${XRAY_BIN}" "${INSTALL_DIR}/data/xray/xray"
  echo "==> copied private xray binary"
elif [[ -f "${PROJECT_ROOT}/data/xray/xray" ]]; then
  install -m 0755 "${PROJECT_ROOT}/data/xray/xray" "${INSTALL_DIR}/data/xray/xray"
  echo "==> copied xray binary from local data/xray/xray"
else
  echo "==> no xray binary copied; place it at ${INSTALL_DIR}/data/xray/xray before running service"
fi

echo "==> writing systemd unit"
UNIT_SRC="${PROJECT_ROOT}/deploy/systemd/xraytool.service"
UNIT_DST="/etc/systemd/system/${SERVICE_NAME}.service"
ENV_FILE="/etc/default/${SERVICE_NAME}"
sed -e "s#/opt/xraytool#${INSTALL_DIR}#g" -e "s#/etc/default/xraytool#${ENV_FILE}#g" "${UNIT_SRC}" > "${UNIT_DST}"

if [[ ! -f "${ENV_FILE}" ]]; then
  cat > "${ENV_FILE}" <<EOF
XTOOL_LISTEN=:18080
XTOOL_DATA_DIR=${INSTALL_DIR}/data
XTOOL_DB_PATH=${INSTALL_DIR}/data/xraytool.db
XTOOL_BACKUP_DIR=${INSTALL_DIR}/data/backups
XTOOL_JWT_SECRET=change-me-please
XTOOL_ADMIN_USER=admin
XTOOL_ADMIN_PASS=admin123456
XTOOL_SERVICE_NAME=${SERVICE_NAME}
XTOOL_MANAGED_XRAY=true
XTOOL_XRAY_DIR=${INSTALL_DIR}/data/xray
XTOOL_XRAY_BIN=${INSTALL_DIR}/data/xray/xray
XTOOL_XRAY_CONFIG=${INSTALL_DIR}/data/xray/config.json
XTOOL_XRAY_API=127.0.0.1:10085
XTOOL_DEFAULT_PORT=23457
XTOOL_SCHEDULER_SECONDS=30
EOF
  echo "==> created ${ENV_FILE} (please edit secrets before production use)"
fi

echo "==> enabling and starting service"
systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

echo "==> done"
echo "Service status: systemctl status ${SERVICE_NAME}"
echo "Panel URL: http://<your-server-ip>:18080"
echo "Reset admin: ${INSTALL_DIR}/deploy/xtool"
