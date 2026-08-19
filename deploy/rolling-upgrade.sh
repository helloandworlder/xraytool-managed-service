#!/usr/bin/env bash
set -Eeuo pipefail

# One-host-at-a-time systemd rollout. The inventory contains no passwords.

INVENTORY=""
PACKAGE=""
PACKAGE_SHA256=""
RELEASE_VERSION=""
PASSWORD_DIR=""
SSH_KEY=""
DRY_RUN=false

usage() {
  cat <<'EOF'
Usage:
  bash deploy/rolling-upgrade.sh --inventory inventory.csv \
    --package xraytool-linux-amd64.tar.gz --package-sha256 <sha256> \
    --version <immutable-tag> [--password-dir ./passwords | --ssh-key <file>] \
    [--dry-run]

Inventory format (header required, no passwords):
  host,port,user

Password mode reads <password-dir>/<host>. Each password file must be mode 0600
or 0400. Hosts are processed in file order. The first failed host stops the
rollout; a failed service is rolled back from its new snapshot first.
EOF
}

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

trim() {
  local value="$1"
  value="${value#${value%%[![:space:]]*}}"
  value="${value%${value##*[![:space:]]}}"
  printf '%s' "$value"
}

shell_quote() {
  printf '%q' "$1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --inventory) INVENTORY="$2"; shift 2 ;;
    --package) PACKAGE="$2"; shift 2 ;;
    --package-sha256) PACKAGE_SHA256="$2"; shift 2 ;;
    --version) RELEASE_VERSION="$2"; shift 2 ;;
    --password-dir) PASSWORD_DIR="$2"; shift 2 ;;
    --ssh-key) SSH_KEY="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; fail "unknown argument: $1" ;;
  esac
done

[[ -f "${INVENTORY}" ]] || fail "inventory not found: ${INVENTORY}"
[[ -f "${PACKAGE}" ]] || fail "package not found: ${PACKAGE}"
[[ "${PACKAGE_SHA256}" =~ ^[[:xdigit:]]{64}$ ]] || fail "invalid package SHA256"
[[ -n "${RELEASE_VERSION}" && "${RELEASE_VERSION}" != latest ]] || fail "immutable --version is required"
[[ -n "${SSH_KEY}" || -n "${PASSWORD_DIR}" ]] || fail "provide --ssh-key or --password-dir"
[[ -z "${SSH_KEY}" || -r "${SSH_KEY}" ]] || fail "SSH key is not readable"
[[ -z "${PASSWORD_DIR}" || -d "${PASSWORD_DIR}" ]] || fail "password directory not found"
if [[ -n "${PASSWORD_DIR}" ]]; then
  command -v sshpass >/dev/null 2>&1 || fail "sshpass is required with --password-dir"
fi

actual_sha256="$(sha256_file "${PACKAGE}")"
[[ "${actual_sha256}" == "${PACKAGE_SHA256}" ]] || fail "local package SHA256 mismatch"

declare -A SEEN_HOSTS=()
inventory_lines=0
while IFS=, read -r host port user extra; do
  host="$(trim "${host:-}")"
  port="$(trim "${port:-}")"
  user="$(trim "${user:-}")"
  [[ -z "${host}" || "${host}" == \#* || "${host}" == host ]] && continue
  [[ "${host}" =~ ^[A-Za-z0-9._-]+$ ]] || fail "invalid host: ${host}"
  [[ "${port}" =~ ^[0-9]+$ ]] && (( port >= 1 && port <= 65535 )) || fail "invalid port for ${host}"
  [[ "${user}" =~ ^[A-Za-z0-9._-]+$ ]] || fail "invalid user for ${host}"
  [[ -z "${SEEN_HOSTS[${host}]+x}" ]] || fail "duplicate host: ${host}"
  SEEN_HOSTS["${host}"]=1
  inventory_lines=$((inventory_lines + 1))
done < "${INVENTORY}"
(( inventory_lines > 0 )) || fail "inventory has no usable hosts"

if [[ -n "${PASSWORD_DIR}" ]]; then
  for host in "${!SEEN_HOSTS[@]}"; do
    password_file="${PASSWORD_DIR}/${host}"
    [[ -f "${password_file}" ]] || fail "password file missing for ${host}"
    mode="$(stat -f '%Lp' "${password_file}" 2>/dev/null || stat -c '%a' "${password_file}")"
    [[ "${mode}" == 600 || "${mode}" == 400 ]] || fail "password file must be 0600 or 0400: ${password_file}"
  done
fi

TMP_DIR="$(mktemp -d -t xraytool-rollout.XXXXXX)"
KNOWN_HOSTS="${TMP_DIR}/known_hosts"
REMOTE_PACKAGE="/var/tmp/xraytool-release-${PACKAGE_SHA256}.tar.gz"
trap 'rm -rf "${TMP_DIR}"' EXIT

SSH_OPTIONS=(
  -o StrictHostKeyChecking=accept-new
  -o UserKnownHostsFile="${KNOWN_HOSTS}"
  -o ConnectTimeout=20
  -o ConnectionAttempts=1
  -o ServerAliveInterval=10
  -o ServerAliveCountMax=2
)

scp_to_host() {
  local host="$1" port="$2" user="$3" password_file=""
  if [[ -n "${SSH_KEY}" ]]; then
    scp "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" -P "${port}" "${PACKAGE}" "${user}@${host}:${REMOTE_PACKAGE}"
  else
    password_file="${PASSWORD_DIR}/${host}"
    sshpass -f "${password_file}" scp "${SSH_OPTIONS[@]}" -P "${port}" "${PACKAGE}" "${user}@${host}:${REMOTE_PACKAGE}"
  fi
}

ssh_script() {
  local host="$1" port="$2" user="$3" command="$4" password_file=""
  if [[ -n "${SSH_KEY}" ]]; then
    ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" -p "${port}" "${user}@${host}" "${command}"
  else
    password_file="${PASSWORD_DIR}/${host}"
    sshpass -f "${password_file}" ssh "${SSH_OPTIONS[@]}" -p "${port}" "${user}@${host}" "${command}"
  fi
}

remote_upgrade() {
  local host="$1" port="$2" user="$3"
  local package_arg version_arg sha_arg dry_run_arg
  package_arg="$(shell_quote "${REMOTE_PACKAGE}")"
  version_arg="$(shell_quote "${RELEASE_VERSION}")"
  sha_arg="$(shell_quote "${PACKAGE_SHA256}")"
  dry_run_arg="$(shell_quote "${DRY_RUN}")"
  ssh_script "${host}" "${port}" "${user}" "bash -s -- ${package_arg} ${version_arg} ${sha_arg} ${dry_run_arg}" <<'REMOTE_SCRIPT'
set -Eeuo pipefail

remote_package="$1"
release_version="$2"
expected_sha="$3"
dry_run="$4"
[[ -f "$remote_package" ]] || { echo "REMOTE_PACKAGE_MISSING" >&2; exit 1; }
actual_sha="$(sha256sum "$remote_package" | awk '{print $1}')"
[[ "$actual_sha" == "$expected_sha" ]] || { echo "REMOTE_PACKAGE_SHA_MISMATCH" >&2; exit 1; }

work_root="$(mktemp -d -p /var/tmp xraytool-release.XXXXXX)"
cleanup_success=false
cleanup() {
  if [[ "$cleanup_success" == true ]]; then
    rm -rf "$work_root" "$remote_package"
  fi
}
trap cleanup EXIT

tar -xzf "$remote_package" -C "$work_root"
release_root="$work_root/release"
upgrade_script="$release_root/deploy/online-upgrade.sh"
rollback_script="$release_root/deploy/rollback.sh"
[[ -x "$upgrade_script" && -x "$rollback_script" ]] || { echo "RELEASE_SCRIPTS_MISSING" >&2; exit 1; }

units="$({
  systemctl list-unit-files --type=service --no-legend 2>/dev/null || true
  systemctl list-units --type=service --all --no-legend 2>/dev/null || true
} | awk '$1 ~ /^xraytool(-[^[:space:]]+)?\.service$/ {print $1}' | sort -u)"
[[ -n "$units" ]] || { echo "NO_XRAYTOOL_SERVICES" >&2; exit 1; }
printf 'REMOTE_SERVICES=%s\n' "$(printf '%s' "$units" | tr '\n' ' ')"
if [[ "$dry_run" == true ]]; then
  cleanup_success=true
  exit 0
fi

for unit in $units; do
  service="${unit%.service}"
  state="$(systemctl is-active "$unit" 2>/dev/null || true)"
  [[ "$state" == active ]] || { echo "SERVICE_NOT_ACTIVE=$service state=$state" >&2; exit 1; }
  install_dir="$(systemctl show "$unit" -p WorkingDirectory --value 2>/dev/null || true)"
  [[ "$install_dir" == /* && -d "$install_dir" ]] || { echo "INSTALL_DIR_UNKNOWN=$service" >&2; exit 1; }
  backup_dir="$install_dir/upgrade-backups/rolling-${expected_sha:0:16}-${service}"
  [[ ! -e "$backup_dir" ]] || { echo "BACKUP_DIR_EXISTS=$backup_dir" >&2; exit 1; }

  printf 'UPGRADE_SERVICE=%s install_dir=%s\n' "$service" "$install_dir"
  if ! bash "$upgrade_script" --version "$release_version" --package-path "$remote_package" \
      --package-sha256 "$expected_sha" --service-name "$service" --install-dir "$install_dir" \
      --backup-dir "$backup_dir"; then
    echo "UPGRADE_FAILED=$service" >&2
    if [[ -d "$backup_dir" ]]; then
      bash "$rollback_script" --service-name "$service" --install-dir "$install_dir" --backup-dir "$backup_dir" || {
        echo "ROLLBACK_FAILED=$service backup_dir=$backup_dir" >&2
        exit 1
      }
    fi
    exit 1
  fi
  systemctl is-active --quiet "$unit"
  xray_bin="$(awk -F= '$1 == "XTOOL_XRAY_BIN" {print substr($0, index($0, $2)); exit}' "/etc/default/$service" 2>/dev/null || true)"
  [[ -x "$xray_bin" ]] || { echo "FORK_BINARY_MISSING=$service" >&2; exit 1; }
  printf 'SERVICE_OK=%s fork_sha256=%s\n' "$service" "$(sha256sum "$xray_bin" | awk '{print $1}')"
done

cleanup_success=true
echo "HOST_ROLLOUT_OK"
REMOTE_SCRIPT
}

completed=0
while IFS=, read -r host port user extra; do
  host="$(trim "${host:-}")"
  port="$(trim "${port:-}")"
  user="$(trim "${user:-}")"
  [[ -z "${host}" || "${host}" == \#* || "${host}" == host ]] && continue
  completed=$((completed + 1))
  echo "===== HOST ${completed}/${inventory_lines} ${host}:${port} ====="
  scp_to_host "${host}" "${port}" "${user}" || fail "package transfer failed for ${host}; stopped before next host"
  remote_upgrade "${host}" "${port}" "${user}" || fail "rollout failed for ${host}; no later host was touched"
  echo "HOST_OK ${host}"
done < "${INVENTORY}"

echo "ROLLING_UPGRADE_OK hosts=${completed} package_sha256=${PACKAGE_SHA256} version=${RELEASE_VERSION}"
