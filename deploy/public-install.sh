#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

REPO_OWNER="${REPO_OWNER:-helloandworlder}"
REPO_NAME="${REPO_NAME:-xraytool-managed-service}"
RELEASE_VERSION="${RELEASE_VERSION:-latest}"
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

NON_INTERACTIVE=false
LISTEN_PORT_INPUT="${XTOOL_INSTALL_PORT:-}"
ADMIN_USER_INPUT="${XTOOL_INSTALL_ADMIN_USER:-}"
ADMIN_PASS_INPUT="${XTOOL_INSTALL_ADMIN_PASS:-}"
XRAY_API_PORT_INPUT="${XTOOL_INSTALL_XRAY_API_PORT:-}"
INSTANCE_ID_INPUT="${XTOOL_INSTALL_INSTANCE_ID:-}"
SERVICE_NAME_INPUT="${XTOOL_INSTALL_SERVICE_NAME:-}"
PACKAGE_PATH="${XTOOL_PACKAGE_PATH:-}"
PACKAGE_SHA256_EXPECTED="${XTOOL_PACKAGE_SHA256:-}"
XRAY_BIN_PATH="${XTOOL_XRAY_BIN_PATH:-}"
PRESERVE_EXISTING_ENV="${XTOOL_PRESERVE_EXISTING_ENV:-false}"
INSTANCE_ID=""

usage() {
  cat <<'EOF'
Usage:
  sudo bash public-install.sh [options]

Options:
  --install-dir <dir>    Install directory (default: /opt/xraytool)
  --instance-id <id>     Instance id suffix (service: xraytool-<id>)
  --service-name <name>  Custom systemd service name (without .service)
  --version <tag|latest> Release version tag (default: latest)
  --port <1-65535>       Web panel port
  --admin-user <name>    Admin username
  --admin-pass <pass>    Admin password
  --xray-bin-path <path> Local xray binary path
  --package-path <file>  Local xraytool package tar.gz path
  --package-sha256 <sha256> Expected SHA256 for the local package
  --xray-api-port <p|random>  Managed Xray API port (default: auto-choose)
  --preserve-existing-env  Reuse credentials and ports from the existing service
  -y, --non-interactive  Skip prompts and use provided/random values
  -h, --help             Show help

Env overrides (for testing):
  XTOOL_PACKAGE_PATH     Local xraytool package tar.gz path
  XTOOL_PACKAGE_SHA256   Expected SHA256 for XTOOL_PACKAGE_PATH
  XTOOL_XRAY_BIN_PATH    Local xray binary path
  XTOOL_PRESERVE_EXISTING_ENV  Reuse existing service values when true
  XTOOL_INSTALL_PORT     Same as --port
  XTOOL_INSTALL_ADMIN_USER  Same as --admin-user
  XTOOL_INSTALL_ADMIN_PASS  Same as --admin-pass
  XTOOL_INSTALL_INSTANCE_ID Same as --instance-id
  XTOOL_INSTALL_SERVICE_NAME Same as --service-name
  XTOOL_INSTALL_XRAY_API_PORT Same as --xray-api-port
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
    --version)
      RELEASE_VERSION="$2"
      shift 2
      ;;
    --port)
      LISTEN_PORT_INPUT="$2"
      shift 2
      ;;
    --admin-user)
      ADMIN_USER_INPUT="$2"
      shift 2
      ;;
    --admin-pass)
      ADMIN_PASS_INPUT="$2"
      shift 2
      ;;
    --xray-bin-path)
      XRAY_BIN_PATH="$2"
      shift 2
      ;;
    --package-path)
      PACKAGE_PATH="$2"
      shift 2
      ;;
    --package-sha256)
      PACKAGE_SHA256_EXPECTED="$2"
      shift 2
      ;;
    --xray-api-port)
      XRAY_API_PORT_INPUT="$2"
      shift 2
      ;;
    --preserve-existing-env)
      PRESERVE_EXISTING_ENV=true
      shift
      ;;
    -y|--non-interactive)
      NON_INTERACTIVE=true
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

PKG_MANAGER=""
APT_UPDATED=false

detect_pkg_manager() {
  if command -v apt-get >/dev/null 2>&1; then
    PKG_MANAGER="apt-get"
  elif command -v dnf >/dev/null 2>&1; then
    PKG_MANAGER="dnf"
  elif command -v yum >/dev/null 2>&1; then
    PKG_MANAGER="yum"
  elif command -v pacman >/dev/null 2>&1; then
    PKG_MANAGER="pacman"
  else
    PKG_MANAGER=""
  fi
}

package_for_cmd() {
  local cmd="$1"
  case "$cmd" in
    curl) echo "curl" ;;
    tar) echo "tar" ;;
    unzip) echo "unzip" ;;
    ss)
      case "$PKG_MANAGER" in
        apt-get) echo "iproute2" ;;
        dnf|yum) echo "iproute" ;;
        pacman) echo "iproute2" ;;
        *) echo "" ;;
      esac
      ;;
    *) echo "" ;;
  esac
}

install_pkg() {
  local pkg="$1"
  case "$PKG_MANAGER" in
    apt-get)
      if [[ "$APT_UPDATED" != true ]]; then
        DEBIAN_FRONTEND=noninteractive apt-get update -y
        APT_UPDATED=true
      fi
      DEBIAN_FRONTEND=noninteractive apt-get install -y "$pkg"
      ;;
    dnf)
      dnf install -y "$pkg"
      ;;
    yum)
      yum install -y "$pkg"
      ;;
    pacman)
      pacman -Sy --noconfirm "$pkg"
      ;;
    *)
      fail "No supported package manager found for auto-installing $pkg"
      ;;
  esac
}

ensure_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    return
  fi
  local pkg
  pkg="$(package_for_cmd "$cmd")"
  if [[ -z "$pkg" ]]; then
    fail "Missing command: $cmd"
  fi
  log "missing '$cmd', installing package '$pkg'"
  install_pkg "$pkg"
  command -v "$cmd" >/dev/null 2>&1 || fail "Failed to install dependency: $cmd"
}

random_alnum() {
  local n="$1"
  (set +o pipefail; LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c "$n")
}

read_existing_env() {
  local key="$1"
  local env_file="/etc/default/${SERVICE_NAME}"
  [[ -f "$env_file" ]] || return 0
  awk -F'=' -v k="$key" '$1==k {print substr($0, index($0,$2)); exit}' "$env_file"
}

extract_port_from_addr() {
  local value="$1"
  if [[ "$value" == *:* ]]; then
    echo "${value##*:}"
    return
  fi
  echo "$value"
}

port_in_use() {
  local port="$1"
  ss -lntH | awk -v p=":${port}" '$4 ~ p"$" {found=1} END {exit found ? 0 : 1}'
}

random_port() {
  local p
  for _ in $(seq 1 50); do
    p="$(( (RANDOM % 20000) + 20000 ))"
    if ! port_in_use "$p"; then
      echo "$p"
      return 0
    fi
  done
  fail "Could not find a free random port"
}

is_valid_port() {
  local port="$1"
  [[ "$port" =~ ^[0-9]+$ ]] || return 1
  ((port >= 1 && port <= 65535)) || return 1
  return 0
}

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
  INSTANCE_ID="$(trim_ws "${INSTANCE_ID_INPUT}" | tr 'A-Z' 'a-z')"
  SERVICE_NAME_INPUT="$(trim_ws "${SERVICE_NAME_INPUT}")"

  if [[ -n "${INSTANCE_ID}" ]]; then
    is_valid_instance_id "${INSTANCE_ID}" || fail "Invalid instance id: ${INSTANCE_ID} (allowed: a-z 0-9 _ -)"
  fi

  if [[ -n "${SERVICE_NAME_INPUT}" ]]; then
    is_valid_service_name "${SERVICE_NAME_INPUT}" || fail "Invalid service name: ${SERVICE_NAME_INPUT}"
    if [[ "${SERVICE_NAME_INPUT}" == *.service ]]; then
      fail "--service-name should not include '.service' suffix"
    fi
    SERVICE_NAME="${SERVICE_NAME_INPUT}"
  elif [[ -n "${INSTANCE_ID}" ]]; then
    SERVICE_NAME="xraytool-${INSTANCE_ID}"
  else
    SERVICE_NAME="${SERVICE_NAME_DEFAULT}"
  fi

  if [[ "${INSTALL_DIR_SET}" != true && -n "${INSTANCE_ID}" ]]; then
    INSTALL_DIR="/opt/xraytool-${INSTANCE_ID}"
  fi
}

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      fail "Unsupported architecture: $machine"
      ;;
  esac
}

resolve_runtime_values() {
  local suggested_port suggested_user suggested_pass answer
  if [[ "${PRESERVE_EXISTING_ENV}" == true ]]; then
    [[ -n "${LISTEN_PORT_INPUT}" ]] || LISTEN_PORT_INPUT="$(extract_port_from_addr "$(read_existing_env XTOOL_LISTEN || true)")"
    [[ -n "${ADMIN_USER_INPUT}" ]] || ADMIN_USER_INPUT="$(read_existing_env XTOOL_ADMIN_USER || true)"
    [[ -n "${ADMIN_PASS_INPUT}" ]] || ADMIN_PASS_INPUT="$(read_existing_env XTOOL_ADMIN_PASS || true)"
    [[ -n "${XRAY_API_PORT_INPUT}" ]] || XRAY_API_PORT_INPUT="$(extract_port_from_addr "$(read_existing_env XTOOL_XRAY_API || true)")"
  fi
  suggested_port="$(random_port)"
  suggested_user="admin$(random_alnum 4 | tr 'A-Z' 'a-z')"
  suggested_pass="$(random_alnum 18)"

  if [[ -n "$LISTEN_PORT_INPUT" ]]; then
    LISTEN_PORT="$LISTEN_PORT_INPUT"
  elif [[ "$NON_INTERACTIVE" == true || ! -t 0 ]]; then
    LISTEN_PORT="$suggested_port"
  else
    read -r -p "Panel listen port [${suggested_port}] (Enter=random): " answer
    if [[ -z "$answer" ]]; then
      LISTEN_PORT="$suggested_port"
    elif [[ "$answer" == "random" || "$answer" == "r" ]]; then
      LISTEN_PORT="$(random_port)"
    else
      LISTEN_PORT="$answer"
    fi
  fi

  is_valid_port "$LISTEN_PORT" || fail "Invalid port: $LISTEN_PORT"
  if port_in_use "$LISTEN_PORT"; then
    local existing_port
    existing_port="$(read_existing_env XTOOL_LISTEN || true)"
    existing_port="${existing_port#:}"
    if [[ -n "$existing_port" && "$existing_port" == "$LISTEN_PORT" ]]; then
      log "port ${LISTEN_PORT} is currently used by service ${SERVICE_NAME}, reusing it"
    else
      fail "Port is already in use: $LISTEN_PORT"
    fi
  fi

  if [[ -n "$ADMIN_USER_INPUT" ]]; then
    ADMIN_USER="$ADMIN_USER_INPUT"
  elif [[ "$NON_INTERACTIVE" == true || ! -t 0 ]]; then
    ADMIN_USER="$suggested_user"
  else
    read -r -p "Admin username [${suggested_user}] (Enter=random): " answer
    if [[ -z "$answer" || "$answer" == "random" || "$answer" == "r" ]]; then
      ADMIN_USER="$suggested_user"
    else
      ADMIN_USER="$answer"
    fi
  fi

  if [[ -n "$ADMIN_PASS_INPUT" ]]; then
    ADMIN_PASS="$ADMIN_PASS_INPUT"
  elif [[ "$NON_INTERACTIVE" == true || ! -t 0 ]]; then
    ADMIN_PASS="$suggested_pass"
  else
    read -r -p "Admin password [${suggested_pass}] (Enter=random): " answer
    if [[ -z "$answer" || "$answer" == "random" || "$answer" == "r" ]]; then
      ADMIN_PASS="$suggested_pass"
    else
      ADMIN_PASS="$answer"
    fi
  fi

  [[ -n "$ADMIN_USER" ]] || fail "Admin username cannot be empty"
  [[ -n "$ADMIN_PASS" ]] || fail "Admin password cannot be empty"
  if [[ "$ADMIN_USER" =~ [[:space:]] ]]; then
    fail "Admin username cannot contain spaces"
  fi
}

resolve_xray_api_port() {
  local preferred_port existing_api existing_port

  preferred_port="${XRAY_API_PORT_INPUT}"
  if [[ -z "$preferred_port" ]]; then
    preferred_port="10085"
  fi

  if [[ "$preferred_port" == "random" || "$preferred_port" == "r" ]]; then
    XRAY_API_PORT="$(random_port)"
  else
    XRAY_API_PORT="$preferred_port"
  fi

  is_valid_port "$XRAY_API_PORT" || fail "Invalid xray api port: $XRAY_API_PORT"
  if port_in_use "$XRAY_API_PORT"; then
    existing_api="$(read_existing_env XTOOL_XRAY_API || true)"
    existing_port="$(extract_port_from_addr "$existing_api")"
    if [[ -n "$existing_port" && "$existing_port" == "$XRAY_API_PORT" ]]; then
      log "xray api port ${XRAY_API_PORT} is currently used by service ${SERVICE_NAME}, reusing it"
    elif [[ -z "$XRAY_API_PORT_INPUT" ]]; then
      XRAY_API_PORT="$(random_port)"
      log "xray api port 10085 occupied, switched to random ${XRAY_API_PORT}"
    else
      fail "Xray API port is already in use: $XRAY_API_PORT"
    fi
  fi

  XRAY_API_ADDR="127.0.0.1:${XRAY_API_PORT}"
}

build_release_url() {
  local asset="$1"
  if [[ "$RELEASE_VERSION" == "latest" ]]; then
    echo "https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download/${asset}"
  else
    echo "https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${RELEASE_VERSION}/${asset}"
  fi
}

prepare_release_package() {
  PACKAGE_TARBALL="${TMP_DIR}/xraytool-package.tar.gz"
  CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"
  if [[ -n "$PACKAGE_PATH" ]]; then
    [[ -f "$PACKAGE_PATH" ]] || fail "Local package not found: $PACKAGE_PATH"
    cp "$PACKAGE_PATH" "$PACKAGE_TARBALL"
  else
    local asset url checksum_url
    asset="xraytool-linux-${ARCH}.tar.gz"
    url="$(build_release_url "$asset")"
    log "downloading release package: $url"
    curl -fL --retry 3 --connect-timeout 20 "$url" -o "$PACKAGE_TARBALL"
    checksum_url="$(build_release_url checksums.txt)"
    log "downloading release checksums: $checksum_url"
    curl -fL --retry 3 --connect-timeout 20 "$checksum_url" -o "$CHECKSUMS_FILE"
  fi

  local expected actual asset
  asset="xraytool-linux-${ARCH}.tar.gz"
  expected="${PACKAGE_SHA256_EXPECTED}"
  if [[ -z "${expected}" && -f "${CHECKSUMS_FILE}" ]]; then
    expected="$(awk -v asset="$asset" '$2 == asset {print $1; exit}' "${CHECKSUMS_FILE}")"
  fi
  [[ "${expected}" =~ ^[[:xdigit:]]{64}$ ]] || fail "No valid SHA256 found for ${asset}; refusing unverified package"
  actual="$(sha256sum "${PACKAGE_TARBALL}" | awk '{print $1}')"
  [[ "${actual}" == "${expected}" ]] || fail "Package SHA256 mismatch: expected ${expected}, got ${actual}"
  log "verified package SHA256: ${actual}"
}

prepare_xray_binary() {
  XRAY_BIN_FINAL="${TMP_DIR}/xray-bin"
  if [[ -n "$XRAY_BIN_PATH" ]]; then
    [[ -f "$XRAY_BIN_PATH" ]] || fail "Local xray binary not found: $XRAY_BIN_PATH"
    cp "$XRAY_BIN_PATH" "$XRAY_BIN_FINAL"
    chmod +x "$XRAY_BIN_FINAL"
    return
  fi

  [[ -f "${RELEASE_ROOT}/xray" ]] || fail "release package does not contain the managed Xray Fork binary"
  cp "${RELEASE_ROOT}/xray" "$XRAY_BIN_FINAL"
  chmod +x "$XRAY_BIN_FINAL"
}

extract_release_package() {
  tar -xzf "$PACKAGE_TARBALL" -C "$TMP_DIR"
  RELEASE_ROOT="${TMP_DIR}/release"
  [[ -d "$RELEASE_ROOT" ]] || fail "Invalid package layout: release/ not found"
}

install_runtime_files() {
  [[ -f "${RELEASE_ROOT}/xraytool" ]] || fail "xraytool binary missing in package"
  [[ -f "${RELEASE_ROOT}/xraytoolctl" ]] || fail "xraytoolctl binary missing in package"
  [[ -f "${RELEASE_ROOT}/xray" ]] || fail "managed Xray Fork binary missing in package"
  [[ -d "${RELEASE_ROOT}/web-dist" ]] || fail "web-dist missing in package"
  [[ -f "${RELEASE_ROOT}/deploy/systemd/xraytool.service" ]] || fail "systemd unit template missing"
  [[ -f "${RELEASE_ROOT}/deploy/online-upgrade.sh" ]] || fail "online upgrade script missing in package"
  [[ -f "${RELEASE_ROOT}/deploy/rollback.sh" ]] || fail "rollback script missing in package"

  log "installing files into ${INSTALL_DIR}"
  mkdir -p "${INSTALL_DIR}" "${INSTALL_DIR}/deploy" "${INSTALL_DIR}/web" "${INSTALL_DIR}/data/xray" "${INSTALL_DIR}/data/backups"
  install -m 0755 "${RELEASE_ROOT}/xraytool" "${INSTALL_DIR}/xraytool"
  install -m 0755 "${RELEASE_ROOT}/xraytoolctl" "${INSTALL_DIR}/xraytoolctl"
  install -m 0755 "${RELEASE_ROOT}/deploy/xtool" "${INSTALL_DIR}/deploy/xtool"
  install -m 0755 "${RELEASE_ROOT}/deploy/online-upgrade.sh" "${INSTALL_DIR}/deploy/online-upgrade.sh"
  install -m 0755 "${RELEASE_ROOT}/deploy/public-install.sh" "${INSTALL_DIR}/deploy/public-install.sh"
  install -m 0755 "${RELEASE_ROOT}/deploy/rollback.sh" "${INSTALL_DIR}/deploy/rollback.sh"
  cp -R "${RELEASE_ROOT}/deploy/systemd" "${INSTALL_DIR}/deploy/"

  rm -rf "${INSTALL_DIR}/web/dist"
  cp -R "${RELEASE_ROOT}/web-dist" "${INSTALL_DIR}/web/dist"

  if [[ -f "${RELEASE_ROOT}/.env.example" ]]; then
    install -m 0644 "${RELEASE_ROOT}/.env.example" "${INSTALL_DIR}/.env.example"
  fi
  if [[ -f "${RELEASE_ROOT}/README.md" ]]; then
    install -m 0644 "${RELEASE_ROOT}/README.md" "${INSTALL_DIR}/README.md"
  fi

  install -m 0755 "$XRAY_BIN_FINAL" "${INSTALL_DIR}/data/xray/xray"
}

write_systemd_and_env() {
  local unit_template unit_target env_file jwt_secret backup_file instance_uplink instance_downlink
  unit_template="${INSTALL_DIR}/deploy/systemd/xraytool.service"
  unit_target="/etc/systemd/system/${SERVICE_NAME}.service"
  env_file="/etc/default/${SERVICE_NAME}"
  jwt_secret="$(read_existing_env XTOOL_JWT_SECRET || true)"
  [[ -n "${jwt_secret}" ]] || jwt_secret="$(random_alnum 40)"
  instance_uplink="$(read_existing_env XTOOL_INSTANCE_UPLINK_LIMIT_BPS || true)"
  instance_downlink="$(read_existing_env XTOOL_INSTANCE_DOWNLINK_LIMIT_BPS || true)"
  [[ "${instance_uplink}" =~ ^[0-9]+$ ]] && (( instance_uplink > 0 )) || instance_uplink=30000000
  [[ "${instance_downlink}" =~ ^[0-9]+$ ]] && (( instance_downlink > 0 )) || instance_downlink=30000000

  sed -e "s#/opt/xraytool#${INSTALL_DIR}#g" -e "s#/etc/default/xraytool#${env_file}#g" "$unit_template" > "$unit_target"

  if [[ -f "$env_file" ]]; then
    backup_file="${env_file}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$env_file" "$backup_file"
    log "existing env file backed up to ${backup_file}"
  fi

  cat > "$env_file" <<EOF
XTOOL_LISTEN=:${LISTEN_PORT}
XTOOL_DATA_DIR=${INSTALL_DIR}/data
XTOOL_DB_PATH=${INSTALL_DIR}/data/xraytool.db
XTOOL_BACKUP_DIR=${INSTALL_DIR}/data/backups
XTOOL_JWT_SECRET=${jwt_secret}
XTOOL_ADMIN_USER=${ADMIN_USER}
XTOOL_ADMIN_PASS=${ADMIN_PASS}
XTOOL_SERVICE_NAME=${SERVICE_NAME}
XTOOL_MANAGED_XRAY=true
XTOOL_XRAY_DIR=${INSTALL_DIR}/data/xray
XTOOL_XRAY_BIN=${INSTALL_DIR}/data/xray/xray
XTOOL_XRAY_CONFIG=${INSTALL_DIR}/data/xray/config.json
XTOOL_XRAY_API=${XRAY_API_ADDR}
XTOOL_DEFAULT_PORT=23457
XTOOL_INSTANCE_UPLINK_LIMIT_BPS=${instance_uplink}
XTOOL_INSTANCE_DOWNLINK_LIMIT_BPS=${instance_downlink}
XTOOL_SCHEDULER_SECONDS=30
EOF
  chmod 600 "$env_file"
}

start_service_and_verify() {
  local health_url
  health_url="http://127.0.0.1:${LISTEN_PORT}/healthz"

  log "starting systemd service: ${SERVICE_NAME}"
  systemctl daemon-reload
  systemctl enable --now "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"

  for _ in $(seq 1 45); do
    if curl -fsS "$health_url" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done

  systemctl --no-pager status "$SERVICE_NAME" || true
  journalctl -u "$SERVICE_NAME" -n 120 --no-pager || true
  fail "Service failed health check: $health_url"
}

detect_pkg_manager
ensure_cmd curl
ensure_cmd tar
ensure_cmd ss
ensure_cmd systemctl

resolve_instance_identity
detect_arch
resolve_runtime_values
resolve_xray_api_port

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

prepare_release_package
extract_release_package
prepare_xray_binary
install_runtime_files
write_systemd_and_env
start_service_and_verify

echo
echo "Install completed successfully."
echo "Install dir : ${INSTALL_DIR}"
echo "Service name: ${SERVICE_NAME}"
echo "Env file    : /etc/default/${SERVICE_NAME}"
echo "Panel URL   : http://<server-ip>:${LISTEN_PORT}"
echo "Admin user  : ${ADMIN_USER}"
echo "Admin pass  : ${ADMIN_PASS}"
echo "Xray API    : ${XRAY_API_ADDR}"
echo "Service     : systemctl status ${SERVICE_NAME}"
echo "Reset admin : ${INSTALL_DIR}/deploy/xtool"
