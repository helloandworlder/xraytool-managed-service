#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (sudo)."
  exit 1
fi

REPO_OWNER="${REPO_OWNER:-helloandworlder}"
REPO_NAME="${REPO_NAME:-xraytool-managed-service}"
RELEASE_VERSION="${RELEASE_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/opt/xraytool}"
SERVICE_NAME="xraytool"

NON_INTERACTIVE=false
LISTEN_PORT_INPUT="${XTOOL_INSTALL_PORT:-}"
ADMIN_USER_INPUT="${XTOOL_INSTALL_ADMIN_USER:-}"
ADMIN_PASS_INPUT="${XTOOL_INSTALL_ADMIN_PASS:-}"
PACKAGE_PATH="${XTOOL_PACKAGE_PATH:-}"
XRAY_BIN_PATH="${XTOOL_XRAY_BIN_PATH:-}"

usage() {
  cat <<'EOF'
Usage:
  sudo bash public-install.sh [options]

Options:
  --install-dir <dir>    Install directory (default: /opt/xraytool)
  --version <tag|latest> Release version tag (default: latest)
  --port <1-65535>       Web panel port
  --admin-user <name>    Admin username
  --admin-pass <pass>    Admin password
  -y, --non-interactive  Skip prompts and use provided/random values
  -h, --help             Show help

Env overrides (for testing):
  XTOOL_PACKAGE_PATH     Local xraytool package tar.gz path
  XTOOL_XRAY_BIN_PATH    Local xray binary path
  XTOOL_INSTALL_PORT     Same as --port
  XTOOL_INSTALL_ADMIN_USER  Same as --admin-user
  XTOOL_INSTALL_ADMIN_PASS  Same as --admin-pass
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="$2"
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

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64)
      ARCH="amd64"
      XRAY_ARCHIVE="Xray-linux-64.zip"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      XRAY_ARCHIVE="Xray-linux-arm64-v8a.zip"
      ;;
    *)
      fail "Unsupported architecture: $machine"
      ;;
  esac
}

resolve_runtime_values() {
  local suggested_port suggested_user suggested_pass answer
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
      log "port ${LISTEN_PORT} is currently used by existing xraytool service, reusing it"
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
  if [[ -n "$PACKAGE_PATH" ]]; then
    [[ -f "$PACKAGE_PATH" ]] || fail "Local package not found: $PACKAGE_PATH"
    cp "$PACKAGE_PATH" "$PACKAGE_TARBALL"
    return
  fi

  local asset url
  asset="xraytool-linux-${ARCH}.tar.gz"
  url="$(build_release_url "$asset")"
  log "downloading release package: $url"
  curl -fL --retry 3 --connect-timeout 20 "$url" -o "$PACKAGE_TARBALL"
}

prepare_xray_binary() {
  XRAY_BIN_FINAL="${TMP_DIR}/xray-bin"
  if [[ -n "$XRAY_BIN_PATH" ]]; then
    [[ -f "$XRAY_BIN_PATH" ]] || fail "Local xray binary not found: $XRAY_BIN_PATH"
    cp "$XRAY_BIN_PATH" "$XRAY_BIN_FINAL"
    chmod +x "$XRAY_BIN_FINAL"
    return
  fi

  local url xray_zip
  xray_zip="${TMP_DIR}/xray.zip"
  url="https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ARCHIVE}"
  log "downloading xray core: $url"
  curl -fL --retry 3 --connect-timeout 20 "$url" -o "$xray_zip"
  unzip -q "$xray_zip" -d "${TMP_DIR}/xray"
  [[ -f "${TMP_DIR}/xray/xray" ]] || fail "xray binary not found in archive"
  cp "${TMP_DIR}/xray/xray" "$XRAY_BIN_FINAL"
  chmod +x "$XRAY_BIN_FINAL"
}

install_runtime_files() {
  tar -xzf "$PACKAGE_TARBALL" -C "$TMP_DIR"
  local release_root
  release_root="${TMP_DIR}/release"
  [[ -d "$release_root" ]] || fail "Invalid package layout: release/ not found"

  [[ -f "${release_root}/xraytool" ]] || fail "xraytool binary missing in package"
  [[ -f "${release_root}/xraytoolctl" ]] || fail "xraytoolctl binary missing in package"
  [[ -d "${release_root}/web-dist" ]] || fail "web-dist missing in package"
  [[ -f "${release_root}/deploy/systemd/xraytool.service" ]] || fail "systemd unit template missing"

  log "installing files into ${INSTALL_DIR}"
  mkdir -p "${INSTALL_DIR}" "${INSTALL_DIR}/deploy" "${INSTALL_DIR}/web" "${INSTALL_DIR}/data/xray" "${INSTALL_DIR}/data/backups"
  install -m 0755 "${release_root}/xraytool" "${INSTALL_DIR}/xraytool"
  install -m 0755 "${release_root}/xraytoolctl" "${INSTALL_DIR}/xraytoolctl"
  install -m 0755 "${release_root}/deploy/xtool" "${INSTALL_DIR}/deploy/xtool"
  if [[ -f "${release_root}/deploy/public-install.sh" ]]; then
    install -m 0755 "${release_root}/deploy/public-install.sh" "${INSTALL_DIR}/deploy/public-install.sh"
  fi
  cp -R "${release_root}/deploy/systemd" "${INSTALL_DIR}/deploy/"

  rm -rf "${INSTALL_DIR}/web/dist"
  cp -R "${release_root}/web-dist" "${INSTALL_DIR}/web/dist"

  if [[ -f "${release_root}/.env.example" ]]; then
    install -m 0644 "${release_root}/.env.example" "${INSTALL_DIR}/.env.example"
  fi
  if [[ -f "${release_root}/README.md" ]]; then
    install -m 0644 "${release_root}/README.md" "${INSTALL_DIR}/README.md"
  fi

  install -m 0755 "$XRAY_BIN_FINAL" "${INSTALL_DIR}/data/xray/xray"
}

write_systemd_and_env() {
  local unit_template unit_target env_file jwt_secret backup_file
  unit_template="${INSTALL_DIR}/deploy/systemd/xraytool.service"
  unit_target="/etc/systemd/system/${SERVICE_NAME}.service"
  env_file="/etc/default/${SERVICE_NAME}"
  jwt_secret="$(random_alnum 40)"

  sed "s#/opt/xraytool#${INSTALL_DIR}#g" "$unit_template" > "$unit_target"

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
XTOOL_MANAGED_XRAY=true
XTOOL_XRAY_DIR=${INSTALL_DIR}/data/xray
XTOOL_XRAY_BIN=${INSTALL_DIR}/data/xray/xray
XTOOL_XRAY_CONFIG=${INSTALL_DIR}/data/xray/config.json
XTOOL_XRAY_API=127.0.0.1:10085
XTOOL_DEFAULT_PORT=23457
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
ensure_cmd unzip
ensure_cmd ss
ensure_cmd systemctl

detect_arch
resolve_runtime_values

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

prepare_release_package
prepare_xray_binary
install_runtime_files
write_systemd_and_env
start_service_and_verify

echo
echo "Install completed successfully."
echo "Install dir : ${INSTALL_DIR}"
echo "Panel URL   : http://<server-ip>:${LISTEN_PORT}"
echo "Admin user  : ${ADMIN_USER}"
echo "Admin pass  : ${ADMIN_PASS}"
echo "Service     : systemctl status ${SERVICE_NAME}"
echo "Reset admin : ${INSTALL_DIR}/deploy/xtool"
